package server

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Store interface {
	TrackStore
	TrackTypeStore
}

type Track struct {
	ID        uuid.UUID `json:"id,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Name      string    `json:"name,omitempty"`
	Path      string    `json:"path,omitempty"`
	TypeID    uuid.UUID `json:"type_id,omitempty"`
}

type TrackStore interface {
	SaveTrack(ctx context.Context, track *Track) error
	GetTracks(ctx context.Context) ([]Track, error)
	GetTrackByID(ctx context.Context, trackID uuid.UUID) (Track, error)
	DeleteTrack(ctx context.Context, trackID uuid.UUID) error
}

type TrackType struct {
	ID                    uuid.UUID `json:"id,omitempty"`
	Name                  string    `json:"name,omitempty"`
	Color                 string    `json:"color,omitempty"`
	IsRepeating           bool      `json:"is_repeating"`
	AllowSimultaneousPlay bool      `json:"allow_simultaneous_play"`
}

type TrackTypeStore interface {
	GetTrackTypes(ctx context.Context) ([]TrackType, error)
	GetTrackTypeByID(ctx context.Context, id uuid.UUID) (TrackType, error)
}
