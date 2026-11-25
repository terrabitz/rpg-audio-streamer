package library

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

type Upload struct {
	Name   string
	TypeID uuid.UUID
	// TableID optionally adds the new track to a table; uuid.Nil keeps it library-only.
	TableID uuid.UUID
	Audio   io.Reader
}

// Upload queues an uploaded audio file for processing into a new track.
func (s *Service) Upload(ctx context.Context, viewer domain.Viewer, upload Upload) (domain.Track, error) {
	if err := requireGM(viewer); err != nil {
		return domain.Track{}, err
	}
	if err := s.requireTable(ctx, upload.TableID); err != nil {
		return domain.Track{}, err
	}
	if err := s.requireTrackType(ctx, upload.TypeID); err != nil {
		return domain.Track{}, err
	}
	track, err := s.audio.Upload(ctx, upload.Name, upload.TypeID, upload.TableID, upload.Audio)
	if err != nil {
		return domain.Track{}, queueError(err)
	}
	return track, nil
}
