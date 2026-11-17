package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
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

	respondJSON(w, http.StatusOK, tracks)
}

func (s *Server) handleFileUsage(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	trackID, err := uuid.Parse(r.PathValue("trackID"))
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}
	count, err := s.store.CountTrackTables(r.Context(), trackID)
	if err != nil {
		s.tableError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, struct {
		TableCount int64 `json:"tableCount"`
	}{TableCount: count})
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // 10MB max
	if err != nil {
		s.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	var tableID uuid.UUID
	if value := r.FormValue("tableID"); value != "" {
		tableID, err = uuid.Parse(value)
		if err != nil {
			http.Error(w, "Invalid table ID", http.StatusBadRequest)
			return
		}
		if _, err := s.store.GetTable(r.Context(), tableID); err != nil {
			s.tableError(w, err)
			return
		}
	}

	file, _, err := r.FormFile("files")
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

	track, err := s.processing.Upload(r.Context(), name, typeID, tableID, file)
	if err != nil {
		s.logger.Error("failed to queue uploaded audio", "error", err)
		http.Error(w, "Failed to queue audio processing. Please try again.", http.StatusServiceUnavailable)
		return
	}
	respondJSON(w, http.StatusAccepted, track)
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	switch r.Method {
	case http.MethodDelete:
		s.handleFileDelete(w, r, token)
	case http.MethodPut:
		s.handleFileUpdate(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleFileUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateTrackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("failed to decode update request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	trackIDString := r.PathValue("trackID")
	trackID, err := uuid.Parse(trackIDString)
	if err != nil {
		http.Error(w, "Invalid track ID", http.StatusBadRequest)
		return
	}

	if req.Playback != nil {
		if err := req.Playback.Validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	track, err := s.store.UpdateTrack(r.Context(), trackID, req)
	if err != nil {
		s.logger.Error("failed to update track", "error", err)
		http.Error(w, "Failed to update track", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, track)
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

	var expectedCount *int64
	if values, ok := r.URL.Query()["expectedTableCount"]; ok {
		if len(values) != 1 {
			http.Error(w, "Invalid expected table count", http.StatusBadRequest)
			return
		}
		count, parseErr := strconv.ParseInt(values[0], 10, 64)
		if parseErr != nil || count < 0 {
			http.Error(w, "Invalid expected table count", http.StatusBadRequest)
			return
		}
		expectedCount = &count
	}

	// Retrieve the track to get its folder path
	track, err := s.store.GetTrackByID(r.Context(), trackID)
	if err != nil {
		http.Error(w, "Track not found", http.StatusNotFound)
		return
	}
	if track.Status == "processing" {
		http.Error(w, "Track is still processing", http.StatusConflict)
		return
	}

	if expectedCount == nil {
		err = s.store.DeleteTrack(r.Context(), trackID)
	} else {
		var deleted bool
		deleted, err = s.store.DeleteTrackIfTableCount(r.Context(), trackID, *expectedCount)
		if err == nil && !deleted {
			if _, countErr := s.store.CountTrackTables(r.Context(), trackID); countErr != nil {
				s.tableError(w, countErr)
				return
			}
			http.Error(w, "Track table usage changed", http.StatusConflict)
			return
		}
	}
	if err != nil {
		s.logger.Error("failed to delete track", "error", err)
		http.Error(w, "Failed to remove track record", http.StatusInternalServerError)
		return
	}
	removeErr := os.RemoveAll(track.Path)
	if !s.sendEvent(w, events.TrackRemoved, events.TableEvent[events.TrackReference]{
		Data: events.TrackReference{FileID: trackID.String()},
	}) {
		return
	}
	if removeErr != nil {
		http.Error(w, "Failed to delete folder", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Track deleted successfully"))
}

func (s *Server) streamDirectory(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	relativePath := r.PathValue("path")
	tableIDValue := r.PathValue("tableID")
	if tableIDValue != "" {
		table, err := s.store.GetTableByExternalID(r.Context(), tableIDValue)
		if err != nil {
			s.tableError(w, err)
			return
		}
		if token.Role == auth.RolePlayer {
			if token.TableID != table.ID.String() {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			parts := strings.Split(relativePath, "/")
			trackID, err := uuid.Parse(parts[0])
			if err != nil {
				http.Error(w, "Invalid track ID", http.StatusBadRequest)
				return
			}
			allowed, err := s.store.HasTableTrack(r.Context(), table.ID, trackID)
			if err != nil {
				s.tableError(w, err)
				return
			}
			if !allowed {
				http.Error(w, "Track not found", http.StatusNotFound)
				return
			}
		}
	} else if token.Role != auth.RoleGM {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	filePath := filepath.Join(s.cfg.UploadDir, relativePath)
	uploadPath, err := filepath.Abs(s.cfg.UploadDir)
	if err != nil {
		s.logger.Error("failed to resolve upload directory", "error", err)
		http.Error(w, "Failed to stream file", http.StatusInternalServerError)
		return
	}
	absolutePath, err := filepath.Abs(filePath)
	if err != nil {
		s.logger.Error("failed to resolve stream path", "error", err)
		http.Error(w, "Failed to stream file", http.StatusInternalServerError)
		return
	}
	relativeToUpload, err := filepath.Rel(uploadPath, absolutePath)
	if err != nil || relativeToUpload == ".." || strings.HasPrefix(relativeToUpload, ".."+string(filepath.Separator)) {
		http.Error(w, "Invalid stream path", http.StatusBadRequest)
		return
	}
	trackID, err := uuid.Parse(strings.Split(relativeToUpload, string(filepath.Separator))[0])
	if err != nil {
		http.Error(w, "Invalid track path", http.StatusBadRequest)
		return
	}
	track, err := s.store.GetTrackByID(r.Context(), trackID)
	if err != nil {
		s.tableError(w, err)
		return
	}
	if !track.Ready() {
		http.Error(w, "Track is not ready", http.StatusConflict)
		return
	}
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	scoped := *token
	if token.Role == auth.RoleGM {
		id := DefaultTableID
		if value := r.URL.Query().Get("tableID"); value != "" {
			table, err := s.store.GetTableByExternalID(r.Context(), value)
			if err != nil {
				s.tableError(w, err)
				return
			}
			id = table.ID
		}
		scoped.TableID = id.String()
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	s.hub.Register(conn, &scoped)
}

type authStatusResponse struct {
	Authenticated bool      `json:"authenticated"`
	Role          auth.Role `json:"role,omitempty"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token, err := s.getToken(r)
	if err != nil {
		respondJSON(w, http.StatusOK, authStatusResponse{Authenticated: false})
		return
	}

	respondJSON(w, http.StatusOK, authStatusResponse{
		Authenticated: true,
		Role:          token.Role,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Always clear the cookie, regardless of authentication method
	s.clearCookie(w, authCookieName)
	w.WriteHeader(http.StatusOK)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
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
		respondJSON(w, http.StatusUnauthorized, loginResponse{
			Success: false,
			Error:   "Invalid credentials",
		})
		return
	}

	// Set auth cookie
	s.writeCookie(w, authCookieName, token.String(), token.ExpiresAt)

	respondJSON(w, http.StatusOK, loginResponse{
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

	respondJSON(w, http.StatusOK, trackTypes)
}
