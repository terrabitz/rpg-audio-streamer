package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

type Hub struct {
	clientsMu      sync.RWMutex
	clients        map[*Client]bool
	eventQueue     chan queuedEvent
	done           chan struct{}
	register       chan *Client
	unregister     chan *Client
	logger         *slog.Logger
	handlers       map[string]HandlerFunc
	serverHandlers map[string]HandlerFunc
	mixer          server.Mixer
	stateMu        sync.Mutex
	playback       map[string]map[string]playingTrack
	store          PlaybackStore
}

const eventQueueCapacity = 256

var (
	ErrEventQueueFull = errors.New("websocket event queue is full")
	ErrHubClosed      = errors.New("websocket hub is closed")
)

type queuedEvent struct {
	message Message
	client  *Client
}

func NewHub(logger *slog.Logger, store PlaybackStore) *Hub {
	hub := &Hub{
		clients:        make(map[*Client]bool),
		eventQueue:     make(chan queuedEvent, eventQueueCapacity),
		done:           make(chan struct{}),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		logger:         logger,
		handlers:       make(map[string]HandlerFunc),
		serverHandlers: make(map[string]HandlerFunc),
		playback:       make(map[string]map[string]playingTrack),
		store:          store,
	}

	OnEvent(hub, events.Ping, hub.handlePing)
	OnEvent(hub, events.Broadcast, hub.handleBroadcast)
	OnEvent(hub, events.PlaybackSyncRequested, hub.handlePlaybackSyncRequested)
	OnEvent(hub, events.TrackPlaybackChanged, hub.handleTrackPlaybackChanged)
	OnEvent(hub, events.MixChanged, hub.handleMixChanged)
	OnEvent(hub, events.MixVolumesChanged, hub.handleMixVolumesChanged)
	OnServerEvent(hub, events.TableDeleted, hub.handleTableDeleted)
	OnServerEvent(hub, events.TrackRemoved, hub.handleTrackRemoved)
	OnServerEvent(hub, events.TableSettingsChanged, hub.handleTableSettingsChanged)
	OnServerEvent(hub, events.TrackStatusChanged, hub.handleTrackStatusChanged)
	OnServerEvent(hub, events.DiscordStatusChanged, hub.handleDiscordStatusChanged)

	return hub
}

// SetMixer installs the optional consumer before the hub accepts connections.
func (h *Hub) SetMixer(mixer server.Mixer) {
	h.mixer = mixer
}

