package server

import (
	"context"
	"testing"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

type reconnectDiscord struct {
	*testDiscord
	started chan discord.Destination
	stopped chan struct{}
}

func TestClosingGMTableDoesNotWaitForDecodeOrRecreateDiscordSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	bot := &reconnectDiscord{testDiscord: &testDiscord{}, started: make(chan discord.Destination, 2), stopped: make(chan struct{}, 2)}
	ts.discord = bot
	entered, release := make(chan struct{}), make(chan struct{})
	mix, err := livemixer.New(48000, func(context.Context, string, string) (livemixer.Clip, error) {
		close(entered)
		<-release
		return livemixer.Clip{Frames: []mixer.Frame{{Left: 0.5}}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := newDiscordMixer(mix, ts.Server)
	result := make(chan error, 1)
	defer func() {
		close(release)
		if err := <-result; err == nil {
			t.Error("closed table accepted an in-flight snapshot")
		}
	}()
	go func() { result <- lifecycle.Sync(DefaultTableID.String(), []byte(`{"tracks":[{"fileID":"tone"}]}`)) }()
	<-entered
	closed := make(chan struct{})
	go func() { lifecycle.CloseTable(DefaultTableID.String()); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("closing GM table waited for audio decoding")
	}
	select {
	case <-bot.started:
		t.Fatal("closed table reconnected Discord")
	default:
	}
}

func (d *reconnectDiscord) Connect(ctx context.Context, table string, target discord.Destination) error {
	d.started <- target
	return nil
}

func (d *reconnectDiscord) Disconnect(context.Context, string) error {
	d.stopped <- struct{}{}
	return nil
}

func TestDiscordReconnectsOncePerGMTableSessionAndClosesWithIt(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	target := discord.Destination{GuildID: "1", ChannelID: "2"}
	if err := ts.store.SaveTableDiscord(context.Background(), DefaultTableID, target); err != nil {
		t.Fatal(err)
	}
	bot := &reconnectDiscord{testDiscord: &testDiscord{}, started: make(chan discord.Destination, 2), stopped: make(chan struct{}, 2)}
	ts.discord = bot
	mix, err := livemixer.New(48000, ts.mixerSource)
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := newDiscordMixer(mix, ts.Server)
	sync := func() {
		t.Helper()
		if err := lifecycle.Sync(DefaultTableID.String(), []byte(`{"tracks":[]}`)); err != nil {
			t.Fatal(err)
		}
	}
	wait := func() {
		t.Helper()
		select {
		case got := <-bot.started:
			if got != target {
				t.Fatalf("reconnect destination = %+v", got)
			}
		case <-time.After(time.Second):
			t.Fatal("saved destination did not reconnect")
		}
	}
	sync()
	wait()
	sync()
	select {
	case <-bot.started:
		t.Fatal("repeated sync reconnected a manually disconnected session")
	default:
	}
	lifecycle.CloseTable(DefaultTableID.String())
	select {
	case <-bot.stopped:
	default:
		t.Fatal("closing table did not disconnect Discord")
	}
	sync()
	wait()
	lifecycle.CloseTable(DefaultTableID.String())
}
