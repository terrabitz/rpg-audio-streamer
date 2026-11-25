package library

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

var ErrUpdateTrackFailed = domain.Internal("Failed to update track")

// UpdateTrack edits a track's name, type, volume, or playback range.
func (s *Service) UpdateTrack(ctx context.Context, viewer domain.Viewer, trackID uuid.UUID, update domain.TrackUpdate) (domain.Track, error) {
	if err := requireGM(viewer); err != nil {
		return domain.Track{}, err
	}
	if err := update.Validate(); err != nil {
		return domain.Track{}, err
	}
	track, err := s.store.UpdateTrack(ctx, trackID, update)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Track{}, ErrTrackNotFound.WithCause(err)
	}
	if err != nil {
		return domain.Track{}, ErrUpdateTrackFailed.WithCause(err)
	}
	if err := s.sendEvent(events.TrackUpdated, track.ID.String()); err != nil {
		return domain.Track{}, err
	}
	return track, nil
}
