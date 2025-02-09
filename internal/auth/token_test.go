package auth

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestTokenGeneration(t *testing.T) {
	auth := New(Config{
		TokenSecret:   []byte("test-secret"),
		TokenDuration: time.Hour,
		TokenIssuer:   "test-issuer",
		TokenAudience: "test-audience",
	}, slog.Default())

	tests := []struct {
		name    string
		subject string
		wantErr bool
	}{
		{
			name:    "Valid token",
			subject: "test-user",
			wantErr: false,
		},
		{
			name:    "Empty subject",
			subject: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := auth.GenerateToken(tt.subject)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				claims, err := auth.ValidateToken(token)
				if err != nil {
					t.Errorf("ValidateToken() error = %v", err)
					return
				}
				if claims.Subject != tt.subject {
					t.Errorf("ValidateToken() subject = %v, want %v", claims.Subject, tt.subject)
				}
			}
		})
	}
}

func TestTokenValidation(t *testing.T) {
	testSecret := []byte("test-secret")
	auth := New(Config{
		TokenSecret:   testSecret,
		TokenDuration: time.Hour,
		TokenIssuer:   "test-issuer",
		TokenAudience: "test-audience",
	}, slog.Default())

	tests := []struct {
		name    string
		setup   func() string
		wantErr error
		wantSub string
	}{
		{
			name: "Valid token",
			setup: func() string {
				token, _ := auth.GenerateToken("test-user")
				return token
			},
			wantErr: nil,
			wantSub: "test-user",
		},
		{
			name: "Expired token",
			setup: func() string {
				claims := Claims{
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
						IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
						NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
						Issuer:    "test-issuer",
						Subject:   "test-user",
						Audience:  jwt.ClaimStrings{"test-audience"},
					},
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString(testSecret)
				return tokenString
			},
			wantErr: jwt.ErrTokenExpired,
		},
		{
			name: "Future token (not valid yet)",
			setup: func() string {
				claims := Claims{
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
						IssuedAt:  jwt.NewNumericDate(time.Now()),
						NotBefore: jwt.NewNumericDate(time.Now().Add(time.Hour)),
						Issuer:    "test-issuer",
						Subject:   "test-user",
						Audience:  jwt.ClaimStrings{"test-audience"},
					},
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString(testSecret)
				return tokenString
			},
			wantErr: jwt.ErrTokenNotValidYet,
		},
		{
			name: "Wrong issuer",
			setup: func() string {
				claims := Claims{
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
						IssuedAt:  jwt.NewNumericDate(time.Now()),
						NotBefore: jwt.NewNumericDate(time.Now()),
						Issuer:    "wrong-issuer",
						Subject:   "test-user",
						Audience:  jwt.ClaimStrings{"test-audience"},
					},
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString(testSecret)
				return tokenString
			},
			wantErr: jwt.ErrTokenInvalidIssuer,
		},
		{
			name: "Wrong audience",
			setup: func() string {
				claims := Claims{
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
						IssuedAt:  jwt.NewNumericDate(time.Now()),
						NotBefore: jwt.NewNumericDate(time.Now()),
						Issuer:    "test-issuer",
						Subject:   "test-user",
						Audience:  jwt.ClaimStrings{"wrong-audience"},
					},
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString(testSecret)
				return tokenString
			},
			wantErr: jwt.ErrTokenInvalidAudience,
		},
		{
			name: "Wrong signing method",
			setup: func() string {
				claims := Claims{
					RegisteredClaims: jwt.RegisteredClaims{
						ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
						Issuer:    "test-issuer",
						Subject:   "test-user",
						Audience:  jwt.ClaimStrings{"test-audience"},
					},
				}
				token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
				tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
				return tokenString
			},
			wantErr: jwt.ErrTokenSignatureInvalid,
		},
		{
			name: "Tampered signature",
			setup: func() string {
				token, _ := auth.GenerateToken("test-user")
				return token[:len(token)-2] + "00" // Modify last two chars
			},
			wantErr: jwt.ErrTokenSignatureInvalid,
		},
		{
			name: "Invalid token format",
			setup: func() string {
				return "invalid.token.format"
			},
			wantErr: jwt.ErrTokenMalformed,
		},
		{
			name: "Empty token",
			setup: func() string {
				return ""
			},
			wantErr: jwt.ErrTokenMalformed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.setup()
			claims, err := auth.ValidateToken(token)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("ValidateToken() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateToken() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateToken() unexpected error = %v", err)
				return
			}

			if claims.Subject != tt.wantSub {
				t.Errorf("ValidateToken() subject = %v, want %v", claims.Subject, tt.wantSub)
			}
		})
	}
}
