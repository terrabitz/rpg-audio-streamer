package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/middlewares"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
	"github.com/terrabitz/rpg-audio-streamer/internal/youtube"
)

const (
	authCookieName = "auth_token"
	cookiePath     = "/"
)

type Authenticator interface {
	ValidateCredentials(creds auth.Credentials) (*auth.Token, error)
	ValidateToken(tokenStr string) (*auth.Token, error)
}

type WSRegisterer interface {
	events.Bus
	Register(conn *websocket.Conn, token *auth.Token)
	RevokePlayerAccess(tableID string)
}

type DiscordController interface {
	Connect(context.Context, string, discord.Destination) error
	Setup(context.Context) (discord.Setup, error)
	Channels(context.Context, string) ([]discord.Choice, error)
	Resolve(context.Context, discord.Destination) (discord.Destination, error)
	Fail(string, error)
	Disconnect(context.Context, string) error
	Status(string) discord.Status
	Close()
}

type Server struct {
	cfg        Config
	logger     *slog.Logger
	frontend   fs.FS
	hub        WSRegisterer
	upgrader   websocket.Upgrader
	auth       Authenticator
	store      Store
	mixer      *livemixer.Service
	discord    DiscordController
	discordMix *discordMixer
	processing *musicprocessing.Service
	videos     VideoScanner
}

type VideoScanner interface {
	Scan(ctx context.Context, sourceURL string) (youtube.VideoMetadata, error)
}

type Config struct {
	Host                    string
	Port                    int
	UploadDir               string
	FrontendDir             string
	DevMode                 bool
	CORS                    middlewares.CorsConfig
	DiscordStreamingEnabled bool
	Discord                 discord.Config
}

