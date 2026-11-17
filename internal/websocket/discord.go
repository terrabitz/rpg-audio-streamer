package websocket

import "github.com/terrabitz/rpg-audio-streamer/internal/discord"

// DiscordStatusChanged queues the latest state without blocking the Discord
// service on socket delivery or mixer cleanup.
func (h *Hub) DiscordStatusChanged(tableID string, status discord.Status) {
	h.discordMu.Lock()
	h.discordPending[tableID] = status
	h.discordMu.Unlock()
	select {
	case h.discordChanged <- struct{}{}:
	default:
	}
}

func (h *Hub) broadcastDiscordStatuses() {
	h.discordMu.Lock()
	pending := h.discordPending
	h.discordPending = make(map[string]discord.Status)
	h.discordMu.Unlock()
	for tableID, status := range pending {
		if err := h.BroadcastPayload(DiscordStatus, status, ToGMOnly(), ToTable(tableID)); err != nil {
			h.logger.Error("failed to announce Discord status", "tableID", tableID, "error", err)
		}
	}
}
