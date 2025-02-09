package auth

import "time"

type Config struct {
	TokenSecret   []byte
	TokenDuration time.Duration
	TokenIssuer   string
	TokenAudience string
}
