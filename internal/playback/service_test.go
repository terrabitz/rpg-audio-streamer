package playback

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
	"log/slog"
	"testing"
	"time"
)

var (
	testTrackID = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	testOtherID = uuid.MustParse("00000000-0000-4000-8000-000000000003")
	testTypeID  = uuid.MustParse("00000000-0000-4000-8000-000000000004")
)

type playbackTestStore struct{}

func (playbackTestStore) HasTableTrack(_ context.Context, _ uuid.UUID, id uuid.UUID) (bool, error) {
	return id == testTrackID || id == testOtherID, nil
}

func (playbackTestStore) GetTrackByID(_ context.Context, id uuid.UUID) (domain.Track, error) {
	volume := 100.0
	if id == testTrackID {
		volume = 50
	}
	return domain.Track{ID: id, Name: "Music", TypeID: testTypeID, Volume: volume}, nil
}

func (playbackTestStore) GetTrackTypeByID(context.Context, uuid.UUID) (domain.TrackType, error) {
	return domain.TrackType{ID: testTypeID, Name: "Music", IsRepeating: true}, nil
}

func (playbackTestStore) GetTableSettings(context.Context, uuid.UUID) (domain.TableSettings, error) {
	return domain.TableSettings{MasterVolume: 40}, nil
}

var tableID = uuid.New()

type discardPublisher struct{}

func (discardPublisher) Publish(string, any, events.Metadata) error { return nil }

func update(t *testing.T, service *Service, raw json.RawMessage) {
	t.Helper()
	var patch PlaybackPatch
	if err := json.Unmarshal(raw, &patch); err != nil {
		t.Fatal(err)
	}
	if err := service.Update(t.Context(), tableID, testTrackID, patch); err != nil {
		t.Fatal(err)
	}
}

func readSnapshot(t *testing.T, service *Service) events.Snapshot {
	t.Helper()
	snapshot, err := service.Snapshot(t.Context(), tableID)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestSeekSnapshotSeparatesTheLandingTargetFromTheAdvancingClock(t *testing.T) {
	for _, tc := range []struct {
		start, end, target, landing, position float64
		playing                               bool
	}{
		{0, 30, 20, 20, 18, true},
		{0, 30, 30, 28, 26, true},
		{10, 25, 25, 23, 21, true},
		{10, 11, 11, 10, 10, true},
		{0, 30, 1, 1, 0, true},
		{0, 30, 30, 28, 28, false},
	} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			h := New(slog.Default(), playbackTestStore{}, discardPublisher{})
			h.playback[tableID.String()] = map[string]playingTrack{
				testTrackID.String(): {
					FileID: testTrackID.String(), CurrentTime: tc.start, Duration: 30,
					IsPlaying: tc.playing, updatedAt: time.Now(),
				},
			}
			event := fmt.Sprintf(`{"fileID":%q,"currentTime":%v,"playback":{"startTime":%v,"endTime":%v,"loopBackTime":null}}`,
				testTrackID, tc.target, tc.start, tc.end)
			update(t, h, json.RawMessage(event))
			state := readSnapshot(t, h).Tracks[0]
			if state.SeekTarget == nil || *state.SeekTarget != tc.landing {
				t.Fatalf("seek target = %v, want %v", state.SeekTarget, tc.landing)
			}
			if state.CurrentTime < tc.position || state.CurrentTime > tc.position+0.1 {
				t.Fatalf("seek clock = %v, want %v", state.CurrentTime, tc.position)
			}
		})
	}
}

func TestSeekNearBeginningRetainsTheCommandUntilTheFadeLands(t *testing.T) {
	for _, start := range []float64{0, 10} {
		for _, offset := range []float64{0, 1, 2} {
			t.Run(fmt.Sprint(start, "/", offset), func(t *testing.T) {
				h := New(slog.Default(), playbackTestStore{}, discardPublisher{})
				h.playback[tableID.String()] = map[string]playingTrack{
					testTrackID.String(): {
						FileID: testTrackID.String(), CurrentTime: start + 10, Duration: 30,
						IsPlaying: true, updatedAt: time.Now(),
					},
				}
				target := start + offset
				event := fmt.Sprintf(`{"fileID":%q,"currentTime":%v,"playback":{"startTime":%v,"endTime":null,"loopBackTime":null}}`, testTrackID, target, start)
				update(t, h, json.RawMessage(event))
				snapshot := readSnapshot(t, h).Tracks[0]
				if snapshot.SeekTarget == nil || *snapshot.SeekTarget != target ||
					snapshot.SeekLanding == nil || *snapshot.SeekLanding != start+2 ||
					snapshot.CurrentTime < start || snapshot.CurrentTime > start+0.1 {
					t.Fatalf("initial seek snapshot = %+v", snapshot)
				}
				state := h.playback[tableID.String()][testTrackID.String()]
				during := state.at(state.updatedAt.Add(time.Second))
				if during.SeekTarget == nil || *during.SeekTarget != target ||
					during.CurrentTime < start+1 || during.CurrentTime > start+1.1 {
					t.Fatalf("seek target expired during the opening: %+v", during)
				}
				landed := state.at(state.updatedAt.Add(2100 * time.Millisecond))
				if landed.SeekTarget != nil || landed.CurrentTime < start+2.1 {
					t.Fatalf("seek did not finish at the end of the opening: %+v", landed)
				}
			})
		}
	}
}

