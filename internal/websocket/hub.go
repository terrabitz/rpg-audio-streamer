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
	h.register <- c
}

type Message struct {
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload"`
}

func (h *Hub) Broadcast(msg Message) error {
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("couldn't marshal JSON: %w", err)
	}

	h.broadcast <- jsonMsg

	return nil
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

	fn, ok := h.handlers[msg.Method]
	if !ok {
		h.logger.Warn("unknown method", slog.String("method", msg.Method))
		return
	}

	h.logger.Debug("executing WS handler",
		slog.String("method", msg.Method),
		slog.String("payload", string(msg.Payload)),
	)

	fn(msg.Payload, c)
}
