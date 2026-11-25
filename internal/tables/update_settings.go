package tables

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

// UpdateSettings applies a partial settings change and shares it with the table's clients.
func (s *Service) UpdateSettings(ctx context.Context, viewer domain.Viewer, externalID string, patch domain.TableSettingsPatch) (domain.TableSettings, error) {
	if err := requireGM(viewer); err != nil {
		return domain.TableSettings{}, err
	}
	table, err := s.Resolve(ctx, viewer, externalID)
	if err != nil {
		return domain.TableSettings{}, err
	}
	if err := patch.Validate(); err != nil {
		return domain.TableSettings{}, err
	}
	settings, err := s.store.UpdateTableSettings(ctx, table.ID, patch)
	if err != nil {
		return domain.TableSettings{}, err
	}
	err = s.sendEvent(events.TableSettingsChanged, events.TableEvent[domain.TableSettingsPatch]{
		TableID: table.ID.String(),
		Data:    patch,
	})
	if err != nil {
		return domain.TableSettings{}, err
	}
	return settings, nil
}
