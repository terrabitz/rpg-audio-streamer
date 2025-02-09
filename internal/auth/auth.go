package auth

import (
	"log/slog"
)

type Auth struct {
	config Config
	logger *slog.Logger
}

func New(config Config, logger *slog.Logger) *Auth {
	return &Auth{
		config: config,
		logger: logger,
	}
}
