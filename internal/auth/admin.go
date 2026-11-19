package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	ErrSetupComplete = errors.New("instance has already been claimed")
	ErrInvalidCode   = errors.New("invalid or expired setup code; check the container logs or restart the unclaimed instance")
	ErrPassword      = errors.New("current password is incorrect")
	ErrInvalidInput  = errors.New("invalid settings")
)

type Admin struct {
	Username     string
	PasswordHash string
}

type Store interface {
	LoadAdmin(context.Context) (Admin, error)
	CreateAdmin(context.Context, Admin) (bool, error)
	UpdateAdminPassword(context.Context, string, string) (bool, error)
	LoadSigningSecret(context.Context) (string, error)
	InitializeSigningSecret(context.Context, string) (string, error)
}

type BootstrapConfig struct {
	Username string
	Password *string
	Hash     *string
}

type claimState struct {
	mu       sync.Mutex
	username string
	codeHash [32]byte
	expires  time.Time
}

type SetupStatus struct {
	Required bool   `json:"required"`
	Username string `json:"username"`
}

type Profile struct {
	Username string `json:"username"`
}

func New(ctx context.Context, cfg Config, bootstrap BootstrapConfig, store Store, logger *slog.Logger) (*Auth, string, error) {
	admin, err := store.LoadAdmin(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("load admin: %w", err)
	}
	if bootstrap.Username == "" {
		bootstrap.Username = "admin"
	}
	bootstrapped := admin.PasswordHash == "" && (bootstrap.Password != nil || bootstrap.Hash != nil)
	if bootstrapped {
		if err := bootstrapAdmin(ctx, bootstrap, store); err != nil {
			return nil, "", err
		}
	}
	if cfg.TokenSecret == "" {
		cfg.TokenSecret, err = signingSecret(ctx, store)
		if err != nil {
			return nil, "", err
		}
	}

	a := &Auth{
		cfg:    cfg,
		store:  store,
		logger: logger,
	}
	if bootstrapped || admin.PasswordHash != "" {
		return a, "", nil
	}
	code := rand.Text()
	a.claim.username = bootstrap.Username
	a.claim.codeHash = sha256.Sum256([]byte(code))
	a.claim.expires = time.Now().Add(30 * time.Minute)
	return a, code, nil
}

func bootstrapAdmin(ctx context.Context, cfg BootstrapConfig, store Store) error {
	if err := validateUsername(cfg.Username); err != nil {
		return fmt.Errorf("ROOT_USERNAME: %w", err)
	}
	var hash string
	switch {
	case cfg.Password != nil && cfg.Hash != nil:
		return errors.New("set only one of ROOT_PASSWORD or ROOT_PASSWORD_HASH")
	case cfg.Password != nil:
		var err error
		hash, err = hashPassword(*cfg.Password)
		if err != nil {
			return fmt.Errorf("ROOT_PASSWORD: %w", err)
		}
	default:
		hash = *cfg.Hash
		if err := ValidatePasswordHash(hash); err != nil {
			return fmt.Errorf("ROOT_PASSWORD_HASH: %w", err)
		}
	}
	_, err := store.CreateAdmin(ctx, Admin{
		Username:     cfg.Username,
		PasswordHash: hash,
	})
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	return nil
}

func signingSecret(ctx context.Context, store Store) (string, error) {
	secret, err := store.LoadSigningSecret(ctx)
	if err != nil {
		return "", fmt.Errorf("load signing secret: %w", err)
	}
	if secret != "" {
		return secret, nil
	}
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return "", fmt.Errorf("generate signing secret: %w", err)
	}
	secret, err = store.InitializeSigningSecret(ctx, base64.RawURLEncoding.EncodeToString(key[:]))
	if err != nil {
		return "", fmt.Errorf("save signing secret: %w", err)
	}
	return secret, nil
}

func (a *Auth) SetupStatus(ctx context.Context) (SetupStatus, error) {
	admin, err := a.store.LoadAdmin(ctx)
	if err != nil {
		return SetupStatus{}, fmt.Errorf("load admin: %w", err)
	}
	if admin.PasswordHash != "" {
		return SetupStatus{}, nil
	}
	return SetupStatus{
		Required: true,
		Username: a.claim.username,
	}, nil
}

func (a *Auth) Claim(ctx context.Context, code, username, password string) error {
	a.claim.mu.Lock()
	defer a.claim.mu.Unlock()
	admin, err := a.store.LoadAdmin(ctx)
	if err != nil {
		return fmt.Errorf("load admin: %w", err)
	}
	if admin.PasswordHash != "" {
		return ErrSetupComplete
	}
	digest := sha256.Sum256([]byte(code))
	if !time.Now().Before(a.claim.expires) || subtle.ConstantTimeCompare(digest[:], a.claim.codeHash[:]) != 1 {
		return ErrInvalidCode
	}
	if err := validateUsername(username); err != nil {
		return err
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	created, err := a.store.CreateAdmin(ctx, Admin{
		Username:     username,
		PasswordHash: hash,
	})
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	if !created {
		return ErrSetupComplete
	}
	a.claim.codeHash = [32]byte{}
	a.claim.expires = time.Time{}
	return nil
}

func (a *Auth) Profile(ctx context.Context) (Profile, error) {
	admin, err := a.store.LoadAdmin(ctx)
	if err != nil {
		return Profile{}, fmt.Errorf("load admin: %w", err)
	}
	if admin.PasswordHash == "" {
		return Profile{}, ErrInvalidCredentials
	}
	return Profile{Username: admin.Username}, nil
}

func (a *Auth) ChangePassword(ctx context.Context, current, password string) error {
	admin, err := a.store.LoadAdmin(ctx)
	if err != nil {
		return fmt.Errorf("load admin: %w", err)
	}
	if admin.PasswordHash == "" || len(current) > 1024 {
		return ErrPassword
	}
	valid, err := VerifyPassword(current, admin.PasswordHash)
	if err != nil {
		return fmt.Errorf("verify current password: %w", err)
	}
	if !valid {
		return ErrPassword
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	updated, err := a.store.UpdateAdminPassword(ctx, admin.PasswordHash, hash)
	if err != nil {
		return fmt.Errorf("save new password: %w", err)
	}
	if !updated {
		return ErrPassword
	}
	return nil
}

func validateUsername(username string) error {
	if strings.TrimSpace(username) != username || username == "" || utf8.RuneCountInString(username) > 100 {
		return fmt.Errorf("%w: username must be 1-100 characters without surrounding whitespace", ErrInvalidInput)
	}
	return nil
}

func hashPassword(password string) (string, error) {
	if utf8.RuneCountInString(password) < 12 || len(password) > 1024 {
		return "", fmt.Errorf("%w: password must be at least 12 characters and at most 1024 bytes", ErrInvalidInput)
	}
	hash, err := HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return hash, nil
}
