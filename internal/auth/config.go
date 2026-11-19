package auth

import "time"

type Config struct {
	TokenSecret   string
	TokenDuration time.Duration
	TokenIssuer   string
	TokenAudience string
}
