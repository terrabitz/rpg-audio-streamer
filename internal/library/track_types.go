package library

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

var ErrTrackTypesFailed = domain.Internal("Failed to retrieve track types")

// TrackTypes lists the categories a track can belong to, such as music or ambience.
// Players see them too, so their table can group tracks the same way.
func (s *Service) TrackTypes(ctx context.Context) ([]domain.TrackType, error) {
	types, err := s.store.GetTrackTypes(ctx)
	if err != nil {
		return nil, ErrTrackTypesFailed.WithCause(err)
	}
	return types, nil
}
