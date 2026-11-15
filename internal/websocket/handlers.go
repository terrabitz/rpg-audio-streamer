package websocket

import (
	"encoding/json"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

type Message struct {
	Method   string          `json:"method"`
	Payload  json.RawMessage `json:"payload"`
	SenderID string          `json:"senderId"`
}

func (h *Hub) handlePing(payload json.RawMessage, c *Client) {
	if err := c.Send(Message{
		Method:  "pong",
		Payload: payload,
	}); err != nil {
		h.logger.Error("failed to send pong message", "error", err)
	}
}

func (h *Hub) handleBroadcast(payload json.RawMessage, c *Client) {
	if c.Token.Role != auth.RoleGM {
		h.logger.Warn("unauthorized broadcast attempt", "role", c.Token.Role)
		return
	}

	h.Broadcast(Message{
		Method:   "broadcast",
		SenderID: c.ID,
		Payload:  payload,
	}, ExceptClient(c), ToTable(c.Token.TableID))
}

func (h *Hub) handleSyncRequest(payload json.RawMessage, c *Client) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	settings, err := h.tableSettings(c.Token.TableID)
	if err != nil {
		h.playbackError(c, err)
		return
	}
	h.sendState(c.Token.TableID, settings, c.ID)
}

func (h *Hub) handleSyncTrack(payload json.RawMessage, c *Client) {
	if c.Token.Role != auth.RoleGM {
		h.logger.Warn("unauthorized syncTrack command", "role", c.Token.Role)
		return
	}
	h.updatePlayback(payload, c)
}

func (h *Hub) handleMixSync(payload json.RawMessage, c *Client) {
	if c.Token.Role != auth.RoleGM || h.mixer == nil {
		return
	}
	h.applyMixer(c, "mixSync", func() error { return h.mixer.Sync(c.Token.TableID, payload) })
}

func (h *Hub) handleMixVolumes(payload json.RawMessage, c *Client) {
	if c.Token.Role != auth.RoleGM || h.mixer == nil {
		return
	}
	h.applyMixer(c, "mixVolumes", func() error { return h.mixer.Volumes(c.Token.TableID, payload) })
}

func (h *Hub) applyMixer(c *Client, command string, apply func() error) {
	method := "mixerAck"
	message := ""
	if err := apply(); err != nil {
		h.logger.Error("mixer command failed", "command", command, "tableID", c.Token.TableID, "error", err)
		method = "mixerError"
		message = "Server mixing failed. Check the server logs and reconnect to retry."
	}
	payload, _ := json.Marshal(struct {
		Command string `json:"command"`
		Message string `json:"message,omitempty"`
	}{Command: command, Message: message})
	if err := c.Send(Message{Method: method, Payload: payload}); err != nil {
		h.logger.Error("failed to report mixer command result", "error", err)
	}
}
