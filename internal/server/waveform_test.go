package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
)

func TestRoutedWaveform(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	handler := ts.Handler()
	track := saveWaveformTrack(t, ts)
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi",
		"-i", "aevalsrc=if(lt(t\\,2)\\,0\\,sin(2*PI*440*t)):s=44100:d=4",
		"-c:a", "aac", "-f", "hls", "-hls_time", "6", filepath.Join(track.Path, "index.m3u8"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create HLS: %v: %s", err, out)
	}

	rec := routedRequest(handler, http.MethodGet, "/api/v1/files/"+track.ID.String()+"/waveform", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("waveform = %d: %s", rec.Code, rec.Body)
	}
	var waveform musicprocessing.Waveform
	if err := json.Unmarshal(rec.Body.Bytes(), &waveform); err != nil {
		t.Fatal(err)
	}
	peaks := waveform.Peaks
	if len(peaks) != 400 {
		t.Fatalf("got %d peaks, want 400", len(peaks))
	}
	if peak := slices.Max(peaks); peak != 1 {
		t.Fatalf("maximum peak = %v, want 1", peak)
	}
	if quiet, loud := slices.Max(peaks[:len(peaks)/3]), slices.Min(peaks[len(peaks)*2/3:len(peaks)*9/10]); quiet > 0.05 || loud < 0.5 {
		t.Fatalf("silent half peaks at %v; tone half dips to %v", quiet, loud)
	}

	if _, err := os.Stat(filepath.Join(track.Path, "waveform.json")); !os.IsNotExist(err) {
		t.Fatalf("waveform must not be written to disk: %v", err)
	}
	if err := os.Remove(filepath.Join(track.Path, "index.m3u8")); err != nil {
		t.Fatal(err)
	}
	cached := routedRequest(handler, http.MethodGet, "/api/v1/files/"+track.ID.String()+"/waveform", "", true)
	if cached.Code != http.StatusOK || cached.Body.String() != rec.Body.String() {
		t.Fatalf("stored waveform = %d: %q", cached.Code, cached.Body)
	}
}

func TestRoutedWaveformErrors(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	track := saveWaveformTrack(t, ts)
	path := "/api/v1/files/" + track.ID.String() + "/waveform"
	for _, tc := range []struct {
		name, method, path string
		authenticated      bool
		status             int
	}{
		{"unauthenticated", "GET", path, false, 401},
		{"unsupported method", "POST", path, true, 405},
		{"invalid ID", "GET", "/api/v1/files/invalid/waveform", true, 400},
		{"missing track", "GET", "/api/v1/files/" + uuid.NewString() + "/waveform", true, 404},
		{"undecodable audio", "GET", path, true, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := routedRequest(ts.Handler(), tc.method, tc.path, "", tc.authenticated)
			if rec.Code != tc.status {
				t.Fatalf("status = %d; want %d: %s", rec.Code, tc.status, rec.Body)
			}
		})
	}
}

func saveWaveformTrack(t *testing.T, ts *testServer) Track {
	t.Helper()
	id := uuid.New()
	track := Track{ID: id, Name: "Waves", TypeID: uuid.MustParse("1ec000a2-a7c9-11ee-a0e5-0242ac120002"),
		Path: filepath.Join(ts.tempDir, id.String())}
	if err := os.Mkdir(track.Path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := ts.store.SaveTrack(context.Background(), &track); err != nil {
		t.Fatal(err)
	}
	return track
}
