package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultTableID = "00000000-0000-4000-8000-000000000001"
	gmUsername     = "e2e-gm"
	gmPassword     = "e2e-password"
	// Public fixture hash of gmPassword, never a real account credential.
	gmPasswordHash = "$argon2id$v=19$m=65536,t=2,p=2$cnBnLWUyZS10ZXN0LXNhbHQ$TSxyC0hp4Wz6SM6WbZtuwgKY8UrQEWA3Dsy1Ay5IGlk"
)

type app struct {
	baseURL string
	config  liveConfig
}

type gmClient struct {
	app  *app
	http *http.Client
}

type mixerSocket struct {
	conn *websocket.Conn
}

type trackPatch struct {
	FileID      string   `json:"fileID"`
	IsPlaying   *bool    `json:"isPlaying,omitempty"`
	IsRepeating *bool    `json:"isRepeating,omitempty"`
	CurrentTime *float64 `json:"currentTime,omitempty"`
	Volume      *float64 `json:"volume,omitempty"`
}

type socketMessage struct {
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload"`
}

func startApp(t *testing.T, root string, config liveConfig) *app {
	t.Helper()
	scratch := t.TempDir()
	binary := filepath.Join(scratch, "server")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build server: %v\n%s", err, output)
	}

	port := freePort(t)
	server := exec.Command(binary, "serve",
		"--port", strconv.Itoa(port),
		"--dev-mode",
		"--experimental-mixer",
		"--discord-worker", filepath.Join(root, "discord", "stream.mjs"),
		"--root-username", gmUsername,
		"--root-password-hash", gmPasswordHash,
		"--token-secret", "public-e2e-signing-key-not-for-production",
		"--join-token", "e2e-invite",
		"--db-path", filepath.Join(scratch, "app.db"),
		"--upload-dir", filepath.Join(scratch, "uploads"),
	)
	server.Dir = scratch
	server.Env = append(withoutDiscord(os.Environ()),
		"DISCORD_BOT_TOKEN="+config.StreamerToken,
	)
	logs := &lockedBuffer{}
	server.Stdout = logs
	server.Stderr = logs
	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Process.Signal(syscall.SIGTERM)
		_ = server.Wait()
		if t.Failed() {
			t.Logf("server logs:\n%s", logs.String())
		}
	})

	a := &app{baseURL: fmt.Sprintf("http://127.0.0.1:%d", port), config: config}
	deadline := time.Now().Add(30 * time.Second)
	for {
		conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			_ = conn.Close()
			return a
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not listen on port %d", port)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (a *app) login(t *testing.T) *gmClient {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	gm := &gmClient{app: a, http: &http.Client{Jar: jar, Timeout: 2 * time.Minute}}
	body, _ := json.Marshal(map[string]string{"username": gmUsername, "password": gmPassword})
	gm.do(t, http.MethodPost, "/api/v1/login", "application/json", bytes.NewReader(body), nil)
	return gm
}

func (gm *gmClient) upload(t *testing.T, name string, frequency int) string {
	t.Helper()
	tone, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=%d:duration=30", frequency), "-f", "mp3", "pipe:1").Output()
	if err != nil {
		t.Fatalf("generate %d Hz tone: %v", frequency, err)
	}

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	file, _ := writer.CreateFormFile("files", "tone.mp3")
	_, _ = file.Write(tone)
	_ = writer.WriteField("name", name)
	_ = writer.WriteField("typeID", gm.trackTypeID(t, "Ambiance"))
	_ = writer.WriteField("tableID", defaultTableID)
	_ = writer.Close()
	gm.do(t, http.MethodPost, "/api/v1/files", writer.FormDataContentType(), &form, nil)

	var tracks []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	gm.do(t, http.MethodGet, "/api/v1/files", "", nil, &tracks)
	for _, track := range tracks {
		if track.Name == name {
			return track.ID
		}
	}
	t.Fatalf("uploaded track %q not listed", name)
	return ""
}

