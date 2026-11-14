package discord

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

func TestConnectionStreamsExistingMixAndReservesBotUntilDisconnect(t *testing.T) {
	mix := testMix(t)
	service, err := New(Config{Token: "test-only"}, mix)
	if err != nil {
		t.Fatal(err)
	}
	read, write := io.Pipe()
	service.open = func(ctx context.Context, _ Config) (*worker, error) {
		done := make(chan struct{})
		go func() {
			<-ctx.Done()
			write.Close()
			close(done)
		}()
		return &worker{input: write, done: done}, nil
	}
	defer read.Close()
	defer service.Close()
	if err := service.Connect(context.Background(), "table", Destination{GuildID: "1", ChannelID: "2"}); err != nil {
		t.Fatal(err)
	}
	if service.Status("table").State != "connected" || service.Status("other").State != "disconnected" {
		t.Fatalf("connection states: %+v %+v", service.Status("table"), service.Status("other"))
	}
	if err := service.Connect(context.Background(), "other", Destination{GuildID: "1", ChannelID: "3"}); !errors.Is(err, ErrInUse) {
		t.Fatalf("another table stole the bot: %v", err)
	}
	if err := service.Disconnect(context.Background(), "other"); err != nil {
		t.Fatalf("another table disconnected the bot: %v", err)
	}
	data := make(chan []byte, 1)
	go func() {
		frame := make([]byte, 8)
		_, _ = io.ReadFull(read, frame)
		data <- frame
	}()
	mix.Render("table", 1)
	select {
	case frame := <-data:
		left := math.Float32frombits(binary.LittleEndian.Uint32(frame))
		right := math.Float32frombits(binary.LittleEndian.Uint32(frame[4:]))
		if left != 0.25 || right != -0.5 {
			t.Fatalf("wrong stereo PCM: %v %v", left, right)
		}
	case <-time.After(time.Second):
		t.Fatal("mix did not reach Discord transport")
	}
	mix.CloseTable("table")
	waitState(t, service, "disconnected")
	if err := service.Disconnect(context.Background(), "table"); err != nil {
		t.Fatal(err)
	}
}

func TestConnectionFailureAndCancelledSetupAreObservable(t *testing.T) {
	service, _ := New(Config{Token: "test-only"}, testMix(t))
	defer service.Close()
	service.open = func(context.Context, Config) (*worker, error) {
		return nil, errors.New("voice unavailable")
	}
	if err := service.Connect(context.Background(), "missing", Destination{GuildID: "1", ChannelID: "2"}); !errors.Is(err, ErrNotReady) {
		t.Fatalf("missing mix must fail before voice startup: %v", err)
	}
	if err := service.Connect(context.Background(), "table", Destination{GuildID: "1", ChannelID: "2"}); err == nil {
		t.Fatal("voice failure reported success")
	}
	if state := service.Status("table"); state.State != "error" || state.Error == "" {
		t.Fatalf("failure lost: %+v", state)
	}
	entered := make(chan struct{})
	service.open = func(ctx context.Context, _ Config) (*worker, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	started := make(chan error, 1)
	go func() {
		started <- service.Connect(context.Background(), "table", Destination{GuildID: "1", ChannelID: "2"})
	}()
	<-entered
	if service.Status("table").State != "connecting" {
		t.Fatal("pending connection not exposed")
	}
	if err := service.Disconnect(context.Background(), "table"); err != nil {
		t.Fatal(err)
	}
	if err := <-started; err == nil {
		t.Fatal("cancelled connection reported success")
	}
	waitState(t, service, "disconnected")
}

func TestWorkerExitRestoresReconnectableErrorState(t *testing.T) {
	service, _ := New(Config{Token: "test-only"}, testMix(t))
	defer service.Close()
	read, write := io.Pipe()
	defer read.Close()
	done := make(chan struct{})
	process := &worker{input: write, done: done}
	service.open = func(context.Context, Config) (*worker, error) { return process, nil }
	if err := service.Connect(context.Background(), "table", Destination{GuildID: "1", ChannelID: "2"}); err != nil {
		t.Fatal(err)
	}
	process.err = errors.New("voice lost")
	close(done)
	waitState(t, service, "error")
	if service.Status("table").Error != "voice lost" {
		t.Fatal("worker failure was hidden")
	}
}

func TestConfigurationNeedsOnlyBotTokenAndDestinationMustBeValid(t *testing.T) {
	if _, err := New(Config{}, testMix(t)); err == nil {
		t.Fatal("missing token accepted")
	}
	service, err := New(Config{Token: "test-only"}, testMix(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Destination{{}, {GuildID: "1", ChannelID: "invalid"}, {GuildID: "0", ChannelID: "2"}} {
		if err := service.Connect(context.Background(), "table", target); err == nil {
			t.Fatal("invalid destination accepted")
		}
	}
}

func TestTablesCanStreamToDifferentServersWithoutStealingConnections(t *testing.T) {
	mix := testMix(t)
	if err := mix.Sync("other", []byte(`{"tracks":[]}`)); err != nil {
		t.Fatal(err)
	}
	service, err := New(Config{Token: "test-only"}, mix)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	service.open = func(ctx context.Context, cfg Config) (*worker, error) {
		done := make(chan struct{})
		go func() { <-ctx.Done(); close(done) }()
		return &worker{input: discardWriter{}, done: done}, nil
	}
	first := Destination{GuildID: "1", ChannelID: "2"}
	second := Destination{GuildID: "3", ChannelID: "4"}
	for table, target := range map[string]Destination{"table": first, "other": second} {
		if err := service.Connect(context.Background(), table, target); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Connect(context.Background(), "other", first); !errors.Is(err, ErrInUse) {
		t.Fatalf("active table changed destination: %v", err)
	}
	if err := service.Disconnect(context.Background(), "other"); err != nil {
		t.Fatal(err)
	}
	if service.Status("table").State != "connected" || service.Status("table").GuildID != "1" {
		t.Fatal("disconnect affected another server")
	}
}

type discardWriter struct{}

func (discardWriter) Write(data []byte) (int, error) { return len(data), nil }
func (discardWriter) Close() error                   { return nil }

func testMix(t *testing.T) *livemixer.Service {
	t.Helper()
	mix, _ := livemixer.New(48000, func(context.Context, string, string) (livemixer.Clip, error) {
		return livemixer.Clip{Frames: []mixer.Frame{{Left: 0.25, Right: -0.5}}}, nil
	})
	if err := mix.Update("table", []byte(`{"fileID":"tone","isPlaying":true}`)); err != nil {
		t.Fatal(err)
	}
	return mix
}

func waitState(t *testing.T, service *Service, want string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if service.Status("table").State == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("wanted %s, got %+v", want, service.Status("table"))
}
