package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/hlsaudio"
	"github.com/terrabitz/rpg-audio-streamer/internal/library"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore"
	"github.com/terrabitz/rpg-audio-streamer/internal/tables"
	"github.com/terrabitz/rpg-audio-streamer/internal/voice"
	ws "github.com/terrabitz/rpg-audio-streamer/internal/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/youtube"
)

//go:embed sql/migrations/*
var migrations embed.FS

const migrationsPath = "sql/migrations"

type Config struct {
	Server    server.Config
	UploadDir string
	Discord   DiscordConfig
	Log       LogConfig
	Auth      auth.Config
	DB        DBConfig
	Bootstrap auth.BootstrapConfig
}

type DiscordConfig struct {
	Enabled bool
	discord.Config
}

type DBConfig struct {
	Path string
}

type LogConfig struct {
	Format string
	Level  string
}

func main() {
	_ = godotenv.Load()

	var cfg Config
	app := &cli.App{
		Name:  "rpg-audio-streamer",
		Usage: "A simple audio file streaming server for tabletop RPGs",
		Commands: []*cli.Command{
			{
				Name:    "serve",
				Aliases: []string{"s"},
				Usage:   "Start the audio streaming server",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name: "discord-streaming-bot-token", EnvVars: []string{"DISCORD_STREAMING_BOT_TOKEN"},
						Usage:       "Bot token for Discord voice streaming; prefer the environment variable",
						Destination: &cfg.Discord.Token,
					},
					&cli.BoolFlag{
						Name:        "discord-streaming-enabled",
						EnvVars:     []string{"DISCORD_STREAMING_ENABLED"},
						Usage:       "Enable Discord voice streaming",
						Destination: &cfg.Discord.Enabled,
					},
					&cli.StringFlag{
						Name:        "host",
						EnvVars:     []string{"HOST"},
						Usage:       "Host to bind (empty listens on all interfaces)",
						Destination: &cfg.Server.Host,
					},
					&cli.IntFlag{
						Name:        "port",
						EnvVars:     []string{"PORT"},
						Value:       8080,
						Usage:       "Port to listen on",
						Destination: &cfg.Server.Port,
					},
					&cli.StringFlag{
						Name:        "cors-origins",
						EnvVars:     []string{"CORS_ORIGINS"},
						Usage:       "Allowed CORS origins",
						Destination: &cfg.Server.CORS.AllowedOrigins,
					},
					&cli.StringFlag{
						Name:        "log-format",
						EnvVars:     []string{"LOG_FORMAT"},
						Value:       "json",
						Usage:       "Log format (json or pretty)",
						Destination: &cfg.Log.Format,
					},
					&cli.StringFlag{
						Name:        "log-level",
						EnvVars:     []string{"LOG_LEVEL"},
						Value:       "info",
						Usage:       "Log level (debug, info, warn, error)",
						Destination: &cfg.Log.Level,
					},
					&cli.StringFlag{
						Name:        "upload-dir",
						EnvVars:     []string{"UPLOAD_DIR"},
						Value:       "./uploads",
						Usage:       "Directory to store uploaded files",
						Destination: &cfg.UploadDir,
					},
					&cli.StringFlag{
						Name:        "frontend-dir",
						EnvVars:     []string{"FRONTEND_DIR"},
						Usage:       "Directory containing the built SPA (empty disables frontend serving)",
						Destination: &cfg.Server.FrontendDir,
					},
					&cli.StringFlag{
						Name:        "root-username",
						EnvVars:     []string{"ROOT_USERNAME"},
						Value:       "admin",
						Usage:       "Initial admin username",
						Destination: &cfg.Bootstrap.Username,
					},
					&cli.StringFlag{
						Name:    "root-password",
						EnvVars: []string{"ROOT_PASSWORD"},
						Usage:   "Initial admin password (used only when no admin exists)",
					},
					&cli.StringFlag{
						Name:    "root-password-hash",
						EnvVars: []string{"ROOT_PASSWORD_HASH"},
						Usage:   "Initial admin Argon2id password hash (alternative to ROOT_PASSWORD)",
					},
					&cli.StringFlag{
						Name:        "token-secret",
						EnvVars:     []string{"TOKEN_SECRET"},
						Usage:       "Secret for signing JWT tokens",
						Destination: &cfg.Auth.TokenSecret,
					},
					&cli.DurationFlag{
						Name:        "token-duration",
						EnvVars:     []string{"TOKEN_DURATION"},
						Value:       24 * time.Hour,
						Usage:       "Duration for JWT tokens",
						Destination: &cfg.Auth.TokenDuration,
					},
					&cli.StringFlag{
						Name:        "token-issuer",
						EnvVars:     []string{"TOKEN_ISSUER"},
						Value:       "rpg-audio-streamer",
						Usage:       "Issuer for JWT tokens",
						Destination: &cfg.Auth.TokenIssuer,
					},
					&cli.StringFlag{
						Name:        "token-audience",
						EnvVars:     []string{"TOKEN_AUDIENCE"},
						Value:       "rpg-audio-streamer-client",
						Usage:       "Audience for JWT tokens",
						Destination: &cfg.Auth.TokenAudience,
					},
					&cli.BoolFlag{
						Name:        "dev-mode",
						EnvVars:     []string{"DEV_MODE"},
						Usage:       "Enables development mode",
						Destination: &cfg.Server.DevMode,
					},
					&cli.StringFlag{
						Name:        "db-path",
						EnvVars:     []string{"DB_PATH"},
						Value:       "skaldbot.db",
						Usage:       "Path to SQLite database file",
						Destination: &cfg.DB.Path,
					},
				},
				Action: func(cCtx *cli.Context) error {
					if cCtx.IsSet("root-password") {
						password := cCtx.String("root-password")
						cfg.Bootstrap.Password = &password
					}
					if cCtx.IsSet("root-password-hash") {
						hash := cCtx.String("root-password-hash")
						cfg.Bootstrap.Hash = &hash
					}
					return startServer(cCtx.Context, cfg)
				},
			},
			{
				Name:  "migrate",
				Usage: "Run database migrations",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "db-path",
						EnvVars:     []string{"DB_PATH"},
						Value:       "skaldbot.db",
						Usage:       "Path to SQLite database file",
						Destination: &cfg.DB.Path,
					},
				},
				Subcommands: []*cli.Command{
					{
						Name:  "up",
						Usage: "Apply all up migrations",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "steps",
								Value: 0,
								Usage: "Number of migrations to apply",
							},
						},
						Action: func(cCtx *cli.Context) error {
							migrationsSub, err := fs.Sub(migrations, migrationsPath)
							if err != nil {
								log.Fatalf("couldn't find database migrations: %v", err)
							}

							db, err := sqlitedatastore.New(cfg.DB.Path)
							if err != nil {
								return fmt.Errorf("couldn't initialize SQLite DB: %w", err)
							}
							migrations, err := sqlitedatastore.NewMigration(migrationsSub, db)
							if err != nil {
								return fmt.Errorf("couldn't initialize migrations: %w", err)
							}

							steps := cCtx.Int("steps")
							if steps == 0 {
								if err := migrations.Up(); err != nil {
									return fmt.Errorf("couldn't apply migrations: %w", err)
								}
							} else {
								if err := migrations.Steps(steps); err != nil {
									return fmt.Errorf("couldn't apply migrations: %w", err)
								}
							}
							return nil
						},
					},
					{
						Name:  "down",
						Usage: "Revert the last n migrations",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "steps",
								Value: 1,
								Usage: "Number of migrations to revert",
							},
						},
						Action: func(cCtx *cli.Context) error {
							migrationsSub, err := fs.Sub(migrations, migrationsPath)
							if err != nil {
								log.Fatalf("couldn't find database migrations: %v", err)
							}

							db, err := sqlitedatastore.New(cfg.DB.Path)
							if err != nil {
								return fmt.Errorf("couldn't initialize SQLite DB: %w", err)
							}
							migrations, err := sqlitedatastore.NewMigration(migrationsSub, db)
							if err != nil {
								return fmt.Errorf("couldn't initialize migrations: %w", err)
							}

							steps := cCtx.Int("steps")
							if err := migrations.Steps(-steps); err != nil {
								return fmt.Errorf("couldn't revert migrations: %w", err)
							}
							return nil
						},
					},
					{
						Name:  "version",
						Usage: "Print the current migration version",
						Action: func(cCtx *cli.Context) error {
							migrationsSub, err := fs.Sub(migrations, migrationsPath)
							if err != nil {
								log.Fatalf("couldn't find database migrations: %v", err)
							}

							db, err := sqlitedatastore.New(cfg.DB.Path)
							if err != nil {
								return fmt.Errorf("couldn't initialize SQLite DB: %w", err)
							}

							migrations, err := sqlitedatastore.NewMigration(migrationsSub, db)
							if err != nil {
								return fmt.Errorf("couldn't initialize migrations: %w", err)
							}

							version, err := migrations.Version()
							if err != nil {
								return fmt.Errorf("couldn't get migration version: %w", err)
							}

							versionJSON, _ := json.MarshalIndent(version, "", "  ")
							fmt.Println(string(versionJSON))
							return nil
						},
					},
				},
			},
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.RunContext(ctx, os.Args); err != nil {
		log.Fatal(err)
	}
}