func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer func() {
		close(h.done)
		h.clientsMu.Lock()
		defer h.clientsMu.Unlock()
		for client := range h.clients {
			h.dropClient(client)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-h.eventQueue:
			h.dispatch(event)
		case <-ticker.C:
			h.refreshPlayback()
		case client := <-h.register:
			h.clientsMu.Lock()
			h.clients[client] = true
			h.clientsMu.Unlock()
			if h.mixer != nil && client.Token.Role == auth.RoleGM {
				if err := client.sendEvent(events.MixerReady, Empty{}); err != nil {
					h.logger.Error("failed to announce mixer", "error", err)
				}
			}
		case client := <-h.unregister:
			h.clientsMu.Lock()
			if _, ok := h.clients[client]; ok {
				h.dropClient(client)
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
	select {
	case h.register <- client:
	case <-h.done:
		conn.Close()
		return
	}

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

func (h *Hub) handleTableDeleted(tableID string, _ *Client) {
	if h.mixer != nil {
		h.mixer.CloseTable(tableID)
	}
	h.stateMu.Lock()
	delete(h.playback, tableID)
	h.stateMu.Unlock()
	if err := h.publishEvent(events.TableDeleted, Empty{}, ToTable(tableID)); err != nil {
		h.logger.Error("failed to announce table deletion", "error", err)
	}
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

func (h *Hub) RevokePlayerAccess(tableID string) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	for client := range h.clients {
		if client.Token.TableID == tableID && client.Token.Role == auth.RolePlayer {
			client.closed = true
			close(client.send)
			delete(h.clients, client)
		}
	}
}

func (h *Hub) handleTrackRemoved(event events.TableEvent[events.TrackReference], _ *Client) {
	tableID, trackID := event.TableID, event.Data.FileID
	if h.mixer != nil {
		h.mixer.RemoveTrack(tableID, trackID)
	}
	h.stateMu.Lock()
	if tableID == "" {
		for _, tracks := range h.playback {
			delete(tracks, trackID)
		}
	} else {
		delete(h.playback[tableID], trackID)
	}
	h.stateMu.Unlock()
	opts := []BroadcastOption{}
	if tableID != "" {
		opts = append(opts, ToTable(tableID))
	}

	if err := h.publishEvent(events.TrackRemoved, TrackReference{FileID: trackID}, opts...); err != nil {
		h.logger.Error("failed to announce track removal", "error", err)
	}
}

func (h *Hub) handleTrackStatusChanged(track server.Track, _ *Client) {
	update := TrackStatusUpdate{FileID: track.ID.String(), Status: track.Status, ProcessingError: track.ProcessingError}
	if err := h.publishEvent(events.TrackStatusChanged, update, ToGMOnly()); err != nil {
		h.logger.Error("failed to announce track processing status", "error", err)
	}
}

func (h *Hub) publish(msg Message, opts ...BroadcastOption) error {
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
				h.dropClient(client)
			}
		}
	}
	h.clientsMu.Unlock()

	return nil
}

func (h *Hub) dropClient(client *Client) {
	client.closed = true
	close(client.send)
	delete(h.clients, client)
	if h.mixer == nil || client.Token.Role != auth.RoleGM {
		return
	}
	for other := range h.clients {
		if other.Token.Role == auth.RoleGM && other.Token.TableID == client.Token.TableID {
			return
		}
	}
	h.mixer.CloseTable(client.Token.TableID)
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

func (h *Hub) SendEvent(event string, payload any) error {
	if _, ok := h.serverHandlers[event]; !ok {
		return fmt.Errorf("unknown server event %q", event)
	}
	message, err := newMessage(event, payload)
	if err != nil {
		return fmt.Errorf("encode event %q: %w", event, err)
	}
	return h.enqueue(queuedEvent{message: message})
}

func (h *Hub) enqueue(event queuedEvent) error {
	select {
	case <-h.done:
		return ErrHubClosed
	default:
	}
	select {
	case h.eventQueue <- event:
		return nil
	case <-h.done:
		return ErrHubClosed
	default:
		return ErrEventQueueFull
	}
}

func (h *Hub) route(message []byte, c *Client) {
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		h.logger.Error("could not decode websocket event", "error", err)
		return
	}
	msg.SenderID = c.ID
	if err := h.enqueue(queuedEvent{message: msg, client: c}); err != nil {
		h.logger.Error("could not queue websocket event", "event", msg.Event, "error", err)
		// Reconnect reconciliation must replace any unapplied client intent.
		if c.conn != nil {
			c.conn.Close()
		}
	}
}

func (h *Hub) dispatch(event queuedEvent) {
	msg, c := event.message, event.client
	if c == nil {
		h.serverHandlers[msg.Event](msg.Payload, nil)
		return
	}
	h.clientsMu.RLock()
	closed := c.closed
	h.clientsMu.RUnlock()
	if closed {
		return
	}
	fn, ok := h.handlers[msg.Event]
	if !ok {
		h.logger.Warn("unknown client event",
			slog.String("event", msg.Event),
			slog.String("senderId", msg.SenderID),
		)
		return
	}

	h.logger.Debug("executing WS handler",
		slog.String("event", msg.Event),
		slog.String("payload", string(msg.Payload)),
		slog.String("senderId", msg.SenderID),
	)

	fn(msg.Payload, c)
}
