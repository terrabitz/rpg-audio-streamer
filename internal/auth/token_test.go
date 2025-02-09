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
		TokenSecret:   "test-secret", // Changed from []byte to string
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
			authToken, err := auth.GenerateAuthToken(tt.subject)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateAuthToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				claims, err := auth.ValidateAuthToken(authToken)
				if err != nil {
					t.Errorf("ValidateAuthToken() error = %v", err)
					return
				}
				if claims.Subject != tt.subject {
					t.Errorf("ValidateAuthToken() subject = %v, want %v", claims.Subject, tt.subject)
				}
			}
		})
	}
}

func TestTokenValidation(t *testing.T) {
	testSecret := "test-secret" // Changed from []byte to string
	auth := New(Config{
		TokenSecret:   testSecret,
		TokenDuration: time.Hour,
		TokenIssuer:   "test-issuer",
		TokenAudience: "test-audience",
	}, slog.Default())

	tests := []struct {
		name    string
		setup   func() AuthToken
		wantErr error
		wantSub string
	}{
		{
			name: "Valid token",
			setup: func() AuthToken {
				token, _ := auth.GenerateAuthToken("test-user")
				return token
			},
			wantErr: nil,
			wantSub: "test-user",
		},
		{
			name: "Expired token",
			setup: func() AuthToken {
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
				tokenString, _ := token.SignedString([]byte(testSecret))
				return AuthToken{token: tokenString}
			},
			wantErr: jwt.ErrTokenExpired,
		},
		{
			name: "Future token (not valid yet)",
			setup: func() AuthToken {
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
				tokenString, _ := token.SignedString([]byte(testSecret))
				return AuthToken{token: tokenString}
			},
			wantErr: jwt.ErrTokenNotValidYet,
		},
		{
			name: "Wrong issuer",
			setup: func() AuthToken {
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
				tokenString, _ := token.SignedString([]byte(testSecret))
				return AuthToken{token: tokenString}
			},
			wantErr: jwt.ErrTokenInvalidIssuer,
		},
		{
			name: "Wrong audience",
			setup: func() AuthToken {
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
				tokenString, _ := token.SignedString([]byte(testSecret))
				return AuthToken{token: tokenString}
			},
			wantErr: jwt.ErrTokenInvalidAudience,
		},
		{
			name: "Wrong signing method",
			setup: func() AuthToken {
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
				return AuthToken{token: tokenString}
			},
			wantErr: jwt.ErrTokenSignatureInvalid,
		},
		{
			name: "Tampered signature",
			setup: func() AuthToken {
				token, _ := auth.GenerateAuthToken("test-user")
				tokenStr := token.String()
				return AuthToken{token: tokenStr[:len(tokenStr)-2] + "00"}
			},
			wantErr: jwt.ErrTokenSignatureInvalid,
		},
		{
			name: "Invalid token format",
			setup: func() AuthToken {
				return AuthToken{token: "invalid.token.format"}
			},
			wantErr: jwt.ErrTokenMalformed,
		},
		{
			name: "Empty token",
			setup: func() AuthToken {
				return AuthToken{token: ""}
			},
			wantErr: jwt.ErrTokenMalformed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authToken := tt.setup()
			claims, err := auth.ValidateAuthToken(authToken)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("ValidateAuthToken() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateAuthToken() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateAuthToken() unexpected error = %v", err)
				return
			}

			if claims.Subject != tt.wantSub {
				t.Errorf("ValidateAuthToken() subject = %v, want %v", claims.Subject, tt.wantSub)
			}
		})
	}
}
