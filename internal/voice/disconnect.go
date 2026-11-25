package voice

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// Disconnect stops streaming a table to Discord.
func (s *Service) Disconnect(ctx context.Context, viewer domain.Viewer, externalID string) (domain.DiscordStatus, error) {
	table, err := s.resolveTable(ctx, viewer, externalID)
	if err != nil {
		return domain.DiscordStatus{}, err
	}
	if !s.configured() {
		return domain.DiscordStatus{}, ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, tableTimeout)
	defer cancel()

	if err := s.discord.Disconnect(ctx, table.ID.String()); err != nil {
		return domain.DiscordStatus{}, upstreamError(err)
	}
	return s.currentStatus(ctx, table)
}
