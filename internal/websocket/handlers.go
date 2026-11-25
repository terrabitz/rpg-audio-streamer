package websocket

import "github.com/terrabitz/rpg-audio-streamer/internal/events"

func (h *Hub) handleMixerReady(event events.ClientEvent[Empty], _ *Client) {
	h.sendToClient(events.MixerReady, event.Data, event.TableID, event.ClientID)
}

func (h *Hub) handleMixerApplied(event events.TableEvent[MixerResult], _ *Client) {
	h.sendToClient(events.MixerApplied, event.Data, event.TableID, "")
}

func (h *Hub) handleMixerFailed(event events.TableEvent[MixerFailure], _ *Client) {
	h.sendToClient(events.MixerFailed, event.Data, event.TableID, "")
}

func (h *Hub) sendToClient(event string, payload any, tableID, clientID string) {
	metadata := events.Metadata{TableID: tableID, TargetClientID: clientID}
	if err := h.publishEvent(event, payload, metadata); err != nil {
		h.logger.Error("failed to deliver client event", "event", event, "error", err)
	}
}
