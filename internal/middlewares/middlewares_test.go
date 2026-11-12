package middlewares

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	for _, tc := range []struct {
		name, allowed, origin, method string
		dev, permit, called           bool
	}{
		{"allowed origin", "https://gm.test,https://player.test", "https://player.test", "GET", false, true, true},
		{"rejected origin", "https://gm.test", "https://attacker.test", "GET", false, false, true},
		{"wildcard", "*", "https://player.test", "GET", false, true, true},
		{"development", "https://gm.test", "http://localhost:5173", "GET", true, true, true},
		{"no origin", "*", "", "GET", false, false, true},
		{"preflight", "*", "https://player.test", "OPTIONS", false, true, false},
		{"rejected preflight", "https://gm.test", "https://attacker.test", "OPTIONS", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := CORSMiddleware(CorsConfig{AllowedOrigins: tc.allowed, DevMode: tc.dev})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusAccepted)
			}))
			req := httptest.NewRequest(tc.method, "/", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if called != tc.called {
				t.Errorf("handler called = %v; want %v", called, tc.called)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); (got == tc.origin && got != "") != tc.permit {
				t.Errorf("allow origin = %q; permit = %v", got, tc.permit)
			}
			if tc.permit && rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
				t.Error("allowed credentialed request missing credentials header")
			}
			if tc.method == "OPTIONS" && rec.Code != http.StatusOK {
				t.Errorf("preflight status = %d", rec.Code)
			}
		})
	}
	if !isOriginAllowed("https://player.test", nil, false) {
		t.Error("empty allow list should permit origins")
	}
}

func TestRequestLogging(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			handler := LoggerMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if status != http.StatusOK {
					w.WriteHeader(status)
				}
				w.Write([]byte("response"))
			}))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/files", nil))
			var entry struct {
				Method, Path string
				Status       int
			}
			if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			if rec.Code != status || entry.Status != status || entry.Method != "GET" || entry.Path != "/api/v1/files" {
				t.Fatalf("response = %d; log = %+v", rec.Code, entry)
			}
		})
	}
}
