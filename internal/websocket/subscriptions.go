package websocket

import (
	"encoding/json"
	"sync"

	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

type subscription struct {
	handler events.Handler
}

// On observes published domain and lifecycle events independently.
func (h *Hub) On(event string, handler events.Handler) func() {
	listener := &subscription{handler: handler}
	h.subscriptionsMu.Lock()
	h.subscriptions[event] = append(h.subscriptions[event], listener)
	h.subscriptionsMu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			h.subscriptionsMu.Lock()
			defer h.subscriptionsMu.Unlock()
			listeners := h.subscriptions[event]
			for i, candidate := range listeners {
				if candidate == listener {
					listeners = append(listeners[:i], listeners[i+1:]...)
					break
				}
			}
			if len(listeners) == 0 {
				delete(h.subscriptions, event)
			} else {
				h.subscriptions[event] = listeners
			}
		})
	}
}

func (h *Hub) notify(event string, payload any, metadata events.Metadata) {
	raw, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("failed to encode subscriber event", "event", event, "error", err)
		return
	}
	h.notifyRaw(event, raw, metadata)
}

func (h *Hub) notifyRaw(event string, payload json.RawMessage, metadata events.Metadata) {
	h.subscriptionsMu.RLock()
	listeners := append([]*subscription(nil), h.subscriptions[event]...)
	h.subscriptionsMu.RUnlock()
	for _, listener := range listeners {
		if err := listener.handler(append(json.RawMessage(nil), payload...), metadata); err != nil {
			h.logger.Error("event subscriber failed", "event", event, "tableID", metadata.TableID, "error", err)
		}
	}
}
