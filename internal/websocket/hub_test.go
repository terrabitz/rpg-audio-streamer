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
)

func TestMixerReceivesOnlyTableScopedGMCommands(t *testing.T) {
	hub, clients := testHub()
	mix := &testMixer{}
	hub.SetMixer(mix)
	clients["gm"].Token.TableID = "friday"
	clients["player"].Token.TableID = "friday"
	send := func(sender, method, payload string) {
		t.Helper()
		input, err := json.Marshal(Message{Method: method, Payload: json.RawMessage(payload)})
		if err != nil {
			t.Fatal(err)
		}
		hub.route(input, clients[sender])
	}
	send("player", "mixSync", `{"tracks":[]}`)
	send("player", "mixVolumes", `{"typeVolumes":{}}`)
	send("player", "syncTrack", `{"fileID":"rain"}`)
	send("gm", "syncAll", `{"tracks":[],"to":"player"}`)
	if len(mix.calls) != 0 {
		t.Fatal("player or targeted listener sync changed mixer")
	}
	send("gm", "syncTrack", `{"fileID":"rain","volume":0}`)
	send("gm", "mixVolumes", `{"typeVolumes":{"Music":0}}`)
	send("gm", "mixSync", `{"tracks":[]}`)
	send("gm", "syncAll", `{"tracks":[]}`)
	want := []string{"update:friday", "volumes:friday", "sync:friday", "sync:friday"}
	if len(mix.calls) != len(want) {
		t.Fatalf("calls: %v", mix.calls)
	}
	for i := range want {
		if mix.calls[i] != want[i] {
			t.Fatalf("calls: %v", mix.calls)
		}
	}
	mix.err = errors.New("invalid seek")
	send("gm", "syncTrack", `{"fileID":"rain","currentTime":-1}`)
	var last Message
	for len(clients["gm"].send) > 0 {
		if err := json.Unmarshal(<-clients["gm"].send, &last); err != nil {
			t.Fatal(err)
		}
	}
	if last.Method != "mixerError" {
		t.Fatalf("failed command was not reported: %+v", last)
	}
	hub.RemoveTrack("friday", "rain")
	hub.CloseTable("friday")
	if got := mix.calls[len(mix.calls)-2:]; got[0] != "remove:friday:rain" || got[1] != "close:friday" {
		t.Fatalf("cleanup: %v", got)
	}
}

