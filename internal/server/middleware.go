package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
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

	// This is a hack for the WS endpoint, which can't use an Authorization header.
	if tokenParam := r.FormValue("token"); tokenParam != "" {
		return s.playerToken(r, tokenParam)
	}

	authHeaderToken, err := getAuthorizationBearerToken(r)
	if err == nil {
		return s.playerToken(r, authHeaderToken)
	}

	return nil, fmt.Errorf("No valid authentication method found")
}

func (s *Server) playerToken(r *http.Request, invite string) (*auth.Token, error) {
	if invite == "" {
		return nil, auth.ErrInvalidJoinToken
	}
	var table Table
	var err error
	if strings.EqualFold(invite, s.auth.GetJoinToken()) {
		table, err = s.store.GetTable(r.Context(), DefaultTableID)
	} else {
		table, err = s.store.GetTableByInvite(r.Context(), invite)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid table invite: %w", err)
	}
	if table.ID == uuid.Nil {
		return nil, auth.ErrInvalidJoinToken
	}
	return &auth.Token{Role: auth.RolePlayer, TableID: table.ID.String()}, nil
}

func getAuthorizationBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("Missing Authorization header")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", fmt.Errorf("Invalid bearer token format")
	}

	return strings.TrimPrefix(authHeader, prefix), nil
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
