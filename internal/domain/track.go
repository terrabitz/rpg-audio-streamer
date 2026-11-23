package domain

import (
	"time"

	"github.com/google/uuid"
)

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
}

func (t Track) Ready() bool {
	return t.Status == "" || t.Status == "ready"
}
