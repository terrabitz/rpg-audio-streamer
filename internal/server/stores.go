package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
)

type Store interface {
	TrackStore
	TrackTypeStore
	TableStore
}

var DefaultTableID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

type Table struct {
	ID         uuid.UUID `json:"id"`
	ExternalID string    `json:"externalID"`
	Name       string    `json:"name"`
}

type TableStore interface {
	GetTableDiscord(context.Context, uuid.UUID) (*discord.Destination, error)
	SaveTableDiscord(context.Context, uuid.UUID, discord.Destination) error
	GetTables(context.Context) ([]Table, error)
	GetTable(context.Context, uuid.UUID) (Table, error)
	GetTableByExternalID(context.Context, string) (Table, error)
	CreateTable(context.Context, Table) (Table, error)
	UpdateTable(context.Context, uuid.UUID, string) (Table, error)
	RotateTableExternalID(context.Context, uuid.UUID) (Table, error)
	GetTableSettings(context.Context, uuid.UUID) (TableSettings, error)
	UpdateTableSettings(context.Context, uuid.UUID, TableSettingsPatch) (TableSettings, error)
	DeleteTable(context.Context, uuid.UUID) error
	GetTableTracks(context.Context, uuid.UUID) ([]Track, error)
	AddTableTrack(context.Context, uuid.UUID, uuid.UUID) error
	RemoveTableTrack(context.Context, uuid.UUID, uuid.UUID) error
	HasTableTrack(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	CountTrackTables(context.Context, uuid.UUID) (int64, error)
	SaveTrackToTable(context.Context, *Track, uuid.UUID) error
}

type TableSettings = domain.TableSettings

type TableSettingsPatch struct {
	MasterVolume *float64           `json:"masterVolume,omitempty"`
	TypeVolumes  map[string]float64 `json:"typeVolumes,omitempty"`
	TrackOrder   *[]uuid.UUID       `json:"trackOrder,omitempty"`
}

type Track = musicprocessing.Track

type PlaybackSettings = musicprocessing.PlaybackSettings

type UpdateTrackRequest struct {
	ID       uuid.UUID         `json:"id"`
	Name     *string           `json:"name"`
	TypeID   *uuid.UUID        `json:"typeID"`
	Volume   *float64          `json:"volume"`
	Playback *PlaybackSettings `json:"playback"`
}

type TrackStore interface {
	SaveTrack(ctx context.Context, track *Track) error
	GetTracks(ctx context.Context) ([]Track, error)
	GetTrackByID(ctx context.Context, trackID uuid.UUID) (Track, error)
	DeleteTrack(ctx context.Context, trackID uuid.UUID) error
	DeleteTrackIfTableCount(ctx context.Context, trackID uuid.UUID, expectedCount int64) (bool, error)
	UpdateTrack(ctx context.Context, trackID uuid.UUID, update UpdateTrackRequest) (Track, error)
	SetTrackProcessing(ctx context.Context, trackID uuid.UUID, status, message string) error
	GetTrackWaveform(ctx context.Context, trackID uuid.UUID) (musicprocessing.Waveform, error)
	SaveTrackWaveform(ctx context.Context, trackID uuid.UUID, waveform musicprocessing.Waveform) error
}

type TrackType = domain.TrackType

type TrackTypeStore interface {
	GetTrackTypes(ctx context.Context) ([]TrackType, error)
	GetTrackTypeByID(ctx context.Context, id uuid.UUID) (TrackType, error)
}
