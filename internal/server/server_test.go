package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/middlewares"
	"github.com/terrabitz/rpg-audio-streamer/internal/types"
)

type testServer struct {
	*Server
	tempDir string
}

type mockAuth struct {
	validUser     string
	validPassword string
	token         *auth.Token
	joinToken     string
}

// Verify mockAuth implements Authenticator interface
var _ Authenticator = (*mockAuth)(nil)

func (m *mockAuth) ValidateCredentials(creds auth.Credentials) (*auth.Token, error) {
	if creds.Username == m.validUser && creds.Password == m.validPassword {
		return m.token, nil
	}
	return nil, auth.ErrInvalidCredentials
}

func (m *mockAuth) ValidateToken(tokenStr string) (*auth.Token, error) {
	if tokenStr == m.token.String() {
		return m.token, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

func (m *mockAuth) GetJoinToken() string {
	return m.joinToken
}

func (m *mockAuth) ValidateJoinToken(joinToken string) (*auth.Token, error) {
	if joinToken == m.joinToken {
		return m.token, nil
	}
	return nil, auth.ErrInvalidJoinToken
}

func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	// Create temp directory for uploads
	tempDir, err := os.MkdirTemp("", "rpg-audio-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	mockAuth := &mockAuth{
		validUser:     "testuser",
		validPassword: "testpass",
		token:         &auth.Token{Role: auth.RoleGM},
		joinToken:     "valid-join-token",
	}

	// Create test server
	srv, err := New(Config{
		Port:      8080,
		UploadDir: tempDir,
		CORS:      middlewares.CorsConfig{},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, mockAuth)
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	return &testServer{
		Server:  srv,
		tempDir: tempDir,
	}
}

func (ts *testServer) cleanup(t *testing.T) {
	t.Helper()
	if err := os.RemoveAll(ts.tempDir); err != nil {
		t.Errorf("failed to cleanup temp dir: %v", err)
	}
}

func addAuthCookie(req *http.Request, token string) {
	req.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: token,
	})
}

func TestUploadFile(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	// Create test file content
	content := []byte("test audio content")
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("files", "test.mp3")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write(content)
	writer.Close()

	t.Run("with valid auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/files", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		addAuthCookie(req, ts.auth.(*mockAuth).token.String())
		rec := httptest.NewRecorder()

		ts.authMiddleware(ts.uploadFile)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status OK; got %v", rec.Code)
		}
	})

	t.Run("without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/files", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		ts.authMiddleware(ts.uploadFile)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status Unauthorized; got %v", rec.Code)
		}
	})
}

func TestListFiles(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	// Create some test files
	testFiles := []struct {
		name    string
		content string
	}{
		{"test1.mp3", "test content 1"},
		{"test2.mp3", "test content 2"},
	}

	for _, tf := range testFiles {
		path := filepath.Join(ts.tempDir, tf.name)
		if err := os.WriteFile(path, []byte(tf.content), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	t.Run("with valid auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
		addAuthCookie(req, ts.auth.(*mockAuth).token.String())
		rec := httptest.NewRecorder()

		ts.authMiddleware(ts.listFiles)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status OK; got %v", rec.Code)
		}

		var files []types.FileInfo
		if err := json.NewDecoder(rec.Body).Decode(&files); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(files) != len(testFiles) {
			t.Errorf("expected %d files; got %d", len(testFiles), len(files))
		}

		// Verify file names
		fileNames := make([]string, len(files))
		for i, f := range files {
			fileNames[i] = f.Name
		}
		for _, tf := range testFiles {
			found := false
			for _, name := range fileNames {
				if name == tf.name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("file %s not found in response", tf.name)
			}
		}
	})

	t.Run("without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
		rec := httptest.NewRecorder()

		ts.authMiddleware(ts.listFiles)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status Unauthorized; got %v", rec.Code)
		}
	})
}

