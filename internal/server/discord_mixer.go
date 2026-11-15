package server

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
)

type discordMixer struct {
	*livemixer.Service
	server   *Server
	mu       sync.Mutex
	sessions map[string]*discordMixSession
}

type discordMixSession struct {
	ctx           context.Context
	cancel        context.CancelFunc
	restoreCtx    context.Context
	cancelRestore context.CancelFunc
	ready         bool
}

func newDiscordMixer(mix *livemixer.Service, server *Server) *discordMixer {
	return &discordMixer{Service: mix, server: server, sessions: make(map[string]*discordMixSession)}
}

func (m *discordMixer) Sync(tableID string, payload json.RawMessage) error {
	m.mu.Lock()
	current := m.sessions[tableID]
	if current == nil {
		ctx, cancel := context.WithCancel(context.Background())
		restoreCtx, cancelRestore := context.WithCancel(ctx)
		current = &discordMixSession{ctx: ctx, cancel: cancel, restoreCtx: restoreCtx, cancelRestore: cancelRestore}
		m.sessions[tableID] = current
	}
	m.mu.Unlock()
	if err := m.Service.SyncContext(current.ctx, tableID, payload); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := current.ctx.Err(); err != nil {
		return err
	}
	if !current.ready {
		current.ready = true
		go m.restore(current.restoreCtx, tableID)
	}
	return nil
}

func (m *discordMixer) CloseTable(tableID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if current := m.sessions[tableID]; current != nil {
		current.cancel()
		delete(m.sessions, tableID)
	}
	m.Service.CloseTable(tableID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.server.discord.Disconnect(ctx, tableID); err != nil {
		m.server.logger.Error("Failed to disconnect Discord after table session ended", "error", err)
	}
}

func (m *discordMixer) cancelRestore(tableID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if current := m.sessions[tableID]; current != nil {
		current.cancelRestore()
	}
}

func (m *discordMixer) restore(ctx context.Context, tableID string) {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	id, err := uuid.Parse(tableID)
	if err == nil {
		target, loadErr := m.server.store.GetTableDiscord(ctx, id)
		err = loadErr
		if err == nil && target != nil {
			err = m.server.discord.Connect(ctx, tableID, *target)
		}
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		m.server.logger.Warn("Automatic Discord reconnection failed", "tableID", tableID, "error", err)
		m.server.discord.Fail(tableID, err)
	}
}