func TestMixerStopsOnlyAfterLastGMLeaves(t *testing.T) {
	hub, clients := testHub()
	mix := &testMixer{}
	hub.SetMixer(mix)
	clients["gm"].Token.TableID = "friday"
	clients["otherGM"].Token.TableID = "friday"
	hub.clientsMu.Lock()
	hub.dropClient(clients["gm"])
	hub.clientsMu.Unlock()
	if len(mix.calls) != 0 {
		t.Fatal("one GM leaving stopped another GM's mix")
	}
	hub.clientsMu.Lock()
	hub.dropClient(clients["otherGM"])
	hub.clientsMu.Unlock()
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

func TestMessageRouting(t *testing.T) {
	for _, tc := range []struct {
		name, sender, method, payload string
		recipients                    []string
	}{
		{"ping replies to sender", "player", "ping", `{"nonce":42}`, []string{"player"}},
		{"GM broadcast excludes sender", "gm", "broadcast", `{"hello":"table"}`, []string{"otherGM", "player", "otherPlayer"}},
		{"player cannot broadcast", "player", "broadcast", `{}`, nil},
		{"sync request reaches all GMs", "player", "syncRequest", `{}`, []string{"gm", "otherGM"}},
		{"full sync reaches players", "gm", "syncAll", `{"tracks":[]}`, []string{"player", "otherPlayer"}},
		{"targeted full sync", "gm", "syncAll", `{"tracks":[],"to":"player"}`, []string{"player"}},
		{"missing target", "gm", "syncAll", `{"tracks":[],"to":"missing"}`, nil},
		{"player cannot sync all", "player", "syncAll", `{"tracks":[]}`, nil},
		{"invalid sync payload", "gm", "syncAll", `"not an object"`, nil},
		{"track sync reaches players", "gm", "syncTrack", `{"fileID":"rain","isPlaying":true}`, []string{"player", "otherPlayer"}},
		{"player cannot sync track", "player", "syncTrack", `{}`, nil},
		{"unknown method", "gm", "unknown", `{}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hub, clients := testHub()
			input, err := json.Marshal(Message{Method: tc.method, Payload: json.RawMessage(tc.payload)})
			if err != nil {
				t.Fatal(err)
			}
			hub.route(input, clients[tc.sender])
			for id, client := range clients {
				want := false
				for _, recipient := range tc.recipients {
					want = want || id == recipient
				}
				select {
				case raw := <-client.send:
					if !want {
						t.Fatalf("unexpected message to %s: %s", id, raw)
					}
					var got Message
					if err := json.Unmarshal(raw, &got); err != nil {
						t.Fatal(err)
					}
					method := tc.method
					if method == "ping" {
						method = "pong"
					}
					if got.Method != method || string(got.Payload) != tc.payload {
						t.Errorf("message = %s", raw)
					}
					if method != "pong" && got.SenderID != tc.sender {
						t.Errorf("sender = %q; want %q", got.SenderID, tc.sender)
					}
				default:
					if want {
						t.Errorf("missing message to %s", id)
					}
				}
			}
		})
	}
}

func TestBroadcastFiltersAndSlowClients(t *testing.T) {
	hub, clients := testHub()
	visited := map[string]bool{}
	hub.ForEachClient(func(c *Client) { visited[c.ID] = true }, ToPlayersOnly(), ExceptClient(clients["player"]))
	if len(visited) != 1 || !visited["otherPlayer"] {
		t.Fatalf("AND filters visited = %v", visited)
	}
	count := 0
	hub.ForEachClient(func(c *Client) { count++ })
	if count != 4 {
		t.Fatalf("unfiltered clients = %d", count)
	}
	slow := clients["player"]
	slow.send = make(chan []byte)
	if err := hub.Broadcast(Message{Method: "test", Payload: json.RawMessage(`{}`)}); err != nil {
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
	if err := hub.Broadcast(Message{Payload: json.RawMessage(`invalid`)}); err == nil {
		t.Fatal("invalid JSON should fail to marshal")
	}
	if err := clients["gm"].Send(Message{Payload: json.RawMessage(`invalid`)}); err == nil {
		t.Fatal("invalid direct message should fail to marshal")
	}
	hub.route([]byte(`invalid`), clients["gm"])
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
		writerDone := make(chan struct{})
		go func() {
			client.WritePump()
			close(writerDone)
		}()
		client.ReadPump()
		close(client.send)
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
	if err := conn.WriteJSON(Message{Method: "ping", Payload: json.RawMessage(`"hello"`)}); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var response Message
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatal(err)
	}
	if response.Method != "pong" || string(response.Payload) != `"hello"` {
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

func TestTableIsolation(t *testing.T) {
	for _, tc := range []struct {
		sender, method, payload string
	}{
		{"gm", "broadcast", `{}`},
		{"gm", "syncAll", `{"tracks":[]}`},
		{"gm", "syncAll", `{"tracks":[],"to":"outsider"}`},
		{"gm", "syncTrack", `{"fileID":"forest","isPlaying":true}`},
		{"player", "syncRequest", `{}`},
	} {
		t.Run(tc.method+tc.payload, func(t *testing.T) {
			hub, clients := testHub()
			for _, client := range clients {
				client.Token.TableID = "friday"
			}
			outsider := NewClient(hub, nil, &auth.Token{Role: auth.RolePlayer, TableID: "sunday"})
			outsider.ID = "outsider"
			otherGM := NewClient(hub, nil, &auth.Token{Role: auth.RoleGM, TableID: "sunday"})
			hub.clients[outsider] = true
			hub.clients[otherGM] = true
			input, err := json.Marshal(Message{Method: tc.method, Payload: json.RawMessage(tc.payload)})
			if err != nil {
				t.Fatal(err)
			}
			hub.route(input, clients[tc.sender])
			if len(outsider.send) != 0 || len(otherGM.send) != 0 {
				t.Fatal("message crossed table boundary")
			}
			if tc.method == "syncRequest" && len(clients["gm"].send) != 1 {
				t.Fatal("same-table GM did not receive sync request")
			}
		})
	}
}

func TestTableDeletionAndTrackRemoval(t *testing.T) {
	hub, clients := testHub()
	clients["gm"].Token.TableID = "friday"
	clients["player"].Token.TableID = "friday"
	clients["otherGM"].Token.TableID = "sunday"
	clients["otherPlayer"].Token.TableID = "sunday"
	hub.RemoveTrack("friday", "forest")
	for _, id := range []string{"gm", "player"} {
		var message Message
		if err := json.Unmarshal(<-clients[id].send, &message); err != nil {
			t.Fatal(err)
		}
		if message.Method != "removeTrack" || string(message.Payload) != `{"fileID":"forest"}` {
			t.Fatalf("track removal = %+v", message)
		}
	}
	if len(clients["otherPlayer"].send) != 0 {
		t.Fatal("track removal crossed tables")
	}
	hub.CloseTable("friday")
	for _, id := range []string{"gm", "player"} {
		client := clients[id]
		if _, ok := hub.clients[client]; ok {
			t.Fatal("deleted table client still registered")
		}
		var message Message
		if err := json.Unmarshal(<-client.send, &message); err != nil || message.Method != "tableDeleted" {
			t.Fatalf("table deletion message = %+v, %v", message, err)
		}
		if _, open := <-client.send; open {
			t.Fatal("deleted table send channel is open")
		}
		if err := client.Send(Message{Method: "ping"}); err == nil {
			t.Fatal("disconnected client accepted direct message")
		}
		hub.route([]byte(`{"method":"syncRequest","payload":{}}`), client)
	}
	hub.RemoveTrack("", "forest")
	if len(clients["otherPlayer"].send) != 1 {
		t.Fatal("library deletion did not reach other table")
	}
}

func testHub() (*Hub, map[string]*Client) {
	hub := NewHub(slog.New(slog.NewTextHandler(io.Discard, nil)))
	clients := make(map[string]*Client)
	for id, role := range map[string]auth.Role{
		"gm": auth.RoleGM, "otherGM": auth.RoleGM,
		"player": auth.RolePlayer, "otherPlayer": auth.RolePlayer,
	} {
		client := NewClient(hub, nil, &auth.Token{Role: role})
		client.ID = id
		clients[id] = client
		hub.clients[client] = true
	}
	return hub, clients
}
