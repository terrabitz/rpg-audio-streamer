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
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/playback"
)

type Hub struct {
	clientsMu       sync.RWMutex
	clients         map[*Client]bool
	lifecycleMu     sync.Mutex
	inFlight        sync.WaitGroup
	done            chan struct{}
	register        chan *Client
	unregister      chan *Client
	logger          *slog.Logger
	serverHandlers  map[string]HandlerFunc
	subscriptionsMu sync.RWMutex
	subscriptions   map[string][]*subscription
	playback        *playback.Service
}

var ErrHubClosed = errors.New("websocket hub is closed")

func NewHub(logger *slog.Logger, store playback.Store) *Hub {
	hub := &Hub{
		clients:        make(map[*Client]bool),
		done:           make(chan struct{}),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		logger:         logger,
		serverHandlers: make(map[string]HandlerFunc),
		subscriptions:  make(map[string][]*subscription),
	}
	hub.playback = playback.New(logger, store, hub)

	OnServerEvent(hub, events.TableDeleted, hub.handleTableDeleted)
	OnServerEvent(hub, events.TrackRemoved, hub.handleTrackRemoved)
	OnServerEvent(hub, events.TableSettingsChanged, hub.handleTableSettingsChanged)
	OnServerEvent(hub, events.TrackStatusChanged, hub.handleTrackStatusChanged)
	OnServerEvent(hub, events.TrackUpdated, hub.handleTrackUpdated)
	OnServerEvent(hub, events.DiscordStatusChanged, hub.handleDiscordStatusChanged)
	OnServerEvent(hub, events.MixerReady, hub.handleMixerReady)
	OnServerEvent(hub, events.MixerApplied, hub.handleMixerApplied)
	OnServerEvent(hub, events.MixerFailed, hub.handleMixerFailed)
	OnServerEvent(hub, events.MixChanged, func(event events.TableEvent[events.MixSnapshot], _ *Client) {
		hub.notify(events.MixChanged, event.Data, events.Metadata{TableID: event.TableID, Role: auth.RoleGM})
	})
	OnServerEvent(hub, events.MixVolumesChanged, func(event events.TableEvent[events.MixVolumePatch], _ *Client) {
		hub.notify(events.MixVolumesChanged, event.Data, events.Metadata{TableID: event.TableID, Role: auth.RoleGM})
	})

	return hub
}

func (h *Hub) Playback() *playback.Service { return h.playback }

func (h *Hub) refreshPlayback() {
	connected := make(map[string]bool)
	h.ForEachClient(func(client *Client) { connected[client.Token.TableID] = true })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.playback.Refresh(ctx, connected)
}

func (h *Hub) handleTrackUpdated(fileID string, _ *Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.playback.TrackUpdated(ctx, fileID); err != nil {
		h.logger.Error("failed to synchronize track update", "error", err, "trackID", fileID)
	}
}

func (h *Hub) handleTableSettingsChanged(event events.TableEvent[domain.TableSettingsPatch], _ *Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.playback.SettingsChanged(ctx, event.TableID); err != nil {
		h.logger.Error("failed to synchronize table settings", "error", err, "tableID", event.TableID)
	}
}

func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer func() {
		h.lifecycleMu.Lock()
		close(h.done)
		h.lifecycleMu.Unlock()
		h.inFlight.Wait()
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
	h.playback.DeleteTable(tableID)
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
	h.playback.RemoveTrack(tableID, trackID)
	if err := h.publishEvent(events.TrackRemoved, TrackReference{FileID: trackID}, events.Metadata{TableID: tableID, Global: tableID == ""}); err != nil {
		h.logger.Error("failed to announce track removal", "error", err)
	}
}

func (h *Hub) handleTrackStatusChanged(track domain.Track, _ *Client) {
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
	if err := h.beginDispatch(); err != nil {
		return err
	}
	defer h.inFlight.Done()
	h.serverHandlers[event](message.Payload, nil)
	return nil
}

func (h *Hub) beginDispatch() error {
	h.lifecycleMu.Lock()
	defer h.lifecycleMu.Unlock()
	select {
	case <-h.done:
		return ErrHubClosed
	default:
		// Admission must finish before shutdown starts waiting, including nested sends.
		h.inFlight.Add(1)
		return nil
	}
}
