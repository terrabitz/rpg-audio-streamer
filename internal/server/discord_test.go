package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
)

type testDiscord struct {
	connected       string
	fail            error
	connectFailures []error
}

func (d *testDiscord) Connect(_ context.Context, id string, target discord.Destination) error {
	if len(d.connectFailures) > 0 {
		err := d.connectFailures[0]
		d.connectFailures = d.connectFailures[1:]
		if err != nil {
			return err
		}
	}
	if d.fail != nil {
		return d.fail
	}
	d.connected = id
	return nil
}

func (d *testDiscord) Disconnect(_ context.Context, id string) error {
	d.connected = ""
	return d.fail
}

func (d *testDiscord) Status(string) discord.Status {
	state := "disconnected"
	if d.connected != "" {
		state = "connected"
	}
	return discord.Status{Enabled: true, State: state}
}

func (d *testDiscord) Close() {}
func (d *testDiscord) Setup(context.Context) (discord.Setup, error) {
	return discord.Setup{InstallURL: "https://discord.com/oauth2/authorize?client_id=99", Guilds: []discord.Choice{{ID: "1", Name: "Friday"}}}, d.fail
}
func (d *testDiscord) Channels(context.Context, string) ([]discord.Choice, error) {
	return []discord.Choice{{ID: "2", Name: "Music"}}, d.fail
}
func (d *testDiscord) Resolve(_ context.Context, target discord.Destination) (discord.Destination, error) {
	return target, d.fail
}

type testPlaybackSnapshot struct {
	snapshot events.Snapshot
}

func (p *testPlaybackSnapshot) Snapshot(context.Context, uuid.UUID) (events.Snapshot, error) {
	return p.snapshot, nil
}

func (p *testPlaybackSnapshot) Update(context.Context, uuid.UUID, uuid.UUID, events.PlaybackPatch) error {
	return nil
}

type recordingDiscordHub struct {
	*mockWSRegisterer
	event   string
	payload any
}

func (h *recordingDiscordHub) SendEvent(event string, payload any) error {
	h.event, h.payload = event, payload
	return nil
}

func TestDiscordRoutesRequireGMAndControlOnlyRequestedTable(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	bot := &testDiscord{}
	ts.discord = bot
	path := "/api/v1/tables/" + testDefaultExternalID + "/discord"
	for _, method := range []string{"GET", "POST", "DELETE"} {
		if rec := routedRequest(ts.Handler(), method, path, "", false); rec.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s: %d", method, rec.Code)
		}
		for _, body := range []string{"", "{", "{}", `{"guildID":"1","channelID":"bad"}`, `{"guildID":"1","channelID":"2"} {}`} {
			if rec := routedRequest(ts.Handler(), "POST", path, body, true); rec.Code != http.StatusBadRequest {
				t.Fatalf("invalid selection %q: %d %s", body, rec.Code, rec.Body)
			}
		}
	}
	rec := routedRequest(ts.Handler(), "POST", path, `{"guildID":"1","channelID":"2"}`, true)
	if rec.Code != http.StatusOK || bot.connected != DefaultTableID.String() {
		t.Fatalf("connect: %d, table %q", rec.Code, bot.connected)
	}
	rec = routedRequest(ts.Handler(), "GET", path, "", true)
	var status discord.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil || status.State != "connected" ||
		status.Destination == nil || status.GuildID != "1" || status.ChannelID != "2" {
		t.Fatalf("status: %s %v", rec.Body, err)
	}
	rec = routedRequest(ts.Handler(), "DELETE", path, "", true)
	if rec.Code != http.StatusOK || bot.connected != "" {
		t.Fatalf("disconnect: %d, table %q", rec.Code, bot.connected)
	}
	rec = routedRequest(ts.Handler(), "POST", path, "", true)
	if rec.Code != http.StatusOK || bot.connected != DefaultTableID.String() {
		t.Fatalf("saved destination reconnect: %d %s", rec.Code, rec.Body)
	}
	for _, failure := range []struct {
		err  error
		code int
	}{{discord.ErrNotReady, 409}, {discord.ErrInUse, 409}, {errors.New("voice unavailable"), 502}} {
		bot.fail = failure.err
		if rec := routedRequest(ts.Handler(), "POST", path, "", true); rec.Code != failure.code {
			t.Fatalf("failed connect: %d, want %d", rec.Code, failure.code)
		}
	}
	if rec := routedRequest(ts.Handler(), "POST", "/api/v1/tables/not-an-id/discord", "", true); rec.Code != 404 {
		t.Fatalf("invalid table: %d", rec.Code)
	}
}

