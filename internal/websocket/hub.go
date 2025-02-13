package websocket

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

type Hub struct {
	clientsMu  sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	logger     *slog.Logger
	handlers   map[string]HandlerFunc
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
		handlers:   make(map[string]HandlerFunc),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			h.clients[client] = true
			h.clientsMu.Unlock()
		case client := <-h.unregister:
			h.clientsMu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.clientsMu.Unlock()
		case message := <-h.broadcast:
			h.clientsMu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.clientsMu.RUnlock()
		}
	}
}

func (h *Hub) Register(c *Client) {
	h.logger.Debug("registering new client",
		"id", c.ID,
		"role", c.Token.Role,
	)
	h.register <- c
}

type Message struct {
	Method   string          `json:"method"`
	Payload  json.RawMessage `json:"payload"`
	SenderID string          `json:"senderId"`
}

type BroadcastOption func(*Client) bool

// Common broadcast filters
func ToAll() BroadcastOption {
	return func(_ *Client) bool { return true }
}

func ToGMOnly() BroadcastOption {
	return func(c *Client) bool {
		return c.Token.Role == "gm"
	}
}

func ToPlayersOnly() BroadcastOption {
	return func(c *Client) bool {
		return c.Token.Role == "player"
	}
}

func ExceptClient(excludeClient *Client) BroadcastOption {
	return func(c *Client) bool {
		return c != excludeClient
	}
}

func ToClientID(clientID string) BroadcastOption {
	return func(c *Client) bool {
		return c.ID == clientID
	}
}

func (h *Hub) Broadcast(msg Message, opts ...BroadcastOption) error {
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("couldn't marshal JSON: %w", err)
	}

	// Combine all filters using AND logic
	filter := func(c *Client) bool {
		for _, opt := range opts {
			if !opt(c) {
				return false
			}
		}
		return true
	}

	// If no filters provided, broadcast to all
	if len(opts) == 0 {
		filter = ToAll()
	}

	h.clientsMu.RLock()
	for client := range h.clients {
		if filter(client) {
			select {
			case client.send <- jsonMsg:
			default:
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
	h.clientsMu.RUnlock()

	return nil
}

// ForEachClient allows iterating over clients with a filter
func (h *Hub) ForEachClient(fn func(*Client), opts ...BroadcastOption) {
	filter := func(c *Client) bool {
		for _, opt := range opts {
			if !opt(c) {
				return false
			}
		}
		return true
	}

	if len(opts) == 0 {
		filter = ToAll()
	}

	h.clientsMu.RLock()
	for client := range h.clients {
		if filter(client) {
			fn(client)
		}
	}
	h.clientsMu.RUnlock()
}

type HandlerFunc func(payload json.RawMessage, c *Client)

func (h *Hub) HandleFunc(name string, fn func(payload json.RawMessage, c *Client)) {
	h.handlers[name] = fn
}

func (h *Hub) route(message []byte, c *Client) {
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		h.logger.Error("Couldn't unmarshal JSON", "err", err)
		return
	}

	// Add sender ID to the message
	msg.SenderID = c.ID

	fn, ok := h.handlers[msg.Method]
	if !ok {
		h.logger.Warn("unknown method",
			slog.String("method", msg.Method),
			slog.String("senderId", msg.SenderID),
		)
		return
	}

	h.logger.Debug("executing WS handler",
		slog.String("method", msg.Method),
		slog.String("payload", string(msg.Payload)),
		slog.String("senderId", msg.SenderID),
	)

	fn(msg.Payload, c)
}
