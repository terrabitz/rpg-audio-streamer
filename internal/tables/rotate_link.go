package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// RotateLink replaces a table's player link and disconnects players who used the old one.
func (s *Service) RotateLink(ctx context.Context, viewer domain.Viewer, externalID string) (domain.Table, error) {
	if err := requireGM(viewer); err != nil {
		return domain.Table{}, err
	}
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return domain.Table{}, err
	}
	rotated, err := s.store.RotateTableExternalID(ctx, table.ID)
	if err != nil {
		return domain.Table{}, err
	}
	s.access.RevokePlayerAccess(table.ID.String())
	return rotated, nil
}
