package websocket

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/playback"
)

func TestMixerStopsOnlyAfterLastGMLeaves(t *testing.T) {
	hub, clients := testHub()
	mix := &testMixer{}
	t.Cleanup(livemixer.Subscribe(hub, mix))
	clients["gm"].Token.TableID = "friday"
	clients["otherGM"].Token.TableID = "friday"
	hub.dropClient(clients["gm"])
	if len(mix.calls) != 0 {
		t.Fatal("one GM leaving stopped another GM's mix")
	}
	events.OnTableSessionEnded(hub, func(_ events.Empty, metadata events.Metadata) error {
		hub.ForEachClient(func(client *Client) {
			if client.Token.TableID == metadata.TableID && client.Token.Role == auth.RoleGM {
				t.Error("table session ended with another GM connected")
			}
		})
		return nil
	})
	hub.dropClient(clients["otherGM"])
	if len(mix.calls) != 1 || mix.calls[0] != "close:friday" {
		t.Fatalf("last GM cleanup: %v", mix.calls)
	}
}

type testMixer struct {
	calls []string
	err   error
}

func (m *testMixer) Update(tableID string, _ json.RawMessage) error {
	m.calls = append(m.calls, "update:"+tableID)
	return m.err
}
func (m *testMixer) Sync(tableID string, _ json.RawMessage) error {
	m.calls = append(m.calls, "sync:"+tableID)
	return m.err
}
func (m *testMixer) Volumes(tableID string, _ json.RawMessage) error {
	m.calls = append(m.calls, "volumes:"+tableID)
	return m.err
}
func (m *testMixer) RemoveTrack(tableID, fileID string) {
	m.calls = append(m.calls, "remove:"+tableID+":"+fileID)
}
func (m *testMixer) CloseTable(tableID string) {
	m.calls = append(m.calls, "close:"+tableID)
}

func TestClientSubscriptionsAndSlowConsumers(t *testing.T) {
	hub, clients := testHub()
	count := 0
	hub.ForEachClient(func(c *Client) { count++ })
	if count != 4 {
		t.Fatalf("unfiltered clients = %d", count)
	}
	slow := clients["player"]
	slow.send = make(chan []byte)
	if err := hub.publish(Message{Event: events.TrackRemoved, Payload: json.RawMessage(`{}`)}, events.Metadata{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := hub.clients[slow]; ok {
		t.Fatal("slow client was not removed")
	}
	if _, ok := <-slow.send; ok {
		t.Fatal("slow client's send channel should be closed")
	}
	for id, client := range clients {
		if id == "player" {
			continue
		}
		if len(client.send) != 1 {
			t.Errorf("%s did not receive unfiltered broadcast", id)
		}
	}
	if err := hub.publish(Message{Payload: json.RawMessage(`invalid`)}, events.Metadata{}); err == nil {
		t.Fatal("invalid JSON should fail to marshal")
	}
	if err := clients["gm"].Send(Message{Payload: json.RawMessage(`invalid`)}); err == nil {
		t.Fatal("invalid direct message should fail to marshal")
	}
}

func TestWebSocketClientPumps(t *testing.T) {
	hub, _ := testHub()
	hub.unregister = make(chan *Client, 1)
	finished := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&gorilla.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		client := NewClient(hub, conn, &auth.Token{Role: auth.RolePlayer})
		hub.addClient(client)
		writerDone := make(chan struct{})
		go func() {
			client.writePump(10 * time.Millisecond)
			close(writerDone)
		}()
		client.ReadPump()
		hub.dropClient(client)
		<-writerDone
		close(finished)
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	conn, _, err := gorilla.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	heartbeat := make(chan struct{}, 1)
	replyPong := conn.PingHandler()
	conn.SetPingHandler(func(data string) error {
		select {
		case heartbeat <- struct{}{}:
		default:
		}
		return replyPong(data)
	})
	if err := conn.WriteJSON(Message{Event: events.TableDeleted, Payload: json.RawMessage(`"friday"`)}); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	go func() {
		select {
		case <-heartbeat:
			if err := hub.SendEvent(events.TrackRemoved, events.TableEvent[events.TrackReference]{Data: events.TrackReference{FileID: "rain"}}); err != nil {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("idle socket did not send a protocol ping")
		}
	}()
	var response Message
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatal(err)
	}
	if response.Event != events.TrackRemoved {
		t.Fatalf("response = %+v", response)
	}
	conn.Close()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("client pumps did not shut down")
	}
	select {
	case client := <-hub.unregister:
		if client.ID == "" || client.Token.Role != auth.RolePlayer {
			t.Fatalf("unregistered client = %+v", client)
		}
	default:
		t.Fatal("disconnected client was not unregistered")
	}
}

func TestTableDeletionAndTrackRemoval(t *testing.T) {
	hub, clients := testHub()
	clients["gm"].Token.TableID = "friday"
	clients["player"].Token.TableID = "friday"
	clients["otherGM"].Token.TableID = "sunday"
	clients["otherPlayer"].Token.TableID = "sunday"
	sendEvent(t, hub, events.TrackRemoved, events.TableEvent[events.TrackReference]{TableID: "friday", Data: events.TrackReference{FileID: "forest"}})
	for _, id := range []string{"gm", "player"} {
		var message Message
		if err := json.Unmarshal(<-clients[id].send, &message); err != nil {
			t.Fatal(err)
		}
		if message.Event != "trackRemoved" || string(message.Payload) != `{"fileID":"forest"}` {
			t.Fatalf("track removal = %+v", message)
		}
	}
	if len(clients["otherPlayer"].send) != 0 {
		t.Fatal("track removal crossed tables")
	}
	sendEvent(t, hub, events.TableDeleted, "friday")
	for _, id := range []string{"gm", "player"} {
		client := clients[id]
		if _, ok := hub.clients[client]; ok {
			t.Fatal("deleted table client still registered")
		}
		var message Message
		if err := json.Unmarshal(<-client.send, &message); err != nil || message.Event != "tableDeleted" {
			t.Fatalf("table deletion message = %+v, %v", message, err)
		}
		if _, open := <-client.send; open {
			t.Fatal("deleted table send channel is open")
		}
		if err := client.Send(Message{Event: "ping"}); err == nil {
			t.Fatal("disconnected client accepted direct message")
		}
	}
	sendEvent(t, hub, events.TrackRemoved, events.TableEvent[events.TrackReference]{TableID: "", Data: events.TrackReference{FileID: "forest"}})
	if len(clients["otherPlayer"].send) != 1 {
		t.Fatal("library deletion did not reach other table")
	}
}

func TestRevokePlayerAccessDisconnectsOnlyPlayersInTable(t *testing.T) {
	hub, clients := testHub()
	for id, client := range clients {
		client.Token.TableID = "friday"
		if id == "otherGM" || id == "otherPlayer" {
			client.Token.TableID = "sunday"
		}
	}
	hub.RevokePlayerAccess("friday")
	for id, client := range clients {
		_, registered := hub.clients[client]
		disconnected := client.closed
		switch id {
		case "player":
			if registered || !disconnected {
				t.Fatal("table player remained connected after access revocation")
			}
			if _, open := <-client.send; open {
				t.Fatal("revoked player send channel remains open")
			}
		case "gm", "otherGM", "otherPlayer":
			if !registered || disconnected {
				t.Fatalf("%s was disconnected by another table's link rotation", id)
			}
		}
	}
}

func testHub() (*Hub, map[string]*Client) { return testHubWithStore(playbackTestStore{}) }

func testHubWithStore(store playback.Store) (*Hub, map[string]*Client) {
	hub := NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)), store)
	clients := make(map[string]*Client)
	for id, role := range map[string]auth.Role{
		"gm": auth.RoleGM, "otherGM": auth.RoleGM,
		"player": auth.RolePlayer, "otherPlayer": auth.RolePlayer,
	} {
		client := NewClient(hub, nil, &auth.Token{Role: role})
		client.ID = id
		clients[id] = client
		hub.addClient(client)
	}
	return hub, clients
}

