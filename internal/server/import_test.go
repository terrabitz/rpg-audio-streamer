package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
	"github.com/terrabitz/rpg-audio-streamer/internal/youtube"
)

func TestImportHTTP(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	bin := t.TempDir()
	script, err := os.ReadFile("../youtube/testdata/yt-dlp")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), script, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	audio := filepath.Join(t.TempDir(), "sample.wav")
	if output, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i",
		"sine=frequency=440:duration=1", audio).CombinedOutput(); err != nil {
		t.Fatalf("generate audio: %v: %s", err, output)
	}
	t.Setenv("IMPORT_AUDIO_FIXTURE", audio)
	t.Setenv("IMPORT_OVERSIZE_BYTES", strconv.Itoa((200<<20)+1))
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv("IMPORT_ARGS", argsFile)
	validBody := `{"url":"https://youtu.be/aaaaaaaaaaa?list=ignored","name":" Imported audio ","typeID":"1ec000a2-a7c9-11ee-a0e5-0242ac120002"}`
	request := func(ctx context.Context, method, body string, authenticated bool, status int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/api/v1/files/import", strings.NewReader(body)).WithContext(ctx)
		if authenticated {
			addAuthCookie(req, ts.auth.(*mockAuth).token.String())
		}
		rec := httptest.NewRecorder()
		ts.Handler().ServeHTTP(rec, req)
		if rec.Code != status {
			t.Fatalf("import status = %d: %s; want %d", rec.Code, rec.Body, status)
		}
		return rec
	}
	ctx := context.Background()
	request(ctx, "POST", validBody, false, 401)
	ts.auth.(*mockAuth).token.Role = auth.RolePlayer
	request(ctx, "POST", validBody, true, 401)
	ts.auth.(*mockAuth).token.Role = auth.RoleGM
	request(ctx, "GET", validBody, true, 405)
	for _, body := range []string{
		`{`, `null`, `{}`, validBody + `{}`,
		strings.Replace(validBody, "Imported audio", " ", 1),
		strings.Replace(validBody, "Imported audio", strings.Repeat("x", 201), 1),
		strings.Replace(validBody, "aaaaaaaaaaa?list=ignored", "playlist?list=PL123", 1),
		strings.Replace(validBody, "1ec000a2-a7c9-11ee-a0e5-0242ac120002", uuid.NewString(), 1),
		strings.TrimSuffix(validBody, "}") + `,"unexpected":true}`,
	} {
		request(ctx, "POST", body, true, 400)
	}
	request(ctx, "POST", strings.TrimSuffix(validBody, "}")+`,"tableID":"`+uuid.NewString()+`"}`, true, 404)
	request(ctx, "POST", validBody, true, 202)
	ts.waitForProcessing(t)
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--ignore-config", "--no-plugin-dirs", "--no-playlist",
		"--max-filesize\n200M", "--match-filter\n!is_live & duration <= 10800", "--js-runtimes\nnode"} {
		if !bytes.Contains(args, []byte(flag+"\n")) {
			t.Errorf("downloader missing %q in %s", flag, args)
		}
	}
	if !bytes.HasSuffix(args, []byte("--\nhttps://www.youtube.com/watch?v=aaaaaaaaaaa\n")) {
		t.Errorf("downloader received noncanonical URL: %s", args)
	}
	tracks, _ := ts.store.GetTracks(ctx)
	if len(tracks) != 1 || tracks[0].Name != "Imported audio" {
		t.Fatalf("saved tracks = %+v", tracks)
	}
	for _, name := range []string{"index.m3u8", "segment_000.ts"} {
		if _, err := os.Stat(filepath.Join(tracks[0].Path, name)); err != nil {
			t.Errorf("import missing %s: %v", name, err)
		}
	}
	if waveform, err := ts.store.GetTrackWaveform(ctx, tracks[0].ID); err != nil || len(waveform.Peaks) != 400 {
		t.Fatalf("stored import waveform = %d peaks: %v", len(waveform.Peaks), err)
	}
	for _, tc := range []struct {
		id string
	}{
		{"bbbbbbbbbbb"},
		{"ccccccccccc"},
		{"eeeeeeeeeee"},
		{"fffffffffff"},
	} {
		rec := request(ctx, "POST", strings.Replace(validBody, "aaaaaaaaaaa", tc.id, 1), true, 202)
		var queued domain.Track
		if err := json.Unmarshal(rec.Body.Bytes(), &queued); err != nil {
			t.Fatal(err)
		}
		ts.waitForProcessing(t)
		result, err := ts.store.GetTrackByID(ctx, queued.ID)
		if err != nil || result.Status != "failed" || result.ProcessingError == "" {
			t.Fatalf("failed processing state = %+v, %v", result, err)
		}
		if tc.id == "bbbbbbbbbbb" {
			for _, detail := range []string{"Command:\n'yt-dlp'", "Logs:\nVideo unavailable", "Error:\nexit status 1"} {
				if !strings.Contains(result.ProcessingError, detail) {
					t.Errorf("failed import lost %q: %s", detail, result.ProcessingError)
				}
			}
		}
	}
	ts.stopProcessing()
	<-ts.processingDone
	output := strings.Split(strings.Split(string(args), "--output\n")[1], "\n")[0]
	if _, err := os.Stat(filepath.Dir(output)); !os.IsNotExist(err) {
		t.Errorf("staging directory not cleaned up: %v", err)
	}
	ts.store = importFailStore{testStore: ts.store}
	ts.processing = musicprocessing.New(ts.tempDir, ts.logger, ts.store, youtube.New(ts.logger), ts.hub)
	request(ctx, "POST", validBody, true, 503)
	entries, err := os.ReadDir(ts.tempDir)
	if err != nil || len(entries) != 2 {
		t.Fatalf("failed imports left HLS directories: %v, %v", entries, err)
	}
	t.Setenv("PATH", t.TempDir())
	request(ctx, "POST", validBody, true, 503)
}

func TestVideoScan(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	bin := t.TempDir()
	script, err := os.ReadFile("../youtube/testdata/yt-dlp")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), script, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, tc := range []struct {
		id      string
		status  int
		message string
	}{
		{"aaaaaaaaaaa", 200, "YouTube Forest"},
		{"jjjjjjjjjjj", 200, "At the limits"},
		{"ggggggggggg", 422, "3-hour limit"},
		{"hhhhhhhhhhh", 422, "200 MiB limit"},
		{"kkkkkkkkkkk", 422, "200 MiB limit"},
		{"iiiiiiiiiii", 422, "Live streams"},
		{"bbbbbbbbbbb", 502, "Could not retrieve video metadata"},
	} {
		req := httptest.NewRequest("POST", "/api/v1/files/scan", strings.NewReader(`{"url":"https://youtu.be/`+tc.id+`"}`))
		addAuthCookie(req, ts.auth.(*mockAuth).token.String())
		rec := httptest.NewRecorder()
		ts.Handler().ServeHTTP(rec, req)
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.message) {
			t.Errorf("scan %s = %d: %s", tc.id, rec.Code, rec.Body)
		}
	}
}

type importFailStore struct{ testStore }

func (importFailStore) SaveTrack(context.Context, *domain.Track) error {
	return errors.New("injected persistence failure")
}
