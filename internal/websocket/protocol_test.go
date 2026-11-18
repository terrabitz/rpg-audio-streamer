package websocket

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

func TestTypedCommandRegistration(t *testing.T) {
	hub, clients := testHub()
	var received []PlaybackPatch
	OnEvent(hub, events.TrackPlaybackChanged, func(patch PlaybackPatch, client *Client) {
		if client != clients["gm"] {
			t.Error("handler did not receive the authenticated client")
		}
		received = append(received, patch)
	})
	for _, input := range []string{
		`{"event":"trackPlaybackChanged","payload":{"fileID":"rain","volume":0,"isPlaying":false}}`,
		`{"event":"trackPlaybackChanged","payload":{"fileID":"rain","playback":{"startTime":0}}}`,
		`{"event":"trackPlaybackChanged","payload":[]}`,
		`{"event":"trackPlaybackChanged","payload":{"fileID":3}}`,
		`{"event":"trackPlaybackChanged","payload":{"fileID":"rain","volume":"0"}}`,
		`{"event":"trackPlaybackChanged","payload":{"fileID":"rain","playback":{"startTime":"0"}}}`,
		`{"event":"trackPlaybackChanged"}`,
	} {
		hub.route([]byte(input), clients["gm"])
	}
	if len(received) != 2 {
		t.Fatalf("typed handler received undecodable events: %+v", received)
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
	mix, err := livemixer.New(48000, func(context.Context, string, string) (livemixer.Clip, error) {
		t.Error("undecodable payload reached the audio source")
		return livemixer.Clip{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(livemixer.Subscribe(hub, mix))
	for _, input := range []string{
		`{"event":"mixChanged","payload":{"tracks":"rain"}}`,
		`{"event":"mixVolumesChanged","payload":{"typeVolumes":{"Music":"0"}}}`,
		`{"event":"mixVolumesChanged","payload":{"typeVolumes":[]}}`,
	} {
		hub.route([]byte(input), clients["gm"])
		var message Message
		if err := json.Unmarshal(<-clients["gm"].send, &message); err != nil || message.Event != events.MixerFailed {
			t.Fatalf("invalid event response = %+v, %v", message, err)
		}
	}
}

func TestPayloadSending(t *testing.T) {
	hub, clients := testHub()
	if err := hub.publishEvent(events.TrackRemoved, TrackReference{FileID: "rain"}, events.Metadata{TargetClientID: clients["player"].ID}); err != nil {
		t.Fatal(err)
	}
	var message Message
	if err := json.Unmarshal(<-clients["player"].send, &message); err != nil {
		t.Fatal(err)
	}
	if message.Event != events.TrackRemoved || string(message.Payload) != `{"fileID":"rain"}` {
		t.Fatalf("broadcast changed envelope: %+v", message)
	}
	if len(clients["gm"].send) != 0 {
		t.Fatal("broadcast ignored recipient filters")
	}
	if err := clients["gm"].sendEvent(events.Pong, json.RawMessage(`invalid`)); err == nil {
		t.Fatal("invalid diagnostic JSON was sent")
	}
	if err := hub.publishEvent(events.PlaybackStateChanged, Snapshot{Tracks: []events.PlaybackTrack{{Volume: math.NaN()}}}, events.Metadata{}); err == nil {
		t.Fatal("non-finite playback data was encoded")
	}
}

func TestTrackProcessedNotifiesGMs(t *testing.T) {
	hub, clients := testHub()
	clients["otherGM"].Token.TableID = "friday"
	id := uuid.New()
	sendEvent(t, hub, events.TrackStatusChanged, server.Track{ID: id, Status: "failed", ProcessingError: "Conversion failed"})
	var message Message
	if err := json.Unmarshal(<-clients["gm"].send, &message); err != nil {
		t.Fatal(err)
	}
	want := `{"fileID":"` + id.String() + `","status":"failed","processingError":"Conversion failed"}`
	if message.Event != events.TrackStatusChanged || string(message.Payload) != want {
		t.Fatalf("processing status message = %s %s", message.Event, message.Payload)
	}
	if len(clients["otherGM"].send) != 1 || len(clients["player"].send) != 0 {
		t.Fatal("processing status did not reach exactly the GM clients")
	}
}
