package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func TestRoutedTrackLifecycle(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	id := uuid.New()
	typeID := uuid.MustParse("1ec000a2-a7c9-11ee-a0e5-0242ac120002")
	track := domain.Track{ID: id, Name: "Forest", TypeID: typeID, Path: filepath.Join(ts.tempDir, id.String())}
	if err := os.Mkdir(track.Path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(track.Path, "index.m3u8"), []byte("#EXTM3U\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := ts.store.SaveTrack(context.Background(), &track); err != nil {
		t.Fatal(err)
	}
	handler := ts.Handler()
	rec := routedRequest(handler, http.MethodPut, "/api/v1/files/"+id.String(), `{"name":"Rain","volume":35}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", rec.Code, rec.Body)
	}
	var updated domain.Track
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Rain" || updated.TypeID != typeID || updated.Path != track.Path || updated.Volume != 35 {
		t.Fatalf("partial update = %+v", updated)
	}
	rec = routedRequest(handler, http.MethodGet, "/api/v1/files", "", true)
	var listed []domain.Track
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil || len(listed) != 1 || listed[0].Name != "Rain" {
		t.Fatalf("list = %v, %v; status = %d", listed, err, rec.Code)
	}
	rec = routedRequest(handler, http.MethodGet, "/api/v1/stream/"+id.String()+"/index.m3u8", "", true)
	if rec.Code != http.StatusOK || rec.Body.String() != "#EXTM3U\n" {
		t.Fatalf("stream = %d: %s", rec.Code, rec.Body)
	}
	rec = routedRequest(handler, http.MethodDelete, "/api/v1/files/"+id.String(), "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete = %d: %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(track.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("HLS directory still exists: %v", err)
	}
	rec = routedRequest(handler, http.MethodGet, "/api/v1/stream/"+id.String()+"/index.m3u8", "", true)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted stream = %d", rec.Code)
	}
}

func TestRoutedValidation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	id := uuid.New().String()
	for _, tc := range []struct {
		name, method, path, body string
		authenticated            bool
		status                   int
	}{
		{"unauthenticated files", "GET", "/api/v1/files", "", false, 401},
		{"unauthenticated library stream", "GET", "/api/v1/stream/missing/index.m3u8", "", false, 401},
		{"invalid table stream", "GET", "/api/v1/tables/invalid/stream/missing/index.m3u8", "", false, 404},
		{"unsupported files method", "PATCH", "/api/v1/files", "", true, 405},
		{"unsupported file method", "GET", "/api/v1/files/" + id, "", true, 405},
		{"invalid update JSON", "PUT", "/api/v1/files/" + id, "{", true, 400},
		{"invalid update ID", "PUT", "/api/v1/files/invalid", "{}", true, 400},
		{"negative track volume", "PUT", "/api/v1/files/" + id, `{"volume":-1}`, true, 400},
		{"track volume over 100", "PUT", "/api/v1/files/" + id, `{"volume":101}`, true, 400},
		{"unknown update", "PUT", "/api/v1/files/" + id, `{"name":"new"}`, true, 404},
		{"invalid delete ID", "DELETE", "/api/v1/files/invalid", "", true, 400},
		{"missing delete", "DELETE", "/api/v1/files/" + id, "", true, 404},
		{"track types", "GET", "/api/v1/trackTypes", "", true, 200},
		{"unsupported track types method", "POST", "/api/v1/trackTypes", "", true, 405},
		{"public track types", "GET", "/api/v1/trackTypes", "", false, 200},
		{"removed join token endpoint", "GET", "/api/v1/joinToken", "", false, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := routedRequest(ts.Handler(), tc.method, tc.path, tc.body, tc.authenticated)
			if rec.Code != tc.status {
				t.Fatalf("status = %d; want %d: %s", rec.Code, tc.status, rec.Body)
			}
		})
	}
}

func TestStoreFailures(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	ts.store = failingStore{testStore: ts.store}
	for _, path := range []string{"/api/v1/files", "/api/v1/trackTypes", "/api/v1/files/" + uuid.NewString() + "/usage"} {
		rec := routedRequest(ts.Handler(), "GET", path, "", true)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s = %d", path, rec.Code)
		}
	}
	rec := routedRequest(ts.Handler(), "DELETE", "/api/v1/files/"+uuid.NewString(), "", true)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("delete failure = %d", rec.Code)
	}
	rec = routedRequest(ts.Handler(), "DELETE", "/api/v1/files/"+uuid.NewString()+"?expectedTableCount=0", "", true)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("conditional delete failure = %d", rec.Code)
	}
}

func TestUploadValidation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	for _, tc := range []struct {
		name, typeID string
		file         bool
		status       int
	}{
		{"missing file", "", false, 400},
		{"invalid type ID", "invalid", true, 400},
		{"unknown type", uuid.NewString(), true, 400},
		{"invalid audio", "1ec000a2-a7c9-11ee-a0e5-0242ac120002", true, 202},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			if tc.file {
				part, err := writer.CreateFormFile("files", "invalid.mp3")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(part, "not audio"); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.WriteField("typeID", tc.typeID); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("POST", "/api/v1/files", &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			addAuthCookie(req, "")
			rec := httptest.NewRecorder()
			ts.Handler().ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d; want %d: %s", rec.Code, tc.status, rec.Body)
			}
			if rec.Code == http.StatusAccepted {
				ts.waitForProcessing(t)
				tracks, err := ts.store.GetTracks(context.Background())
				if err != nil || len(tracks) != 1 || tracks[0].Status != "failed" || tracks[0].ProcessingError == "" {
					t.Fatalf("invalid audio processing state = %+v, %v", tracks, err)
				}
			}
		})
	}
}

func TestWebSocketRoute(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	ts.frontend = os.DirFS(t.TempDir())
	reg := &recordingWSRegisterer{tokens: make(chan *auth.Token, 1)}
	ts.hub = reg
	httpServer := httptest.NewServer(ts.Handler())
	defer httpServer.Close()
	header := http.Header{}
	header.Set("Cookie", authCookieName+"=")
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/api/v1/ws", header)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	token := <-reg.tokens
	if token.Role != auth.RoleGM || token.TableID != domain.DefaultTableID.String() {
		t.Fatalf("registered GM scope = %+v", token)
	}
	if ts.auth.(*mockAuth).token.TableID != "" {
		t.Fatal("websocket scope mutated the shared authentication token")
	}
	player, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(httpServer.URL, "http")+"/api/v1/ws?tableID="+testDefaultExternalID, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	token = <-reg.tokens
	if token.Role != auth.RolePlayer || token.TableID != domain.DefaultTableID.String() {
		t.Fatalf("player table scope = %+v", token)
	}
	rec := routedRequest(ts.Handler(), "GET", "/api/v1/ws", "", true)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-upgrade status = %d", rec.Code)
	}
	for _, tc := range []struct {
		tableID string
		status  int
	}{{"invalid", http.StatusNotFound}, {uuid.NewString(), http.StatusNotFound}} {
		rec := routedRequest(ts.Handler(), "GET", "/api/v1/ws?tableID="+tc.tableID, "", true)
		if rec.Code != tc.status {
			t.Fatalf("GM socket table %s = %d; want %d", tc.tableID, rec.Code, tc.status)
		}
	}
}

func TestFrontendRouting(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"index.html":      "<html>SPA</html>",
		"assets/app.js":   "console.log('app')",
		"assets/main.css": "body {}",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	ts.cfg.FrontendDir = dir
	handler := ts.Handler()
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"GET", "/", "<html>SPA</html>", 200},
		{"GET", "/tables/123", "<html>SPA</html>", 200},
		{"GET", "/assets/app.js", "console.log('app')", 200},
		{"HEAD", "/assets/main.css", "", 200},
		{"GET", "/assets/missing.js", "404 page not found\n", 404},
		{"GET", "/assets/", "404 page not found\n", 404},
		{"GET", "/api/v1/missing", "404 page not found\n", 404},
		{"GET", "/api", "404 page not found\n", 404},
		{"POST", "/tables/123", "method not allowed\n", 405},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := routedRequest(handler, tc.method, tc.path, "", false)
			if rec.Code != tc.status || rec.Body.String() != tc.body {
				t.Fatalf("response = %d %q; want %d %q", rec.Code, rec.Body.String(), tc.status, tc.body)
			}
		})
	}
}

func TestFrontendConfiguration(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(Config{FrontendDir: dir}, nil, nil, nil, nil, nil, nil); err == nil {
		t.Fatal("missing frontend index should fail startup")
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("index"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Config{FrontendDir: dir}, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("valid frontend directory: %v", err)
	}
}

func routedRequest(handler http.Handler, method, path, body string, authenticated bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if authenticated {
		addAuthCookie(req, "")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

type failingStore struct{ testStore }

func (f failingStore) GetTracks(context.Context) ([]domain.Track, error) {
	return nil, errors.New("database unavailable")
}

func (f failingStore) GetTrackTypes(context.Context) ([]domain.TrackType, error) {
	return nil, errors.New("database unavailable")
}

func (f failingStore) GetTrackByID(context.Context, uuid.UUID) (domain.Track, error) {
	return domain.Track{}, nil
}

func (f failingStore) DeleteTrack(context.Context, uuid.UUID) error {
	return errors.New("database unavailable")
}

func (f failingStore) DeleteTrackIfTableCount(context.Context, uuid.UUID, int64) (bool, error) {
	return false, errors.New("database unavailable")
}

func (f failingStore) CountTrackTables(context.Context, uuid.UUID) (int64, error) {
	return 0, errors.New("database unavailable")
}

type recordingWSRegisterer struct {
	tokens chan *auth.Token
}

func (r *recordingWSRegisterer) SendEvent(string, any) error      { return nil }
func (r *recordingWSRegisterer) RevokePlayerAccess(string)        {}
func (r *recordingWSRegisterer) On(string, events.Handler) func() { return func() {} }

func (r *recordingWSRegisterer) Register(conn *websocket.Conn, token *auth.Token) {
	defer conn.Close()
	r.tokens <- token
}
