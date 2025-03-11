package server

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/middlewares"
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

type WSRegisterer interface {
	Register(conn *websocket.Conn, token *auth.Token)
}

type Server struct {
	cfg      Config
	logger   *slog.Logger
	frontend fs.FS
	hub      WSRegisterer
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

func New(cfg Config, logger *slog.Logger, auth Authenticator, store Store, hub WSRegisterer) (*Server, error) {
	srv := &Server{
		logger: logger,
		cfg:    cfg,
		hub:    hub,
		auth:   auth,
		store:  store,
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

func (s *Server) Start() error {
	// Ensure upload directory exists
	if err := os.MkdirAll(s.cfg.UploadDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create upload directory: %w", err)
	}

	mux := s.registerHandlers()

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

	s.logger.Info("starting server",
		slog.Int("port", s.cfg.Port),
		slog.Bool("devMode", s.cfg.DevMode),
	)
	return srv.ListenAndServe()
}

func (s *Server) registerHandlers() *http.ServeMux {
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/api/v1/login", s.handleLogin)
	mux.HandleFunc("/api/v1/join", s.handleJoin)
	mux.HandleFunc("/api/v1/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/v1/auth/logout", s.handleLogout)

	// Protected endpoints with role validation
	mux.HandleFunc("/api/v1/files", s.gmOnlyMiddleware(s.handleFiles))
	mux.HandleFunc("/api/v1/files/{trackID}", s.gmOnlyMiddleware(s.handleFileDelete))
	mux.HandleFunc("/api/v1/join-token", s.gmOnlyMiddleware(s.handleJoinToken))
	mux.HandleFunc("/api/v1/stream/", s.authMiddleware(s.streamDirectory))
	mux.HandleFunc("/api/v1/trackTypes", s.authMiddleware(s.handleTrackTypes))

	return mux
}
