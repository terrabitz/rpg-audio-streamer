package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type GitHubConfig struct {
	ClientID     string
	ClientSecret string
	JWTSecret    string
}

type GitHubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func ExchangeCodeForToken(code string, config GitHubConfig) (string, error) {
	// Exchange code for GitHub access token
	tokenURL := fmt.Sprintf("https://github.com/login/oauth/access_token"+
		"?client_id=%s&client_secret=%s&code=%s",
		config.ClientID, config.ClientSecret, code)

	req, _ := http.NewRequest("POST", tokenURL, nil)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode access token response: %w", err)
	}

	// Get user info using the access token
	userReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	userReq.Header.Set("Authorization", "Bearer "+result.AccessToken)
	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		return "", fmt.Errorf("failed to get user info: %w", err)
	}
	defer userResp.Body.Close()

	var user GitHubUser
	if err := json.NewDecoder(userResp.Body).Decode(&user); err != nil {
		return "", fmt.Errorf("failed to decode user info: %w", err)
	}

	// Create JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":        user.ID,
		"name":       user.Name,
		"login":      user.Login,
		"email":      user.Email,
		"avatar_url": user.AvatarURL,
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(config.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to create JWT: %w", err)
	}

	return tokenString, nil
}
