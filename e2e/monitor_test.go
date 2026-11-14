package e2e

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMonitorAnalyzesOneSecondWindows(t *testing.T) {
	packets := encodeOpus(t, "aevalsrc=0.4*sin(440*2*PI*t):s=48000:d=1.2")
	m := newMonitor(decodeOpus)
	start := time.Now()
	for i, packet := range packets[:packetsPerWindow-1] {
		m.receive(packet, start.Add(time.Duration(i)*20*time.Millisecond))
	}
	if audio := m.latest(); audio.Frames != 0 || audio.Signal != (Signal{}) {
		t.Fatalf("analyzed a partial window: %+v", audio)
	}
	last := packets[packetsPerWindow-1]
	m.receive(last, start.Add(time.Second))
	clear(last)
	audio := m.latest()
	if audio.Err != nil || audio.Frames != sampleRate || !audio.Last.Equal(start.Add(time.Second)) {
		t.Fatalf("first window = %+v", audio)
	}
	if audio.Low < 0.37 || audio.High > 0.02 {
		t.Fatalf("window tones = %+v", audio.Signal)
	}
}

func TestMonitorKeepsFirstFailure(t *testing.T) {
	failure := errors.New("decode failed")
	m := newMonitor(func(context.Context, [][]byte) ([]byte, error) { return nil, failure })
	for range packetsPerWindow {
		m.receive([]byte{0xf8, 0xff, 0xfe}, time.Now())
	}
	m.fail(errors.New("later failure"))
	if err := m.latest().Err; !errors.Is(err, failure) {
		t.Fatalf("error = %v", err)
	}
}

func TestMonitorTracksPresence(t *testing.T) {
	m := newMonitor(decodeOpus)
	m.setPresent(true)
	if !m.latest().Present {
		t.Fatal("streamer not present")
	}
	m.setPresent(false)
	if m.latest().Present {
		t.Fatal("streamer still present")
	}
}
