package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

type AuthedHandlerFunc func(http.ResponseWriter, *http.Request, *auth.Token)

func (s *Server) authMiddleware(next AuthedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := s.getToken(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r, token)
	}
}

func (s *Server) getToken(r *http.Request) (*auth.Token, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		return s.getTokenFromAuthorizationHeader(authHeader)
	}

	return s.getTokenFromCookie(r)
}

func (s *Server) getTokenFromAuthorizationHeader(authHeader string) (*auth.Token, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return nil, fmt.Errorf("Invalid bearer token format")
	}

	joinToken := strings.TrimPrefix(authHeader, prefix)
	token, err := s.auth.ValidateJoinToken(joinToken)
	if err != nil {
		return nil, fmt.Errorf("Invalid join token")
	}

	return token, nil
}

func (s *Server) getTokenFromCookie(r *http.Request) (*auth.Token, error) {
	cookie, err := readCookie(r, authCookieName)
	if err != nil {
		return nil, fmt.Errorf("Unauthorized")
	}

	token, err := s.auth.ValidateToken(cookie)
	if err != nil {
		return nil, fmt.Errorf("Invalid auth token")
	}

	return token, nil
}

func (s *Server) gmOnlyMiddleware(next AuthedHandlerFunc) http.HandlerFunc {
	return s.authMiddleware(func(w http.ResponseWriter, r *http.Request, token *auth.Token) {
		if token.Role != auth.RoleGM {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r, token)
	})
}
