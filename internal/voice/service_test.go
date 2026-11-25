package voice

import (
	"context"
	"errors"
	"testing"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

var (
	gm        = domain.GMViewer()
	selection = &domain.Destination{
		GuildID:   "1",
		ChannelID: "2",
	}
)

func TestUseCasesRejectPlayers(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	player := domain.PlayerViewer(f.table.ID)
	attempts := map[string]func() error{
		"Status":     func() error { _, err := f.service.Status(ctx, player, f.table.ExternalID); return err },
		"Connect":    func() error { _, err := f.service.Connect(ctx, player, f.table.ExternalID, selection); return err },
		"Disconnect": func() error { _, err := f.service.Disconnect(ctx, player, f.table.ExternalID); return err },
		"Setup":      func() error { _, err := f.service.Setup(ctx, player); return err },
		"Channels":   func() error { _, err := f.service.Channels(ctx, player, "1"); return err },
	}
	for name, attempt := range attempts {
		if err := attempt(); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s by player: %v", name, err)
		}
	}
	if len(f.discord.connected) != 0 || len(f.discord.disconnected) != 0 {
		t.Fatal("rejected use cases reached Discord")
	}
}

func TestUnconfiguredDiscordReportsDisconnected(t *testing.T) {
	f := newFixture()
	service := f.unconfigured()
	ctx := context.Background()
	status, err := service.Status(ctx, gm, f.table.ExternalID)
	if err != nil || status != disconnected {
		t.Fatalf("status: %+v %v", status, err)
	}
	if _, err := service.Status(ctx, gm, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing table: %v", err)
	}
	commands := map[string]func() error{
		"Connect":    func() error { _, err := service.Connect(ctx, gm, f.table.ExternalID, selection); return err },
		"Disconnect": func() error { _, err := service.Disconnect(ctx, gm, f.table.ExternalID); return err },
		"Setup":      func() error { _, err := service.Setup(ctx, gm); return err },
		"Channels":   func() error { _, err := service.Channels(ctx, gm, "1"); return err },
	}
	for name, command := range commands {
		if err := command(); !errors.Is(err, ErrNotConfigured) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestStatusFallsBackToSavedDestination(t *testing.T) {
	f := newFixture()
	f.store.destinations[f.table.ID] = *selection
	status, err := f.service.Status(context.Background(), gm, f.table.ExternalID)
	if err != nil || status.State != "disconnected" || status.Destination == nil || status.GuildID != "1" {
		t.Fatalf("status: %+v %v", status, err)
	}
}

func TestConnectJoinsAndSavesResolvedSelection(t *testing.T) {
	f := newFixture()
	status, err := f.service.Connect(context.Background(), gm, f.table.ExternalID, selection)
	if err != nil || status.State != "connected" {
		t.Fatalf("connect: %+v %v", status, err)
	}
	saved := f.store.destinations[f.table.ID]
	if saved.GuildName != "Guild 1" || saved.ChannelName != "Channel 2" {
		t.Fatalf("saved: %+v", saved)
	}
	if f.discord.connected[f.table.ID.String()] != saved {
		t.Fatalf("connected: %+v", f.discord.connected)
	}
}

func TestConnectWithoutSelectionUsesSavedDestination(t *testing.T) {
	f := newFixture()
	if _, err := f.service.Connect(context.Background(), gm, f.table.ExternalID, nil); !errors.Is(err, ErrNoDestination) {
		t.Fatalf("nothing saved: %v", err)
	}
	f.store.destinations[f.table.ID] = *selection
	if _, err := f.service.Connect(context.Background(), gm, f.table.ExternalID, nil); err != nil {
		t.Fatal(err)
	}
	if f.discord.connected[f.table.ID.String()] != *selection {
		t.Fatalf("connected: %+v", f.discord.connected)
	}
}

func TestConnectRejectsBadSelections(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	invalid := &domain.Destination{GuildID: "0", ChannelID: "abc"}
	if _, err := f.service.Connect(ctx, gm, f.table.ExternalID, invalid); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("invalid selection: %v", err)
	}
	f.discord.resolveFail = errDiscord
	_, err := f.service.Connect(ctx, gm, f.table.ExternalID, selection)
	if !errors.Is(err, domain.ErrUpstream) || err.Error() != errDiscord.Error() {
		t.Fatalf("unresolvable selection: %v", err)
	}
	if len(f.discord.connected) != 0 {
		t.Fatalf("connected: %+v", f.discord.connected)
	}
}

func TestConnectClassifiesDiscordFailures(t *testing.T) {
	inUse := domain.Conflict("Discord bot is already connected to another table in this server")
	tests := []struct {
		name string
		fail error
		want error
	}{
		{name: "user-facing", fail: inUse, want: inUse},
		{name: "unexpected", fail: errDiscord, want: domain.ErrUpstream},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			f.discord.connectFailures = []error{tt.fail}
			if _, err := f.service.Connect(context.Background(), gm, f.table.ExternalID, selection); !errors.Is(err, tt.want) {
				t.Fatalf("connect: %v", err)
			}
			if _, saved := f.store.destinations[f.table.ID]; saved {
				t.Fatal("failed destination was saved")
			}
		})
	}
}

