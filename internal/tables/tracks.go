package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// Tracks lists the tracks on a table. Players do not see processing diagnostics.
func (s *Service) Tracks(ctx context.Context, viewer domain.Viewer, externalID string) ([]domain.Track, error) {
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return nil, err
	}
	tracks, err := s.store.GetTableTracks(ctx, table.ID)
	if err != nil {
		return nil, err
	}
	if !viewer.IsGM() {
		for i := range tracks {
			tracks[i].ProcessingError = ""
		}
	}
	return tracks, nil
}
