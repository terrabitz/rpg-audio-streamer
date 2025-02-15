package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/middlewares"
	ws "github.com/terrabitz/rpg-audio-streamer/internal/websocket"
)

const (
	authCookieName = "auth_token"
	cookiePath     = "/"
)

type Authenticator interface {
	ValidateCredentials(creds auth.Credentials) (*auth.Token, error)
	ValidateToken(tokenStr string) (*auth.Token, error)
	GetJoinToken() string
	ValidateJoinToken(joinToken string) (*auth.Token, error)
}

type Server struct {
	cfg      Config
	logger   *slog.Logger
	frontend fs.FS
	hub      *ws.Hub
	upgrader websocket.Upgrader
	auth     Authenticator
	store    Store
}

type Config struct {
	Port      int
	UploadDir string
	DevMode   bool
	CORS      middlewares.CorsConfig
}

func New(cfg Config, logger *slog.Logger, frontend fs.FS, auth Authenticator, store Store) (*Server, error) {
	hub := ws.NewHub(logger)

	srv := &Server{
		logger:   logger,
		frontend: frontend,
		cfg:      cfg,
		hub:      hub,
		auth:     auth,
		store:    store,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	return srv, nil
}

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := readCookie(r, authCookieName)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := s.auth.ValidateToken(cookie)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := SetAuthToken(r.Context(), token)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) gmOnlyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return s.authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		token, ok := GetAuthToken(r.Context())
		if !ok || token.Role != auth.RoleGM {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

func (s *Server) Start() error {
	// Ensure upload directory exists
	if err := os.MkdirAll(s.cfg.UploadDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create upload directory: %w", err)
	}

	frontendFS := http.FileServer(http.FS(s.frontend))

	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/", frontendFS.ServeHTTP)
	mux.HandleFunc("/api/v1/login", s.handleLogin)
	mux.HandleFunc("/api/v1/join", s.handleJoin)
	mux.HandleFunc("/api/v1/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/v1/auth/logout", s.handleLogout)

	// Protected endpoints
	mux.HandleFunc("/api/v1/stream/", s.authMiddleware(s.streamDirectory))
	mux.HandleFunc("/api/v1/files", s.gmOnlyMiddleware(s.handleFiles))
	mux.HandleFunc("/api/v1/files/{trackID}", s.gmOnlyMiddleware(s.handleFileDelete))
	mux.HandleFunc("/api/v1/trackTypes", s.authMiddleware(s.handleTrackTypes))
	mux.HandleFunc("/api/v1/join-token", s.gmOnlyMiddleware(s.handleJoinToken))

	// Apply global middleware
	handler := middlewares.LoggerMiddleware(s.logger)(
		middlewares.CORSMiddleware(middlewares.CorsConfig{
			AllowedOrigins: s.cfg.CORS.AllowedOrigins,
			DevMode:        s.cfg.DevMode,
		})(mux),
	)

	mux = http.NewServeMux()
	mux.HandleFunc("/api/v1/ws", s.authMiddleware(s.handleWebSocket))
	mux.Handle("/", handler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: mux,
	}

	s.hub.HandleFunc("ping", func(payload json.RawMessage, c *ws.Client) {
		if err := c.Send(ws.Message{
			Method:  "pong",
			Payload: payload,
		}); err != nil {
			s.logger.Error("failed to send pong message", "error", err)
		}
	})

	s.hub.HandleFunc("broadcast", func(payload json.RawMessage, c *ws.Client) {
		if c.Token.Role != auth.RoleGM {
			s.logger.Warn("unauthorized broadcast attempt", "role", c.Token.Role)
			return
		}

		s.hub.Broadcast(ws.Message{
			Method:   "broadcast",
			SenderID: c.ID,
			Payload:  payload,
		}, ws.ExceptClient(c)) // Don't send back to sender
	})

	s.hub.HandleFunc("syncRequest", func(payload json.RawMessage, c *ws.Client) {
		// Only forward to GM clients
		s.hub.Broadcast(ws.Message{
			Method:   "syncRequest",
			SenderID: c.ID,
			Payload:  payload,
		}, ws.ToGMOnly())
	})

	s.hub.HandleFunc("syncAll", func(payload json.RawMessage, c *ws.Client) {
		if c.Token.Role != auth.RoleGM {
			s.logger.Warn("unauthorized broadcast attempt", "role", c.Token.Role)
			return
		}

		// Extract target client ID from payload
		var syncPayload struct {
			Tracks []any  `json:"tracks"`
			To     string `json:"to"`
		}
		if err := json.Unmarshal(payload, &syncPayload); err != nil {
			s.logger.Error("failed to unmarshal sync payload", "error", err)
			return
		}

		// If a target client is specified, only send to them
		if syncPayload.To != "" {
			s.hub.Broadcast(ws.Message{
				Method:   "syncAll",
				SenderID: c.ID,
				Payload:  payload,
			}, ws.ToClientID(syncPayload.To))
		} else {
			// Otherwise broadcast to all players
			s.hub.Broadcast(ws.Message{
				Method:   "syncAll",
				SenderID: c.ID,
				Payload:  payload,
			}, ws.ToPlayersOnly())
		}
	})

	s.hub.HandleFunc("syncTrack", func(payload json.RawMessage, c *ws.Client) {
		if c.Token.Role != auth.RoleGM {
			s.logger.Warn("unauthorized syncTrack command", "role", c.Token.Role)
			return
		}
		s.hub.Broadcast(ws.Message{
			Method:   "syncTrack",
			SenderID: c.ID,
			Payload:  payload,
		}, ws.ToPlayersOnly())
	})

	go s.hub.Run()

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

func (s *Server) handleFileDelete(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) streamDirectory(w http.ResponseWriter, r *http.Request) {
	relativePath := strings.TrimPrefix(r.URL.Path, "/api/v1/stream/")
	filePath := filepath.Join(s.cfg.UploadDir, relativePath)
	http.ServeFile(w, r, filePath)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	token, ok := GetAuthToken(r.Context())
	if !ok {
		s.logger.Error("failed to get auth token from context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	client := ws.NewClient(s.hub, conn, token)
	s.hub.Register(client)

	s.logger.Debug("new websocket connection",
		"clientId", client.ID,
		"role", token.Role,
	)

	go client.WritePump()
	go client.ReadPump()
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

func (s *Server) handleJoinToken(w http.ResponseWriter, r *http.Request) {
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
