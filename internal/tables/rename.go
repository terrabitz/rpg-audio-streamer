package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// Rename changes a table's display name.
func (s *Service) Rename(ctx context.Context, viewer domain.Viewer, externalID, rawName string) (domain.Table, error) {
	if err := requireGM(viewer); err != nil {
		return domain.Table{}, err
	}
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return domain.Table{}, err
	}
	name, err := domain.ParseTableName(rawName)
	if err != nil {
		return domain.Table{}, err
	}
	return s.store.UpdateTable(ctx, table.ID, name)
}