func New(cfg Config, logger *slog.Logger, auth Authenticator, store Store, hub WSRegisterer, processing *musicprocessing.Service, videos VideoScanner) (*Server, error) {
	srv := &Server{
		logger:     logger,
		cfg:        cfg,
		hub:        hub,
		auth:       auth,
		store:      store,
		processing: processing,
		videos:     videos,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}

	if cfg.FrontendDir != "" {
		srv.frontend = os.DirFS(cfg.FrontendDir)
		info, err := fs.Stat(srv.frontend, "index.html")
		if err != nil {
			return nil, fmt.Errorf("open frontend index: %w", err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("frontend index is not a regular file")
		}
	}

	if cfg.DiscordStreamingEnabled {
		if cfg.Discord.Token == "" {
			return nil, fmt.Errorf("Discord streaming requires DISCORD_STREAMING_BOT_TOKEN")
		}
		mix, err := livemixer.New(mixerSampleRate, srv.mixerSource)
		if err != nil {
			return nil, fmt.Errorf("initialize live mixer: %w", err)
		}
		srv.mixer = mix
		bot, err := discord.New(cfg.Discord, srv.mixer)
		if err != nil {
			return nil, err
		}
		srv.discord = bot
		bot.SetEventSender(hub)
		srv.discordMix = newDiscordMixer(srv.mixer, srv)
		livemixer.Subscribe(hub, srv.discordMix)
	}

	return srv, nil
}

func (s *Server) sendEvent(w http.ResponseWriter, event string, payload any) bool {
	if err := s.hub.SendEvent(event, payload); err != nil {
		s.logger.Error("failed to send server event", "event", event, "error", err)
		http.Error(w, "The change was saved, but live synchronization failed. Reload before retrying.", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (s *Server) Start(ctx context.Context) error {
	if s.discord != nil {
		defer s.discord.Close()
	}
	go s.RunMixer(ctx)

	srv := &http.Server{
		Addr:    net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port)),
		Handler: s.Handler(),
	}
	stopped := make(chan struct{})
	shutdownDone := make(chan struct{})
	defer func() {
		close(stopped)
		<-shutdownDone
	}()
	go func() {
		defer close(shutdownDone)
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				s.logger.Error("HTTP shutdown failed", "error", err)
				if err := srv.Close(); err != nil {
					s.logger.Error("HTTP connection cleanup failed", "error", err)
				}
			}
		case <-stopped:
		}
	}()

	s.logger.Info("starting server",
		slog.String("host", s.cfg.Host),
		slog.Int("port", s.cfg.Port),
		slog.Bool("devMode", s.cfg.DevMode),
	)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Handler serves the same HTTP and WebSocket routes used by Start.
func (s *Server) Handler() http.Handler {
	apiMux := s.registerHandlers()

	// Apply global middleware
	apiHandler := middlewares.LoggerMiddleware(s.logger)(
		middlewares.CORSMiddleware(middlewares.CorsConfig{
			AllowedOrigins: s.cfg.CORS.AllowedOrigins,
			DevMode:        s.cfg.DevMode,
		})(apiMux),
	)

	mux := http.NewServeMux()
	// NOTE, we want to register this separately, because we don't want to apply
	// the same default middleware we apply to the API routes.
	mux.HandleFunc("/api/v1/ws", s.tableWebSocketMiddleware(s.handleWebSocket))
	mux.Handle("/api", apiHandler)
	mux.Handle("/api/", apiHandler)
	if s.frontend != nil {
		mux.HandleFunc("/", s.serveFrontend)
	} else {
		mux.Handle("/", apiHandler)
	}

	return mux
}

func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	if !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	info, err := fs.Stat(s.frontend, name)
	if err != nil && !os.IsNotExist(err) {
		s.logger.Error("stat frontend file", slog.String("path", name), slog.Any("error", err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err == nil && info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if os.IsNotExist(err) {
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		name = "index.html"
	}
	http.ServeFileFS(w, r, s.frontend, name)
}

func (s *Server) registerHandlers() *http.ServeMux {
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/api/v1/login", s.handleLogin)
	mux.HandleFunc("/api/v1/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/v1/auth/logout", s.handleLogout)

	// Protected endpoints with role validation
	mux.HandleFunc("/api/v1/files", s.gmOnlyMiddleware(s.handleFiles))
	mux.HandleFunc("/api/v1/files/import", s.gmOnlyMiddleware(s.handleImport))
	mux.HandleFunc("/api/v1/files/scan", s.gmOnlyMiddleware(s.handleScan))
	mux.HandleFunc("/api/v1/files/{trackID}", s.gmOnlyMiddleware(s.handleFile))
	mux.HandleFunc("/api/v1/files/{trackID}/usage", s.gmOnlyMiddleware(s.handleFileUsage))
	mux.HandleFunc("/api/v1/files/{trackID}/waveform", s.gmOnlyMiddleware(s.handleFileWaveform))
	mux.HandleFunc("/api/v1/stream/{path...}", s.gmOnlyMiddleware(s.streamDirectory))
	mux.HandleFunc("/api/v1/trackTypes", s.publicMiddleware(s.handleTrackTypes))
	mux.HandleFunc("/api/v1/tables", s.gmOnlyMiddleware(s.handleTables))
	mux.HandleFunc("/api/v1/tables/{tableID}", s.tableAccessMiddleware(s.handleTable))
	mux.HandleFunc("/api/v1/tables/{tableID}/external-id", s.gmOnlyMiddleware(s.handleRotateTableExternalID))
	mux.HandleFunc("/api/v1/tables/{tableID}/settings", s.gmOnlyMiddleware(s.handleTableSettings))
	mux.HandleFunc("/api/v1/tables/{tableID}/discord", s.gmOnlyMiddleware(s.handleDiscord))
	mux.HandleFunc("/api/v1/discord/setup", s.gmOnlyMiddleware(s.handleDiscordSetup))
	mux.HandleFunc("/api/v1/discord/servers/{guildID}/channels", s.gmOnlyMiddleware(s.handleDiscordSetup))
	mux.HandleFunc("/api/v1/tables/{tableID}/tracks", s.tableAccessMiddleware(s.handleTableTracks))
	mux.HandleFunc("/api/v1/tables/{tableID}/stream/{path...}", s.tableAccessMiddleware(s.streamDirectory))
	mux.HandleFunc("/api/v1/tables/{tableID}/tracks/{trackID}", s.gmOnlyMiddleware(s.handleTableTrack))

	return mux
}
