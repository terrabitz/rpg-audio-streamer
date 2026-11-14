package server

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
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

type TableSettings struct {
	MasterVolume float64            `json:"masterVolume"`
	TypeVolumes  map[string]float64 `json:"typeVolumes"`
	TrackVolumes map[string]float64 `json:"trackVolumes"`
	TrackOrder   []uuid.UUID        `json:"trackOrder"`
}

type TableSettingsPatch struct {
	MasterVolume *float64           `json:"masterVolume,omitempty"`
	TypeVolumes  map[string]float64 `json:"typeVolumes,omitempty"`
	TrackVolumes map[string]float64 `json:"trackVolumes,omitempty"`
	TrackOrder   *[]uuid.UUID       `json:"trackOrder,omitempty"`
}

type Track struct {
	ID        uuid.UUID        `json:"id,omitempty"`
	CreatedAt time.Time        `json:"createdAt,omitempty"`
	Name      string           `json:"name,omitempty"`
	Path      string           `json:"path,omitempty"`
	TypeID    uuid.UUID        `json:"typeID,omitempty"`
	Playback  PlaybackSettings `json:"playback"`
}

type PlaybackSettings struct {
	StartTime    float64  `json:"startTime"`
	EndTime      *float64 `json:"endTime"`
	LoopBackTime *float64 `json:"loopBackTime"`
}

func (p PlaybackSettings) Validate() error {
	if math.IsNaN(p.StartTime) || math.IsInf(p.StartTime, 0) || p.StartTime < 0 {
		return fmt.Errorf("beginning must be a finite, nonnegative time")
	}
	if p.EndTime != nil && (math.IsNaN(*p.EndTime) || math.IsInf(*p.EndTime, 0) || *p.EndTime <= p.StartTime) {
		return fmt.Errorf("end must be after the beginning")
	}
	if p.LoopBackTime != nil && (math.IsNaN(*p.LoopBackTime) || math.IsInf(*p.LoopBackTime, 0) ||
		*p.LoopBackTime < p.StartTime || (p.EndTime != nil && *p.LoopBackTime >= *p.EndTime)) {
		return fmt.Errorf("loop-back point must be at or after the beginning and before the end")
	}
	return nil
}

type UpdateTrackRequest struct {
	ID       uuid.UUID         `json:"id"`
	Name     *string           `json:"name"`
	TypeID   *uuid.UUID        `json:"typeID"`
	Playback *PlaybackSettings `json:"playback"`
}

type TrackStore interface {
	SaveTrack(ctx context.Context, track *Track) error
	GetTracks(ctx context.Context) ([]Track, error)
	GetTrackByID(ctx context.Context, trackID uuid.UUID) (Track, error)
	DeleteTrack(ctx context.Context, trackID uuid.UUID) error
	DeleteTrackIfTableCount(ctx context.Context, trackID uuid.UUID, expectedCount int64) (bool, error)
	UpdateTrack(ctx context.Context, trackID uuid.UUID, update UpdateTrackRequest) (Track, error)
}

type TrackType struct {
	ID                    uuid.UUID `json:"id,omitempty"`
	Name                  string    `json:"name,omitempty"`
	Color                 string    `json:"color,omitempty"`
	IsRepeating           bool      `json:"isRepeating"`
	AllowSimultaneousPlay bool      `json:"allowSimultaneousPlay"`
}

type TrackTypeStore interface {
	GetTrackTypes(ctx context.Context) ([]TrackType, error)
	GetTrackTypeByID(ctx context.Context, id uuid.UUID) (TrackType, error)
}
