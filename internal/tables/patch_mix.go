package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

// PatchMix changes the live mixer's per-type volumes.
func (s *Service) PatchMix(ctx context.Context, viewer domain.Viewer, externalID string, patch events.MixVolumePatch) error {
	if err := requireGM(viewer); err != nil {
		return err
	}
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return err
	}
	if patch.TypeVolumes == nil {
		return ErrMixRequiresVolumes
	}
	return s.dispatchMix(events.MixVolumesChanged, events.TableEvent[events.MixVolumePatch]{
		TableID: table.ID.String(),
		Data:    patch,
	})
}
