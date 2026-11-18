package musicprocessing

import (
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

type Track struct {
	ID              uuid.UUID        `json:"id,omitempty"`
	CreatedAt       time.Time        `json:"createdAt,omitempty"`
	Name            string           `json:"name,omitempty"`
	Path            string           `json:"path,omitempty"`
	TypeID          uuid.UUID        `json:"typeID,omitempty"`
	Playback        PlaybackSettings `json:"playback"`
	Status          string           `json:"status,omitempty"`
	ProcessingError string           `json:"processingError,omitempty"`
}

func (t Track) Ready() bool {
	return t.Status == "" || t.Status == "ready"
}

type PlaybackSettings = domain.PlaybackSettings
