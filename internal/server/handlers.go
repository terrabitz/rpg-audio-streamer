package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	switch r.Method {
	case http.MethodGet:
		s.listFiles(w, r)
	case http.MethodPost:
		s.uploadFile(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	tracks, err := s.store.GetTracks(r.Context())
	if err != nil {
		s.logger.Error("failed to retrieve tracks", "error", err)
		http.Error(w, "Failed to list files", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(tracks); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // 10MB max
	if err != nil {
		s.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("files")
	if err != nil {
		s.logger.Error("failed to get file", "error", err)
		http.Error(w, "Failed to retrieve file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Retrieve additional metadata
	name := r.FormValue("name")
	typeIDStr := r.FormValue("typeID")

	typeID, err := uuid.Parse(typeIDStr)
	if err != nil {
		s.logger.Error("invalid type ID", "error", err)
		http.Error(w, "Invalid track type ID", http.StatusBadRequest)
		return
	}

	// Validate track type exists
	if _, err := s.store.GetTrackTypeByID(r.Context(), typeID); err != nil {
		s.logger.Error("track type not found", "error", err)
		http.Error(w, "Invalid track type", http.StatusBadRequest)
		return
	}

	id, err := uuid.NewV7()
	if err != nil {
		s.logger.Error("failed to generate UUID", "error", err)
		http.Error(w, "Failed to save track information", http.StatusInternalServerError)
	}

	tempDir := os.TempDir()
	dstPath := filepath.Join(tempDir, id.String())
	dstFile, err := os.Create(dstPath)
	if err != nil {
		s.logger.Error("failed to create file", "error", err, "path", dstPath)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, file); err != nil {
		s.logger.Error("failed to write file", "error", err, "path", dstPath)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	hlsDir := filepath.Join(s.cfg.UploadDir, id.String())
	if err := os.MkdirAll(hlsDir, os.ModePerm); err != nil {
		s.logger.Error("failed to create HLS directory", "error", err, "path", hlsDir)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	cmd := exec.Command("ffmpeg",
		"-i", dstPath,
		"-v", "verbose",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		"-ar", "44100",
		"-hls_time", "6",
		"-hls_playlist_type", "event",
		"-hls_segment_filename", hlsDir+"/segment_%03d.ts",
		"-vn",
		"-f", "hls",
		filepath.Join(hlsDir, "index.m3u8"))
	s.logger.Info("executing ffmpeg command", "command", cmd.String())
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		s.logger.Error("failed to convert file to HLS",
			"err", err,
			"path", dstPath,
			"ffmpegStderr", stderr.String(),
		)

		http.Error(w, "Failed to convert file", http.StatusInternalServerError)
		return
	}

	if err := os.Remove(dstPath); err != nil {
		s.logger.Warn("failed to remove original file", "error", err, "path", dstPath)
	}

	// Save track information to the datastore
	track := Track{
		ID:        id,
		CreatedAt: time.Now(),
		Name:      name,
		Path:      hlsDir,
		TypeID:    typeID,
	}

	if err := s.store.SaveTrack(r.Context(), &track); err != nil {
		s.logger.Error("failed to save track information", "error", err)
		http.Error(w, "Failed to save track information", http.StatusInternalServerError)
		return
	}

	s.logger.Info("file uploaded and converted to HLS", "filename", handler.Filename)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File uploaded and converted to HLS successfully"))
}

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	trackIDString := r.PathValue("trackID")
	trackID, err := uuid.Parse(trackIDString)
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}

	// Retrieve the track to get its folder path
	track, err := s.store.GetTrackByID(r.Context(), trackID)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}

	// Remove the database record
	if err := s.store.DeleteTrack(r.Context(), trackID); err != nil {
		http.Error(w, "Failed to remove track record", http.StatusInternalServerError)
		return
	}

	if err := os.RemoveAll(track.Path); err != nil {
		http.Error(w, "Failed to delete folder", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Track deleted successfully"))
}

func (s *Server) streamDirectory(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	relativePath := strings.TrimPrefix(r.URL.Path, "/api/v1/stream/")
	filePath := filepath.Join(s.cfg.UploadDir, relativePath)
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	s.hub.Register(conn, token)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type authStatusResponse struct {
	Authenticated bool      `json:"authenticated"`
	Role          auth.Role `json:"role,omitempty"`
}

type joinTokenResponse struct {
	Token string `json:"token"`
}

type joinRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := readCookie(r, authCookieName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(authStatusResponse{Authenticated: false})
		return
	}

	token, err := s.auth.ValidateToken(cookie)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(authStatusResponse{Authenticated: false})
		return
	}

	json.NewEncoder(w).Encode(authStatusResponse{
		Authenticated: true,
		Role:          token.Role,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.clearCookie(w, authCookieName)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("failed to decode login request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	creds := auth.Credentials{
		Username: req.Username,
		Password: req.Password,
	}

	token, err := s.auth.ValidateCredentials(creds)
	if err != nil {
		s.logger.Info("login failed", "username", req.Username)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(loginResponse{
			Success: false,
			Error:   "Invalid credentials",
		})
		return
	}

	// Set auth cookie
	s.writeCookie(w, authCookieName, token.String(), token.ExpiresAt)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResponse{
		Success: true,
	})
}

func (s *Server) handleJoinToken(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := joinTokenResponse{
		Token: s.auth.GetJoinToken(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req joinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("failed to decode join request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	token, err := s.auth.ValidateJoinToken(req.Token)
	if err != nil {
		s.logger.Info("join failed", "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(loginResponse{
			Success: false,
			Error:   "Invalid join token",
		})
		return
	}

	s.writeCookie(w, authCookieName, token.String(), token.ExpiresAt)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResponse{
		Success: true,
	})
}

func (s *Server) handleTrackTypes(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	trackTypes, err := s.store.GetTrackTypes(r.Context())
	if err != nil {
		s.logger.Error("failed to get track types", "error", err)
		http.Error(w, "Failed to retrieve track types", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(trackTypes)
}
