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
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

func TestUploadSurvivesRequestCancellation(t *testing.T) {
	service, store, notifier := testService(t)
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
	if got := notifier.received(); len(got) != 1 || got[0].ID != track.ID || got[0].Status != "ready" {
		t.Fatalf("ready notifications = %+v", got)
	}
	for _, file := range []string{"index.m3u8", "segment_000.ts"} {
		if _, err := os.Stat(filepath.Join(track.Path, file)); err != nil {
			t.Errorf("processed track missing %s: %v", file, err)
		}
	}
	if _, err := os.Stat(filepath.Dir(service.inputPath(track.ID))); !os.IsNotExist(err) {
		t.Fatalf("staging not cleaned up: %v", err)
	}
}

func TestRunCancellationDrainsQueuedJobs(t *testing.T) {
	service, store, notifier := testService(t)
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
	if sent := notifier.received(); len(sent) != 1 || sent[0].Status != "failed" || sent[0].ProcessingError != got.ProcessingError {
		t.Fatalf("failure notifications = %+v", sent)
	}
	if _, err := os.Stat(filepath.Dir(service.inputPath(track.ID))); !os.IsNotExist(err) {
		t.Fatalf("cancelled staging not removed: %v", err)
	}
	if _, err := service.Upload(context.Background(), "Closed", uuid.New(), uuid.Nil, strings.NewReader("audio")); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed service accepted work: %v", err)
	}
}

func TestRunCancellationStopsActiveDownload(t *testing.T) {
	service, store, _ := testService(t)
	downloader := service.downloader.(*blockingDownloader)
	track, err := service.Import(context.Background(), "Downloading", uuid.New(), uuid.Nil, "source")
	if err != nil {
		t.Fatal(err)
	}
	stop := runService(t, service)
	select {
	case <-downloader.started:
	case <-time.After(5 * time.Second):
		t.Fatal("download did not start")
	}
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

func TestImportRejectsUnresolvedSource(t *testing.T) {
	service, store, _ := testService(t)
	service.downloader.(*blockingDownloader).resolveErr = errUnsupported
	if _, err := service.Import(context.Background(), "Invalid", uuid.New(), uuid.Nil, "source"); !errors.Is(err, errUnsupported) {
		t.Fatalf("unresolved import error = %v", err)
	}
	if tracks, _ := store.GetTracks(context.Background()); len(tracks) != 0 {
		t.Fatalf("unresolved import persisted tracks: %+v", tracks)
	}
}

func TestImportPersistsAndNotifiesFailureDiagnostics(t *testing.T) {
	service, store, notifier := testService(t)
	const diagnostics = "Command:\nyt-dlp -- source\n\nLogs:\nDownload attempt\nERROR: blocked\n\nError:\nexit status 1"
	service.downloader.(*blockingDownloader).downloadErr = errors.New(diagnostics)
	track, err := service.Import(context.Background(), "Failed import", uuid.New(), uuid.Nil, "source")
	if err != nil {
		t.Fatal(err)
	}
	stop := runService(t, service)
	waitStatus(t, store, track.ID, "failed")
	stop()
	got := store.get(track.ID)
	if !strings.Contains(got.ProcessingError, diagnostics) {
		t.Fatalf("lost persisted diagnostics: %q", got.ProcessingError)
	}
	sent := notifier.received()
	if len(sent) != 1 || sent[0].ProcessingError != got.ProcessingError {
		t.Fatalf("lost pushed diagnostics: %+v", sent)
	}
}

func TestQueuedDeadlineAndAdmission(t *testing.T) {
	service, store, _ := testService(t)
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
	service, store, _ := testService(t)
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

func testService(t *testing.T) (*Service, *memoryStore, *recordingNotifier) {
	t.Helper()
	store := &memoryStore{tracks: make(map[uuid.UUID]domain.Track)}
	notifier := &recordingNotifier{}
	downloader := &blockingDownloader{started: make(chan struct{})}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(t.TempDir(), logger, store, downloader, notifier), store, notifier
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
	tracks  map[uuid.UUID]domain.Track
	tableID uuid.UUID
}

func (s *memoryStore) SaveTrack(ctx context.Context, track *domain.Track) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tracks[track.ID] = *track
	return nil
}

func (s *memoryStore) SaveTrackToTable(ctx context.Context, track *domain.Track, tableID uuid.UUID) error {
	s.tableID = tableID
	return s.SaveTrack(ctx, track)
}

func (s *memoryStore) GetTracks(ctx context.Context) ([]domain.Track, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tracks := make([]domain.Track, 0, len(s.tracks))
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

func (s *memoryStore) SaveTrackWaveform(ctx context.Context, _ uuid.UUID, _ domain.Waveform) error {
	return ctx.Err()
}

func (s *memoryStore) get(id uuid.UUID) domain.Track {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tracks[id]
}

var errUnsupported = errors.New("unsupported source")

type blockingDownloader struct {
	started     chan struct{}
	resolveErr  error
	downloadErr error
}

func (d *blockingDownloader) Resolve(source string) (string, error) {
	return source, d.resolveErr
}

func (d *blockingDownloader) Download(ctx context.Context, _, _ string) error {
	close(d.started)
	if d.downloadErr != nil {
		return d.downloadErr
	}
	<-ctx.Done()
	return ctx.Err()
}

type recordingNotifier struct {
	mu     sync.Mutex
	tracks []domain.Track
}

func (n *recordingNotifier) SendEvent(_ string, payload any) error {
	track := payload.(domain.Track)
	n.mu.Lock()
	defer n.mu.Unlock()
	n.tracks = append(n.tracks, track)
	return nil
}

func (n *recordingNotifier) received() []domain.Track {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]domain.Track(nil), n.tracks...)
}
