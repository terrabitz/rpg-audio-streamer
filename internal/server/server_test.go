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

	"github.com/terrabitz/rpg-audio-streamer/internal/middlewares"
	"github.com/terrabitz/rpg-audio-streamer/internal/types"
)

type testServer struct {
	*Server
	tempDir string
}

func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	// Create temp directory for uploads
	tempDir, err := os.MkdirTemp("", "rpg-audio-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create test server
	srv, err := New(Config{
		Port:      8080,
		UploadDir: tempDir,
		CORS:      middlewares.CorsConfig{},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
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

	// Create test request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	// Test handler
	ts.uploadFile(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status OK; got %v", rec.Code)
	}

	// Verify file was created
	uploadedFile := filepath.Join(ts.tempDir, "test.mp3")
	if _, err := os.Stat(uploadedFile); os.IsNotExist(err) {
		t.Error("uploaded file doesn't exist")
	}
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

	// Test handler
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	rec := httptest.NewRecorder()

	ts.listFiles(rec, req)

	// Verify response
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
}

func TestDeleteFile(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)

	// Create test file
	testFile := filepath.Join(ts.tempDir, "test.mp3")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test handler
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/test.mp3", nil)
	rec := httptest.NewRecorder()
	req.SetPathValue("fileName", "test.mp3")

	ts.handleFileDelete(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status OK; got %v", rec.Code)
	}

	// Verify file was deleted
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("file still exists after deletion")
	}

	// Test deleting non-existent file
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/files/nonexistent.mp3", nil)
	rec = httptest.NewRecorder()
	req.SetPathValue("fileName", "nonexistent.mp3")

	ts.handleFileDelete(rec, req)
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

	// Test handler
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/test.mp3", nil)
	rec := httptest.NewRecorder()
	req.SetPathValue("fileName", "test.mp3")

	ts.streamFile(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status OK; got %v", rec.Code)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "audio/mp3" {
		t.Errorf("expected Content-Type audio/mp3; got %s", ct)
	}

	if body := rec.Body.String(); body != content {
		t.Errorf("expected body %q; got %q", content, body)
	}

	// Test streaming non-existent file
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stream/nonexistent.mp3", nil)
	rec = httptest.NewRecorder()
	req.SetPathValue("fileName", "nonexistent.mp3")

	ts.streamFile(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status NotFound; got %v", rec.Code)
	}
}