func TestDiscordReconnectRestoresLiveMixBeforeRetry(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	ts.discord = &testDiscord{connectFailures: []error{discord.ErrNotReady}}
	ts.mixer = &livemixer.Service{}
	ts.playback = &testPlaybackSnapshot{snapshot: events.Snapshot{
		Tracks: []events.PlaybackTrack{
			{FileID: "playing", IsPlaying: true, IsRepeating: true, Volume: 40, CurrentTime: 12, Duration: 60},
			{FileID: "paused", IsPlaying: false},
		},
		Settings: &domain.TableSettings{
			MasterVolume: 50,
			TypeVolumes:  map[string]float64{"Music": 50},
		},
	}}
	hub := &recordingDiscordHub{mockWSRegisterer: &mockWSRegisterer{t: t}}
	ts.hub = hub

	path := "/api/v1/tables/" + testDefaultExternalID + "/discord"
	rec := routedRequest(ts.Handler(), "POST", path, `{"guildID":"1","channelID":"2"}`, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("reconnect: %d %s", rec.Code, rec.Body)
	}
	bot := ts.discord.(*testDiscord)
	if bot.connected != DefaultTableID.String() || len(bot.connectFailures) != 0 {
		t.Fatalf("Discord was not retried successfully: connected=%q remaining failures=%d", bot.connected, len(bot.connectFailures))
	}
	if hub.event != events.MixChanged {
		t.Fatalf("sent event %q, want %q", hub.event, events.MixChanged)
	}
	payload, ok := hub.payload.(events.TableEvent[events.MixSnapshot])
	if !ok {
		t.Fatalf("unexpected event payload type %T", hub.payload)
	}
	if payload.TableID != DefaultTableID.String() || len(payload.Data.Tracks) != 1 {
		t.Fatalf("unexpected mix snapshot: %+v", payload)
	}
	track := payload.Data.Tracks[0]
	if track.FileID != "playing" || track.IsPlaying == nil || !*track.IsPlaying ||
		track.Volume == nil || *track.Volume != 40 ||
		track.CurrentTime == nil || *track.CurrentTime != 12 ||
		track.Duration == nil || *track.Duration != 60 {
		t.Fatalf("unexpected playing track patch: %+v", track)
	}
	if payload.Data.TypeVolumes["Music"] != 25 || payload.Data.TypeVolumes["Ambiance"] != 50 {
		t.Fatalf("unexpected effective type volumes: %+v", payload.Data.TypeVolumes)
	}
}

func TestDiscordRoutesExposeDisabledConfiguration(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup(t)
	path := "/api/v1/tables/" + testDefaultExternalID + "/discord"
	rec := routedRequest(ts.Handler(), "GET", path, "", true)
	if rec.Code != 200 || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("disabled status: %d %s", rec.Code, rec.Body)
	}
	if rec := routedRequest(ts.Handler(), "POST", path, "", true); rec.Code != 404 {
		t.Fatalf("disabled connect: %d", rec.Code)
	}
	for _, path := range []string{"/api/v1/discord/setup", "/api/v1/discord/servers/1/channels"} {
		if rec := routedRequest(ts.Handler(), "GET", path, "", true); rec.Code != 404 {
			t.Fatalf("disabled setup: %d", rec.Code)
		}
		ts.discord = &testDiscord{}
		if rec := routedRequest(ts.Handler(), "GET", path, "", false); rec.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated setup: %d", rec.Code)
		}
		if rec := routedRequest(ts.Handler(), "GET", path, "", true); rec.Code != http.StatusOK {
			t.Fatalf("setup: %d %s", rec.Code, rec.Body)
		}
		ts.discord = nil
	}
}
