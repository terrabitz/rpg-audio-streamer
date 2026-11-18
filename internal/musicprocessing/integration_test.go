package musicprocessing_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
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

func TestUploadAudioFormatsIntegration(t *testing.T) {
	for _, tc := range []struct {
		name, extension, codec string
		options                []string
	}{
		{name: "PCM WAV", extension: "wav", codec: "pcm_s16le", options: []string{"-ac", "1", "-ar", "22050"}},
		{name: "MP3", extension: "mp3", codec: "libmp3lame"},
		{name: "AAC M4A", extension: "m4a", codec: "aac"},
		{name: "ALAC M4A", extension: "m4a", codec: "alac"},
		{name: "FLAC", extension: "flac", codec: "flac", options: []string{"-ar", "96000"}},
		// Native Opus and Vorbis encoders need experimental mode; the runtime only needs their decoders.
		{name: "Opus WebM", extension: "webm", codec: "opus", options: []string{"-strict", "experimental"}},
		{name: "Vorbis Ogg", extension: "ogg", codec: "vorbis", options: []string{"-strict", "experimental"}},
		{name: "WMA", extension: "wma", codec: "wmav2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			dir := t.TempDir()
			inputPath := filepath.Join(dir, "tone."+tc.extension)
			args := []string{"-nostdin", "-v", "error", "-f", "lavfi", "-i",
				"sine=frequency=440:sample_rate=48000:duration=12", "-ac", "2", "-c:a", tc.codec}
			args = append(args, tc.options...)
			args = append(args, inputPath)
			if output, err := exec.CommandContext(ctx, "ffmpeg", args...).CombinedOutput(); err != nil {
				t.Fatalf("generate %s fixture: %v: %s", tc.name, err, output)
			}

			db := integrationDatabase(t, dir)
			processed := make(processedTracks, 1)
			service := musicprocessing.New(filepath.Join(dir, "uploads"), slog.Default(), db, nil, processed)
			stop := runIntegrationProcessing(t, ctx, service)
			types, err := db.GetTrackTypes(ctx)
			if err != nil || len(types) == 0 {
				t.Fatalf("load track types: %v", err)
			}
			input, err := os.Open(inputPath)
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			queued, err := service.Upload(ctx, tc.name, types[0].ID, uuid.Nil, input)
			if err != nil {
				t.Fatalf("upload %s: %v", tc.name, err)
			}
			if queued.Status != "processing" {
				t.Fatalf("accepted track status = %q", queued.Status)
			}
			select {
			case <-ctx.Done():
				t.Fatalf("waiting for %s processing: %v", tc.name, ctx.Err())
			case notified := <-processed:
				if notified.ID != queued.ID || notified.Status != "ready" {
					t.Fatalf("processing notification = %+v", notified)
				}
			}
			track, err := db.GetTrackByID(ctx, queued.ID)
			if err != nil || track.Status != "ready" || track.Name != tc.name || track.TypeID != queued.TypeID {
				t.Fatalf("processed track = %+v, %v", track, err)
			}
			assertProcessedAudio(t, ctx, track.Path)
			stop()
			if _, err := os.Stat(filepath.Join(dir, "uploads", ".processing", queued.ID.String())); !os.IsNotExist(err) {
				t.Fatalf("upload staging not cleaned up: %v", err)
			}
		})
	}
}

