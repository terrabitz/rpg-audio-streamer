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

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/middlewares"
	"github.com/terrabitz/rpg-audio-streamer/internal/types"
	ws "github.com/terrabitz/rpg-audio-streamer/internal/websocket"
)

type Server struct {
	cfg      Config
	logger   *slog.Logger
	frontend fs.FS
	hub      *ws.Hub
	upgrader websocket.Upgrader
}

// Config holds the configuration for the server
type Config struct {
	// Port the server will listen on
	Port int
	// CORS configuration for the server
	CORS middlewares.CorsConfig
	// Optional upload directory path. Defaults to "./uploads"
	UploadDir string
	// GitHub OAuth configuration
	GitHub auth.GitHubConfig
}

func New(cfg Config, logger *slog.Logger, frontend fs.FS) (*Server, error) {
	hub := ws.NewHub(logger)

	srv := &Server{
		logger:   logger,
		frontend: frontend,
		cfg:      cfg,
		hub:      hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // You might want to make this more secure
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
	httpMux.HandleFunc("/api/v1/files", s.handleFiles)
	httpMux.HandleFunc("/api/v1/files/{fileName}", s.handleFileDelete)
	httpMux.HandleFunc("/api/v1/stream/{fileName}", s.streamFile)
	httpMux.HandleFunc("/api/v1/auth/github", s.handleGitHubAuth)
	httpMux.HandleFunc("/api/v1/auth/github/callback", s.handleGitHubCallback)
	httpMux.HandleFunc("/api/v1/auth/status", s.handleAuthStatus)
	httpMux.HandleFunc("/api/v1/auth/logout", s.handleLogout)
	mux.Handle("/", middlewares.LoggerMiddleware(s.logger)(
		middlewares.CORSMiddleware(s.cfg.CORS)(httpMux),
	))

	mux.HandleFunc("/ws", s.handleWebSocket)

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

func (s *Server) handleGitHubAuth(w http.ResponseWriter, r *http.Request) {
	redirectURL := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&scope=user:email",
		s.cfg.GitHub.ClientID)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	token, err := auth.ExchangeCodeForToken(code, s.cfg.GitHub)
	if err != nil {
		s.logger.Error("failed to exchange code for token", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Set token as HTTP-only cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   86400, // 24 hours
	})

	// Redirect to frontend
	http.Redirect(w, r, "http://localhost:5173/", http.StatusTemporaryRedirect)
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	isAuthenticated := false
	var userData jwt.MapClaims

	if cookie, err := r.Cookie("auth_token"); err == nil && cookie != nil {
		claims, err := auth.ValidateToken(cookie.Value, s.cfg.GitHub.JWTSecret)
		if err == nil {
			isAuthenticated = true
			userData = claims
		} else {
			s.logger.Debug("invalid token", "error", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": isAuthenticated,
		"user":          userData,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusOK)
}
