package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// Get returns a table to the GM or to a player who joined it.
func (s *Service) Get(ctx context.Context, viewer domain.Viewer, externalID string) (domain.Table, error) {
	return s.Resolve(ctx, viewer, externalID)
}
