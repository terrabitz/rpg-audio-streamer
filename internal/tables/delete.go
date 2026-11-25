package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

// Delete removes a table and tells its connected clients that it is gone.
func (s *Service) Delete(ctx context.Context, viewer domain.Viewer, externalID string) error {
	if err := requireGM(viewer); err != nil {
		return err
	}
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return err
	}
	if err := s.store.DeleteTable(ctx, table.ID); err != nil {
		return err
	}
	return s.sendEvent(events.TableDeleted, table.ID.String())
}
