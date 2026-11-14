package server

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/terrabitz/rpg-audio-streamer/internal/websocket"
)

func TestMixerPCMDecodesHLS(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	track := saveWaveformTrack(t, ts)
	ctx := context.Background()
	cmd := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi",
		"-i", "sine=frequency=440:sample_rate=44100:duration=1",
		"-c:a", "aac", "-f", "hls", filepath.Join(track.Path, "index.m3u8"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate HLS: %v: %s", err, output)
	}
	frames, err := decodeMixerPCM(ctx, filepath.Join(track.Path, "index.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) < 48000 || len(frames) > 51000 {
		t.Fatalf("decoded clip: %d frames", len(frames))
	}
	peak := 0.0
	for _, frame := range frames {
		peak = max(peak, frame.Left)
	}
	if peak < 0.05 {
		t.Fatalf("decoded silence, peak=%v", peak)
	}
	if _, err := decodeMixerPCM(ctx, filepath.Join(track.Path, "missing.m3u8")); err == nil {
		t.Fatal("missing audio decoded successfully")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := decodeMixerPCM(cancelled, filepath.Join(track.Path, "index.m3u8")); err == nil {
		t.Fatal("cancelled decode succeeded")
	}
}

func TestMixerStatusDisabledAndUnauthorized(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	path := "/api/v1/tables/" + DefaultTableID.String() + "/mix"
	for _, tc := range []struct {
		method string
		gm     bool
		want   int
	}{
		{"GET", false, 401}, {"GET", true, 404}, {"POST", true, 405},
	} {
		rec := routedRequest(ts.Handler(), tc.method, path, "", tc.gm)
		if rec.Code != tc.want {
			t.Fatalf("status: %d, want %d: %s", rec.Code, tc.want, rec.Body)
		}

	}
}

func TestMixerStatusEnabled(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	srv, err := New(Config{ExperimentalMixer: true}, ts.logger, ts.auth, ts.store, websocket.NewHub(ts.logger))
	if err != nil {
		t.Fatal(err)
	}
	rec := routedRequest(srv.Handler(), "GET", "/api/v1/tables/"+DefaultTableID.String()+"/mix", "", true)
	if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("mixer status: %d: %s", rec.Code, rec.Body)
	}
	if _, err := New(Config{ExperimentalMixer: true}, ts.logger, ts.auth, ts.store, ts.hub); err == nil {
		t.Fatal("enabled mixer accepted an incompatible hub")
	}
}
