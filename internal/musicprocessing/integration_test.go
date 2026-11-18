package musicprocessing_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore"
	"github.com/terrabitz/rpg-audio-streamer/internal/youtube"
)

type youtubeIntegrationConfig struct {
	Link string
}

func TestYouTubeIntegration(t *testing.T) {
	if _, configured := os.LookupEnv("INTEGRATION_TEST_YT_DLP_LINK"); !configured {
		t.Skip("set INTEGRATION_TEST_YT_DLP_LINK to test real YouTube scanning, download, and conversion")
	}
	cfg, err := youtubeIntegrationConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 17*time.Minute)
	defer cancel()
	dir := t.TempDir()
	db, err := sqlitedatastore.New(filepath.Join(dir, "tracks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	migrations, err := sqlitedatastore.NewMigration(os.DirFS("../../sql/migrations"), db)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Up(); err != nil {
		t.Fatal(err)
	}
	videos := youtube.New(slog.Default())
	processed := make(processedTracks, 1)
	service := musicprocessing.New(filepath.Join(dir, "uploads"), slog.Default(), db, videos, processed)
	if err := service.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		service.Run(ctx)
	}()
	stop := func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Error("YouTube processing service did not stop")
		}
	}
	t.Cleanup(stop)

	metadata, err := videos.Scan(ctx, cfg.Link)
	if err != nil {
		t.Fatalf("scan real YouTube video: %v", err)
	}
	canonical, _ := youtube.CanonicalURL(cfg.Link)
	if metadata.URL != canonical || strings.TrimSpace(metadata.Name) == "" || metadata.Duration <= 0 {
		t.Fatalf("invalid real video metadata: %+v", metadata)
	}
	types, err := db.GetTrackTypes(ctx)
	if err != nil || len(types) == 0 {
		t.Fatalf("load track types: %v", err)
	}
	queued, err := service.Import(ctx, metadata.Name, types[0].ID, uuid.Nil, cfg.Link)
	if err != nil {
		t.Fatalf("queue real YouTube import: %v", err)
	}
	if queued.Status != "processing" {
		t.Fatalf("accepted track status = %q", queued.Status)
	}
	select {
	case <-ctx.Done():
		t.Fatalf("waiting for real YouTube processing: %v", ctx.Err())
	case notified := <-processed:
		if notified.ID != queued.ID || notified.Status != "ready" {
			t.Fatalf("real YouTube processing notification = %+v", notified)
		}
	}
	track, err := db.GetTrackByID(ctx, queued.ID)
	if err != nil || !track.Ready() || track.Name != metadata.Name || track.TypeID != queued.TypeID {
		t.Fatalf("processed track = %+v, %v", track, err)
	}
	for _, file := range []string{"index.m3u8", "segment_000.ts", "waveform.json"} {
		info, err := os.Stat(filepath.Join(queued.Path, file))
		if err != nil || info.Size() == 0 {
			t.Fatalf("missing or empty processed %s: %v", file, err)
		}
	}
	waveform, err := musicprocessing.LoadWaveform(ctx, queued.Path)
	if err != nil || len(waveform.Peaks) != 400 {
		t.Fatalf("processed waveform = %d peaks: %v", len(waveform.Peaks), err)
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-i",
		filepath.Join(queued.Path, "index.m3u8"), "-map", "0:a:0", "-t", "1", "-f", "null", "-")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("decode real imported HLS audio: %v: %s", err, output)
	}
	stop()
	if _, err := os.Stat(filepath.Join(dir, "uploads", ".processing", queued.ID.String())); !os.IsNotExist(err) {
		t.Fatalf("YouTube import staging not cleaned up: %v", err)
	}
}

type processedTracks chan musicprocessing.Track

func (p processedTracks) SendEvent(_ string, payload any) error {
	p <- payload.(musicprocessing.Track)
	return nil
}

func youtubeIntegrationConfigFromEnv() (youtubeIntegrationConfig, error) {
	cfg := youtubeIntegrationConfig{Link: strings.TrimSpace(os.Getenv("INTEGRATION_TEST_YT_DLP_LINK"))}
	if _, valid := youtube.CanonicalURL(cfg.Link); !valid {
		return youtubeIntegrationConfig{}, fmt.Errorf("INTEGRATION_TEST_YT_DLP_LINK must contain a YouTube video URL")
	}
	return cfg, nil
}

func TestYouTubeIntegrationConfig(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		valid       bool
	}{
		{"empty", "", false},
		{"blank", " ", false},
		{"invalid URL", "not a URL", false},
		{"not YouTube", "https://example.com/watch?v=aaaaaaaaaaa", false},
		{"video", " https://www.youtube.com/watch?v=aaaaaaaaaaa ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("INTEGRATION_TEST_YT_DLP_LINK", tc.value)
			cfg, err := youtubeIntegrationConfigFromEnv()
			if (err == nil) != tc.valid {
				t.Fatalf("config = %+v, %v", cfg, err)
			}
			if tc.valid && cfg.Link != strings.TrimSpace(tc.value) {
				t.Fatalf("configuration did not use the environment: %+v", cfg)
			}
		})
	}
}
