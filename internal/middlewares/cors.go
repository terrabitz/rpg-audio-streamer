package middlewares

import (
	"net/http"
	"strings"
)

type CorsConfig struct {
	AllowedOrigins string
	DevMode        bool
}

func isOriginAllowed(origin string, allowedOrigins []string, devMode bool) bool {
	if devMode || len(allowedOrigins) == 0 {
		return true // Allow all origins in dev mode or if none specified
	}
	for _, allowed := range allowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

func CORSMiddleware(cfg CorsConfig) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowedOrigins := strings.Split(cfg.AllowedOrigins, ",")
			if origin != "" && isOriginAllowed(origin, allowedOrigins, cfg.DevMode) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "*")
				w.Header().Set("Access-Control-Max-Age", "3600")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			handler.ServeHTTP(w, r)
		})
	}
}
