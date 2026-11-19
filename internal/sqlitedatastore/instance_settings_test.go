package sqlitedatastore

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

func TestAdminBootstrapDoesNotOverwriteSavedCredentials(t *testing.T) {
	ctx := context.Background()
	db, path := migratedDatabase(t)
	initial := "initial admin password"
	bootstrap := auth.BootstrapConfig{
		Username: "owner",
		Password: &initial,
	}
	cfg := auth.Config{
		TokenDuration: time.Hour,
		TokenIssuer:   "test",
		TokenAudience: "test",
	}
	instance, code, err := auth.New(ctx, cfg, bootstrap, db, slog.Default())
	if err != nil || code != "" {
		t.Fatalf("bootstrap should bypass claiming: %v", err)
	}
	admin, err := db.LoadAdmin(ctx)
	if err != nil || admin.Username != "owner" {
		t.Fatalf("admin was not saved: %v", err)
	}
	if ok, err := auth.VerifyPassword(initial, admin.PasswordHash); !ok || err != nil {
		t.Fatalf("stored hash did not verify: %v", err)
	}

	newPassword := "changed admin password"
	session, err := instance.ValidateCredentials(ctx, auth.Credentials{
		Username: "owner",
		Password: initial,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := instance.ChangePassword(ctx, initial, newPassword); err != nil {
		t.Fatal(err)
	}
	if updated, err := db.UpdateAdminPassword(ctx, admin.PasswordHash, admin.PasswordHash); err != nil || updated {
		t.Fatalf("stale writer should not overwrite a newer password: updated=%v, err=%v", updated, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	differentPassword := "different bootstrap password"
	bootstrap.Password = &differentPassword
	bootstrap.Username = "other"
	restarted, code, err := auth.New(ctx, cfg, bootstrap, reopened, slog.Default())
	if err != nil || code != "" {
		t.Fatalf("restart should not reopen claiming: %v", err)
	}
	if _, err := restarted.ValidateToken(session.String()); err != nil {
		t.Fatalf("password change or restart invalidated the session: %v", err)
	}
	for _, invalid := range []string{initial, differentPassword} {
		if err := restarted.ChangePassword(ctx, invalid, newPassword); !errors.Is(err, auth.ErrPassword) {
			t.Fatalf("bootstrap input remained active: %v", err)
		}
	}
	if err := restarted.ChangePassword(ctx, newPassword, "another admin password"); err != nil {
		t.Fatalf("new password did not survive restart: %v", err)
	}
}

func TestClaimCreatesAdminOnlyOnce(t *testing.T) {
	ctx := context.Background()
	db, _ := migratedDatabase(t)
	first, code, err := auth.New(ctx, auth.Config{}, auth.BootstrapConfig{}, db, slog.Default())
	if err != nil || code == "" {
		t.Fatalf("empty database did not offer claiming: %v", err)
	}
	status, err := first.SetupStatus(ctx)
	if err != nil || !status.Required {
		t.Fatalf("setup status did not require claiming: %v", err)
	}
	second, secondCode, err := auth.New(ctx, auth.Config{}, auth.BootstrapConfig{}, db, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Claim(ctx, "wrong", "owner", "a long owner password"); !errors.Is(err, auth.ErrInvalidCode) {
		t.Fatalf("wrong setup code got %v", err)
	}
	if err := first.Claim(ctx, code, "owner", "short"); !errors.Is(err, auth.ErrInvalidInput) {
		t.Fatalf("short password got %v", err)
	}
	if err := first.Claim(ctx, code, "owner", "a long owner password"); err != nil {
		t.Fatal(err)
	}
	for _, claim := range []struct {
		instance *auth.Auth
		code     string
	}{
		{first, code},
		{second, secondCode},
	} {
		if err := claim.instance.Claim(ctx, claim.code, "other", "another long password"); !errors.Is(err, auth.ErrSetupComplete) {
			t.Fatalf("duplicate claim got %v", err)
		}
	}
	admin, err := db.LoadAdmin(ctx)
	if err != nil || admin.Username != "owner" {
		t.Fatalf("claim did not save the correct user: %v", err)
	}
	if _, err := second.ValidateCredentials(ctx, auth.Credentials{
		Username: "owner",
		Password: "a long owner password",
	}); err != nil {
		t.Fatalf("login did not read the newly claimed account: %v", err)
	}
}
