package voice

import (
	"context"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

// Setup lists the Discord servers the bot has joined and how to invite it to more.
func (s *Service) Setup(ctx context.Context, viewer domain.Viewer) (domain.DiscordSetup, error) {
	if !viewer.IsGM() {
		return domain.DiscordSetup{}, ErrForbidden
	}
	if !s.configured() {
		return domain.DiscordSetup{}, ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	setup, err := s.discord.Setup(ctx)
	if err != nil {
		return domain.DiscordSetup{}, upstreamError(err)
	}
	return setup, nil
}

// Channels lists the voice channels the bot can join in a Discord server.
func (s *Service) Channels(ctx context.Context, viewer domain.Viewer, guildID string) ([]domain.DiscordChoice, error) {
	if !viewer.IsGM() {
		return nil, ErrForbidden
	}
	if !s.configured() {
		return nil, ErrNotConfigured
	}
	ctx, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	channels, err := s.discord.Channels(ctx, guildID)
	if err != nil {
		return nil, upstreamError(err)
	}
	return channels, nil
}
