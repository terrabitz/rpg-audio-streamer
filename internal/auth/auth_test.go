package auth

import (
	"log/slog"
	"testing"
	"time"
)

func TestValidateCredentials(t *testing.T) {
	hashedPassword, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	auth := New(Config{
		RootUsername:   "admin",
		HashedPassword: hashedPassword,
		TokenSecret:    "test-secret",
		TokenDuration:  time.Hour,
		TokenIssuer:    "test-issuer",
		TokenAudience:  "test-audience",
		JoinToken:      "test-join-token",
	}, slog.Default())

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
			token, err := auth.ValidateCredentials(tt.creds)

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
			_, err = auth.ValidateToken(token.String())
			if err != nil {
				t.Errorf("Failed to validate generated token: %v", err)
				return
			}
		})
	}
}

func TestValidateJoinToken(t *testing.T) {
	auth := New(Config{
		TokenSecret:   "test-secret",
		TokenDuration: time.Hour,
		TokenIssuer:   "test-issuer",
		TokenAudience: "test-audience",
		JoinToken:     "test-join-token",
	}, slog.Default())

	tests := []struct {
		name    string
		token   string
		wantErr error
	}{
		{
			name:    "Valid join token",
			token:   "test-join-token",
			wantErr: nil,
		},
		{
			name:    "Invalid join token",
			token:   "wrong-token",
			wantErr: ErrInvalidJoinToken,
		},
		{
			name:    "Empty join token",
			token:   "",
			wantErr: ErrInvalidJoinToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := auth.ValidateJoinToken(tt.token)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("ValidateJoinToken() error = %v, wantErr %v", err, tt.wantErr)
				}
				if token != nil {
					t.Errorf("ValidateJoinToken() token = %v, want nil on error", token)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateJoinToken() unexpected error: %v", err)
				return
			}

			// Verify token is valid and contains correct claims
			validatedToken, err := auth.ValidateToken(token.String())
			if err != nil {
				t.Errorf("Failed to validate generated token: %v", err)
				return
			}

			if validatedToken.Role != RolePlayer {
				t.Errorf("ValidateJoinToken() role = %v, want %v", validatedToken.Role, RolePlayer)
			}
		})
	}
}
