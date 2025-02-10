package server

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

type contextKey string

const authTokenKey contextKey = "authToken"

// SetAuthToken stores the auth token in the context
func SetAuthToken(ctx context.Context, token *auth.Token) context.Context {
	return context.WithValue(ctx, authTokenKey, token)
}

// GetAuthToken retrieves the auth token from the context
func GetAuthToken(ctx context.Context) (*auth.Token, bool) {
	token, ok := ctx.Value(authTokenKey).(*auth.Token)
	return token, ok
}
