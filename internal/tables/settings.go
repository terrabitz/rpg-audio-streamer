package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// Settings returns a table's mixer settings.
func (s *Service) Settings(ctx context.Context, viewer domain.Viewer, externalID string) (domain.TableSettings, error) {
	if err := requireGM(viewer); err != nil {
		return domain.TableSettings{}, err
	}
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return domain.TableSettings{}, err
	}
	return s.store.GetTableSettings(ctx, table.ID)
}
