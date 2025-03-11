package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

func TestAuthMiddleware(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	mockHandler := func(w http.ResponseWriter, r *http.Request, token *auth.Token) {
		w.WriteHeader(http.StatusOK)
	}

	tests := []struct {
		name       string
		cookie     *http.Cookie
		wantStatus int
	}{
		{
			name: "valid token",
			cookie: &http.Cookie{
				Name:  authCookieName,
				Value: ts.auth.(*mockAuth).token.String(),
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "no cookie",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "invalid token",
			cookie: &http.Cookie{
				Name:  authCookieName,
				Value: "invalid-token",
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rec := httptest.NewRecorder()

			ts.authMiddleware(mockHandler)(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("want status %v; got %v", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestGMOnlyMiddleware(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	mockHandler := func(w http.ResponseWriter, r *http.Request, token *auth.Token) {
		w.WriteHeader(http.StatusOK)
	}

	tests := []struct {
		name       string
		token      *auth.Token
		wantStatus int
	}{
		{
			name:       "GM role",
			token:      &auth.Token{Role: auth.RoleGM},
			wantStatus: http.StatusOK,
		},
		{
			name:       "Player role",
			token:      &auth.Token{Role: auth.RolePlayer},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.auth.(*mockAuth).token = tt.token
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			addAuthCookie(req, tt.token.String())
			rec := httptest.NewRecorder()

			ts.gmOnlyMiddleware(mockHandler)(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("want status %v; got %v", tt.wantStatus, rec.Code)
			}
		})
	}
}
