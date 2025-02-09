package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/middlewares"
	"github.com/terrabitz/rpg-audio-streamer/internal/types"
	ws "github.com/terrabitz/rpg-audio-streamer/internal/websocket"
)

const (
	authCookieName = "auth_token"
	cookiePath     = "/"
)

type Authenticator interface {
	ValidateCredentials(creds auth.Credentials) (*auth.Token, error)
	ValidateToken(tokenStr string) (*auth.Token, error)
}

type Server struct {
	cfg      Config
	logger   *slog.Logger
	frontend fs.FS
	hub      *ws.Hub
	upgrader websocket.Upgrader
	auth     Authenticator
}

type Config struct {
	Port      int
	UploadDir string

	CORS middlewares.CorsConfig
}

func New(cfg Config, logger *slog.Logger, frontend fs.FS, auth Authenticator) (*Server, error) {
	hub := ws.NewHub(logger)

	srv := &Server{
		logger:   logger,
		frontend: frontend,
		cfg:      cfg,
		hub:      hub,
		auth:     auth,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	go hub.Run()
	return srv, nil
}

func (s *Server) Start() error {
	// Ensure upload directory exists
	if err := os.MkdirAll(s.cfg.UploadDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create upload directory: %w", err)
	}

	frontendFS := http.FileServer(http.FS(s.frontend))

	mux := http.NewServeMux()
	httpMux := http.NewServeMux()
	httpMux.Handle("/", frontendFS)
	httpMux.HandleFunc("/api/v1/login", s.handleLogin)
	httpMux.HandleFunc("/api/v1/files", s.handleFiles)
	httpMux.HandleFunc("/api/v1/files/{fileName}", s.handleFileDelete)
	httpMux.HandleFunc("/api/v1/stream/{fileName}", s.streamFile)
	mux.Handle("/", middlewares.LoggerMiddleware(s.logger)(
		middlewares.CORSMiddleware(s.cfg.CORS)(httpMux),
	))

	mux.HandleFunc("/api/v1/ws", s.handleWebSocket)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: mux,
	}

	s.logger.Info("starting server", "port", s.cfg.Port)
	return srv.ListenAndServe()
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listFiles(w, r)
	case http.MethodPost:
		s.uploadFile(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	dstPath := filepath.Join(s.cfg.UploadDir, handler.Filename)
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

	s.logger.Info("file uploaded", "filename", handler.Filename)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File uploaded successfully"))
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(s.cfg.UploadDir)
	if err != nil {
		s.logger.Error("failed to read directory", "error", err)
		http.Error(w, "Failed to list files", http.StatusInternalServerError)
		return
	}

	var fileList []types.FileInfo
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			s.logger.Warn("failed to get file info", "error", err, "filename", file.Name())
			continue
		}
		fileList = append(fileList, types.FileInfo{
			Name: info.Name(),
			Size: info.Size(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(fileList); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileName := r.PathValue("fileName")
	filePath := filepath.Join(s.cfg.UploadDir, fileName)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			s.logger.Warn("file not found", "filename", fileName)
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		s.logger.Error("failed to delete file", "error", err, "filename", fileName)
		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}

	s.logger.Info("file deleted", "filename", fileName)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File deleted successfully"))
}

func (s *Server) streamFile(w http.ResponseWriter, r *http.Request) {
	fileName := r.PathValue("fileName")
	filePath := filepath.Join(s.cfg.UploadDir, fileName)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.logger.Warn("file not found", "filename", fileName)
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		s.logger.Error("failed to open file", "error", err, "filename", fileName)
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "audio/mp3")
	http.ServeContent(w, r, fileName, time.Time{}, file)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	client := ws.NewClient(s.hub, conn)
	s.hub.Register(client)

	s.logger.Debug("registering new client")

	go client.WritePump()
	go client.ReadPump()
}

// writeCookie sets a secure HTTP-only cookie
func (s *Server) writeCookie(w http.ResponseWriter, name, value string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Expires:  expires,
		Path:     cookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

// readCookie safely reads a cookie by name
func (s *Server) readCookie(r *http.Request, name string) (string, error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// clearCookie removes a cookie by setting its expiration to the past
func (s *Server) clearCookie(w http.ResponseWriter, name string) {
	s.writeCookie(w, name, "", time.Unix(0, 0))
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
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(loginResponse{
			Success: false,
			Error:   "Invalid credentials",
		})
		return
	}

	// Set auth cookie
	s.writeCookie(w, authCookieName, token.String(), token.ExpiresAt())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(loginResponse{
		Success: true,
	})
}
