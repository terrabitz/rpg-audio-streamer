package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Service struct {
	config GitHubConfig
	logger *slog.Logger
}

func NewService(config GitHubConfig, logger *slog.Logger) *Service {
	return &Service{
		config: config,
		logger: logger,
	}
}

func (s *Service) HandleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	redirectURL := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&scope=user:email",
		s.config.ClientID)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (s *Service) HandleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code not found", http.StatusBadRequest)
		return
	}

	token, err := ExchangeCodeForToken(code, s.config)
	if err != nil {
		s.logger.Error("failed to exchange code for token", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	s.setAuthCookie(w, token)
	http.Redirect(w, r, "http://localhost:5173/", http.StatusTemporaryRedirect)
}

func (s *Service) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	isAuthenticated := false
	isAuthorized := false
	var userData *GitHubUser

	if claims, err := s.ValidateRequest(r); err == nil {
		isAuthenticated = true
		userData = claims
		isAuthorized = s.IsAuthorized(r)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": isAuthenticated,
		"authorized":    isAuthorized,
		"user":          userData,
	})
}

func (s *Service) HandleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearAuthCookie(w)
	w.WriteHeader(http.StatusOK)
}

func (s *Service) ValidateRequest(r *http.Request) (*GitHubUser, error) {
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		return nil, fmt.Errorf("no auth cookie: %w", err)
	}

	claims, err := ValidateToken(cookie.Value, s.config.JWTSecret)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	var GitHubUser GitHubUser
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal claims: %w", err)
	}

	if err := json.Unmarshal(claimsBytes, &GitHubUser); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
	}

	return &GitHubUser, nil
}

func (s *Service) IsAuthorized(r *http.Request) bool {
	claims, err := s.ValidateRequest(r)
	if err != nil {
		return false
	}

	return claims.Login == "terrabitz"
}

func (s *Service) setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   int((24 * time.Hour).Seconds()), // 24 hours
	})
}

func (s *Service) clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   -1,
	})
}