func sendEvent(t *testing.T, hub *Hub, event string, payload any) {
	t.Helper()
	if err := hub.SendEvent(event, payload); err != nil {
		t.Fatal(err)
	}
}

func TestMixerResultsReachOnlyGMsAtTheCommandTable(t *testing.T) {
	hub, clients := testHub()
	for _, name := range []string{"gm", "otherGM", "player"} {
		clients[name].Token.TableID = "friday"
	}
	externalGM := NewClient(hub, nil, &auth.Token{Role: auth.RoleGM, TableID: "sunday"})
	hub.addClient(externalGM)
	mix := &testMixer{}
	t.Cleanup(livemixer.Subscribe(hub, mix))
	sendEvent(t, hub, events.MixChanged, events.TableEvent[events.MixSnapshot]{
		TableID: "friday",
		Data:    events.MixSnapshot{Tracks: []events.PlaybackPatch{}},
	})
	mix.err = errors.New("unable to open audio")
	sendEvent(t, hub, events.MixVolumesChanged, events.TableEvent[events.MixVolumePatch]{
		TableID: "friday",
		Data:    events.MixVolumePatch{TypeVolumes: map[string]float64{"Music": 0}},
	})
	for _, name := range []string{"gm", "otherGM"} {
		if len(clients[name].send) != 2 {
			t.Fatalf("%s received %d results; want 2", name, len(clients[name].send))
		}
		for _, event := range []string{events.MixerApplied, events.MixerFailed} {
			var message Message
			if err := json.Unmarshal(<-clients[name].send, &message); err != nil || message.Event != event {
				t.Fatalf("%s result = %+v, %v; want %s", name, message, err, event)
			}
		}
		if len(externalGM.send) != 0 || len(clients["player"].send) != 0 || len(clients["otherPlayer"].send) != 0 {
			t.Fatal("mixer results escaped the GM table scope")
		}
	}
}
