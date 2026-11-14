package livemixer

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

var (
	ErrSessionClosed   = errors.New("mix session closed")
	ErrSlowMonitor     = errors.New("audio monitor fell behind")
	ErrRendererStopped = errors.New("mixer renderer stopped")
)

type Subscription struct {
	frames chan []mixer.Frame
	mu     sync.Mutex
	err    error
}

// Subscribe observes the existing timeline; it never starts or advances playback.
func (s *Service) Subscribe(tableID string) (*Subscription, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tables[tableID] == nil {
		return nil, nil, fmt.Errorf("mix session is not ready")
	}
	sub := &Subscription{frames: make(chan []mixer.Frame, 2)}
	if s.listeners[tableID] == nil {
		s.listeners[tableID] = make(map[*Subscription]bool)
	}
	s.listeners[tableID][sub] = true
	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.listeners[tableID][sub] {
			delete(s.listeners[tableID], sub)
			sub.close(nil)
		}
	}
	return sub, cancel, nil
}

// Frames must be treated as read-only; a block may be shared by multiple monitors.
func (s *Subscription) Frames() <-chan []mixer.Frame {
	return s.frames
}

func (s *Subscription) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *Service) publish(tableID string, frames []mixer.Frame) {
	if len(frames) == 0 || len(s.listeners[tableID]) == 0 {
		return
	}
	block := slices.Clone(frames)
	for sub := range s.listeners[tableID] {
		select {
		case sub.frames <- block:
		default:
			delete(s.listeners[tableID], sub)
			sub.close(ErrSlowMonitor)
		}
	}
}

func (s *Service) closeStreams(tableID string, err error) {
	for sub := range s.listeners[tableID] {
		sub.close(err)
	}
	delete(s.listeners, tableID)
}

func (s *Subscription) close(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
	close(s.frames)
}
