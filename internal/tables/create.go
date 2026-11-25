package tables

import (
	"context"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// Create starts a new table with a fresh player link.
func (s *Service) Create(ctx context.Context, viewer domain.Viewer, rawName string) (domain.Table, error) {
	if err := requireGM(viewer); err != nil {
		return domain.Table{}, err
	}
	name, err := domain.ParseTableName(rawName)
	if err != nil {
		return domain.Table{}, err
	}
	return s.store.CreateTable(ctx, domain.Table{
		ID:   uuid.New(),
		Name: name,
	})
}
