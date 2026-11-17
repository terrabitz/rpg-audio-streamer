package websocket

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

func TestTypedCommandRegistration(t *testing.T) {
	hub, clients := testHub()
	var received []PlaybackPatch
	Handle(hub, SyncTrack, func(patch PlaybackPatch, client *Client) {
		if client != clients["gm"] {
			t.Error("handler did not receive the authenticated client")
		}
		received = append(received, patch)
	})
	for _, input := range []string{
		`{"method":"syncTrack","payload":{"fileID":"rain","volume":0,"isPlaying":false}}`,
		`{"method":"syncTrack","payload":{"fileID":"rain","playback":{"startTime":0}}}`,
		`{"method":"syncTrack","payload":[]}`,
		`{"method":"syncTrack","payload":{"fileID":3}}`,
		`{"method":"syncTrack","payload":{"fileID":"rain","volume":"0"}}`,
		`{"method":"syncTrack","payload":{"fileID":"rain","playback":{"startTime":"0"}}}`,
		`{"method":"syncTrack"}`,
	} {
		hub.route([]byte(input), clients["gm"])
	}
	if len(received) != 2 {
		t.Fatalf("typed handler received undecodable commands: %+v", received)
	}
	patch := received[0]
	if patch.FileID != "rain" || patch.Volume == nil || *patch.Volume != 0 ||
		patch.IsPlaying == nil || *patch.IsPlaying || patch.CurrentTime != nil {
		t.Fatalf("partial patch lost zero, false or omitted fields: %+v", patch)
	}
	if received[1].Playback == nil || received[1].Playback.StartTime != 0 {
		t.Fatalf("nested payload was not decoded: %+v", received[1])
	}
}

func TestTypedMixerRejectsUndecodablePayloads(t *testing.T) {
	hub, clients := testHub()
	mix := &testMixer{}
	hub.SetMixer(mix)
	for _, input := range []string{
		`{"method":"mixSync","payload":{"tracks":"rain"}}`,
		`{"method":"mixVolumes","payload":{"typeVolumes":{"Music":"0"}}}`,
		`{"method":"mixVolumes","payload":{"typeVolumes":[]}}`,
	} {
		hub.route([]byte(input), clients["gm"])
		var message Message
		if err := json.Unmarshal(<-clients["gm"].send, &message); err != nil || message.Method != MixerError {
			t.Fatalf("invalid command response = %+v, %v", message, err)
		}
	}
	if len(mix.calls) != 0 {
		t.Fatalf("undecodable commands reached mixer: %v", mix.calls)
	}
}

func TestPayloadSending(t *testing.T) {
	hub, clients := testHub()
	if err := hub.BroadcastPayload(RemoveTrack, TrackReference{FileID: "rain"}, ToPlayersOnly()); err != nil {
		t.Fatal(err)
	}
	var message Message
	if err := json.Unmarshal(<-clients["player"].send, &message); err != nil {
		t.Fatal(err)
	}
	if message.Method != RemoveTrack || string(message.Payload) != `{"fileID":"rain"}` {
		t.Fatalf("broadcast changed envelope: %+v", message)
	}
	if len(clients["gm"].send) != 0 {
		t.Fatal("broadcast ignored recipient filters")
	}
	if err := clients["gm"].SendPayload(Pong, json.RawMessage(`invalid`)); err == nil {
		t.Fatal("invalid diagnostic JSON was sent")
	}
	if err := hub.BroadcastPayload(SyncAll, Snapshot{Tracks: []playingTrack{{Volume: math.NaN()}}}); err == nil {
		t.Fatal("non-finite playback data was encoded")
	}
}

func TestTrackProcessedNotifiesGMs(t *testing.T) {
	hub, clients := testHub()
	id := uuid.New()
	hub.TrackProcessed(server.Track{ID: id, Status: "failed", ProcessingError: "Conversion failed"})
	var message Message
	if err := json.Unmarshal(<-clients["gm"].send, &message); err != nil {
		t.Fatal(err)
	}
	want := `{"fileID":"` + id.String() + `","status":"failed","processingError":"Conversion failed"}`
	if message.Method != TrackStatus || string(message.Payload) != want {
		t.Fatalf("processing status message = %s %s", message.Method, message.Payload)
	}
	if len(clients["otherGM"].send) != 1 || len(clients["player"].send) != 0 {
		t.Fatal("processing status did not reach exactly the GM clients")
	}
}
