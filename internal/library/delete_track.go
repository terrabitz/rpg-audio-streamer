package library

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

var (
	ErrDeleteRecordFailed = domain.Internal("Failed to remove track record")
	ErrDeleteFilesFailed  = domain.Internal("Failed to delete folder")
)

// DeleteTrack removes a track from the library and every table.
//
// When expectedTableCount is set, the delete only happens if the track is
// still on that many tables, so the GM never removes a track from a table
// they were not warned about.
func (s *Service) DeleteTrack(ctx context.Context, viewer domain.Viewer, trackID uuid.UUID, expectedTableCount *int64) error {
	if err := requireGM(viewer); err != nil {
		return err
	}
	track, err := s.findTrack(ctx, trackID)
	if err != nil {
		return err
	}
	if track.Processing() {
		return ErrTrackProcessing
	}
	if err := s.deleteRecord(ctx, trackID, expectedTableCount); err != nil {
		return err
	}
	removeErr := s.audio.Remove(track)
	err = s.sendEvent(events.TrackRemoved, events.TableEvent[events.TrackReference]{
		Data: events.TrackReference{FileID: trackID.String()},
	})
	if err != nil {
		return err
	}
	if removeErr != nil {
		return ErrDeleteFilesFailed.WithCause(removeErr)
	}
	return nil
}

func (s *Service) findTrack(ctx context.Context, trackID uuid.UUID) (domain.Track, error) {
	track, err := s.store.GetTrackByID(ctx, trackID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Track{}, ErrTrackNotFound.WithCause(err)
	}
	return track, err
}

func (s *Service) deleteRecord(ctx context.Context, trackID uuid.UUID, expectedTableCount *int64) error {
	if expectedTableCount == nil {
		if err := s.store.DeleteTrack(ctx, trackID); err != nil {
			return ErrDeleteRecordFailed.WithCause(err)
		}
		return nil
	}
	deleted, err := s.store.DeleteTrackIfTableCount(ctx, trackID, *expectedTableCount)
	if err != nil {
		return ErrDeleteRecordFailed.WithCause(err)
	}
	if deleted {
		return nil
	}
	// A track deleted by someone else reports as missing rather than as changed usage.
	if _, err := s.store.CountTrackTables(ctx, trackID); err != nil {
		return err
	}
	return ErrTableUsageChanged
}
