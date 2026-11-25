package tables

import (
	"context"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// AddTrack puts a library track on a table.
func (s *Service) AddTrack(ctx context.Context, viewer domain.Viewer, externalID string, trackID uuid.UUID) error {
	if err := requireGM(viewer); err != nil {
		return err
	}
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return err
	}
	return s.store.AddTableTrack(ctx, table.ID, trackID)
}