func (gm *gmClient) trackTypeID(t *testing.T, name string) string {
	t.Helper()
	var types []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	gm.do(t, http.MethodGet, "/api/v1/trackTypes", "", nil, &types)
	for _, trackType := range types {
		if trackType.Name == name {
			return trackType.ID
		}
	}
	t.Fatalf("track type %q not found", name)
	return ""
}

func (gm *gmClient) mixer(t *testing.T) *mixerSocket {
	t.Helper()
	base, _ := url.Parse(gm.app.baseURL)
	socketURL := "ws://" + base.Host + "/api/v1/ws?tableID=" + defaultTableID
	header := http.Header{"Cookie": {cookieHeader(gm.http.Jar.Cookies(base))}}
	conn, _, err := websocket.DefaultDialer.DialContext(t.Context(), socketURL, header)
	if err != nil {
		t.Fatalf("open GM WebSocket: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	m := &mixerSocket{conn: conn}
	m.await(t, func(message socketMessage) bool { return message.Method == "mixerReady" })
	m.send(t, "mixSync", map[string]any{"tracks": []trackPatch{}, "typeVolumes": map[string]float64{}})
	return m
}

func (m *mixerSocket) sync(t *testing.T, patch trackPatch) {
	t.Helper()
	m.send(t, "syncTrack", patch)
}

func (m *mixerSocket) send(t *testing.T, method string, payload any) {
	t.Helper()
	data, _ := json.Marshal(payload)
	if err := m.conn.WriteJSON(socketMessage{Method: method, Payload: data}); err != nil {
		t.Fatalf("send %s: %v", method, err)
	}
	result := m.await(t, func(message socketMessage) bool {
		return message.Method == "mixerAck" || message.Method == "mixerError"
	})
	if result.Method == "mixerError" {
		t.Fatalf("%s %s failed: %s", method, data, result.Payload)
	}
}

func (m *mixerSocket) await(t *testing.T, match func(socketMessage) bool) socketMessage {
	t.Helper()
	_ = m.conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	for {
		var message socketMessage
		if err := m.conn.ReadJSON(&message); err != nil {
			t.Fatalf("read GM WebSocket: %v", err)
		}
		if match(message) {
			return message
		}
	}
}

func (gm *gmClient) connectDiscord(t *testing.T) {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"guildID": gm.app.config.GuildID.String(), "channelID": gm.app.config.ChannelID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	gm.do(t, http.MethodPost, "/api/v1/tables/"+defaultTableID+"/discord", "application/json", bytes.NewReader(body), nil)
	gm.waitDiscord(t, "connected")
}

func (gm *gmClient) waitDiscord(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for {
		var status struct {
			State string `json:"state"`
			Error string `json:"error"`
		}
		gm.do(t, http.MethodGet, "/api/v1/tables/"+defaultTableID+"/discord", "", nil, &status)
		if status.State == want {
			return
		}
		if status.State == "error" || time.Now().After(deadline) {
			t.Fatalf("Discord connection state %q: %s", status.State, status.Error)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (gm *gmClient) disconnectDiscord(t *testing.T) {
	t.Helper()
	gm.do(t, http.MethodDelete, "/api/v1/tables/"+defaultTableID+"/discord", "", nil, nil)
}

func (gm *gmClient) do(t *testing.T, method, path, contentType string, body io.Reader, out any) {
	t.Helper()
	request, err := http.NewRequestWithContext(context.Background(), method, gm.app.baseURL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response, err := gm.http.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = response.Body.Close() }()
	data, _ := io.ReadAll(response.Body)
	if response.StatusCode >= 300 {
		t.Fatalf("%s %s: %s: %s", method, path, response.Status, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatalf("%s %s: decode %s: %v", method, path, data, err)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port
}

func withoutDiscord(environment []string) []string {
	var kept []string
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "DISCORD_") && !strings.HasPrefix(entry, "INTEGRATION_TEST_DISCORD_") {
			kept = append(kept, entry)
		}
	}
	return kept
}

func cookieHeader(cookies []*http.Cookie) string {
	parts := make([]string, len(cookies))
	for i, cookie := range cookies {
		parts[i] = cookie.Name + "=" + cookie.Value
	}
	return strings.Join(parts, "; ")
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}
