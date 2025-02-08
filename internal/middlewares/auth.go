package middlewares

import (
	"net/http"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

func AuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("auth_token")
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			claims, err := auth.ValidateToken(cookie.Value, jwtSecret)
			if err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			if username, ok := claims["login"].(string); !ok || username != "terrabitz" {
				http.Error(w, "Unauthorized user", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
