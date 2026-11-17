package websocket

import (
	"encoding/json"

	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

type Message = events.Event

type Empty = events.Empty

type TrackReference = events.TrackReference

type Snapshot = events.Snapshot

type TrackStatusUpdate = events.TrackStatusUpdate

type MixSnapshot = events.MixSnapshot

type MixVolumePatch = events.MixVolumePatch

type MixerResult = events.MixerResult

type Failure = events.Failure

type MixerFailure = events.MixerFailure

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
	metadata := c.metadata()
	metadata.TargetClientID = c.ID
	c.hub.notifyRaw(event, message.Payload, metadata)
	return nil
}

func (h *Hub) publishEvent(event string, payload any, metadata events.Metadata) error {
	message, err := newMessage(event, payload)
	if err != nil {
		return err
	}
	return h.publish(message, metadata)
}

func newMessage(event string, payload any) (Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{Event: event, Payload: raw}, nil
}
