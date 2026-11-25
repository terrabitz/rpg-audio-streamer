package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// Find looks up a table by the link players use to join it.
func (s *Service) Find(ctx context.Context, externalID string) (domain.Table, error) {
	return s.store.GetTableByExternalID(ctx, externalID)
}

// Resolve finds a table by its link and checks that the viewer may use it.
func (s *Service) Resolve(ctx context.Context, viewer domain.Viewer, externalID string) (domain.Table, error) {
	table, err := s.Find(ctx, externalID)
	if err != nil {
		return domain.Table{}, err
	}
	if !viewer.CanAccessTable(table.ID) {
		return domain.Table{}, ErrForbidden
	}
	return table, nil
}
