package server

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Store interface {
	TrackStore
}

type Track struct {
	ID        uuid.UUID `json:"id,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Name      string    `json:"name,omitempty"`
	Path      string    `json:"path,omitempty"`
	Type      string    `json:"type,omitempty"`
}

type TrackStore interface {
	SaveTrack(ctx context.Context, track *Track) error
	GetTracks(ctx context.Context) ([]Track, error)
}