func assertProcessedAudio(t *testing.T, ctx context.Context, dir string) {
	t.Helper()
	playlist := filepath.Join(dir, "index.m3u8")
	manifest, err := os.ReadFile(playlist)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(manifest), "#EXTINF:") < 2 || !strings.Contains(string(manifest), "#EXT-X-ENDLIST") {
		t.Fatalf("expected a complete, multi-segment HLS playlist: %s", manifest)
	}
	data, err := os.ReadFile(filepath.Join(dir, "waveform.json"))
	if err != nil {
		t.Fatalf("waveform was not pre-generated: %v", err)
	}
	var waveform musicprocessing.Waveform
	if err := json.Unmarshal(data, &waveform); err != nil {
		t.Fatal(err)
	}
	if len(waveform.Peaks) != 400 {
		t.Fatalf("waveform has %d peaks, want 400", len(waveform.Peaks))
	}
	loudest := 0.0
	for _, peak := range waveform.Peaks {
		if math.IsNaN(peak) || math.IsInf(peak, 0) || peak < 0 || peak > 1 {
			t.Fatalf("invalid waveform peak: %v", peak)
		}
		loudest = max(loudest, peak)
	}
	if loudest != 1 {
		t.Fatalf("waveform maximum = %v, want 1", loudest)
	}

	var stdout, stderr bytes.Buffer
	probe := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "a",
		"-show_entries", "stream=codec_name,sample_rate,channels", "-of", "json", playlist)
	probe.Stdout, probe.Stderr = &stdout, &stderr
	if err := probe.Run(); err != nil {
		t.Fatalf("probe processed HLS: %v: %s", err, &stderr)
	}
	var metadata struct {
		Streams []struct {
			CodecName  string `json:"codec_name"`
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata.Streams) != 1 || metadata.Streams[0].CodecName != "aac" ||
		metadata.Streams[0].SampleRate != "44100" || metadata.Streams[0].Channels != 2 {
		t.Fatalf("processed HLS audio streams = %+v, want stereo 44100 Hz AAC", metadata.Streams)
	}

	stdout.Reset()
	stderr.Reset()
	// Seek past the first HLS segment to exercise playback of later segments too.
	decode := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-v", "error", "-ss", "7", "-i", playlist,
		"-map", "0:a:0", "-t", "1", "-ac", "2", "-ar", "48000", "-f", "f32le", "-")
	decode.Stdout, decode.Stderr = &stdout, &stderr
	if err := decode.Run(); err != nil {
		t.Fatalf("decode processed HLS after seeking: %v: %s", err, &stderr)
	}
	pcm := stdout.Bytes()
	frames := len(pcm) / 8
	if len(pcm)%8 != 0 || frames < 47520 || frames > 48480 {
		t.Fatalf("decoded %d bytes, want about one second of stereo 48000 Hz float32 PCM", len(pcm))
	}
	for channel := range 2 {
		energy, previous := 0.0, 0.0
		crossings := 0
		for i := range frames {
			sample := float64(math.Float32frombits(binary.LittleEndian.Uint32(pcm[i*8+channel*4:])))
			if math.IsNaN(sample) || math.IsInf(sample, 0) {
				t.Fatalf("decoded invalid sample: %v", sample)
			}
			energy += sample * sample
			if i > 0 && (sample < 0) != (previous < 0) {
				crossings++
			}
			previous = sample
		}
		if rms := math.Sqrt(energy / float64(frames)); rms < 0.01 {
			t.Fatalf("channel %d is silent: RMS = %v", channel, rms)
		}
		if frequency := float64(crossings) * 48000 / (2 * float64(frames)); math.Abs(frequency-440) > 5 {
			t.Fatalf("channel %d decoded %.1f Hz, want 440 Hz", channel, frequency)
		}
	}
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
	db := integrationDatabase(t, dir)
	videos := youtube.New(slog.Default())
	processed := make(processedTracks, 1)
	service := musicprocessing.New(filepath.Join(dir, "uploads"), slog.Default(), db, videos, processed)
	stop := runIntegrationProcessing(t, ctx, service)

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

func integrationDatabase(t *testing.T, dir string) *sqlitedatastore.SQLiteDatastore {
	t.Helper()
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
	return db
}

func runIntegrationProcessing(t *testing.T, ctx context.Context, service *musicprocessing.Service) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(ctx)
	if err := service.Recover(ctx); err != nil {
		cancel()
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
			t.Error("processing service did not stop")
		}
	}
	t.Cleanup(stop)
	return stop
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
