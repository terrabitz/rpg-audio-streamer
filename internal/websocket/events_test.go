package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func TestEventQueueDoesNotWaitForSubscribersAndPreservesAcceptedEvents(t *testing.T) {
	hub, _ := testHub()
	started := make(chan struct{})
	release := make(chan struct{})
	delivered := make(chan int, eventQueueCapacity+1)
	OnServerEvent(hub, "queued", func(payload map[string]int, _ *Client) {
		hub.notify("queued", payload, events.Metadata{})
	})
	hub.On("queued", func(raw json.RawMessage, _ events.Metadata) error {
		var payload map[string]int
		if err := json.Unmarshal(raw, &payload); err != nil {
			return err
		}
		if payload["sequence"] == 0 {
			close(started)
			<-release
		}
		delivered <- payload["sequence"]
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		hub.Run(ctx)
	}()
	waitStopped := func() {
		t.Helper()
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("hub did not stop after cancellation")
		}
	}
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
		cancel()
		waitStopped()
	}()
	if err := hub.SendEvent("queued", map[string]int{"sequence": 0}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("hub did not start the first event")
	}
	enqueued := make(chan error, 1)
	go func() {
		for i := 1; i <= eventQueueCapacity; i++ {
			payload := map[string]int{"sequence": i}
			if err := hub.SendEvent("queued", payload); err != nil {
				enqueued <- err
				return
			}
			payload["sequence"] = -1
		}
		enqueued <- hub.SendEvent("queued", map[string]int{"sequence": -2})
	}()
	select {
	case err := <-enqueued:
		if !errors.Is(err, ErrEventQueueFull) {
			t.Fatalf("overflow = %v; want ErrEventQueueFull", err)
		}
	case <-time.After(time.Second):
		t.Fatal("producer waited for the blocked subscriber")
	}
	close(release)
	for i := 0; i <= eventQueueCapacity; i++ {
		select {
		case sequence := <-delivered:
			if sequence != i {
				t.Fatalf("event %d was lost, reordered, or mutated: got %d", i, sequence)
			}
		case <-time.After(time.Second):
			t.Fatalf("event %d was not delivered", i)
		}
	}
	cancel()
	waitStopped()
	if err := hub.SendEvent("queued", map[string]int{}); !errors.Is(err, ErrHubClosed) {
		t.Fatalf("stopped hub accepted an event: %v", err)
	}
}

func TestServerEventsCannotBePublishedBySocketClients(t *testing.T) {
	hub, clients := testHub()
	called := false
	OnServerEvent(hub, events.TableDeleted, func(string, *Client) { called = true })
	for _, id := range []string{"gm", "player"} {
		routeAndDrain(t, hub, []byte(`{"event":"tableDeleted","payload":"friday"}`), clients[id])
	}
	if called {
		t.Fatal("untrusted socket event reached a server-only handler")
	}
	for _, client := range clients {
		if client.closed || len(client.send) != 0 {
			t.Fatal("client forged a table deletion")
		}
	}
	if err := hub.SendEvent(events.Ping, map[string]int{}); err == nil {
		t.Fatal("server emitter accepted an unregistered server event")
	}
	if err := hub.SendEvent(events.TrackStatusChanged, math.NaN()); err == nil {
		t.Fatal("server emitter accepted an unencodable payload")
	}
}

func TestEventSenderComesFromAuthenticatedSocket(t *testing.T) {
	hub, clients := testHub()
	routeAndDrain(t, hub, []byte(`{"event":"broadcast","senderId":"forged","payload":{"nonce":0}}`), clients["gm"])
	var message Message
	if err := json.Unmarshal(<-clients["player"].send, &message); err != nil {
		t.Fatal(err)
	}
	if message.Event != events.Broadcast || message.SenderID != clients["gm"].ID {
		t.Fatalf("socket event trusted sender metadata: %+v", message)
	}
}
