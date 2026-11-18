package websocket

import (
	"encoding/json"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func (h *Hub) handlePing(payload json.RawMessage, c *Client) {
	h.notifyRaw(events.Ping, payload, c.metadata())
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
	if err := h.publish(message, c.metadata()); err != nil {
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
	h.notify(events.PlaybackSyncRequested, Empty{}, c.metadata())
}

func (h *Hub) handleTrackPlaybackChanged(payload PlaybackPatch, c *Client) {
	if c.Token.Role != auth.RoleGM {
		h.logger.Warn("unauthorized track playback event", "role", c.Token.Role)
		return
	}
	h.updatePlayback(payload, c)
}

func (h *Hub) handleMixChanged(payload events.MixSnapshot, c *Client) {
	if c.Token.Role != auth.RoleGM {
		return
	}
	h.notify(events.MixChanged, payload, c.metadata())
}

func (h *Hub) handleMixVolumesChanged(payload events.MixVolumePatch, c *Client) {
	if c.Token.Role != auth.RoleGM {
		return
	}
	h.notify(events.MixVolumesChanged, payload, c.metadata())
}

func (h *Hub) handleMixerReady(event events.ClientEvent[Empty], _ *Client) {
	h.sendToClient(events.MixerReady, event.Data, event.TableID, event.ClientID)
}

func (h *Hub) handleMixerApplied(event events.ClientEvent[MixerResult], _ *Client) {
	h.sendToClient(events.MixerApplied, event.Data, event.TableID, event.ClientID)
}

func (h *Hub) handleMixerFailed(event events.ClientEvent[MixerFailure], _ *Client) {
	h.sendToClient(events.MixerFailed, event.Data, event.TableID, event.ClientID)
}

func (h *Hub) sendToClient(event string, payload any, tableID, clientID string) {
	metadata := events.Metadata{TableID: tableID, TargetClientID: clientID}
	if err := h.publishEvent(event, payload, metadata); err != nil {
		h.logger.Error("failed to deliver client event", "event", event, "error", err)
	}
}

func (h *Hub) invalidEvent(event string, client *Client, err error) {
	switch event {
	case events.TrackPlaybackChanged:
		h.playbackError(client, err)
	case events.MixChanged, events.MixVolumesChanged:
		if sendErr := client.sendEvent(events.MixerFailed, events.MixerFailure{
			Event: event, Message: "Server mixing failed. Check the server logs and reconnect to retry.",
		}); sendErr != nil {
			h.logger.Error("failed to report invalid mixer event", "error", sendErr)
		}
	}
}
