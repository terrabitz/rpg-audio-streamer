package domain

import (
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidTrackVolume = Invalid("Invalid track volume")
	ErrInvalidTrackName   = Invalid("Track name must be between 1 and 200 characters")
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
}

func (t Track) Ready() bool {
	return t.Status == "" || t.Status == "ready"
}

func (t Track) Processing() bool {
	return t.Status == "processing"
}

// TrackUpdate changes only the track fields that are set.
type TrackUpdate struct {
	Name     *string           `json:"name"`
	TypeID   *uuid.UUID        `json:"typeID"`
	Volume   *float64          `json:"volume"`
	Playback *PlaybackSettings `json:"playback"`
}

func (u TrackUpdate) Validate() error {
	if u.Playback != nil {
		if err := u.Playback.Validate(); err != nil {
			return Invalid(err.Error())
		}
	}
	if u.Volume != nil && !ValidTrackVolume(*u.Volume) {
		return ErrInvalidTrackVolume
	}
	return nil
}

func ValidTrackVolume(volume float64) bool {
	return volume >= 0 && volume <= 100
}

// Waveform holds normalized peak amplitudes used to draw a track overview.
type Waveform struct {
	Peaks []float64 `json:"peaks"`
}
