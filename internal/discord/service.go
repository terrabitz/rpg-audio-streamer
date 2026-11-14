package discord

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/disgoorg/disgo/rest"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

var (
	ErrInUse    = errors.New("Discord bot is already connecting or connected to another table in this server")
	ErrNotReady = errors.New("table mix is not ready; reconnect the table first")
)

type Config struct {
	Token     string
	GuildID   string
	ChannelID string
	Worker    string
}

type Destination struct {
	GuildID     string `json:"guildID"`
	ChannelID   string `json:"channelID"`
	GuildName   string `json:"guildName,omitempty"`
	ChannelName string `json:"channelName,omitempty"`
}

func (d Destination) Validate() error {
	if !snowflake(d.GuildID) || !snowflake(d.ChannelID) {
		return fmt.Errorf("select a valid Discord server and voice channel")
	}
	return nil
}

type Status struct {
	Enabled bool   `json:"enabled"`
	State   string `json:"state"`
	*Destination
	Error string `json:"error,omitempty"`
}

type Service struct {
	mu        sync.Mutex
	cfg       Config
	mix       *livemixer.Service
	active    map[string]*session
	status    map[string]Status
	open      func(context.Context, Config) (*worker, error)
	apiURL    string
	http      *http.Client
	apiMu     sync.Mutex
	apiClient rest.Client
	closeREST sync.Once
}

type session struct {
	tableID string
	target  Destination
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	ready   chan error
	started sync.Once
}

type worker struct {
	input io.WriteCloser
	done  <-chan struct{}
	err   error
}

func New(cfg Config, mix *livemixer.Service) (*Service, error) {
	if cfg.Token == "" || mix == nil {
		return nil, fmt.Errorf("discord requires a bot token and the live mixer")
	}
	if cfg.Worker == "" {
		cfg.Worker = "discord/stream.mjs"
	}
	return &Service{
		cfg: cfg, mix: mix, open: startWorker,
		active: make(map[string]*session), status: make(map[string]Status),
		apiURL: "https://discord.com/api/v10", http: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (s *Service) Connect(ctx context.Context, tableID string, target Destination) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := target.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	if current := s.active[tableID]; current != nil {
		if current.target.GuildID == target.GuildID && current.target.ChannelID == target.ChannelID &&
			s.status[tableID].State == "connected" {
			s.mu.Unlock()
			return nil
		}
		s.mu.Unlock()
		return ErrInUse
	}
	for _, current := range s.active {
		if current.target.GuildID == target.GuildID {
			s.mu.Unlock()
			return ErrInUse
		}
	}
	_, unsubscribe, err := s.mix.Subscribe(tableID)
	if err != nil {
		s.mu.Unlock()
		return ErrNotReady
	}
	unsubscribe()
	runCtx, cancel := context.WithCancel(context.Background())
	current := &session{tableID: tableID, target: target, ctx: runCtx, cancel: cancel, done: make(chan struct{}), ready: make(chan error, 1)}
	s.active[tableID] = current
	s.status[tableID] = Status{Enabled: true, State: "connecting", Destination: &current.target}
	s.mu.Unlock()
	go s.run(current)
	select {
	case err := <-current.ready:
		return err
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}
}

func (s *Service) Fail(tableID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[tableID] == nil {
		s.status[tableID] = Status{Enabled: true, State: "error", Error: err.Error()}
	}
}

func (s *Service) Disconnect(ctx context.Context, tableID string) error {
	s.mu.Lock()
	current := s.active[tableID]
	s.mu.Unlock()
	if current == nil {
		return nil
	}
	current.cancel()
	select {
	case <-current.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) Status(tableID string) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.status[tableID]
	if !ok {
		return Status{Enabled: true, State: "disconnected"}
	}
	return status
}

func (s *Service) Close() {
	s.mu.Lock()
	sessions := make([]*session, 0, len(s.active))
	for _, current := range s.active {
		sessions = append(sessions, current)
		current.cancel()
	}
	s.mu.Unlock()
	for _, current := range sessions {
		<-current.done
	}
	s.closeREST.Do(func() {
		s.apiMu.Lock()
		client := s.apiClient
		s.apiMu.Unlock()
		if client != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			client.Close(ctx)
		}
	})
}

func (s *Service) run(current *session) {
	err := s.stream(current)
	current.cancel()
	s.mu.Lock()
	if s.active[current.tableID] == current {
		delete(s.active, current.tableID)
		status := Status{Enabled: true, State: "disconnected", Destination: &current.target}
		if err != nil && !errors.Is(err, context.Canceled) {
			status.State, status.Error = "error", err.Error()
		}
		s.status[current.tableID] = status
	}
	s.mu.Unlock()
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("Discord stream ended", "error", err)
	}
	current.started.Do(func() { current.ready <- err })
	close(current.done)
}

func (s *Service) stream(current *session) error {
	cfg := s.cfg
	cfg.GuildID, cfg.ChannelID = current.target.GuildID, current.target.ChannelID
	process, err := s.open(current.ctx, cfg)
	if process != nil {
		defer func() {
			current.cancel()
			_ = process.input.Close()
			<-process.done
		}()
	}
	if err != nil {
		return err
	}
	sub, unsubscribe, err := s.mix.Subscribe(current.tableID)
	if err != nil {
		return ErrNotReady
	}
	defer unsubscribe()
	if err := current.ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	status := s.status[current.tableID]
	status.State = "connected"
	s.status[current.tableID] = status
	s.mu.Unlock()
	current.started.Do(func() { current.ready <- nil })
	for {
		select {
		case <-current.ctx.Done():
			return current.ctx.Err()
		case <-process.done:
			if current.ctx.Err() != nil {
				return current.ctx.Err()
			}
			if process.err == nil {
				return fmt.Errorf("Discord voice worker stopped unexpectedly")
			}
			return process.err
		case frames, open := <-sub.Frames():
			if !open {
				if errors.Is(sub.Err(), livemixer.ErrSessionClosed) || errors.Is(sub.Err(), livemixer.ErrRendererStopped) {
					return nil
				}
				return sub.Err()
			}
			timeout := time.AfterFunc(2*time.Second, current.cancel)
			_, err := process.input.Write(mixer.EncodeFloat32(frames))
			stopped := timeout.Stop()
			if !stopped {
				return fmt.Errorf("Discord PCM transport stalled")
			}
			if err != nil {
				if current.ctx.Err() != nil {
					return current.ctx.Err()
				}
				return fmt.Errorf("Discord PCM transport failed")
			}
		}
	}
}

func snowflake(value string) bool {
	if value == "" || len(value) > 20 || value[0] == '0' {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}
