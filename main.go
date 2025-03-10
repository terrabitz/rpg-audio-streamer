package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore"
	ws "github.com/terrabitz/rpg-audio-streamer/internal/websocket"
)

//go:embed sql/migrations/*
var migrations embed.FS

const migrationsPath = "sql/migrations"

type Config struct {
	Server server.Config
	Log    LogConfig
	Auth   auth.Config
	DB     DBConfig
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
						Destination: &cfg.Server.UploadDir,
					},
					&cli.StringFlag{
						Name:        "root-username",
						EnvVars:     []string{"ROOT_USERNAME"},
						Value:       "admin",
						Usage:       "Root username for authentication",
						Destination: &cfg.Auth.RootUsername,
					},
					&cli.StringFlag{
						Name:        "root-password-hash",
						EnvVars:     []string{"ROOT_PASSWORD_HASH"},
						Required:    true,
						Usage:       "Argon2id hash of root password",
						Destination: &cfg.Auth.HashedPassword,
					},
					&cli.StringFlag{
						Name:        "token-secret",
						EnvVars:     []string{"TOKEN_SECRET"},
						Required:    true,
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
					&cli.StringFlag{
						Name:        "join-token",
						EnvVars:     []string{"JOIN_TOKEN"},
						Usage:       "The static join token to use",
						Destination: &cfg.Auth.JoinToken,
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
					return startServer(cfg)
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

	if err := app.Run(os.Args); err != nil {
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

func startServer(cfg Config) error {
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

	migrations, err := sqlitedatastore.NewMigration(migrationsSub, db)
	if err != nil {
		return fmt.Errorf("couldn't initialize migrations: %w", err)
	}

	if err := migrations.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("couldn't run migrations: %w", err)
	}

	authService := auth.New(cfg.Auth, logger)

	hub := ws.NewHub(logger)

	srv, err := server.New(cfg.Server, logger, authService, db, hub)
	if err != nil {
		return fmt.Errorf("couldn't create server: %w", err)
	}

	// FIXME use cleaner shutdown handling
	go hub.Run()

	return srv.Start()
}
