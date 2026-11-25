package websocket

import (
	"encoding/json"

	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

type Message = events.Event

type Empty = events.Empty

type TrackReference = events.TrackReference

type Snapshot = events.Snapshot

type PlaybackPatch = events.PlaybackPatch

type TrackStatusUpdate = events.TrackStatusUpdate

type MixSnapshot = events.MixSnapshot

type MixVolumePatch = events.MixVolumePatch

type MixerResult = events.MixerResult

type MixerFailure = events.MixerFailure

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

func (h *Hub) publishEvent(event string, payload any, metadata events.Metadata) error {
	message, err := newMessage(event, payload)
	if err != nil {
		return err
	}
	return h.publish(message, metadata)
}

func (h *Hub) Publish(event string, payload any, metadata events.Metadata) error {
	if err := h.beginDispatch(); err != nil {
		return err
	}
	defer h.inFlight.Done()
	return h.publishEvent(event, payload, metadata)
}

func newMessage(event string, payload any) (Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{Event: event, Payload: raw}, nil
}
