package library

import (
	"context"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// TrackUsage counts the tables a track is on, so the GM can confirm a delete.
func (s *Service) TrackUsage(ctx context.Context, viewer domain.Viewer, trackID uuid.UUID) (int64, error) {
	if err := requireGM(viewer); err != nil {
		return 0, err
	}
	return s.store.CountTrackTables(ctx, trackID)
}
