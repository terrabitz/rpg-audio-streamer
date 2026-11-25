// Package tables holds the use cases for game tables: their links, settings,
// track lists, and live playback.
package tables

import (
	"context"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

var (
	ErrForbidden          = domain.Forbidden("Forbidden")
	ErrMixRequiresTracks  = domain.Invalid("Mix snapshot requires tracks")
	ErrMixRequiresVolumes = domain.Invalid("Mix patch requires typeVolumes")
	ErrMixDispatchFailed  = domain.Unavailable("Unable to dispatch mixer command")
)

type Store interface {
	GetTables(context.Context) ([]domain.Table, error)
	GetTable(context.Context, uuid.UUID) (domain.Table, error)
	GetTableByExternalID(context.Context, string) (domain.Table, error)
	CreateTable(context.Context, domain.Table) (domain.Table, error)
	UpdateTable(context.Context, uuid.UUID, string) (domain.Table, error)
	RotateTableExternalID(context.Context, uuid.UUID) (domain.Table, error)
	DeleteTable(context.Context, uuid.UUID) error
	GetTableSettings(context.Context, uuid.UUID) (domain.TableSettings, error)
	UpdateTableSettings(context.Context, uuid.UUID, domain.TableSettingsPatch) (domain.TableSettings, error)
	GetTableTracks(context.Context, uuid.UUID) ([]domain.Track, error)
	AddTableTrack(context.Context, uuid.UUID, uuid.UUID) error
	RemoveTableTrack(context.Context, uuid.UUID, uuid.UUID) error
}

// Playback tracks what each table is currently playing.
type Playback interface {
	Snapshot(context.Context, uuid.UUID) (events.Snapshot, error)
	Update(context.Context, uuid.UUID, uuid.UUID, events.PlaybackPatch) error
}

// PlayerAccess ends live sessions of players who joined through an old table link.
type PlayerAccess interface {
	RevokePlayerAccess(tableID string)
}

type Service struct {
	store    Store
	playback Playback
	access   PlayerAccess
	events   events.Sender
}

func New(store Store, playback Playback, access PlayerAccess, sender events.Sender) *Service {
	return &Service{
		store:    store,
		playback: playback,
		access:   access,
		events:   sender,
	}
}

func requireGM(viewer domain.Viewer) error {
	if !viewer.IsGM() {
		return ErrForbidden
	}
	return nil
}

func (s *Service) sendEvent(event string, payload any) error {
	if err := s.events.SendEvent(event, payload); err != nil {
		return domain.ErrLiveSyncFailed.WithCause(err)
	}
	return nil
}
