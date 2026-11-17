package websocket

import (
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func (h *Hub) handleDiscordStatusChanged(event events.TableEvent[discord.Status], _ *Client) {
	if err := h.publishEvent(events.DiscordStatusChanged, event.Data, ToGMOnly(), ToTable(event.TableID)); err != nil {
		h.logger.Error("failed to announce Discord status", "tableID", event.TableID, "error", err)
	}
}
