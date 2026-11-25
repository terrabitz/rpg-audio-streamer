package discord

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

type recordingDisconnector struct {
	stopped chan string
}

func (d *recordingDisconnector) Disconnect(_ context.Context, tableID string) error {
	d.stopped <- tableID
	return nil
}

func TestClosingTableDoesNotWaitForDecodeAndCancelsInFlightSync(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	mix, err := livemixer.New(48000, func(context.Context, string, string) (livemixer.Clip, error) {
		close(entered)
		<-release
		return livemixer.Clip{Frames: []mixer.Frame{{Left: 0.5}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	voice := &recordingDisconnector{stopped: make(chan string, 1)}
	sessions := NewMixerSessions(mix, voice, slog.New(slog.NewTextHandler(io.Discard, nil)))
	result := make(chan error, 1)
	defer func() {
		close(release)
		if err := <-result; err == nil {
			t.Error("closed table accepted an in-flight snapshot")
		}
	}()
	go func() { result <- sessions.Sync("table", []byte(`{"tracks":[{"fileID":"tone"}]}`)) }()
	<-entered
	closed := make(chan struct{})
	go func() { sessions.CloseTable("table"); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("closing table waited for audio decoding")
	}
	if got := <-voice.stopped; got != "table" {
		t.Fatalf("disconnected %q, want table", got)
	}
}

func TestClosingTableDisconnectsDiscordAndAllowsNewSession(t *testing.T) {
	mix, err := livemixer.New(48000, func(context.Context, string, string) (livemixer.Clip, error) {
		return livemixer.Clip{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	voice := &recordingDisconnector{stopped: make(chan string, 2)}
	sessions := NewMixerSessions(mix, voice, slog.New(slog.NewTextHandler(io.Discard, nil)))
	sync := func() {
		t.Helper()
		if err := sessions.Sync("table", []byte(`{"tracks":[]}`)); err != nil {
			t.Fatal(err)
		}
	}
	sync()
	sessions.CloseTable("table")
	select {
	case <-voice.stopped:
	default:
		t.Fatal("closing table did not disconnect Discord")
	}
	sync()
	sessions.CloseTable("table")
}
