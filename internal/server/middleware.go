package server

import (
	"net/http"
	"strings"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

type AuthedHandlerFunc func(http.ResponseWriter, *http.Request, *auth.Token)

func (s *Server) authMiddleware(next AuthedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var joinToken string
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			const prefix = "Bearer "
			if strings.HasPrefix(authHeader, prefix) {
				joinToken = strings.TrimPrefix(authHeader, prefix)
			}
		}

		if joinToken != "" {
			token, err := s.auth.ValidateJoinToken(joinToken)
			if err != nil {
				http.Error(w, "Invalid join token", http.StatusUnauthorized)
				return
			}

			next(w, r, token)
			return
		}

		cookie, err := readCookie(r, authCookieName)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token, err := s.auth.ValidateToken(cookie)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next(w, r, token)
	}
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