func TestSeekNearBeginningExpiresAtAShortenedFadeLanding(t *testing.T) {
	h := New(slog.Default(), playbackTestStore{}, discardPublisher{})
	h.playback[tableID.String()] = map[string]playingTrack{
		testTrackID.String(): {
			FileID: testTrackID.String(), CurrentTime: 29, Duration: 30,
			IsPlaying: true, updatedAt: time.Now(),
		},
	}
	update(t, h, json.RawMessage(fmt.Sprintf(`{"fileID":%q,"currentTime":2}`, testTrackID)))
	snapshot := readSnapshot(t, h).Tracks[0]
	if snapshot.SeekTarget == nil || snapshot.SeekLanding == nil ||
		*snapshot.SeekLanding > 1 || *snapshot.SeekLanding < 0.9 || snapshot.CurrentTime > 0.1 {
		t.Fatalf("short seek snapshot = %+v", snapshot)
	}
	state := h.playback[tableID.String()][testTrackID.String()]
	during := state.at(state.updatedAt.Add(500 * time.Millisecond))
	if during.SeekTarget == nil {
		t.Fatal("seek target expired before the shortened fade landed")
	}
	landed := state.at(state.updatedAt.Add(1100 * time.Millisecond))
	if landed.SeekTarget != nil || landed.CurrentTime > 1.2 {
		t.Fatalf("seek target outlived the shortened fade: %+v", landed)
	}
}

func TestPauseDuringSeekClearsThePendingSelection(t *testing.T) {
	h := New(slog.Default(), playbackTestStore{}, discardPublisher{})
	h.playback[tableID.String()] = map[string]playingTrack{
		testTrackID.String(): {
			FileID: testTrackID.String(), CurrentTime: 10, Duration: 30,
			IsPlaying: true, updatedAt: time.Now(),
		},
	}
	update(t, h, json.RawMessage(fmt.Sprintf(`{"fileID":%q,"currentTime":0}`, testTrackID)))
	if state := readSnapshot(t, h).Tracks[0]; state.SeekTarget == nil {
		t.Fatal("seek selection was not retained during the fade")
	}
	update(t, h, json.RawMessage(fmt.Sprintf(`{"fileID":%q,"isPlaying":false}`, testTrackID)))
	state := readSnapshot(t, h).Tracks[0]
	if state.IsPlaying || state.SeekTarget != nil || state.SeekLanding != nil {
		t.Fatalf("pause retained an interrupted seek selection: %+v", state)
	}
}

func TestRegionEditClearsAnOutOfBoundsSeekSelection(t *testing.T) {
	h := New(slog.Default(), playbackTestStore{}, discardPublisher{})
	h.playback[tableID.String()] = map[string]playingTrack{
		testTrackID.String(): {
			FileID: testTrackID.String(), CurrentTime: 10, Duration: 60,
			IsPlaying: true, updatedAt: time.Now(),
		},
	}
	update(t, h, json.RawMessage(fmt.Sprintf(`{"fileID":%q,"currentTime":45}`, testTrackID)))
	readSnapshot(t, h)
	update(t, h, json.RawMessage(fmt.Sprintf(`{"fileID":%q,"playback":{"startTime":0,"endTime":30,"loopBackTime":null}}`, testTrackID)))
	state := readSnapshot(t, h).Tracks[0]
	if state.SeekTarget != nil || state.SeekLanding != nil {
		t.Fatalf("region edit retained an unreachable seek selection: %+v", state)
	}
}

func TestSeekLandingTargetExpiresBeforeTheNextLoop(t *testing.T) {
	target := 28.0
	start := time.Now()
	state := playingTrack{
		CurrentTime: 26, Duration: 30, IsPlaying: true, IsRepeating: true,
		SeekTarget: &target, SeekLanding: &target, updatedAt: start,
	}
	during := state.at(start.Add(time.Second))
	if during.SeekTarget == nil || during.CurrentTime != 27 {
		t.Fatalf("seek target expired during preroll: %+v", during)
	}
	landed := state.at(start.Add(2 * time.Second))
	if landed.SeekTarget != nil || landed.CurrentTime != 28 {
		t.Fatalf("seek target did not expire on landing: %+v", landed)
	}
	looped := state.at(start.Add(4 * time.Second))
	if looped.SeekTarget != nil || looped.CurrentTime != 2 {
		t.Fatalf("loop retained the previous seek target: %+v", looped)
	}
}

func TestPlaybackClock(t *testing.T) {
	now := time.Now()
	end, loop := 25.0, 15.0
	for _, tc := range []struct {
		name      string
		repeating bool
		elapsed   float64
		want      float64
		playing   bool
	}{
		{"playing", false, 2, 12, true},
		{"one-shot finishes", false, 16, 10, false},
		{"first crossfade", true, 16, 18, true},
		{"multiple crossfades", true, 40, 18, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := playingTrack{IsPlaying: true, IsRepeating: tc.repeating, CurrentTime: 10,
				Duration: 60, Playback: domain.PlaybackSettings{StartTime: 10, EndTime: &end, LoopBackTime: &loop},
				updatedAt: now.Add(-time.Duration(tc.elapsed * float64(time.Second)))}
			got := state.at(now)
			if got.CurrentTime != tc.want || got.IsPlaying != tc.playing {
				t.Fatalf("clock = %+v; want %v, playing %v", got, tc.want, tc.playing)
			}
		})
	}
}
