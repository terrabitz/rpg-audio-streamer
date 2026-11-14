package websocket

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
)

type Hub struct {
	clientsMu  sync.RWMutex
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	logger     *slog.Logger
	handlers   map[string]HandlerFunc
	stateMu    sync.Mutex
	playback   map[string]map[string]playingTrack
	store      PlaybackStore
}

func NewHub(logger *slog.Logger, store PlaybackStore) *Hub {
	hub := &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
		handlers:   make(map[string]HandlerFunc),
		playback:   make(map[string]map[string]playingTrack),
		store:      store,
	}

	hub.HandleFunc("ping", hub.handlePing)
	hub.HandleFunc("broadcast", hub.handleBroadcast)
	hub.HandleFunc("syncRequest", hub.handleSyncRequest)
	hub.HandleFunc("syncTrack", hub.handleSyncTrack)

	return hub
}

func (h *Hub) Run() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.refreshPlayback()
		case client := <-h.register:
			h.clientsMu.Lock()
			h.clients[client] = true
			h.clientsMu.Unlock()
		case client := <-h.unregister:
			h.clientsMu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.closed = true
				close(client.send)
			}
			h.clientsMu.Unlock()
		case message := <-h.broadcast:
			h.clientsMu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					client.closed = true
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.clientsMu.Unlock()
		}
	}
}

func (h *Hub) Register(conn *websocket.Conn, token *auth.Token) {
	client := &Client{
		ID:    uuid.New().String(),
		hub:   h,
		conn:  conn,
		send:  make(chan []byte, 256),
		Token: token,
	}

	h.logger.Debug("new websocket connection",
		"clientId", client.ID,
		"role", token.Role,
	)
	h.register <- client

	go client.WritePump()
	go client.ReadPump()
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

func ToTable(tableID string) BroadcastOption {
	return func(c *Client) bool {
		return c.Token.TableID == tableID
	}
}

func (h *Hub) CloseTable(tableID string) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	delete(h.playback, tableID)
	h.Broadcast(Message{Method: "tableDeleted", Payload: json.RawMessage(`{}`)}, ToTable(tableID))
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	for client := range h.clients {
		if client.Token.TableID == tableID {
			client.closed = true
			close(client.send)
			delete(h.clients, client)
		}
	}
}

func (h *Hub) RemoveTrack(tableID, trackID string) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	for id, tracks := range h.playback {
		if tableID == "" || tableID == id {
			delete(tracks, trackID)
		}
	}
	payload, _ := json.Marshal(struct {
		FileID string `json:"fileID"`
	}{FileID: trackID})
	opts := []BroadcastOption{}
	if tableID != "" {
		opts = append(opts, ToTable(tableID))
	}

	h.Broadcast(Message{Method: "removeTrack", Payload: payload}, opts...)
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

	h.clientsMu.Lock()
	for client := range h.clients {
		if filter(client) {
			select {
			case client.send <- jsonMsg:
			default:
				client.closed = true
				close(client.send)
				delete(h.clients, client)
			}
		}
	}
	h.clientsMu.Unlock()

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
	h.clientsMu.RLock()
	closed := c.closed
	h.clientsMu.RUnlock()
	if closed {
		return
	}
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
