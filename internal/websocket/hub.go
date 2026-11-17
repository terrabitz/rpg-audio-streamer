package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

type Hub struct {
	clientsMu       sync.RWMutex
	clients         map[*Client]bool
	eventQueue      chan queuedEvent
	done            chan struct{}
	register        chan *Client
	unregister      chan *Client
	logger          *slog.Logger
	handlers        map[string]HandlerFunc
	serverHandlers  map[string]HandlerFunc
	subscriptionsMu sync.RWMutex
	subscriptions   map[string][]*subscription
	stateMu         sync.Mutex
	playback        map[string]map[string]playingTrack
	store           PlaybackStore
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
		subscriptions:  make(map[string][]*subscription),
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
	OnServerEvent(hub, events.MixerReady, hub.handleMixerReady)
	OnServerEvent(hub, events.MixerApplied, hub.handleMixerApplied)
	OnServerEvent(hub, events.MixerFailed, hub.handleMixerFailed)

	return hub
}

func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer func() {
		close(h.done)
		var clients []*Client
		h.ForEachClient(func(client *Client) { clients = append(clients, client) })
		for _, client := range clients {
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
			h.addClient(client)
			h.notify(events.ClientConnected, Empty{}, client.metadata())
			close(client.ready)
		case client := <-h.unregister:
			h.dropClient(client)
		}
	}
}

func (h *Hub) Register(conn *websocket.Conn, token *auth.Token) {
	client := NewClient(h, conn, token)

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
	select {
	case <-client.ready:
	case <-h.done:
		conn.Close()
		return
	}

	go client.WritePump()
	go client.ReadPump()
}

func (h *Hub) addClient(client *Client) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	h.clients[client] = true
	client.subscribe()
}

func (h *Hub) handleTableDeleted(tableID string, _ *Client) {
	h.stateMu.Lock()
	delete(h.playback, tableID)
	h.stateMu.Unlock()
	if err := h.publishEvent(events.TableDeleted, Empty{}, events.Metadata{TableID: tableID}); err != nil {
		h.logger.Error("failed to announce table deletion", "error", err)
	}
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	for client := range h.clients {
		if client.Token.TableID == tableID {
			h.removeClient(client)
		}
	}
}

func (h *Hub) RevokePlayerAccess(tableID string) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	for client := range h.clients {
		if client.Token.TableID == tableID && client.Token.Role == auth.RolePlayer {
			h.removeClient(client)
		}
	}
}

func (h *Hub) handleTrackRemoved(event events.TableEvent[events.TrackReference], _ *Client) {
	tableID, trackID := event.TableID, event.Data.FileID
	h.stateMu.Lock()
	if tableID == "" {
		for _, tracks := range h.playback {
			delete(tracks, trackID)
		}
	} else {
		delete(h.playback[tableID], trackID)
	}
	h.stateMu.Unlock()
	if err := h.publishEvent(events.TrackRemoved, TrackReference{FileID: trackID}, events.Metadata{TableID: tableID, Global: tableID == ""}); err != nil {
		h.logger.Error("failed to announce track removal", "error", err)
	}
}

func (h *Hub) handleTrackStatusChanged(track server.Track, _ *Client) {
	update := TrackStatusUpdate{FileID: track.ID.String(), Status: track.Status, ProcessingError: track.ProcessingError}
	if err := h.publishEvent(events.TrackStatusChanged, update, events.Metadata{Global: true}); err != nil {
		h.logger.Error("failed to announce track processing status", "error", err)
	}
}

func (h *Hub) publish(msg Message, metadata events.Metadata) error {
	if !json.Valid(msg.Payload) {
		return fmt.Errorf("invalid JSON event payload")
	}

	h.notifyRaw(msg.Event, msg.Payload, metadata)
	return nil
}

func (h *Hub) dropClient(client *Client) {
	h.clientsMu.Lock()
	if _, ok := h.clients[client]; !ok {
		h.clientsMu.Unlock()
		return
	}
	h.removeClient(client)
	if client.Token.Role != auth.RoleGM {
		h.clientsMu.Unlock()
		return
	}
	for other := range h.clients {
		if other.Token.Role == auth.RoleGM && other.Token.TableID == client.Token.TableID {
			h.clientsMu.Unlock()
			return
		}
	}
	h.clientsMu.Unlock()
	h.notify(events.TableSessionEnded, Empty{}, events.Metadata{TableID: client.Token.TableID})
}

// removeClient requires clientsMu and does not publish lifecycle events.
func (h *Hub) removeClient(client *Client) {
	for _, stop := range client.subscriptions {
		stop()
	}
	client.subscriptions = nil
	client.closed = true
	close(client.send)
	delete(h.clients, client)
}

func (h *Hub) ForEachClient(fn func(*Client)) {
	h.clientsMu.RLock()
	for client := range h.clients {
		fn(client)
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
