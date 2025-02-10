package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Role string

const (
	RoleGM     Role = "gm"
	RolePlayer Role = "player"
)

type Claims struct {
	Role Role `json:"role"`
	jwt.RegisteredClaims
}

type Token struct {
	token     string
	ExpiresAt time.Time
	Role      Role
}

func (a Token) String() string {
	return a.token
}

func (a *Auth) NewToken(subject string, role Role) (*Token, error) {
	now := time.Now()
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.cfg.TokenIssuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{a.cfg.TokenAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(a.cfg.TokenDuration)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(a.cfg.TokenSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	return &Token{
		token:     signedToken,
		ExpiresAt: now.Add(a.cfg.TokenDuration),
		Role:      role,
	}, nil
}

func (a *Auth) ValidateToken(token string) (*Token, error) {
	options := []jwt.ParserOption{
		jwt.WithAudience(a.cfg.TokenAudience),
		jwt.WithIssuer(a.cfg.TokenIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"HS256"}),
	}

	parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.cfg.TokenSecret), nil
	}, options...)

	if err != nil {
		return nil, err
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok || !parsedToken.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}

	return &Token{
		token:     token,
		ExpiresAt: claims.ExpiresAt.Time,
		Role:      claims.Role,
	}, nil
}
