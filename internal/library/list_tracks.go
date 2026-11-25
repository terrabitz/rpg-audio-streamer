package library

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

var ErrListTracksFailed = domain.Internal("Failed to list files")

// ListTracks returns every track in the library.
func (s *Service) ListTracks(ctx context.Context, viewer domain.Viewer) ([]domain.Track, error) {
	if err := requireGM(viewer); err != nil {
		return nil, err
	}
	tracks, err := s.store.GetTracks(ctx)
	if err != nil {
		return nil, ErrListTracksFailed.WithCause(err)
	}
	return tracks, nil
}
