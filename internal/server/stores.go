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
	ID        uuid.UUID
	CreatedAt time.Time
	Name      string
	Path      string
	Type      string
}

type TrackStore interface {
	SaveTrack(ctx context.Context, track *Track) error
}
