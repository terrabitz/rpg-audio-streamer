package websocket

import (
	"encoding/json"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func (h *Hub) handlePing(payload json.RawMessage, c *Client) {
	if err := c.sendEvent(events.Pong, payload); err != nil {
		h.logger.Error("failed to send pong message", "error", err)
	}
}

func (h *Hub) handleBroadcast(payload json.RawMessage, c *Client) {
	if c.Token.Role != auth.RoleGM {
		h.logger.Warn("unauthorized broadcast attempt", "role", c.Token.Role)
		return
	}
	message, err := newMessage(events.Broadcast, payload)
	if err != nil {
		h.logger.Error("failed to encode broadcast", "error", err)
		return
	}
	message.SenderID = c.ID
	if err := h.publish(message, ExceptClient(c), ToTable(c.Token.TableID)); err != nil {
		h.logger.Error("failed to broadcast", "error", err)
	}
}

func (h *Hub) handlePlaybackSyncRequested(_ Empty, c *Client) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	settings, err := h.tableSettings(c.Token.TableID)
	if err != nil {
		h.playbackError(c, err)
		return
	}
	h.sendState(c.Token.TableID, settings, c.ID)
}

func (h *Hub) handleTrackPlaybackChanged(payload PlaybackPatch, c *Client) {
	if c.Token.Role != auth.RoleGM {
		h.logger.Warn("unauthorized track playback event", "role", c.Token.Role)
		return
	}
	h.updatePlayback(payload, c)
}

func (h *Hub) handleMixChanged(payload MixSnapshot, c *Client) {
	if c.Token.Role != auth.RoleGM || h.mixer == nil {
		return
	}
	h.applyMixer(c, events.MixChanged, func() error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return h.mixer.Sync(c.Token.TableID, raw)
	})
}

func (h *Hub) handleMixVolumesChanged(payload MixVolumePatch, c *Client) {
	if c.Token.Role != auth.RoleGM || h.mixer == nil {
		return
	}
	h.applyMixer(c, events.MixVolumesChanged, func() error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return h.mixer.Volumes(c.Token.TableID, raw)
	})
}

func (h *Hub) applyMixer(c *Client, event string, apply func() error) {
	if err := apply(); err != nil {
		h.logger.Error("mixer event failed", "event", event, "tableID", c.Token.TableID, "error", err)
		if err := c.sendEvent(events.MixerFailed, MixerFailure{
			Event:   event,
			Message: "Server mixing failed. Check the server logs and reconnect to retry.",
		}); err != nil {
			h.logger.Error("failed to report mixer event failure", "error", err)
		}

		return
	}
	if err := c.sendEvent(events.MixerApplied, MixerResult{Event: event}); err != nil {
		h.logger.Error("failed to report mixer event result", "error", err)
	}
}

func (h *Hub) invalidEvent(event string, client *Client, err error) {
	switch event {
	case events.TrackPlaybackChanged:
		h.playbackError(client, err)
	case events.MixChanged, events.MixVolumesChanged:
		h.applyMixer(client, event, func() error { return err })
	}
}
