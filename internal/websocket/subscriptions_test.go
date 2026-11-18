package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

func TestSubscriptionsAreIndependentAndIsolateFailuresAndPayloads(t *testing.T) {
	hub, _ := testHub()
	events.OnTrackRemoved(hub, func(payload events.TrackReference, _ events.Metadata) error {
		payload.FileID = "mutated"
		return errors.New("subscriber failed")
	})
	var received []events.Metadata
	handler := func(payload events.TrackReference, metadata events.Metadata) error {
		if payload.FileID != "rain" {
			t.Errorf("subscriber received a mutated payload: %+v", payload)
		}
		received = append(received, metadata)
		return nil
	}
	first := events.OnTrackRemoved(hub, handler)
	second := events.OnTrackRemoved(hub, handler)
	defer second()
	events.OnTableDeleted(hub, func(events.Empty, events.Metadata) error {
		t.Error("subscriber received a different event")
		return nil
	})
	send := func() {
		sendEvent(t, hub, events.TrackRemoved, events.TableEvent[events.TrackReference]{
			TableID: "friday", Data: events.TrackReference{FileID: "rain"},
		})
	}
	send()
	if len(received) != 2 || received[0].TableID != "friday" {
		t.Fatalf("independent subscriptions lost event scope: %+v", received)
	}
	first()
	first()
	send()
	if len(received) != 3 {
		t.Fatalf("unsubscribe removed another subscription: %+v", received)
	}
	second()
	send()
	if len(received) != 3 {
		t.Fatal("unsubscribed handler received an event")
	}
}

func TestMixerSubscriptionCanBeRemovedWithoutChangingPlayback(t *testing.T) {
	hub, clients := testHub()
	client := clients["gm"]
	client.Token.TableID = server.DefaultTableID.String()
	mix := &testMixer{}
	stop := livemixer.Subscribe(hub, mix)
	stop()
	stop()
	routeSyncTrack(hub, json.RawMessage(`{"fileID":"`+testTrackID.String()+`","isPlaying":true}`), client)
	snapshot := readPlayback(t, client)
	if len(snapshot.Tracks) != 1 || !snapshot.Tracks[0].IsPlaying {
		t.Fatalf("unsubscribing the mixer stopped authoritative playback: %+v", snapshot)
	}
	if len(mix.calls) != 0 || len(client.send) != 0 {
		t.Fatal("unsubscribed mixer applied or acknowledged an event")
	}
}

func TestPlaybackSubscriptionsReceiveOnlyValidatedChanges(t *testing.T) {
	hub, clients := testHub()
	for _, client := range clients {
		client.Token.TableID = server.DefaultTableID.String()
	}
	var changes []PlaybackPatch
	events.OnPlaybackChanged(hub, func(payload events.PlaybackChanges[events.PlaybackPatch], metadata events.Metadata) error {
		if metadata != clients["gm"].metadata() {
			t.Errorf("accepted playback has incorrect scope: %+v", metadata)
		}
		changes = append(changes, payload.Tracks...)
		return nil
	})
	play := json.RawMessage(`{"fileID":"` + testTrackID.String() + `","isPlaying":true}`)
	routeSyncTrack(hub, play, clients["player"])
	routeSyncTrack(hub, json.RawMessage(`{"fileID":"invalid","isPlaying":true}`), clients["gm"])
	routeSyncTrack(hub, json.RawMessage(`{"fileID":"`+testTrackID.String()+`","volume":101}`), clients["gm"])
	if len(changes) != 0 {
		t.Fatal("unauthorized or invalid playback reached a subscriber")
	}
	routeSyncTrack(hub, play, clients["gm"])
	if len(changes) != 1 || changes[0].Volume == nil || *changes[0].Volume != 50 || changes[0].Playback == nil {
		t.Fatalf("playback subscriber did not receive resolved settings: %+v", changes)
	}
}

