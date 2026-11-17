package discord

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sync"
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
	service.open = func(ctx context.Context, _ Config) (*transport, error) {
		done := make(chan struct{})
		go func() {
			<-ctx.Done()
			write.Close()
			close(done)
		}()
		return &transport{input: write, done: done}, nil
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
	service.open = func(context.Context, Config) (*transport, error) {
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
	service.open = func(ctx context.Context, _ Config) (*transport, error) {
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

func TestTransportExitRestoresReconnectableErrorState(t *testing.T) {
	service, _ := New(Config{Token: "test-only"}, testMix(t))
	defer service.Close()
	updates := make(chan Status, 4)
	service.SetStatusListener(func(tableID string, status Status) {
		if tableID != "table" {
			t.Errorf("status crossed tables: %s", tableID)
		}
		updates <- status
	})
	read, write := io.Pipe()
	defer read.Close()
	done := make(chan struct{})
	voiceTransport := &transport{input: write, done: done}
	service.open = func(context.Context, Config) (*transport, error) { return voiceTransport, nil }
	if err := service.Connect(context.Background(), "table", Destination{GuildID: "1", ChannelID: "2"}); err != nil {
		t.Fatal(err)
	}
	voiceTransport.err = errors.New("voice lost")
	close(done)
	waitState(t, service, "error")
	if service.Status("table").Error != "voice lost" {
		t.Fatal("transport failure was hidden")
	}
	for _, state := range []string{"connecting", "connected", "error"} {
		select {
		case update := <-updates:
			if update.State != state || update.Destination == nil || update.ChannelID != "2" {
				t.Fatalf("unexpected transition: %+v; want %s with destination", update, state)
			}
			if state == "error" && update.Error != "voice lost" {
				t.Fatalf("failure event lost diagnostics: %+v", update)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing %s event", state)
		}
	}
}

func TestStatusListenerReportsDisconnectAndRestoreFailure(t *testing.T) {
	service, _ := New(Config{Token: "test-only"}, testMix(t))
	defer service.Close()
	updates := make(chan Status, 4)
	service.SetStatusListener(func(_ string, status Status) { updates <- status })
	service.open = func(ctx context.Context, _ Config) (*transport, error) {
		done := make(chan struct{})
		go func() { <-ctx.Done(); close(done) }()
		return &transport{input: discardWriter{}, done: done}, nil
	}
	if err := service.Connect(context.Background(), "table", Destination{GuildID: "1", ChannelID: "2"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Disconnect(context.Background(), "table"); err != nil {
		t.Fatal(err)
	}
	service.Fail("table", errors.New("saved destination unavailable"))
	for _, state := range []string{"connecting", "connected", "disconnected", "error"} {
		select {
		case update := <-updates:
			if update.State != state || update.Destination == nil || update.ChannelID != "2" {
				t.Fatalf("unexpected transition: %+v; want %s", update, state)
			}
			if state == "error" && update.Error != "saved destination unavailable" {
				t.Fatalf("restore failure event lost diagnostics: %+v", update)
			}
		case <-time.After(time.Second):
			t.Fatalf("missing %s event", state)
		}
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
	for _, target := range []Destination{{}, {GuildID: "1", ChannelID: "invalid"}, {GuildID: "0", ChannelID: "2"}, {GuildID: "18446744073709551616", ChannelID: "2"}} {
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
	service.open = func(ctx context.Context, cfg Config) (*transport, error) {
		done := make(chan struct{})
		go func() { <-ctx.Done(); close(done) }()
		return &transport{input: discardWriter{}, done: done}, nil
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

// skewedVoiceOutput models live Discord timing: per-packet UDP jitter, a DAVE
// key rotation holding the encrypt lock, and a speaking refresh waiting on the
// voice gateway lock while a listener joins.
type skewedVoiceOutput struct {
	mu       sync.Mutex
	packets  int
	speaking int
}

func (*skewedVoiceOutput) Ready() bool { return true }

func (o *skewedVoiceOutput) Speaking(ctx context.Context) error {
	o.mu.Lock()
	o.speaking++
	refresh := o.speaking > 1
	o.mu.Unlock()
	if refresh {
		select {
		case <-time.After(300 * time.Millisecond):
		case <-ctx.Done():
		}
	}
	return nil
}

func (o *skewedVoiceOutput) Write(packet []byte) error {
	o.mu.Lock()
	sent := o.packets
	if len(packet) > 0 {
		o.packets++
	}
	o.mu.Unlock()
	delay := time.Duration(sent%7) * time.Millisecond
	if sent == 40 {
		delay = 250 * time.Millisecond
	}
	time.Sleep(delay)
	return nil
}

func (o *skewedVoiceOutput) sent() uint64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return uint64(o.packets)
}

func TestTransportSustainsRealTimeMixThroughSendStalls(t *testing.T) {
	mix := testMix(t)
	renderCtx, stopRender := context.WithCancel(context.Background())
	rendered := make(chan struct{})
	go func() { mix.Run(renderCtx); close(rendered) }()
	defer func() { stopRender(); <-rendered }()
	service, err := New(Config{Token: "test-only"}, mix)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	output := &skewedVoiceOutput{}
	refresh := make(chan struct{}, 1)
	service.open = func(ctx context.Context, _ Config) (*transport, error) {
		return startTransport(ctx, output, nil, refresh, func() {})
	}
	if err := service.Connect(context.Background(), "table", Destination{GuildID: "1", ChannelID: "2"}); err != nil {
		t.Fatal(err)
	}
	start := mix.Status("table").FramesRendered
	time.Sleep(time.Second)
	refresh <- struct{}{}
	time.Sleep(1500 * time.Millisecond)
	if state := service.Status("table"); state.State != "connected" {
		t.Fatalf("stream did not survive send stalls: %+v", state)
	}
	want := (mix.Status("table").FramesRendered - start) / opusSamples
	if want < 100 {
		t.Fatalf("mixer rendered only %d blocks", want)
	}
	deadline := time.Now().Add(2 * time.Second)
	for output.sent() < want {
		if time.Now().After(deadline) {
			t.Fatalf("sent %d of %d rendered blocks", output.sent(), want)
		}
		time.Sleep(5 * time.Millisecond)
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
