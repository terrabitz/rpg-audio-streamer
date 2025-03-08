package server

import (
	"net/http"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

type AuthedHandlerFunc func(http.ResponseWriter, *http.Request, *auth.Token)

func (s *Server) authMiddleware(next AuthedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
