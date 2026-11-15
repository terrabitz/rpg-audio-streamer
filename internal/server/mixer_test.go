package server

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
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

func TestDiscordStreamingConfiguration(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	if _, err := New(Config{DiscordStreamingEnabled: true}, ts.logger, ts.auth, ts.store, ts.hub); err == nil {
		t.Fatal("enabled Discord streaming accepted a missing token")
	}
	cfg := Config{}
	cfg.Discord.Token = "test-token"
	srv, err := New(cfg, ts.logger, ts.auth, ts.store, ts.hub)
	if err != nil {
		t.Fatal(err)
	}
	if srv.discord != nil || srv.mixer != nil {
		t.Fatal("token alone enabled Discord streaming")
	}
	cfg.DiscordStreamingEnabled = true
	if _, err := New(cfg, ts.logger, ts.auth, ts.store, ts.hub); err == nil {
		t.Fatal("enabled Discord streaming accepted an incompatible hub")
	}
}