func TestDeleteFile(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	// Create test file
	testFile := filepath.Join(ts.tempDir, "test.mp3")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	t.Run("with valid auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/test.mp3", nil)
		addAuthCookie(req, ts.auth.(*mockAuth).token.String())
		rec := httptest.NewRecorder()
		req.SetPathValue("fileName", "test.mp3")

		ts.authMiddleware(ts.handleFileDelete)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status OK; got %v", rec.Code)
		}

		// Verify file was deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("file still exists after deletion")
		}
	})

	t.Run("without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/test.mp3", nil)
		rec := httptest.NewRecorder()
		req.SetPathValue("fileName", "test.mp3")

		ts.authMiddleware(ts.handleFileDelete)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status Unauthorized; got %v", rec.Code)
		}
	})

	// Test deleting non-existent file
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/nonexistent.mp3", nil)
	addAuthCookie(req, ts.auth.(*mockAuth).token.String())
	rec := httptest.NewRecorder()
	req.SetPathValue("fileName", "nonexistent.mp3")

	ts.authMiddleware(ts.handleFileDelete)(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status NotFound; got %v", rec.Code)
	}
}

func TestStreamFile(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	// Create test file
	content := "test audio content"
	testFile := filepath.Join(ts.tempDir, "test.mp3")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	t.Run("with valid auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/test.mp3", nil)
		addAuthCookie(req, ts.auth.(*mockAuth).token.String())
		rec := httptest.NewRecorder()
		req.SetPathValue("fileName", "test.mp3")

		ts.authMiddleware(ts.streamFile)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status OK; got %v", rec.Code)
		}

		if ct := rec.Header().Get("Content-Type"); ct != "audio/mp3" {
			t.Errorf("expected Content-Type audio/mp3; got %s", ct)
		}

		if body := rec.Body.String(); body != content {
			t.Errorf("expected body %q; got %q", content, body)
		}
	})

	t.Run("without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/test.mp3", nil)
		rec := httptest.NewRecorder()
		req.SetPathValue("fileName", "test.mp3")

		ts.authMiddleware(ts.streamFile)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status Unauthorized; got %v", rec.Code)
		}
	})

	// Test streaming non-existent file
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/nonexistent.mp3", nil)
	addAuthCookie(req, ts.auth.(*mockAuth).token.String())
	rec := httptest.NewRecorder()
	req.SetPathValue("fileName", "nonexistent.mp3")

	ts.authMiddleware(ts.streamFile)(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status NotFound; got %v", rec.Code)
	}
}

func TestHandleLogin(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	tests := []struct {
		name            string
		credentials     loginRequest
		expectedCode    int
		expectedError   string
		checkAuthCookie bool
	}{
		{
			name: "successful login",
			credentials: loginRequest{
				Username: "testuser",
				Password: "testpass",
			},
			expectedCode:    http.StatusOK,
			checkAuthCookie: true,
		},
		{
			name: "invalid credentials",
			credentials: loginRequest{
				Username: "wronguser",
				Password: "wrongpass",
			},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Invalid credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.credentials)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			ts.handleLogin(rec, req)

			if rec.Code != tt.expectedCode {
				t.Errorf("expected status %v; got %v", tt.expectedCode, rec.Code)
			}

			var resp loginResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.expectedError != "" && resp.Error != tt.expectedError {
				t.Errorf("expected error %q; got %q", tt.expectedError, resp.Error)
			}

			if tt.checkAuthCookie {
				cookies := rec.Result().Cookies()
				var authCookie *http.Cookie
				for _, cookie := range cookies {
					if cookie.Name == authCookieName {
						authCookie = cookie
						break
					}
				}
				if authCookie == nil {
					t.Error("auth cookie not set")
				}
				if !authCookie.HttpOnly {
					t.Error("auth cookie should be HTTP-only")
				}
				if !authCookie.Secure {
					t.Error("auth cookie should be secure")
				}
			}
		})
	}

	t.Run("invalid method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/login", nil)
		rec := httptest.NewRecorder()

		ts.handleLogin(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status MethodNotAllowed; got %v", rec.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		ts.handleLogin(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status BadRequest; got %v", rec.Code)
		}
	})
}

func TestHandleAuthStatus(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	t.Run("with valid auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
		addAuthCookie(req, ts.auth.(*mockAuth).token.String())
		rec := httptest.NewRecorder()

		ts.handleAuthStatus(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status OK; got %v", rec.Code)
		}

		var resp authStatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !resp.Authenticated {
			t.Error("expected authenticated true; got false")
		}

		if resp.Role != auth.RoleGM {
			t.Errorf("expected role %v; got %v", auth.RoleGM, resp.Role)
		}
	})

	t.Run("without auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/status", nil)
		rec := httptest.NewRecorder()

		ts.handleAuthStatus(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status OK; got %v", rec.Code)
		}

		var resp authStatusResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Authenticated {
			t.Error("expected authenticated false; got true")
		}
	})

	t.Run("invalid method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/status", nil)
		rec := httptest.NewRecorder()

		ts.handleAuthStatus(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status MethodNotAllowed; got %v", rec.Code)
		}
	})
}

