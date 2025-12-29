package server

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Store interface {
	TrackStore
	TrackTypeStore
	TableStore
}

type Track struct {
	ID        uuid.UUID `json:"id,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	Name      string    `json:"name,omitempty"`
	Path      string    `json:"path,omitempty"`
	TypeID    uuid.UUID `json:"typeID,omitempty"`
}

type UpdateTrackRequest struct {
	ID     uuid.UUID  `json:"id"`
	Name   *string    `json:"name"`
	TypeID *uuid.UUID `json:"typeID"`
}

type TrackStore interface {
	SaveTrack(ctx context.Context, track *Track) error
	GetTracks(ctx context.Context) ([]Track, error)
	GetTrackByID(ctx context.Context, trackID uuid.UUID) (Track, error)
	DeleteTrack(ctx context.Context, trackID uuid.UUID) error
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


type Table struct {
	ID         uuid.UUID `json:"id,omitempty"`
	Name       string    `json:"name,omitempty"`
	InviteCode string    `json:"inviteCode,omitempty"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
}

type TableStore interface {
	GetTables(ctx context.Context) ([]Table, error)
}
