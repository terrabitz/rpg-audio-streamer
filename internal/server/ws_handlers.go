package server

import (
	"encoding/json"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	ws "github.com/terrabitz/rpg-audio-streamer/internal/websocket"
)

func (s *Server) handlePing(payload json.RawMessage, c *ws.Client) {
	if err := c.Send(ws.Message{
		Method:  "pong",
		Payload: payload,
	}); err != nil {
		s.logger.Error("failed to send pong message", "error", err)
	}
}

func (s *Server) handleBroadcast(payload json.RawMessage, c *ws.Client) {
	if c.Token.Role != auth.RoleGM {
		s.logger.Warn("unauthorized broadcast attempt", "role", c.Token.Role)
		return
	}

	s.hub.Broadcast(ws.Message{
		Method:   "broadcast",
		SenderID: c.ID,
		Payload:  payload,
	}, ws.ExceptClient(c)) // Don't send back to sender
}

func (s *Server) handleSyncRequest(payload json.RawMessage, c *ws.Client) {
	// Only forward to GM clients
	s.hub.Broadcast(ws.Message{
		Method:   "syncRequest",
		SenderID: c.ID,
		Payload:  payload,
	}, ws.ToGMOnly())
}

func (s *Server) handleSyncAll(payload json.RawMessage, c *ws.Client) {
	if c.Token.Role != auth.RoleGM {
		s.logger.Warn("unauthorized broadcast attempt", "role", c.Token.Role)
		return
	}

	// Extract target client ID from payload
	var syncPayload struct {
		Tracks []any  `json:"tracks"`
		To     string `json:"to"`
	}
	if err := json.Unmarshal(payload, &syncPayload); err != nil {
		s.logger.Error("failed to unmarshal sync payload", "error", err)
		return
	}

	// If a target client is specified, only send to them
	if syncPayload.To != "" {
		s.hub.Broadcast(ws.Message{
			Method:   "syncAll",
			SenderID: c.ID,
			Payload:  payload,
		}, ws.ToClientID(syncPayload.To))
	} else {
		// Otherwise broadcast to all players
		s.hub.Broadcast(ws.Message{
			Method:   "syncAll",
			SenderID: c.ID,
			Payload:  payload,
		}, ws.ToPlayersOnly())
	}
}

func (s *Server) handleSyncTrack(payload json.RawMessage, c *ws.Client) {
	if c.Token.Role != auth.RoleGM {
		s.logger.Warn("unauthorized syncTrack command", "role", c.Token.Role)
		return
	}
	s.hub.Broadcast(ws.Message{
		Method:   "syncTrack",
		SenderID: c.ID,
		Payload:  payload,
	}, ws.ToPlayersOnly())
}
