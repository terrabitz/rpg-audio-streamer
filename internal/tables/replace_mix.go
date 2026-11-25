package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

// ReplaceMix sends the GM's complete local mix to the table's live mixer.
func (s *Service) ReplaceMix(ctx context.Context, viewer domain.Viewer, externalID string, mix events.MixSnapshot) error {
	if err := requireGM(viewer); err != nil {
		return err
	}
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return err
	}
	if mix.Tracks == nil {
		return ErrMixRequiresTracks
	}
	return s.dispatchMix(events.MixChanged, events.TableEvent[events.MixSnapshot]{
		TableID: table.ID.String(),
		Data:    mix,
	})
}

func (s *Service) dispatchMix(event string, payload any) error {
	if err := s.events.SendEvent(event, payload); err != nil {
		return ErrMixDispatchFailed.WithCause(err)
	}
	return nil
}
