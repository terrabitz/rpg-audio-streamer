package middlewares

import (
	"net/http"
)

type CorsConfig struct {
	AllowedOrigins string
}

func CORSMiddleware(cfg CorsConfig) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.AllowedOrigins != "" {
				w.Header().Set("Access-Control-Allow-Origin", cfg.AllowedOrigins)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Max-Age", "3600")
			}

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}
