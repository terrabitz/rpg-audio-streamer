package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/events"
)

func TestSendEventWaitsForSubscribersAndAllowsNestedEvents(t *testing.T) {
	hub, _ := testHub()
	var handled []string
	OnServerEvent(hub, "outer", func(payload Empty, _ *Client) {
		hub.notify("outer", payload, events.Metadata{})
	})
	OnServerEvent(hub, "inner", func(Empty, *Client) {
		handled = append(handled, "inner")
	})
	hub.On("outer", func(json.RawMessage, events.Metadata) error {
		handled = append(handled, "before")
		if err := hub.SendEvent("inner", Empty{}); err != nil {
			return err
		}
		handled = append(handled, "after")
		return nil
	})
	if err := hub.SendEvent("outer", Empty{}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(handled, []string{"before", "inner", "after"}) {
		t.Fatalf("SendEvent returned before nested handlers finished: %v", handled)
	}
}

func TestBlockedEventOnlyBlocksItsCallerAndShutdownWaits(t *testing.T) {
	hub, _ := testHub()
	started := make(chan struct{})
	release := make(chan struct{})
	var released sync.Once
	unblock := func() { released.Do(func() { close(release) }) }
	OnServerEvent(hub, "blocked", func(payload Empty, _ *Client) {
		hub.notify("blocked", payload, events.Metadata{})
	})
	hub.On("blocked", func(json.RawMessage, events.Metadata) error {
		close(started)
		<-release
		return nil
	})
	OnServerEvent(hub, "independent", func(Empty, *Client) {})
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
		unblock()
		cancel()
		waitStopped()
	}()
	sent := make(chan error, 1)
	go func() { sent <- hub.SendEvent("blocked", Empty{}) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("hub did not start the first event")
	}
	select {
	case err := <-sent:
		t.Fatalf("SendEvent returned before its subscriber finished: %v", err)
	default:
	}
	independent := make(chan error, 1)
	go func() { independent <- hub.SendEvent("independent", Empty{}) }()
	select {
	case err := <-independent:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("an unrelated producer waited for the blocked subscriber")
	}
	cancel()
	select {
	case <-hub.done:
	case <-time.After(time.Second):
		t.Fatal("hub did not stop accepting events")
	}
	select {
	case <-finished:
		t.Fatal("hub stopped before an in-flight handler finished")
	default:
	}
	if err := hub.SendEvent("blocked", Empty{}); !errors.Is(err, ErrHubClosed) {
		t.Fatalf("stopping hub accepted an event: %v", err)
	}
	unblock()
	select {
	case err := <-sent:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("released event did not finish")
	}
	waitStopped()
}

func TestServerEventsCannotBePublishedBySocketClients(t *testing.T) {
	hub, clients := testHub()
	called := false
	OnServerEvent(hub, events.TableDeleted, func(string, *Client) { called = true })
	for _, id := range []string{"gm", "player"} {
		hub.route([]byte(`{"event":"tableDeleted","payload":"friday"}`), clients[id])
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
	hub.route([]byte(`{"event":"broadcast","senderId":"forged","payload":{"nonce":0}}`), clients["gm"])
	var message Message
	if err := json.Unmarshal(<-clients["player"].send, &message); err != nil {
		t.Fatal(err)
	}
	if message.Event != events.Broadcast || message.SenderID != clients["gm"].ID {
		t.Fatalf("socket event trusted sender metadata: %+v", message)
	}
}