func TestSubscriptionsUseADispatchSnapshot(t *testing.T) {
	hub, _ := testHub()
	var received []string
	var stopSecond func()
	var stopThird func()
	events.OnMixChanged(hub, func(events.MixSnapshot, events.Metadata) error {
		received = append(received, "first")
		stopSecond()
		if stopThird == nil {
			stopThird = events.OnMixChanged(hub, func(events.MixSnapshot, events.Metadata) error {
				received = append(received, "third")
				return nil
			})
		}
		return nil
	})
	stopSecond = events.OnMixChanged(hub, func(events.MixSnapshot, events.Metadata) error {
		received = append(received, "second")
		return nil
	})
	hub.notify(events.MixChanged, Empty{}, events.Metadata{})
	hub.notify(events.MixChanged, Empty{}, events.Metadata{})
	if strings.Join(received, ",") != "first,second,first,third" {
		t.Fatalf("registration changes altered an in-flight dispatch: %v", received)
	}
	stopThird()
}

func TestSocketSubscribersProjectOneCanonicalPlaybackSnapshot(t *testing.T) {
	hub, clients := testHub()
	for _, client := range clients {
		client.Token.TableID = server.DefaultTableID.String()
	}
	snapshots := 0
	events.OnPlaybackStateChanged(hub, func(payload events.Snapshot, _ events.Metadata) error {
		snapshots++
		if payload.Settings == nil || payload.Settings.MasterVolume != 40 ||
			len(payload.Tracks) != 1 || payload.Tracks[0].Volume != 50 {
			t.Errorf("socket subscribers mutated the canonical snapshot: %+v", payload)
		}
		return nil
	})
	routeSyncTrack(hub, json.RawMessage(`{"fileID":"`+testTrackID.String()+`","isPlaying":true}`), clients["gm"])
	if snapshots != 1 {
		t.Fatalf("one playback change published %d snapshots", snapshots)
	}
	gm := readPlayback(t, clients["gm"])
	player := readPlayback(t, clients["player"])
	if gm.Settings == nil || gm.Tracks[0].Volume != 50 ||
		player.Settings != nil || player.Tracks[0].Volume != 20 {
		t.Fatalf("incorrect role-specific socket projection: GM=%+v player=%+v", gm, player)
	}
}

func TestSocketSubscriptionsAreRemovedWithTheirConnection(t *testing.T) {
	for _, removal := range []string{"disconnect", "revocation", "table deletion"} {
		t.Run(removal, func(t *testing.T) {
			hub, _ := testHub()
			count := func() int {
				total := 0
				for _, listeners := range hub.subscriptions {
					total += len(listeners)
				}
				return total
			}
			baseline := count()
			client := NewClient(hub, nil, &auth.Token{Role: auth.RolePlayer, TableID: "friday"})
			hub.addClient(client)
			if count() <= baseline {
				t.Fatal("joining did not register socket subscriptions")
			}
			switch removal {
			case "disconnect":
				hub.dropClient(client)
			case "revocation":
				hub.RevokePlayerAccess("friday")
			case "table deletion":
				sendEvent(t, hub, events.TableDeleted, "friday")
			}
			if !client.closed || count() != baseline {
				t.Fatalf("connection left subscriptions behind: closed=%v subscriptions=%d baseline=%d", client.closed, count(), baseline)
			}
		})
	}
}

func TestLibraryBroadcastDoesNotCrossIntoTableSubscriptions(t *testing.T) {
	hub, clients := testHub()
	clients["otherGM"].Token.TableID = "friday"
	clients["player"].Token.TableID = "friday"
	hub.route([]byte(`{"event":"broadcast","payload":{"hello":"library"}}`), clients["gm"])
	for id, client := range clients {
		want := 0
		if id == "otherPlayer" {
			want = 1
		}
		if got := len(client.send); got != want {
			t.Errorf("%s received %d library events; want %d", id, got, want)
		}
	}
}

func TestMixerAnnouncesReadinessBeforeInitialSnapshot(t *testing.T) {
	hub, _ := testHub()
	stop := livemixer.Subscribe(hub, &testMixer{})
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		hub.Run(ctx)
	}()
	defer func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("hub did not stop")
		}
		stop()
	}()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&gorilla.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		hub.Register(conn, &auth.Token{Role: auth.RoleGM, TableID: server.DefaultTableID.String()})
	}))
	defer srv.Close()
	conn, _, err := gorilla.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(Message{Event: events.PlaybackSyncRequested, Payload: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for _, name := range []string{events.MixerReady, events.PlaybackStateChanged} {
		var message Message
		if err := conn.ReadJSON(&message); err != nil || message.Event != name {
			t.Fatalf("connection bootstrap = %+v, %v; want %s", message, err, name)
		}
	}
}
