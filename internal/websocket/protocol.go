package websocket

import (
	"encoding/json"

	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

const (
	Ping          = "ping"
	Pong          = "pong"
	Broadcast     = "broadcast"
	SyncRequest   = "syncRequest"
	SyncTrack     = "syncTrack"
	SyncAll       = "syncAll"
	RemoveTrack   = "removeTrack"
	TableDeleted  = "tableDeleted"
	MixSync       = "mixSync"
	MixVolumes    = "mixVolumes"
	MixerReady    = "mixerReady"
	MixerAck      = "mixerAck"
	MixerError    = "mixerError"
	PlaybackError = "playbackError"
	TrackStatus   = "trackStatus"
)

type Message struct {
	Method   string          `json:"method"`
	Payload  json.RawMessage `json:"payload"`
	SenderID string          `json:"senderId,omitempty"`
}

type Empty struct{}

type TrackReference struct {
	FileID string `json:"fileID"`
}

type Snapshot struct {
	Tracks   []playingTrack        `json:"tracks"`
	Settings *server.TableSettings `json:"settings,omitempty"`
}

type TrackStatusUpdate struct {
	FileID          string `json:"fileID"`
	Status          string `json:"status"`
	ProcessingError string `json:"processingError,omitempty"`
}

type MixSnapshot struct {
	Tracks      []PlaybackPatch    `json:"tracks"`
	TypeVolumes map[string]float64 `json:"typeVolumes,omitempty"`
}

type MixVolumePatch struct {
	TypeVolumes map[string]float64 `json:"typeVolumes"`
}

type MixerResult struct {
	Command string `json:"command"`
}

type Failure struct {
	Message string `json:"message"`
}

type MixerFailure struct {
	Command string `json:"command"`
	Message string `json:"message"`
}

// Handle registers a callback before the hub accepts connections.
// Go infers T from the callback; only the adapter handles raw JSON.
func Handle[T any](h *Hub, method string, handler func(T, *Client)) {
	h.handlers[method] = func(raw json.RawMessage, client *Client) {
		var payload T
		if err := json.Unmarshal(raw, &payload); err != nil {
			h.logger.Warn("invalid websocket payload", "method", method, "error", err)
			h.invalidCommand(method, client, err)
			return
		}
		handler(payload, client)
	}
}

func (c *Client) SendPayload(method string, payload any) error {
	message, err := newMessage(method, payload)
	if err != nil {
		return err
	}
	return c.Send(message)
}

func (h *Hub) BroadcastPayload(method string, payload any, options ...BroadcastOption) error {
	message, err := newMessage(method, payload)
	if err != nil {
		return err
	}
	return h.Broadcast(message, options...)
}

func newMessage(method string, payload any) (Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{Method: method, Payload: raw}, nil
}
