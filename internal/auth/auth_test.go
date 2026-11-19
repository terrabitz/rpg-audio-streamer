package auth

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

type testStore struct {
	Store
	admin Admin
}

func (s testStore) LoadAdmin(ctx context.Context) (Admin, error) {
	return s.admin, ctx.Err()
}

func TestValidateCredentials(t *testing.T) {
	hashedPassword, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	auth := &Auth{
		cfg: Config{
			TokenSecret:   "test-secret",
			TokenDuration: time.Hour,
			TokenIssuer:   "test-issuer",
			TokenAudience: "test-audience",
		},
		store: testStore{
			admin: Admin{
				Username:     "admin",
				PasswordHash: hashedPassword,
			},
		},
		logger: slog.Default(),
	}

	tests := []struct {
		name    string
		creds   Credentials
		wantErr error
	}{
		{
			name: "Valid credentials",
			creds: Credentials{
				Username: "admin",
				Password: "correct-password",
			},
			wantErr: nil,
		},
		{
			name: "Wrong username",
			creds: Credentials{
				Username: "wrong-user",
				Password: "correct-password",
			},
			wantErr: ErrInvalidCredentials,
		},
		{
			name: "Wrong password",
			creds: Credentials{
				Username: "admin",
				Password: "wrong-password",
			},
			wantErr: ErrInvalidCredentials,
		},
		{
			name:    "Empty credentials",
			creds:   Credentials{},
			wantErr: ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := auth.ValidateCredentials(context.Background(), tt.creds)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("ValidateCredentials() error = %v, wantErr %v", err, tt.wantErr)
				}
				if token != nil {
					t.Errorf("ValidateCredentials() token = %v, want empty on error", token)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateCredentials() unexpected error: %v", err)
				return
			}

			// Verify token is valid and contains correct claims
			validatedToken, err := auth.ValidateToken(token.String())
			if err != nil {
				t.Errorf("Failed to validate generated token: %v", err)
				return
			}
			if validatedToken.Role != RoleGM {
				t.Errorf("ValidateCredentials() role = %v, want %v", validatedToken.Role, RoleGM)
			}
		})
	}
}
