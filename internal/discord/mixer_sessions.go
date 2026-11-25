package discord

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
)

// Disconnector leaves a table's Discord voice channel.
type Disconnector interface {
	Disconnect(ctx context.Context, tableID string) error
}

// MixerSessions ties each table's live mix to its Discord voice session, so ending
// the table cancels in-flight decoding and leaves the voice channel.
type MixerSessions struct {
	*livemixer.Service
	voice    Disconnector
	logger   *slog.Logger
	mu       sync.Mutex
	sessions map[string]*mixSession
}

type mixSession struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func NewMixerSessions(mix *livemixer.Service, voice Disconnector, logger *slog.Logger) *MixerSessions {
	return &MixerSessions{
		Service:  mix,
		voice:    voice,
		logger:   logger,
		sessions: make(map[string]*mixSession),
	}
}

func (m *MixerSessions) Sync(tableID string, payload json.RawMessage) error {
	current := m.session(tableID)
	if err := m.Service.SyncContext(current.ctx, tableID, payload); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return current.ctx.Err()
}

func (m *MixerSessions) CloseTable(tableID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if current := m.sessions[tableID]; current != nil {
		current.cancel()
		delete(m.sessions, tableID)
	}
	m.Service.CloseTable(tableID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.voice.Disconnect(ctx, tableID); err != nil {
		m.logger.Error("Failed to disconnect Discord after table session ended", "error", err)
	}
}

func (m *MixerSessions) session(tableID string) *mixSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.sessions[tableID]
	if current == nil {
		ctx, cancel := context.WithCancel(context.Background())
		current = &mixSession{
			ctx:    ctx,
			cancel: cancel,
		}
		m.sessions[tableID] = current
	}
	return current
}
