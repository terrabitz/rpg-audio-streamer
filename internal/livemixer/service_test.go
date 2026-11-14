package livemixer

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"runtime"
	"testing"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
)

func TestPartialCommandsChangeRenderedAudio(t *testing.T) {
	s := testService(t)
	update(t, s, "table", `{"fileID":"tone","isPlaying":true}`)
	frames := s.Render("table", 2)
	assertSample(t, frames[1].Left, 0.5)
	update(t, s, "table", `{"fileID":"tone","volume":0}`)
	frames = s.Render("table", 13)
	assertSample(t, frames[0].Left, 0.5)
	assertSample(t, frames[6].Left, 0.25)
	assertSample(t, frames[12].Left, 0)
	update(t, s, "table", `{"fileID":"tone","currentTime":3}`)
	status := s.Status("table")
	if status.Tracks[0].CurrentTime != 3 || !status.Tracks[0].IsPlaying {
		t.Fatalf("seek changed transport: %+v", status)
	}
	update(t, s, "table", `{"fileID":"tone","isPlaying":false}`)
	s.Render("table", 6)
	if s.Status("table").Tracks[0].CurrentTime != 3 {
		t.Fatal("paused cursor advanced")
	}
}

func TestSnapshotsVolumesAndTableIsolation(t *testing.T) {
	s := testService(t)
	must(t, s.Sync("first", json.RawMessage(`{"tracks":[{"fileID":"tone","isPlaying":true,"volume":50}],"typeVolumes":{"Music":50}}`)))
	update(t, s, "second", `{"fileID":"tone","isPlaying":true}`)
	s.Render("first", 12)
	assertSample(t, s.Render("first", 1)[0].Left, 0.125)
	assertSample(t, s.Render("second", 1)[0].Left, 0.5)
	must(t, s.Volumes("first", json.RawMessage(`{"typeVolumes":{"Music":0}}`)))
	s.Render("first", 12)
	assertSample(t, s.Render("first", 1)[0].Left, 0)
	must(t, s.Sync("first", json.RawMessage(`{"tracks":[]}`)))
	if len(s.Status("first").Tracks) != 0 || len(s.Status("second").Tracks) != 1 {
		t.Fatal("empty full sync leaked between tables")
	}
	s.RemoveTrack("", "tone")
	if len(s.Status("second").Tracks) != 0 {
		t.Fatal("deleted track retained")
	}
}

func TestRejectInvalidCommandsWithoutPartialMutation(t *testing.T) {
	s := testService(t)
	update(t, s, "table", `{"fileID":"tone","isPlaying":true}`)
	before := s.Status("table")
	for _, payload := range []string{
		`null`, `{}`, `{"fileID":"tone","volume":-1}`, `{"fileID":"tone","volume":201}`,
		`{"fileID":"tone","currentTime":-1,"volume":0}`,
		`{"fileID":"tone","currentTime":99}`,
		`{"fileID":"tone","playback":{"startTime":5,"endTime":2,"loopBackTime":null}}`,
		`{"fileID":"tone","playback":{"startTime":0,"endTime":99,"loopBackTime":null}}`,
		`{"fileID":"tone","playback":{"startTime":0,"endTime":2,"loopBackTime":2}}`,
		`{"fileID":"unassociated","isPlaying":true}`,
	} {
		if err := s.Update("table", json.RawMessage(payload)); err == nil {
			t.Fatalf("accepted %s", payload)
		}
		if got := s.Status("table"); got.Revision != before.Revision || got.Tracks[0] != before.Tracks[0] {
			t.Fatalf("rejected update mutated state: %+v", got)
		}
	}
	if err := s.Sync("table", json.RawMessage(`{"tracks":[{"fileID":"tone","volume":0},{"fileID":"unassociated"}]}`)); err == nil {
		t.Fatal("accepted invalid full snapshot")
	}
	if s.Status("table").Revision != before.Revision {
		t.Fatal("invalid snapshot partially applied")
	}
}

func TestLiveRegionAndRepeatChangesDriveLoopBackAudio(t *testing.T) {
	s, err := New(6, func(context.Context, string, string) (Clip, error) {
		frames := make([]mixer.Frame, 30)
		for i := range frames {
			frames[i].Left = float64(i) / 60
		}
		return Clip{Frames: frames}, nil
	})
	must(t, err)
	update(t, s, "table", `{"fileID":"tone","isPlaying":true}`)
	update(t, s, "table", `{"fileID":"tone","isRepeating":true,"playback":{"startTime":1,"endTime":4,"loopBackTime":2},"currentTime":3.5}`)
	frames := s.Render("table", 4)
	for i, want := range []float64{0.35, 19.0 / 60, 17.0 / 60, 0.25} {
		assertSample(t, frames[i].Left, want)
	}
	update(t, s, "table", `{"fileID":"tone","isRepeating":false,"currentTime":4}`)
	assertSample(t, s.Render("table", 1)[0].Left, 0)
	state := s.Status("table").Tracks[0]
	if state.CurrentTime != 1 || state.IsPlaying {
		t.Fatalf("end seek did not complete trimmed playback: %+v", state)
	}
}

func TestRenderingAndClosingDoNotWaitForDecoding(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	s, err := New(6, func(ctx context.Context, tableID, fileID string) (Clip, error) {
		if fileID == "slow" {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return Clip{}, ctx.Err()
			}
		}
		return Clip{Frames: []mixer.Frame{{Left: 0.5}}, Type: "Music"}, nil
	})
	must(t, err)
	update(t, s, "table", `{"fileID":"tone","isPlaying":true}`)
	result := make(chan error, 1)
	go func() { result <- s.Update("table", json.RawMessage(`{"fileID":"slow","isPlaying":true}`)) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("decoder did not start")
	}
	assertSample(t, s.Render("table", 1)[0].Left, 0.5)
	closed := make(chan struct{})
	go func() {
		s.CloseTable("table")
		close(closed)
	}()
	select {
	case <-closed:
		close(release)
	case <-time.After(time.Second):
		close(release)
		<-result
		<-closed
		t.Fatal("closing waited for decoding")
	}
	if err := <-result; err == nil {
		t.Fatal("pending decode recreated a closed session")
	}
	if len(s.Status("table").Tracks) != 0 {
		t.Fatal("closed session still contains audio")
	}
}

func TestRunRendersAndStopsOnCancellation(t *testing.T) {
	s, err := New(50, func(context.Context, string, string) (Clip, error) {
		return Clip{Frames: make([]mixer.Frame, 100)}, nil
	})
	must(t, err)
	update(t, s, "table", `{"fileID":"tone","isPlaying":true}`)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	for s.Status("table").FramesRendered < 2 && ctx.Err() == nil {
		runtime.Gosched()
	}
	if ctx.Err() != nil {
		t.Fatal("live clock did not render")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("live clock did not stop")
	}
}

func testService(t *testing.T) *Service {
	t.Helper()
	s, err := New(6, func(ctx context.Context, tableID, fileID string) (Clip, error) {
		if fileID != "tone" {
			return Clip{}, errors.New("not associated with table")
		}
		frames := make([]mixer.Frame, 60)
		for i := range frames {
			frames[i] = mixer.Frame{Left: 0.5, Right: -0.5}
		}
		return Clip{Frames: frames, Type: "Music"}, nil
	})
	must(t, err)
	return s
}

func update(t *testing.T, s *Service, tableID, payload string) {
	t.Helper()
	must(t, s.Update(tableID, json.RawMessage(payload)))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func assertSample(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("sample %v, want %v", got, want)
	}
}
