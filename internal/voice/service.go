// Package voice holds the use cases for streaming a table's live mix into a
// Discord voice channel.
package voice

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

const (
	// Discord voice handshakes can take several gateway round trips.
	tableTimeout   = 25 * time.Second
	setupTimeout   = 20 * time.Second
	cleanupTimeout = 5 * time.Second
)

var (
	ErrForbidden        = domain.Forbidden("Forbidden")
	ErrNotConfigured    = domain.NotFound("Discord is not configured")
	ErrNoDestination    = domain.Invalid("Select a Discord server and voice channel first")
	ErrSaveFailed       = domain.Internal("Unable to save Discord destination; connection cancelled")
	ErrMissingSettings  = domain.Internal("Playback snapshot is missing table settings")
	ErrMixRestoreFailed = domain.Unavailable("Unable to restore the table mix for Discord")
)

// Discord is the bot that joins voice channels and streams table mixes into them.
type Discord interface {
	Connect(ctx context.Context, tableID string, destination domain.Destination) error
	Disconnect(ctx context.Context, tableID string) error
	Resolve(context.Context, domain.Destination) (domain.Destination, error)
	Status(tableID string) domain.DiscordStatus
	Setup(context.Context) (domain.DiscordSetup, error)
	Channels(ctx context.Context, guildID string) ([]domain.DiscordChoice, error)
}

type Store interface {
	GetTableDiscord(context.Context, uuid.UUID) (*domain.Destination, error)
	SaveTableDiscord(context.Context, uuid.UUID, domain.Destination) error
	GetTrackTypes(context.Context) ([]domain.TrackType, error)
}

// Tables checks which table a viewer may stream to Discord.
type Tables interface {
	Resolve(ctx context.Context, viewer domain.Viewer, externalID string) (domain.Table, error)
}

// Playback reports what a table is currently playing.
type Playback interface {
	Snapshot(context.Context, uuid.UUID) (events.Snapshot, error)
}

type Service struct {
	discord  Discord
	store    Store
	tables   Tables
	playback Playback
	events   events.Sender
	logger   *slog.Logger
}

// New builds the voice service. A nil discord means Discord streaming is not configured.
func New(discord Discord, store Store, tables Tables, playback Playback, sender events.Sender, logger *slog.Logger) *Service {
	return &Service{
		discord:  discord,
		store:    store,
		tables:   tables,
		playback: playback,
		events:   sender,
		logger:   logger,
	}
}

// resolveTable checks the GM is using a known table before any Discord work happens.
func (s *Service) resolveTable(ctx context.Context, viewer domain.Viewer, externalID string) (domain.Table, error) {
	if !viewer.IsGM() {
		return domain.Table{}, ErrForbidden
	}
	return s.tables.Resolve(ctx, viewer, externalID)
}

func (s *Service) configured() bool {
	return s.discord != nil
}

// upstreamError keeps Discord's explanation visible to the GM.
func upstreamError(err error) error {
	if domain.IsUserFacing(err) {
		return err
	}
	return domain.Upstream(err.Error()).WithCause(err)
}
