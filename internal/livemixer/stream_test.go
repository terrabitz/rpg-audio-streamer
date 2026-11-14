package livemixer

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMonitorReceivesImmutableTablePCMAndSurvivesSnapshots(t *testing.T) {
	s := testService(t)
	update(t, s, "first", `{"fileID":"tone","isPlaying":true}`)
	update(t, s, "second", `{"fileID":"tone","isPlaying":true,"volume":0}`)
	sub, cancel, err := s.Subscribe("first")
	must(t, err)
	defer cancel()
	other, stop, err := s.Subscribe("second")
	must(t, err)
	defer stop()
	frames := s.Render("first", 1)
	frames[0].Left = 1
	assertSample(t, (<-sub.Frames())[0].Left, 0.5)
	select {
	case <-other.Frames():
		t.Fatal("PCM leaked to another table")
	default:
	}
	must(t, s.Sync("first", json.RawMessage(`{"tracks":[]}`)))
	s.Render("first", 1)
	assertSample(t, (<-sub.Frames())[0].Left, 0)
	must(t, s.Sync("first", json.RawMessage(`{"tracks":[{"fileID":"tone","isPlaying":true}]}`)))
	s.Render("first", 1)
	assertSample(t, (<-sub.Frames())[0].Left, 0.5)
	s.CloseTable("first")
	if _, open := <-sub.Frames(); open || !errors.Is(sub.Err(), ErrSessionClosed) {
		t.Fatal("closed mix left a monitor attached")
	}
}

func TestSlowMonitorIsDisconnectedWithoutBlockingRendering(t *testing.T) {
	s := testService(t)
	update(t, s, "table", `{"fileID":"tone","isPlaying":true}`)
	sub, cancel, err := s.Subscribe("table")
	must(t, err)
	defer cancel()
	for i := 0; i < 10; i++ {
		s.Render("table", 1)
	}
	for range sub.Frames() {
	}
	if !errors.Is(sub.Err(), ErrSlowMonitor) {
		t.Fatalf("slow monitor ended with %v", sub.Err())
	}
	if !s.Status("table").Tracks[0].IsPlaying {
		t.Fatal("slow monitor stopped the mixer")
	}
	cancel()
}

func TestMonitorRequiresSessionAndCancellationReleasesIt(t *testing.T) {
	s := testService(t)
	if _, _, err := s.Subscribe("missing"); err == nil {
		t.Fatal("subscribed to a missing mix")
	}
	update(t, s, "table", `{"fileID":"tone","isPlaying":true}`)
	sub, cancel, err := s.Subscribe("table")
	must(t, err)
	cancel()
	cancel()
	if _, open := <-sub.Frames(); open || sub.Err() != nil {
		t.Fatal("cancelled monitor remained open")
	}
	s.Render("table", 1)
}
