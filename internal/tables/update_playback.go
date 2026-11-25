package tables

import (
	"context"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

// UpdatePlayback plays, pauses, seeks, or adjusts one track on a table.
func (s *Service) UpdatePlayback(ctx context.Context, viewer domain.Viewer, externalID string, trackID uuid.UUID, patch events.PlaybackPatch) error {
	if err := requireGM(viewer); err != nil {
		return err
	}
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return err
	}
	return s.playback.Update(ctx, table.ID, trackID, patch)
}
