package tables

import (
	"context"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

// RemoveTrack takes a track off a table and stops it for the table's clients.
func (s *Service) RemoveTrack(ctx context.Context, viewer domain.Viewer, externalID string, trackID uuid.UUID) error {
	if err := requireGM(viewer); err != nil {
		return err
	}
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return err
	}
	if err := s.store.RemoveTableTrack(ctx, table.ID, trackID); err != nil {
		return err
	}
	return s.sendEvent(events.TrackRemoved, events.TableEvent[events.TrackReference]{
		TableID: table.ID.String(),
		Data:    events.TrackReference{FileID: trackID.String()},
	})
}
