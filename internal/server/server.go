package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/middlewares"
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
	Register(conn *websocket.Conn, token *auth.Token)
	CloseTable(tableID string)
	RevokePlayerAccess(tableID string)
	RemoveTrack(tableID, trackID string)
	SyncSettings(tableID string, patch TableSettingsPatch)
}

type Mixer interface {
	Update(tableID string, payload json.RawMessage) error
	Sync(tableID string, payload json.RawMessage) error
	Volumes(tableID string, payload json.RawMessage) error
	CloseTable(tableID string)
	RemoveTrack(tableID, trackID string)
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
}

type Config struct {
	Host                    string
	Port                    int
	UploadDir               string
	DevMode                 bool
	CORS                    middlewares.CorsConfig
	DiscordStreamingEnabled bool
	Discord                 discord.Config
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

	if cfg.DiscordStreamingEnabled {
		if cfg.Discord.Token == "" {
			return nil, fmt.Errorf("Discord streaming requires DISCORD_STREAMING_BOT_TOKEN")
		}
		consumer, ok := hub.(interface{ SetMixer(Mixer) })
		if !ok {
			return nil, fmt.Errorf("Discord streaming requires a mixer-capable WebSocket hub")
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
		srv.discordMix = newDiscordMixer(srv.mixer, srv)
		consumer.SetMixer(srv.discordMix)
	}

	return srv, nil
}

func (s *Server) Start() error {
	// Ensure upload directory exists
	if err := os.MkdirAll(s.cfg.UploadDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create upload directory: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if s.discord != nil {
		defer s.discord.Close()
	}
	go s.RunMixer(ctx)

	srv := &http.Server{
		Addr:    net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port)),
		Handler: s.Handler(),
	}

	s.logger.Info("starting server",
		slog.String("host", s.cfg.Host),
		slog.Int("port", s.cfg.Port),
		slog.Bool("devMode", s.cfg.DevMode),
	)
	return srv.ListenAndServe()
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
	mux.Handle("/", apiHandler)

	return mux
}

func (s *Server) registerHandlers() *http.ServeMux {
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/api/v1/login", s.handleLogin)
	mux.HandleFunc("/api/v1/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/v1/auth/logout", s.handleLogout)

	// Protected endpoints with role validation
	mux.HandleFunc("/api/v1/files", s.gmOnlyMiddleware(s.handleFiles))
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
