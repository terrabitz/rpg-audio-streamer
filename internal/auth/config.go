package auth

import "time"

type Config struct {
	RootUsername   string
	HashedPassword string
	TokenSecret    string
	TokenDuration  time.Duration
	TokenIssuer    string
	TokenAudience  string
}
