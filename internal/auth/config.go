package auth

import "time"

type Config struct {
	RootUsername   string
	HashedPassword string
	TokenSecret    []byte
	TokenDuration  time.Duration
	TokenIssuer    string
	TokenAudience  string
}
