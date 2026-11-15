package server

import (
	"fmt"
	"net/http"

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
	cookie, err := readCookie(r, authCookieName)
	if err == nil {
		return s.auth.ValidateToken(cookie)
	}
	return nil, fmt.Errorf("No valid GM authentication cookie found")
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

func (s *Server) publicMiddleware(next AuthedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next(w, r, &auth.Token{Role: auth.RolePlayer})
	}
}

func (s *Server) tableAccessMiddleware(next AuthedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table, err := s.store.GetTableByExternalID(r.Context(), r.PathValue("tableID"))
		if err != nil {
			s.tableError(w, err)
			return
		}
		token, err := s.getToken(r)
		if err != nil || token.Role != auth.RoleGM {
			token = &auth.Token{Role: auth.RolePlayer, TableID: table.ID.String()}
		}
		next(w, r, token)
	}
}

func (s *Server) tableWebSocketMiddleware(next AuthedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := s.getToken(r)
		if err == nil && token.Role == auth.RoleGM {
			next(w, r, token)
			return
		}
		table, err := s.store.GetTableByExternalID(r.Context(), r.URL.Query().Get("tableID"))
		if err != nil {
			s.tableError(w, err)
			return
		}
		next(w, r, &auth.Token{Role: auth.RolePlayer, TableID: table.ID.String()})
	}
}
