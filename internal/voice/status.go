package voice

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

var disconnected = domain.DiscordStatus{State: "disconnected"}

// Status reports whether a table is streaming to Discord and where.
func (s *Service) Status(ctx context.Context, viewer domain.Viewer, externalID string) (domain.DiscordStatus, error) {
	table, err := s.resolveTable(ctx, viewer, externalID)
	if err != nil {
		return domain.DiscordStatus{}, err
	}
	if !s.configured() {
		return disconnected, nil
	}
	return s.currentStatus(ctx, table)
}

// currentStatus falls back to the saved destination so the GM can reconnect with one click.
func (s *Service) currentStatus(ctx context.Context, table domain.Table) (domain.DiscordStatus, error) {
	status := s.discord.Status(table.ID.String())
	if status.Destination != nil {
		return status, nil
	}
	saved, err := s.store.GetTableDiscord(ctx, table.ID)
	if err != nil {
		return domain.DiscordStatus{}, err
	}
	status.Destination = saved
	return status, nil
}
