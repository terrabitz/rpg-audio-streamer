package e2e

import (
	"context"
	"sync"
	"time"
)

const packetsPerWindow = sampleRate / opusFrameSamples

// Audio is the listener's latest view of the streamer's audio.
type Audio struct {
	Signal
	Frames  int
	Present bool
	Last    time.Time
	Err     error
}

type monitor struct {
	decode  func(context.Context, [][]byte) ([]byte, error)
	mu      sync.Mutex
	pending [][]byte
	audio   Audio
}

func newMonitor(decode func(context.Context, [][]byte) ([]byte, error)) *monitor {
	return &monitor{decode: decode}
}

func (m *monitor) receive(packet []byte, at time.Time) {
	m.mu.Lock()
	m.audio.Last = at
	m.pending = append(m.pending, append([]byte(nil), packet...))
	if len(m.pending) < packetsPerWindow {
		m.mu.Unlock()
		return
	}
	window := m.pending
	m.pending = nil
	m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pcm, err := m.decode(ctx, window)
	if err != nil {
		m.fail(err)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audio.Frames += len(pcm) / 4
	m.audio.Signal = analyze(pcm)
}

func (m *monitor) setPresent(present bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audio.Present = present
}

func (m *monitor) fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.audio.Err == nil {
		m.audio.Err = err
	}
}

func (m *monitor) latest() Audio {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.audio
}
