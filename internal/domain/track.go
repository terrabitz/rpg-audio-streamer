package domain

import (
	"time"

	"github.com/google/uuid"
)

type TrackType struct {
	ID                    uuid.UUID `json:"id,omitempty"`
	Name                  string    `json:"name,omitempty"`
	Color                 string    `json:"color,omitempty"`
	IsRepeating           bool      `json:"isRepeating"`
	AllowSimultaneousPlay bool      `json:"allowSimultaneousPlay"`
}

type Track struct {
	ID              uuid.UUID        `json:"id,omitempty"`
	CreatedAt       time.Time        `json:"createdAt,omitempty"`
	Name            string           `json:"name,omitempty"`
	Path            string           `json:"path,omitempty"`
	TypeID          uuid.UUID        `json:"typeID,omitempty"`
	Volume          float64          `json:"volume"`
	Playback        PlaybackSettings `json:"playback"`
	Status          string           `json:"status,omitempty"`
	ProcessingError string           `json:"processingError,omitempty"`
	TagIDs          []uuid.UUID      `json:"tagIDs,omitempty"`
}

func (t Track) Ready() bool {
	return t.Status == "" || t.Status == "ready"
}
