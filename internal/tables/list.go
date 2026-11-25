package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// List returns every table the GM has created.
func (s *Service) List(ctx context.Context, viewer domain.Viewer) ([]domain.Table, error) {
	if err := requireGM(viewer); err != nil {
		return nil, err
	}
	return s.store.GetTables(ctx)
}
