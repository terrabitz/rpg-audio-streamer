package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		status, err := s.auth.SetupStatus(r.Context())
		if err != nil {
			s.settingsError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, status)
	case http.MethodPost:
		var req struct {
			Code     string `json:"code"`
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !decodeSettingsRequest(w, r, &req) {
			return
		}
		if err := s.auth.Claim(r.Context(), req.Code, req.Username, req.Password); err != nil {
			s.settingsError(w, err)
			return
		}
		// Completing setup never returns the claim code or signs in another client.
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request, _ *auth.Token) {
	w.Header().Set("Cache-Control", "no-store")
	switch r.Method {
	case http.MethodGet:
		profile, err := s.auth.Profile(r.Context())
		if err != nil {
			s.settingsError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, profile)
	case http.MethodPatch:
		var req struct {
			CurrentPassword string `json:"currentPassword"`
			Password        string `json:"password"`
		}
		if !decodeSettingsRequest(w, r, &req) {
			return
		}
		if err := s.auth.ChangePassword(r.Context(), req.CurrentPassword, req.Password); err != nil {
			s.settingsError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func decodeSettingsRequest(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "Cross-site settings requests are not allowed", http.StatusForbidden)
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != r.Host {
			http.Error(w, "Cross-origin settings requests are not allowed", http.StatusForbidden)
			return false
		}
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<10))
	body = bytes.TrimSpace(body)
	if err != nil || len(body) == 0 || body[0] != '{' {
		http.Error(w, "Invalid settings request", http.StatusBadRequest)
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "Invalid settings request", http.StatusBadRequest)
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "Invalid settings request", http.StatusBadRequest)
		return false
	}
	return true
}

func (s *Server) settingsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrSetupComplete):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrInvalidCode), errors.Is(err, auth.ErrPassword):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, auth.ErrInvalidInput):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		s.logger.Error("settings request failed", "error", err)
		http.Error(w, "Unable to access account settings", http.StatusInternalServerError)
	}
}
