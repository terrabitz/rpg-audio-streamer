package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

// Playback returns what a table is playing. Players hear the master volume
// already applied and do not see the GM's mixer settings.
func (s *Service) Playback(ctx context.Context, viewer domain.Viewer, externalID string) (events.Snapshot, error) {
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return events.Snapshot{}, err
	}
	snapshot, err := s.playback.Snapshot(ctx, table.ID)
	if err != nil {
		return events.Snapshot{}, err
	}
	if !viewer.IsGM() {
		return snapshot.ForPlayer(), nil
	}
	return snapshot, nil
}
