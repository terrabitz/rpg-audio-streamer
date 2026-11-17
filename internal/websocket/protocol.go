package websocket

import (
	"encoding/json"

	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

type Message = events.Event

type Empty struct{}

type TrackReference = events.TrackReference

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
	Event string `json:"event"`
}

type Failure struct {
	Message string `json:"message"`
}

type MixerFailure struct {
	Event   string `json:"event"`
	Message string `json:"message"`
}

// OnEvent registers a client callback before the hub accepts connections.
// Go infers T from the callback; only the adapter handles raw JSON.
func OnEvent[T any](h *Hub, event string, handler func(T, *Client)) {
	h.handlers[event] = func(raw json.RawMessage, client *Client) {
		var payload T
		if err := json.Unmarshal(raw, &payload); err != nil {
			h.logger.Warn("invalid websocket payload", "event", event, "error", err)
			h.invalidEvent(event, client, err)
			return
		}

		handler(payload, client)
	}
}

// OnServerEvent registers a trusted producer callback before the hub starts.
func OnServerEvent[T any](h *Hub, event string, handler func(T, *Client)) {
	h.serverHandlers[event] = func(raw json.RawMessage, client *Client) {
		var payload T
		if err := json.Unmarshal(raw, &payload); err != nil {
			h.logger.Error("invalid server event payload", "event", event, "error", err)
			return
		}
		handler(payload, client)
	}
}

func (c *Client) sendEvent(event string, payload any) error {
	message, err := newMessage(event, payload)
	if err != nil {
		return err
	}
	return c.Send(message)
}

func (h *Hub) publishEvent(event string, payload any, options ...BroadcastOption) error {
	message, err := newMessage(event, payload)
	if err != nil {
		return err
	}
	return h.publish(message, options...)
}

func newMessage(event string, payload any) (Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{Event: event, Payload: raw}, nil
}
