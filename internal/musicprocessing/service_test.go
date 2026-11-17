package musicprocessing

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUploadSurvivesRequestCancellation(t *testing.T) {
	service, store := testService(t)
	audio := filepath.Join(t.TempDir(), "sample.wav")
	if output, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i",
		"sine=frequency=440:duration=1", audio).CombinedOutput(); err != nil {
		t.Fatalf("generate audio: %v: %s", err, output)
	}
	data, err := os.ReadFile(audio)
	if err != nil {
		t.Fatal(err)
	}
	requestCtx, cancel := context.WithCancel(context.Background())
	tableID := uuid.New()
	track, err := service.Upload(requestCtx, "Queued", uuid.New(), tableID, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if got := store.get(track.ID); got.Status != "processing" || store.tableID != tableID {
		t.Fatalf("queued track = %+v, table %s", got, store.tableID)
	}
	stop := runService(t, service)
	waitStatus(t, store, track.ID, "ready")
	stop()
	for _, file := range []string{"index.m3u8", "segment_000.ts", waveformFile} {
		if _, err := os.Stat(filepath.Join(track.Path, file)); err != nil {
			t.Errorf("processed track missing %s: %v", file, err)
		}
	}
	if _, err := os.Stat(filepath.Dir(service.inputPath(track.ID))); !os.IsNotExist(err) {
		t.Fatalf("staging not cleaned up: %v", err)
	}
}

func TestRunCancellationDrainsQueuedJobs(t *testing.T) {
	service, store := testService(t)
	track, err := service.Upload(context.Background(), "Queued", uuid.New(), uuid.Nil, strings.NewReader("audio"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.Run(ctx)
	got := store.get(track.ID)
	if got.Status != "failed" || !strings.Contains(got.ProcessingError, "interrupted") {
		t.Fatalf("cancelled job = %+v", got)
	}
	if _, err := os.Stat(filepath.Dir(service.inputPath(track.ID))); !os.IsNotExist(err) {
		t.Fatalf("cancelled staging not removed: %v", err)
	}
	if _, err := service.Upload(context.Background(), "Closed", uuid.New(), uuid.Nil, strings.NewReader("audio")); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed service accepted work: %v", err)
	}
}

func TestRunCancellationStopsActiveDownload(t *testing.T) {
	service, store := testService(t)
	started := fixtureDownloader(t)
	track, err := service.Import(context.Background(), "Downloading", uuid.New(), uuid.Nil, "https://youtu.be/ddddddddddd")
	if err != nil {
		t.Fatal(err)
	}
	stop := runService(t, service)
	waitForFile(t, started)
	stop()
	got := store.get(track.ID)
	if got.Status != "failed" || !strings.Contains(got.ProcessingError, "interrupted") {
		t.Fatalf("interrupted download = %+v", got)
	}
	for _, path := range []string{track.Path, filepath.Dir(service.inputPath(track.ID))} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("cancelled processing files not removed: %v", err)
		}
	}
}

func TestScanUsesRequestContext(t *testing.T) {
	service, _ := testService(t)
	started := fixtureDownloader(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := service.Scan(ctx, "https://youtu.be/ddddddddddd")
		finished <- err
	}()
	waitForFile(t, started)
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("scan ignored request cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("scan did not stop after request cancellation")
	}
}

func TestQueuedDeadlineAndAdmission(t *testing.T) {
	service, store := testService(t)
	for range cap(service.jobs) {
		if _, err := service.Upload(context.Background(), "Queued", uuid.New(), uuid.Nil, strings.NewReader("audio")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.Upload(context.Background(), "Full", uuid.New(), uuid.Nil, strings.NewReader("audio")); err == nil {
		t.Fatal("full queue accepted another upload")
	}
	for range cap(service.jobs) {
		next := <-service.jobs
		next.deadline = time.Now().Add(-time.Second)
		service.jobs <- next
	}
	stop := runService(t, service)
	tracks, _ := store.GetTracks(context.Background())
	for _, track := range tracks {
		waitStatus(t, store, track.ID, "failed")
		if !strings.Contains(store.get(track.ID).ProcessingError, "timed out") {
			t.Fatal("queue time did not count toward the deadline")
		}
	}
	stop()
}

func TestRecoverInterruptedProcessing(t *testing.T) {
	service, store := testService(t)
	track, err := service.newTrack("Interrupted", uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTrack(context.Background(), &track); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{track.Path, filepath.Dir(service.inputPath(track.ID))} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := store.get(track.ID)
	if got.Status != "failed" || !strings.Contains(got.ProcessingError, "restart") {
		t.Fatalf("recovered track = %+v", got)
	}
	for _, path := range []string{track.Path, filepath.Dir(service.inputPath(track.ID))} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("interrupted files not cleaned up: %s: %v", path, err)
		}
	}
}

func fixtureDownloader(t *testing.T) string {
	t.Helper()
	script, err := os.ReadFile("testdata/yt-dlp")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), script, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	started := filepath.Join(t.TempDir(), "started")
	t.Setenv("IMPORT_ARGS", started)
	return started
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("downloader did not start")
}

func testService(t *testing.T) (*Service, *memoryStore) {
	t.Helper()
	store := &memoryStore{tracks: make(map[uuid.UUID]Track)}
	return New(t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)), store), store
}

func runService(t *testing.T, service *Service) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		service.Run(ctx)
	}()
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("processing service did not stop")
		}
	}
	t.Cleanup(stop)
	return stop
}

func waitStatus(t *testing.T, store *memoryStore, id uuid.UUID, status string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if store.get(id).Status == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("track never became %s: %+v", status, store.get(id))
}

type memoryStore struct {
	mu      sync.Mutex
	tracks  map[uuid.UUID]Track
	tableID uuid.UUID
}

func (s *memoryStore) SaveTrack(ctx context.Context, track *Track) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tracks[track.ID] = *track
	return nil
}

func (s *memoryStore) SaveTrackToTable(ctx context.Context, track *Track, tableID uuid.UUID) error {
	s.tableID = tableID
	return s.SaveTrack(ctx, track)
}

func (s *memoryStore) GetTracks(ctx context.Context) ([]Track, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tracks := make([]Track, 0, len(s.tracks))
	for _, track := range s.tracks {
		tracks = append(tracks, track)
	}
	return tracks, ctx.Err()
}

func (s *memoryStore) SetTrackProcessing(ctx context.Context, id uuid.UUID, status, message string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	track := s.tracks[id]
	track.Status, track.ProcessingError = status, message
	s.tracks[id] = track
	return nil
}

func (s *memoryStore) get(id uuid.UUID) Track {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tracks[id]
}
