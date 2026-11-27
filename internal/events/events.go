package events

import (
	"encoding/json"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

const (
	TrackPlaybackChanged = "trackPlaybackChanged"
	PlaybackStateChanged = "playbackStateChanged"
	TrackRemoved         = "trackRemoved"
	TableDeleted         = "tableDeleted"
	MixChanged           = "mixChanged"
	MixVolumesChanged    = "mixVolumesChanged"
	MixerReady           = "mixerReady"
	MixerApplied         = "mixerApplied"
	MixerFailed          = "mixerFailed"
	TrackStatusChanged   = "trackStatusChanged"
	TrackUpdated         = "trackUpdated"
	TagUpdated           = "tagUpdated"
	DiscordStatusChanged = "discordStatusChanged"
	TableSettingsChanged = "tableSettingsChanged"
	ClientConnected      = "clientConnected"
	TableSessionEnded    = "tableSessionEnded"
	PlaybackChanged      = "playbackChanged"
)

type Event struct {
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

// Sender dispatches an event synchronously; socket delivery remains buffered.
// SendEvent reports admission and encoding errors; handler failures are logged.
type Sender interface {
	SendEvent(event string, payload any) error
}

type Metadata struct {
	TableID        string
	ClientID       string
	Role           auth.Role
	TargetClientID string
	Global         bool
}

type Handler func(json.RawMessage, Metadata) error

type Bus interface {
	Sender
	On(event string, handler Handler) func()
}

type ClientEvent[T any] struct {
	TableID  string `json:"tableID"`
	ClientID string `json:"clientID"`
	Data     T      `json:"data"`
}

type PlaybackChanges[T any] struct {
	Tracks []T `json:"tracks"`
}

type MixerResult struct {
	Event string `json:"event"`
}

type MixerFailure struct {
	Event   string `json:"event"`
	Message string `json:"message"`
}

type TableEvent[T any] struct {
	TableID string `json:"tableID"`
	Data    T      `json:"data"`
}

type TrackReference struct {
	FileID string `json:"fileID"`
}
