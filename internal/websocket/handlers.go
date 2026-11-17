package websocket

import (
	"encoding/json"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

func (h *Hub) handlePing(payload json.RawMessage, c *Client) {
	if err := c.SendPayload(Pong, payload); err != nil {
		h.logger.Error("failed to send pong message", "error", err)
	}
}

func (h *Hub) handleBroadcast(payload json.RawMessage, c *Client) {
	if c.Token.Role != auth.RoleGM {
		h.logger.Warn("unauthorized broadcast attempt", "role", c.Token.Role)
		return
	}
	message, err := newMessage(Broadcast, payload)
	if err != nil {
		h.logger.Error("failed to encode broadcast", "error", err)
		return
	}
	message.SenderID = c.ID
	if err := h.Broadcast(message, ExceptClient(c), ToTable(c.Token.TableID)); err != nil {
		h.logger.Error("failed to broadcast", "error", err)
	}
}

func (h *Hub) handleSyncRequest(_ Empty, c *Client) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	settings, err := h.tableSettings(c.Token.TableID)
	if err != nil {
		h.playbackError(c, err)
		return
	}
	h.sendState(c.Token.TableID, settings, c.ID)
}

func (h *Hub) handleSyncTrack(payload PlaybackPatch, c *Client) {
	if c.Token.Role != auth.RoleGM {
		h.logger.Warn("unauthorized syncTrack command", "role", c.Token.Role)
		return
	}
	h.updatePlayback(payload, c)
}

func (h *Hub) handleMixSync(payload MixSnapshot, c *Client) {
	if c.Token.Role != auth.RoleGM || h.mixer == nil {
		return
	}
	h.applyMixer(c, MixSync, func() error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return h.mixer.Sync(c.Token.TableID, raw)
	})
}

func (h *Hub) handleMixVolumes(payload MixVolumePatch, c *Client) {
	if c.Token.Role != auth.RoleGM || h.mixer == nil {
		return
	}
	h.applyMixer(c, MixVolumes, func() error {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		return h.mixer.Volumes(c.Token.TableID, raw)
	})
}

func (h *Hub) applyMixer(c *Client, command string, apply func() error) {
	if err := apply(); err != nil {
		h.logger.Error("mixer command failed", "command", command, "tableID", c.Token.TableID, "error", err)
		if err := c.SendPayload(MixerError, MixerFailure{
			Command: command,
			Message: "Server mixing failed. Check the server logs and reconnect to retry.",
		}); err != nil {
			h.logger.Error("failed to report mixer command failure", "error", err)
		}

		return
	}
	if err := c.SendPayload(MixerAck, MixerResult{Command: command}); err != nil {
		h.logger.Error("failed to report mixer command result", "error", err)
	}
}

func (h *Hub) invalidCommand(command string, client *Client, err error) {
	switch command {
	case SyncTrack:
		h.playbackError(client, err)
	case MixSync, MixVolumes:
		h.applyMixer(client, command, func() error { return err })
	}
}
