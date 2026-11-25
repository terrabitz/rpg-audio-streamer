// Package library holds the use cases for the GM's audio library: uploading,
// importing, editing, streaming, and deleting tracks.
package library

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

var (
	ErrForbidden         = domain.Forbidden("Forbidden")
	ErrTrackNotFound     = domain.NotFound("Track not found")
	ErrTrackNotReady     = domain.Conflict("Track is not ready")
	ErrTrackProcessing   = domain.Conflict("Track is still processing")
	ErrTableUsageChanged = domain.Conflict("Track table usage changed")
	ErrInvalidTrackType  = domain.Invalid("Invalid track type")
	ErrQueueFailed       = domain.Unavailable("Failed to queue audio processing. Please try again.")
)

type Store interface {
	GetTracks(context.Context) ([]domain.Track, error)
	GetTrackByID(context.Context, uuid.UUID) (domain.Track, error)
	UpdateTrack(context.Context, uuid.UUID, domain.TrackUpdate) (domain.Track, error)
	DeleteTrack(context.Context, uuid.UUID) error
	DeleteTrackIfTableCount(ctx context.Context, trackID uuid.UUID, expectedCount int64) (bool, error)
	CountTrackTables(context.Context, uuid.UUID) (int64, error)
	GetTrackTypes(context.Context) ([]domain.TrackType, error)
	GetTrackTypeByID(context.Context, uuid.UUID) (domain.TrackType, error)
	GetTable(context.Context, uuid.UUID) (domain.Table, error)
	HasTableTrack(ctx context.Context, tableID, trackID uuid.UUID) (bool, error)
	GetTrackWaveform(context.Context, uuid.UUID) (domain.Waveform, error)
	SaveTrackWaveform(context.Context, uuid.UUID, domain.Waveform) error
}

// Audio converts source audio into streamable tracks and manages their files.
type Audio interface {
	Upload(ctx context.Context, name string, typeID, tableID uuid.UUID, input io.Reader) (domain.Track, error)
	Import(ctx context.Context, name string, typeID, tableID uuid.UUID, sourceURL string) (domain.Track, error)
	Remove(domain.Track) error
	GenerateWaveform(context.Context, domain.Track) (domain.Waveform, error)
}

// Videos looks up remote videos that can be imported as tracks.
type Videos interface {
	ValidateURL(sourceURL string) error
	Scan(ctx context.Context, sourceURL string) (domain.VideoMetadata, error)
}

// Tables checks which table a viewer may stream tracks from.
type Tables interface {
	Resolve(ctx context.Context, viewer domain.Viewer, externalID string) (domain.Table, error)
}

type Service struct {
	store     Store
	audio     Audio
	videos    Videos
	tables    Tables
	events    events.Sender
	uploadDir string
}

func New(store Store, audio Audio, videos Videos, tables Tables, sender events.Sender, uploadDir string) *Service {
	return &Service{
		store:     store,
		audio:     audio,
		videos:    videos,
		tables:    tables,
		events:    sender,
		uploadDir: uploadDir,
	}
}

func requireGM(viewer domain.Viewer) error {
	if !viewer.IsGM() {
		return ErrForbidden
	}
	return nil
}

func (s *Service) sendEvent(event string, payload any) error {
	if err := s.events.SendEvent(event, payload); err != nil {
		return domain.ErrLiveSyncFailed.WithCause(err)
	}
	return nil
}

// requireTrackType turns an unknown track type into a validation error for the uploader.
func (s *Service) requireTrackType(ctx context.Context, typeID uuid.UUID) error {
	if _, err := s.store.GetTrackTypeByID(ctx, typeID); err != nil {
		return ErrInvalidTrackType.WithCause(err)
	}
	return nil
}

func (s *Service) requireTable(ctx context.Context, tableID uuid.UUID) error {
	if tableID == uuid.Nil {
		return nil
	}
	_, err := s.store.GetTable(ctx, tableID)
	return err
}

func queueError(err error) error {
	if domain.IsUserFacing(err) {
		return err
	}
	return ErrQueueFailed.WithCause(err)
}
