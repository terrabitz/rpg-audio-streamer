package auth

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidJoinToken   = errors.New("invalid join token")
)