func setupLogger(cfg Config) (*slog.Logger, error) {
	level := new(slog.Level)
	if err := level.UnmarshalText([]byte(strings.ToLower(cfg.Log.Level))); err != nil {
		return nil, fmt.Errorf("couldn't parse log level '%s': %w", cfg.Log.Level, err)
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Log.Format) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler), nil
}

func startServer(ctx context.Context, cfg Config) error {
	logger, err := setupLogger(cfg)
	if err != nil {
		return fmt.Errorf("couldn't initialize logger: %w", err)
	}

	migrationsSub, err := fs.Sub(migrations, migrationsPath)
	if err != nil {
		log.Fatalf("couldn't find database migrations: %v", err)
	}

	db, err := sqlitedatastore.New(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("couldn't initialize SQLite DB: %w", err)
	}
	defer db.Close()

	migrations, err := sqlitedatastore.NewMigration(migrationsSub, db)
	if err != nil {
		return fmt.Errorf("couldn't initialize migrations: %w", err)
	}

	if err := migrations.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("couldn't run migrations: %w", err)
	}

	authService, code, err := auth.New(ctx, cfg.Auth, cfg.Bootstrap, db, logger)
	if err != nil {
		return fmt.Errorf("couldn't initialize authentication: %w", err)
	}

	hub := ws.NewHub(logger, db)

	videos := youtube.New(logger)
	processing := musicprocessing.New(cfg.UploadDir, logger, db, videos, hub)
	if err := processing.Recover(ctx); err != nil {
		return fmt.Errorf("recover interrupted processing: %w", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		processing.Run(runCtx)
	}()
	defer func() {
		cancel()
		<-finished
	}()

	var bot voice.Discord
	if cfg.Discord.Enabled {
		discordBot, err := startDiscord(runCtx, cfg.Discord.Config, db, hub, logger)
		if err != nil {
			return err
		}
		defer discordBot.Close()
		bot = discordBot
	}

	tablesService := tables.New(db, hub.Playback(), hub, hub)
	libraryService := library.New(db, processing, videos, tablesService, hub, cfg.UploadDir)
	voiceService := voice.New(bot, db, tablesService, hub.Playback(), hub, logger)

	srv, err := server.New(cfg.Server, logger, authService, hub, tablesService, libraryService, voiceService)
	if err != nil {
		return fmt.Errorf("couldn't create server: %w", err)
	}
	if code != "" {
		fmt.Fprintf(os.Stderr, "First-run setup: open the app and enter this one-time setup code (expires in 30 minutes):\nsetup_code=%s\n", code)
	}

	hubFinished := make(chan struct{})
	go func() {
		defer close(hubFinished)
		hub.Run(runCtx)
	}()
	defer func() {
		cancel()
		<-hubFinished
	}()

	return srv.Start(runCtx)
}

// startDiscord streams each table's live server-side mix into its Discord voice channel.
func startDiscord(ctx context.Context, cfg discord.Config, db *sqlitedatastore.SQLiteDatastore, hub *ws.Hub, logger *slog.Logger) (*discord.Service, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("Discord streaming requires DISCORD_STREAMING_BOT_TOKEN")
	}
	clips := voice.NewClips(db, hlsaudio.Open)
	mix, err := livemixer.New(hlsaudio.SampleRate, clips.Load)
	if err != nil {
		return nil, fmt.Errorf("initialize live mixer: %w", err)
	}
	bot, err := discord.New(cfg, mix)
	if err != nil {
		return nil, err
	}
	bot.SetEventSender(hub)
	livemixer.Subscribe(hub, discord.NewMixerSessions(mix, bot, logger))
	go mix.Run(ctx)
	return bot, nil
}