func TestConnectDisconnectsWhenDestinationCannotBeSaved(t *testing.T) {
	f := newFixture()
	f.store.saveFail = errStore
	_, err := f.service.Connect(context.Background(), gm, f.table.ExternalID, selection)
	if !errors.Is(err, ErrSaveFailed) || !errors.Is(err, errStore) {
		t.Fatalf("connect: %v", err)
	}
	if len(f.discord.disconnected) != 1 || len(f.discord.connected) != 0 {
		t.Fatalf("bot stayed connected: %+v", f.discord.connected)
	}
}

func TestConnectRestoresLiveMixBeforeRetry(t *testing.T) {
	f := newFixture()
	f.discord.connectFailures = []error{domain.ErrTableMixNotReady}
	f.playback.snapshot = events.Snapshot{
		Tracks: []events.PlaybackTrack{
			{
				FileID:      "playing",
				IsPlaying:   true,
				IsRepeating: true,
				Volume:      40,
				CurrentTime: 12,
				Duration:    60,
			},
			{
				FileID:    "paused",
				IsPlaying: false,
			},
		},
		Settings: &domain.TableSettings{
			MasterVolume: 50,
			TypeVolumes:  map[string]float64{"Music": 50},
		},
	}

	if _, err := f.service.Connect(context.Background(), gm, f.table.ExternalID, selection); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.discord.connected[f.table.ID.String()]; !ok {
		t.Fatal("Discord was not retried")
	}
	if len(f.events.sent) != 1 || f.events.sent[0].name != events.MixChanged {
		t.Fatalf("events: %+v", f.events.sent)
	}
	payload := f.events.sent[0].payload.(events.TableEvent[events.MixSnapshot])
	if payload.TableID != f.table.ID.String() || len(payload.Data.Tracks) != 1 {
		t.Fatalf("mix: %+v", payload)
	}
	track := payload.Data.Tracks[0]
	if track.FileID != "playing" || !*track.IsPlaying || !*track.IsRepeating ||
		*track.Volume != 40 || *track.CurrentTime != 12 || *track.Duration != 60 || track.Playback == nil {
		t.Fatalf("playing track: %+v", track)
	}
	if payload.Data.TypeVolumes["Music"] != 25 || payload.Data.TypeVolumes["Ambiance"] != 50 {
		t.Fatalf("type volumes: %+v", payload.Data.TypeVolumes)
	}
}

func TestConnectReportsMixRestoreFailures(t *testing.T) {
	f := newFixture()
	f.discord.connectFailures = []error{domain.ErrTableMixNotReady}
	if _, err := f.service.Connect(context.Background(), gm, f.table.ExternalID, selection); !errors.Is(err, ErrMissingSettings) {
		t.Fatalf("missing settings: %v", err)
	}

	f = newFixture()
	f.discord.connectFailures = []error{domain.ErrTableMixNotReady}
	f.playback.snapshot.Settings = &domain.TableSettings{MasterVolume: 100}
	f.events.fail = errDiscord
	if _, err := f.service.Connect(context.Background(), gm, f.table.ExternalID, selection); !errors.Is(err, ErrMixRestoreFailed) {
		t.Fatalf("dispatch failure: %v", err)
	}
}

func TestDisconnectStopsStreaming(t *testing.T) {
	f := newFixture()
	f.discord.connected[f.table.ID.String()] = *selection
	status, err := f.service.Disconnect(context.Background(), gm, f.table.ExternalID)
	if err != nil || status.State != "disconnected" {
		t.Fatalf("disconnect: %+v %v", status, err)
	}
	f.discord.disconnectFail = errDiscord
	if _, err := f.service.Disconnect(context.Background(), gm, f.table.ExternalID); !errors.Is(err, domain.ErrUpstream) {
		t.Fatalf("disconnect failure: %v", err)
	}
}

func TestSetupAndChannelsListDiscordChoices(t *testing.T) {
	f := newFixture()
	ctx := context.Background()
	f.discord.setup = domain.DiscordSetup{InstallURL: "https://discord.example/install"}
	f.discord.channels = []domain.DiscordChoice{{ID: "2", Name: "Voice"}}
	if setup, err := f.service.Setup(ctx, gm); err != nil || setup.InstallURL == "" {
		t.Fatalf("setup: %+v %v", setup, err)
	}
	if channels, err := f.service.Channels(ctx, gm, "1"); err != nil || len(channels) != 1 {
		t.Fatalf("channels: %+v %v", channels, err)
	}
	f.discord.listFail = errDiscord
	if _, err := f.service.Setup(ctx, gm); !errors.Is(err, domain.ErrUpstream) {
		t.Fatalf("setup failure: %v", err)
	}
	if _, err := f.service.Channels(ctx, gm, "1"); !errors.Is(err, domain.ErrUpstream) {
		t.Fatalf("channels failure: %v", err)
	}
}
