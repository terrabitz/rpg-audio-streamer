package server

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
)

type discordMixer struct {
	*livemixer.Service
	server   *Server
	mu       sync.Mutex
	sessions map[string]*discordMixSession
}

type discordMixSession struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func newDiscordMixer(mix *livemixer.Service, server *Server) *discordMixer {
	return &discordMixer{Service: mix, server: server, sessions: make(map[string]*discordMixSession)}
}

func (m *discordMixer) Sync(tableID string, payload json.RawMessage) error {
	m.mu.Lock()
	current := m.sessions[tableID]
	if current == nil {
		ctx, cancel := context.WithCancel(context.Background())
		current = &discordMixSession{ctx: ctx, cancel: cancel}
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
