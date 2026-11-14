package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSetupDiscoversInstallationAndOnlyRegularVoiceChannels(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bot test-only" {
			t.Error("missing bot authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth2/applications/@me":
			_, _ = w.Write([]byte(`{"id":"99"}`))
		case "/users/@me/guilds":
			if r.URL.Query().Has("after") && r.URL.Query().Get("after") == "" {
				http.Error(w, "empty pagination cursor", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`[{"id":"1","name":"Friday"}]`))
		case "/guilds/1":
			_, _ = w.Write([]byte(`{"id":"1","name":"Friday"}`))
		case "/guilds/1/channels":
			_, _ = w.Write([]byte(`[{"id":"2","name":"Music","type":2},{"id":"3","name":"chat","type":0},{"id":"4","name":"Stage","type":13}]`))
		default:
			http.Error(w, "secret-provider-body", http.StatusForbidden)
		}
	}))
	defer api.Close()
	service, _ := New(Config{Token: "test-only"}, testMix(t))
	defer service.Close()
	service.apiURL = api.URL
	setup, err := service.Setup(context.Background())
	if err != nil || len(setup.Guilds) != 1 || !strings.Contains(setup.InstallURL, "client_id=99") ||
		!strings.Contains(setup.InstallURL, "scope=bot") || !strings.Contains(setup.InstallURL, "permissions=3146752") {
		t.Fatalf("setup = %+v, %v", setup, err)
	}
	channels, err := service.Channels(context.Background(), "1")
	if err != nil || len(channels) != 1 || channels[0].ID != "2" {
		t.Fatalf("channels = %+v, %v", channels, err)
	}
	target, err := service.Resolve(context.Background(), Destination{GuildID: "1", ChannelID: "2", GuildName: "untrusted"})
	if err != nil || target.GuildName != "Friday" || target.ChannelName != "Music" {
		t.Fatalf("destination = %+v, %v", target, err)
	}
	for _, target := range []Destination{{GuildID: "../bad", ChannelID: "2"}, {GuildID: "1", ChannelID: "3"}, {GuildID: "5", ChannelID: "2"}} {
		if _, err := service.Resolve(context.Background(), target); err == nil || strings.Contains(err.Error(), "secret-provider-body") {
			t.Fatalf("invalid destination accepted or provider body leaked: %v", err)
		}
	}
}

func TestDiscordDiscoveryHonorsRateLimits(t *testing.T) {
	var attempts atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0.01")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"retry_after":0.01,"global":false}`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":"2","name":"Music","type":2}]`))
	}))
	defer api.Close()
	service, _ := New(Config{Token: "test-only"}, testMix(t))
	defer service.Close()
	service.apiURL = api.URL
	channels, err := service.Channels(context.Background(), "1")
	if err != nil || len(channels) != 1 || attempts.Load() != 2 {
		t.Fatalf("rate-limited discovery = %+v, %v, attempts %d", channels, err, attempts.Load())
	}
}