func TestHandleLogout(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	t.Run("successful logout", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
		rec := httptest.NewRecorder()

		ts.handleLogout(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status OK; got %v", rec.Code)
		}

		cookies := rec.Result().Cookies()
		var authCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == authCookieName {
				authCookie = cookie
				break
			}
		}
		if authCookie == nil {
			t.Error("auth cookie not cleared")
		}
		if !authCookie.Expires.IsZero() && authCookie.Expires.After(time.Now()) {
			t.Error("auth cookie not expired")
		}
	})

	t.Run("invalid method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/logout", nil)
		rec := httptest.NewRecorder()

		ts.handleLogout(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status MethodNotAllowed; got %v", rec.Code)
		}
	})
}

func TestHandleJoinToken(t *testing.T) {
	t.Run("with GM auth", func(t *testing.T) {
		ts := setupTestServer(t)
		defer ts.cleanup(t)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/join-token", nil)
		addAuthCookie(req, ts.auth.(*mockAuth).token.String())
		rec := httptest.NewRecorder()

		ts.gmOnlyMiddleware(ts.handleJoinToken)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status OK; got %v", rec.Code)
		}

		var resp joinTokenResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Token != ts.auth.(*mockAuth).joinToken {
			t.Errorf("expected token %q; got %q", ts.auth.(*mockAuth).joinToken, resp.Token)
		}
	})

	t.Run("without GM auth", func(t *testing.T) {
		ts := setupTestServer(t)
		defer ts.cleanup(t)

		// Create a non-GM token
		playerToken := &auth.Token{Role: auth.RolePlayer}
		ts.auth.(*mockAuth).token = playerToken

		req := httptest.NewRequest(http.MethodGet, "/api/v1/join-token", nil)
		addAuthCookie(req, playerToken.String())
		rec := httptest.NewRecorder()

		ts.gmOnlyMiddleware(ts.handleJoinToken)(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status Forbidden; got %v", rec.Code)
		}
	})

	t.Run("invalid method", func(t *testing.T) {
		ts := setupTestServer(t)
		defer ts.cleanup(t)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/join-token", nil)
		addAuthCookie(req, ts.auth.(*mockAuth).token.String())
		rec := httptest.NewRecorder()

		ts.gmOnlyMiddleware(ts.handleJoinToken)(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status MethodNotAllowed; got %v", rec.Code)
		}
	})
}

func TestHandleJoin(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	tests := []struct {
		name           string
		joinToken      string
		expectedCode   int
		expectedError  string
		checkAuthToken bool
	}{
		{
			name:           "successful join",
			joinToken:      "valid-join-token",
			expectedCode:   http.StatusOK,
			checkAuthToken: true,
		},
		{
			name:          "invalid join token",
			joinToken:     "invalid-token",
			expectedCode:  http.StatusUnauthorized,
			expectedError: "Invalid join token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := joinRequest{
				Token: tt.joinToken,
			}
			body, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("failed to marshal request body: %v", err)
			}

			httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/join", bytes.NewReader(body))
			httpReq.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			ts.handleJoin(rec, httpReq)

			if rec.Code != tt.expectedCode {
				t.Errorf("expected status %v; got %v", tt.expectedCode, rec.Code)
			}

			var resp loginResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.expectedError != "" && resp.Error != tt.expectedError {
				t.Errorf("expected error %q; got %q", tt.expectedError, resp.Error)
			}

			if tt.checkAuthToken {
				cookies := rec.Result().Cookies()
				var authCookie *http.Cookie
				for _, cookie := range cookies {
					if cookie.Name == authCookieName {
						authCookie = cookie
						break
					}
				}
				if authCookie == nil {
					t.Error("auth cookie not set")
				}
			}
		})
	}

	t.Run("invalid method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/join", nil)
		rec := httptest.NewRecorder()

		ts.handleJoin(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status MethodNotAllowed; got %v", rec.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/join", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		ts.handleJoin(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status BadRequest; got %v", rec.Code)
		}
	})
}
