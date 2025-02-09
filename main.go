package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

//go:embed all:ui/dist
var frontend embed.FS

type Config struct {
	Server server.Config
	Log    LogConfig
	Auth   auth.Config
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
				},
				Action: func(cCtx *cli.Context) error {
					return startServer(cfg)
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
	cfgJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("couldn't marshal config to JSON: %w", err)
	}
	fmt.Printf("Config: %s\n", string(cfgJSON))
	logger, err := setupLogger(cfg)
	if err != nil {
		return fmt.Errorf("couldn't initialize logger: %w", err)
	}

	authService := auth.New(cfg.Auth, logger)

	srv, err := server.New(cfg.Server, logger, frontend, authService)
	if err != nil {
		return fmt.Errorf("couldn't create server: %w", err)
	}

	return srv.Start()
}
