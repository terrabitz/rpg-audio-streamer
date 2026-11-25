package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request, token *auth.Token) {
	scoped := *token
	if token.Role == auth.RoleGM {
		id := domain.DefaultTableID
		if value := r.URL.Query().Get("tableID"); value != "" {
			table, err := s.tables.Find(r.Context(), value)
			if err != nil {
				s.writeError(w, err)
				return
			}
			id = table.ID
		}
		scoped.TableID = id.String()
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	s.hub.Register(conn, &scoped)
}

type authStatusResponse struct {
	Authenticated bool      `json:"authenticated"`
	Role          auth.Role `json:"role,omitempty"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token, err := s.getToken(r)
	if err != nil {
		respondJSON(w, http.StatusOK, authStatusResponse{Authenticated: false})
		return
	}

	respondJSON(w, http.StatusOK, authStatusResponse{
		Authenticated: true,
		Role:          token.Role,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Always clear the cookie, regardless of authentication method
	s.clearCookie(w, authCookieName)
	w.WriteHeader(http.StatusOK)
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("failed to decode login request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	creds := auth.Credentials{
		Username: req.Username,
		Password: req.Password,
	}

	token, err := s.auth.ValidateCredentials(r.Context(), creds)
	if err != nil {
		if !errors.Is(err, auth.ErrInvalidCredentials) {
			s.logger.Error("unable to authenticate", "error", err)
			respondJSON(w, http.StatusInternalServerError, loginResponse{
				Success: false,
				Error:   "Unable to sign in",
			})
			return
		}
		s.logger.Info("login failed", "username", req.Username)
		respondJSON(w, http.StatusUnauthorized, loginResponse{
			Success: false,
			Error:   "Invalid credentials",
		})
		return
	}

	// Set auth cookie
	s.writeCookie(w, authCookieName, token.String(), token.ExpiresAt)

	respondJSON(w, http.StatusOK, loginResponse{
		Success: true,
	})
}
