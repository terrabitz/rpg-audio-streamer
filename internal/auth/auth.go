package auth

import (
	"context"
	"fmt"
	"log/slog"
)

type Auth struct {
	cfg    Config
	store  Store
	logger *slog.Logger
	claim  claimState
}

type Credentials struct {
	Username string
	Password string
}

func (a *Auth) ValidateCredentials(ctx context.Context, creds Credentials) (*Token, error) {
	if len(creds.Password) > 1024 {
		return nil, ErrInvalidCredentials
	}
	admin, err := a.store.LoadAdmin(ctx)
	if err != nil {
		return nil, fmt.Errorf("load admin: %w", err)
	}
	if admin.PasswordHash == "" || creds.Username != admin.Username {
		a.logger.Debug("invalid username attempt", "username", creds.Username)
		return nil, ErrInvalidCredentials
	}

	valid, err := VerifyPassword(creds.Password, admin.PasswordHash)
	if err != nil {
		a.logger.Error("failed to verify password", "error", err)
		return nil, fmt.Errorf("failed to verify password: %w", err)
	}

	if !valid {
		a.logger.Debug("invalid password attempt", "username", creds.Username)
		return nil, ErrInvalidCredentials
	}

	token, err := a.NewToken(creds.Username, RoleGM)
	if err != nil {
		a.logger.Error("failed to generate token", "error", err)
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}
