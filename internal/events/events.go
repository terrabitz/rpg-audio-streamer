package events

import "encoding/json"

const (
	Ping                  = "ping"
	Pong                  = "pong"
	Broadcast             = "broadcast"
	PlaybackSyncRequested = "playbackSyncRequested"
	TrackPlaybackChanged  = "trackPlaybackChanged"
	PlaybackStateChanged  = "playbackStateChanged"
	TrackRemoved          = "trackRemoved"
	TableDeleted          = "tableDeleted"
	MixChanged            = "mixChanged"
	MixVolumesChanged     = "mixVolumesChanged"
	MixerReady            = "mixerReady"
	MixerApplied          = "mixerApplied"
	MixerFailed           = "mixerFailed"
	PlaybackFailed        = "playbackFailed"
	TrackStatusChanged    = "trackStatusChanged"
	DiscordStatusChanged  = "discordStatusChanged"
	TableSettingsChanged  = "tableSettingsChanged"
)

type Event struct {
	Event    string          `json:"event"`
	Payload  json.RawMessage `json:"payload"`
	SenderID string          `json:"senderId,omitempty"`
}

// Sender accepts an event without waiting for handlers or socket delivery.
// An error means the event was not accepted.
type Sender interface {
	SendEvent(event string, payload any) error
}

type TableEvent[T any] struct {
	TableID string `json:"tableID"`
	Data    T      `json:"data"`
}

type TrackReference struct {
	FileID string `json:"fileID"`
}
