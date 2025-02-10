package auth

import (
	"fmt"
	"log/slog"
)

type Auth struct {
	cfg    Config
	logger *slog.Logger
}

type Credentials struct {
	Username string
	Password string
}

func New(config Config, logger *slog.Logger) *Auth {
	return &Auth{
		cfg:    config,
		logger: logger,
	}
}

func (a *Auth) ValidateCredentials(creds Credentials) (*Token, error) {
	// Validate username
	if creds.Username != a.cfg.RootUsername {
		a.logger.Debug("invalid username attempt", "username", creds.Username)
		return nil, ErrInvalidCredentials
	}

	// Validate password
	valid, err := VerifyPassword(creds.Password, a.cfg.HashedPassword)
	if err != nil {
		a.logger.Error("failed to verify password", "error", err)
		return nil, fmt.Errorf("failed to verify password: %w", err)
	}

	if !valid {
		a.logger.Debug("invalid password attempt", "username", creds.Username)
		return nil, ErrInvalidCredentials
	}

	// Generate JWT on successful validation with GM role
	token, err := a.NewToken(creds.Username, RoleGM)
	if err != nil {
		a.logger.Error("failed to generate token", "error", err)
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}

func (a *Auth) GetJoinToken() string {
	return a.cfg.JoinToken
}

func (a *Auth) ValidateJoinToken(joinToken string) (*Token, error) {
	if joinToken != a.cfg.JoinToken {
		a.logger.Debug("invalid join token attempt")
		return nil, ErrInvalidCredentials
	}

	token, err := a.NewToken("player", RolePlayer)
	if err != nil {
		a.logger.Error("failed to generate player token", "error", err)
		return nil, fmt.Errorf("failed to generate player token: %w", err)
	}

	return token, nil
}
