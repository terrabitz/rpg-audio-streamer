package musicprocessing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErrClosed = errors.New("music processing service is closed")

type Store interface {
	SaveTrack(context.Context, *Track) error
	SaveTrackToTable(context.Context, *Track, uuid.UUID) error
	GetTracks(context.Context) ([]Track, error)
	SetTrackProcessing(context.Context, uuid.UUID, string, string) error
}

type Service struct {
	dir    string
	logger *slog.Logger
	store  Store
	jobs   chan job
	limit  chan struct{}
	mu     sync.Mutex
	closed bool
}

type job struct {
	track     Track
	sourceURL string
	deadline  time.Time
}

func New(dir string, logger *slog.Logger, store Store) *Service {
	return &Service{dir: dir, logger: logger, store: store,
		jobs: make(chan job, 16), limit: make(chan struct{}, 16)}
}

// Run owns the workers until ctx ends, then records cancellations and drains accepted jobs.
func (s *Service) Run(ctx context.Context) {
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			for {
				select {
				case next := <-s.jobs:
					s.process(ctx, next)
				case <-ctx.Done():
					return
				}
			}
		})
	}
	<-ctx.Done()
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	workers.Wait()
	for {
		select {
		case next := <-s.jobs:
			s.process(ctx, next)
		default:
			return
		}
	}
}

func (s *Service) Recover(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create upload directory: %w", err)
	}
	tracks, err := s.store.GetTracks(ctx)
	if err != nil {
		return err
	}
	for _, track := range tracks {
		if track.Status != "processing" {
			continue
		}
		if err := s.store.SetTrackProcessing(ctx, track.ID, "failed", "Processing was interrupted by a server restart. Remove the track and try again."); err != nil {
			return err
		}
		s.removeFiles(track.ID, true)
	}
	return os.RemoveAll(filepath.Join(s.dir, ".processing"))
}

func (s *Service) Upload(ctx context.Context, name string, typeID, tableID uuid.UUID, input io.Reader) (Track, error) {
	track, err := s.newTrack(name, typeID)
	if err != nil {
		return Track{}, err
	}
	path := s.inputPath(track.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return Track{}, err
	}
	queued := false
	defer func() {
		if !queued {
			s.removeFiles(track.ID, false)
		}
	}()
	file, err := os.Create(path)
	if err != nil {
		return Track{}, err
	}
	defer file.Close()
	if _, err := io.Copy(file, input); err != nil {
		return Track{}, err
	}
	if err := file.Close(); err != nil {
		return Track{}, err
	}
	if err := s.submit(ctx, track, tableID, ""); err != nil {
		return Track{}, err
	}
	queued = true
	return track, nil
}

func (s *Service) Import(ctx context.Context, name string, typeID, tableID uuid.UUID, sourceURL string) (Track, error) {
	source, ok := YouTubeURL(sourceURL)
	if !ok {
		return Track{}, ErrInvalidURL
	}
	if err := available(); err != nil {
		return Track{}, err
	}
	track, err := s.newTrack(name, typeID)
	if err != nil {
		return Track{}, err
	}
	if err := s.submit(ctx, track, tableID, source); err != nil {
		return Track{}, err
	}
	return track, nil
}

func (s *Service) newTrack(name string, typeID uuid.UUID) (Track, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Track{}, err
	}
	return Track{ID: id, CreatedAt: time.Now(), Name: name, TypeID: typeID,
		Path: filepath.Join(s.dir, id.String()), Status: "processing"}, nil
}

func (s *Service) submit(ctx context.Context, track Track, tableID uuid.UUID, sourceURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s.limit <- struct{}{}:
	default:
		return fmt.Errorf("audio processing queue is full")
	}
	var err error
	if tableID == uuid.Nil {
		err = s.store.SaveTrack(ctx, &track)
	} else {
		err = s.store.SaveTrackToTable(ctx, &track, tableID)
	}
	if err != nil {
		<-s.limit
		return err
	}
	s.jobs <- job{track: track, sourceURL: sourceURL, deadline: time.Now().Add(15 * time.Minute)}
	return nil
}

func (s *Service) process(ctx context.Context, next job) {
	defer func() { <-s.limit }()
	jobCtx, cancel := context.WithDeadline(ctx, next.deadline)
	defer cancel()
	err := jobCtx.Err()
	if err == nil {
		err = s.processAudio(jobCtx, next)
	}
	if err == nil {
		err = s.store.SetTrackProcessing(jobCtx, next.track.ID, "ready", "")
	}
	s.removeFiles(next.track.ID, err != nil)
	if err == nil {
		return
	}
	s.logger.Error("track processing failed", "trackID", next.track.ID, "error", err)
	message := "Audio processing failed. Remove the track and try again."
	if jobCtx.Err() != nil {
		message = "Audio processing timed out or was interrupted. Remove the track and try again."
	}
	failureCtx, failureCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer failureCancel()
	if err := s.store.SetTrackProcessing(failureCtx, next.track.ID, "failed", message); err != nil {
		s.logger.Error("failed to record track processing failure", "trackID", next.track.ID, "error", err)
	}
}

func (s *Service) processAudio(ctx context.Context, next job) error {
	input := s.inputPath(next.track.ID)
	if next.sourceURL != "" {
		if err := os.MkdirAll(filepath.Dir(input), 0700); err != nil {
			return err
		}
		if err := downloadVideo(ctx, next.sourceURL, input); err != nil {
			return err
		}
	}
	dir := next.track.Path
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create HLS directory: %w", err)
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", "-nostdin", "-i", input, "-v", "error",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2", "-ar", "44100",
		"-hls_time", "6", "-hls_playlist_type", "event",
		"-hls_segment_filename", filepath.Join(dir, "segment_%03d.ts"),
		"-vn", "-f", "hls", filepath.Join(dir, "index.m3u8"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("convert audio to HLS: %w: %s", err, stderr.String())
	}
	if _, err := generateWaveform(ctx, dir); err != nil {
		s.logger.Warn("failed to pre-generate waveform", "error", err, "trackID", next.track.ID)
	}
	return ctx.Err()
}

func (s *Service) inputPath(id uuid.UUID) string {
	return filepath.Join(s.dir, ".processing", id.String(), "source")
}

func (s *Service) removeFiles(id uuid.UUID, removeAudio bool) {
	paths := []string{filepath.Dir(s.inputPath(id))}
	if removeAudio {
		paths = append(paths, filepath.Join(s.dir, id.String()))
	}
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			s.logger.Error("failed to clean up track processing files", "path", path, "error", err)
		}
	}
}
