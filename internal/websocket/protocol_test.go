package websocket

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

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

func TestTagUpdatesReachOnlyGMs(t *testing.T) {
	hub, clients := testHub()
	clients["otherGM"].Token.TableID = "friday"
	id := uuid.New()
	sendEvent(t, hub, events.TagUpdated, domain.Tag{
		ID:    id,
		Name:  "Combat",
		Color: "#E53935",
	})
	var message Message
	if err := json.Unmarshal(<-clients["gm"].send, &message); err != nil {
		t.Fatal(err)
	}
	want := `{"id":"` + id.String() + `","name":"Combat","color":"#E53935"}`
	if message.Event != events.TagUpdated || string(message.Payload) != want {
		t.Fatalf("tag message = %s %s", message.Event, message.Payload)
	}
	if len(clients["otherGM"].send) != 1 || len(clients["player"].send) != 0 {
		t.Fatal("tag update did not reach exactly the GM clients")
	}
}
