package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/livemixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/mixer"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
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

func (playbackTestStore) GetTrackByID(_ context.Context, id uuid.UUID) (server.Track, error) {
	return server.Track{ID: id, Name: "Music", TypeID: testTypeID}, nil
}

func (playbackTestStore) GetTrackTypeByID(context.Context, uuid.UUID) (server.TrackType, error) {
	return server.TrackType{ID: testTypeID, Name: "Music", IsRepeating: true}, nil
}

func (playbackTestStore) GetTableSettings(context.Context, uuid.UUID) (server.TableSettings, error) {
	return server.TableSettings{MasterVolume: 40, TrackVolumes: map[string]float64{testTrackID.String(): 50}}, nil
}

func TestSharedPlaybackAndRecovery(t *testing.T) {
	h, clients := testHub()
	for _, client := range clients {
		client.Token.TableID = server.DefaultTableID.String()
	}
	command := fmt.Sprintf(`{"fileID":%q,"isPlaying":true,"duration":60}`, testTrackID.String())
	h.handleSyncTrack(json.RawMessage(command), clients["gm"])
	for id, client := range clients {
		snapshot := readPlayback(t, client)
		if len(snapshot.Tracks) != 1 || !snapshot.Tracks[0].IsPlaying {
			t.Fatalf("%s playback = %+v", id, snapshot)
		}
		wantVolume := 20.0
		if id == "gm" || id == "otherGM" {
			wantVolume = 50
			if snapshot.Settings == nil {
				t.Fatal("GM did not receive saved mix")
			}
		}
		if snapshot.Tracks[0].Volume != wantVolume {
			t.Fatalf("%s volume = %v", id, snapshot.Tracks[0].Volume)
		}
	}
	// No browser supplies time updates or responds to this late join.
	state := h.playback[server.DefaultTableID.String()][testTrackID.String()]
	state.updatedAt = time.Now().Add(-8 * time.Second)
	h.playback[server.DefaultTableID.String()][testTrackID.String()] = state
	delete(h.clients, clients["gm"])
	h.handleSyncRequest(nil, clients["otherGM"])
	snapshot := readPlayback(t, clients["otherGM"])
	if snapshot.Tracks[0].CurrentTime < 8 || !snapshot.Tracks[0].IsPlaying {
		t.Fatalf("recovered state = %+v", snapshot)
	}
	if len(clients["player"].send) != 0 {
		t.Fatal("targeted recovery broadcast to other clients")
	}
	h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"isPlaying":false}`, testTrackID)), clients["otherGM"])
	snapshot = readPlayback(t, clients["otherGM"])
	if snapshot.Tracks[0].IsPlaying || snapshot.Tracks[0].CurrentTime < 8 {
		t.Fatalf("pause did not preserve position: %+v", snapshot)
	}
	h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"currentTime":12}`, testTrackID)), clients["otherGM"])
	snapshot = readPlayback(t, clients["otherGM"])
	if snapshot.Tracks[0].CurrentTime != 12 || snapshot.Tracks[0].IsPlaying {
		t.Fatalf("paused seek = %+v", snapshot)
	}
	if snapshot.Tracks[0].PositionVersion != 3 {
		t.Fatalf("seek command version = %d", snapshot.Tracks[0].PositionVersion)
	}
	if snapshot.Tracks[0].SeekVersion != 1 {
		t.Fatalf("seek version = %d; play/pause must not count as seeks", snapshot.Tracks[0].SeekVersion)
	}
	h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"playback":{"startTime":1}}`, testTrackID)), clients["otherGM"])
	snapshot = readPlayback(t, clients["otherGM"])
	if snapshot.Tracks[0].PositionVersion != 3 || snapshot.Tracks[0].SeekVersion != 1 {
		t.Fatal("marker edits must not issue a seek command")
	}
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
			h, clients := testHub()
			for _, client := range clients {
				client.Token.TableID = server.DefaultTableID.String()
			}
			h.playback[server.DefaultTableID.String()] = map[string]playingTrack{
				testTrackID.String(): {
					FileID: testTrackID.String(), CurrentTime: tc.start, Duration: 30,
					IsPlaying: tc.playing, updatedAt: time.Now(),
				},
			}
			command := fmt.Sprintf(`{"fileID":%q,"currentTime":%v,"playback":{"startTime":%v,"endTime":%v}}`,
				testTrackID, tc.target, tc.start, tc.end)
			h.handleSyncTrack(json.RawMessage(command), clients["gm"])
			state := readPlayback(t, clients["gm"]).Tracks[0]
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
				h, clients := testHub()
				for _, client := range clients {
					client.Token.TableID = server.DefaultTableID.String()
				}
				h.playback[server.DefaultTableID.String()] = map[string]playingTrack{
					testTrackID.String(): {
						FileID: testTrackID.String(), CurrentTime: start + 10, Duration: 30,
						IsPlaying: true, updatedAt: time.Now(),
					},
				}
				target := start + offset
				command := fmt.Sprintf(`{"fileID":%q,"currentTime":%v,"playback":{"startTime":%v}}`, testTrackID, target, start)
				h.handleSyncTrack(json.RawMessage(command), clients["gm"])
				snapshot := readPlayback(t, clients["gm"]).Tracks[0]
				if snapshot.SeekTarget == nil || *snapshot.SeekTarget != target ||
					snapshot.SeekLanding == nil || *snapshot.SeekLanding != start+2 ||
					snapshot.CurrentTime < start || snapshot.CurrentTime > start+0.1 {
					t.Fatalf("initial seek snapshot = %+v", snapshot)
				}
				state := h.playback[server.DefaultTableID.String()][testTrackID.String()]
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
	h, clients := testHub()
	for _, client := range clients {
		client.Token.TableID = server.DefaultTableID.String()
	}
	h.playback[server.DefaultTableID.String()] = map[string]playingTrack{
		testTrackID.String(): {
			FileID: testTrackID.String(), CurrentTime: 29, Duration: 30,
			IsPlaying: true, updatedAt: time.Now(),
		},
	}
	h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"currentTime":2}`, testTrackID)), clients["gm"])
	snapshot := readPlayback(t, clients["gm"]).Tracks[0]
	if snapshot.SeekTarget == nil || snapshot.SeekLanding == nil ||
		*snapshot.SeekLanding > 1 || *snapshot.SeekLanding < 0.9 || snapshot.CurrentTime > 0.1 {
		t.Fatalf("short seek snapshot = %+v", snapshot)
	}
	state := h.playback[server.DefaultTableID.String()][testTrackID.String()]
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
	h, clients := testHub()
	for _, client := range clients {
		client.Token.TableID = server.DefaultTableID.String()
	}
	h.playback[server.DefaultTableID.String()] = map[string]playingTrack{
		testTrackID.String(): {
			FileID: testTrackID.String(), CurrentTime: 10, Duration: 30,
			IsPlaying: true, updatedAt: time.Now(),
		},
	}
	h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"currentTime":0}`, testTrackID)), clients["gm"])
	if state := readPlayback(t, clients["gm"]).Tracks[0]; state.SeekTarget == nil {
		t.Fatal("seek selection was not retained during the fade")
	}
	h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"isPlaying":false}`, testTrackID)), clients["gm"])
	state := readPlayback(t, clients["gm"]).Tracks[0]
	if state.IsPlaying || state.SeekTarget != nil || state.SeekLanding != nil {
		t.Fatalf("pause retained an interrupted seek selection: %+v", state)
	}
}

func TestRegionEditClearsAnOutOfBoundsSeekSelection(t *testing.T) {
	h, clients := testHub()
	for _, client := range clients {
		client.Token.TableID = server.DefaultTableID.String()
	}
	h.playback[server.DefaultTableID.String()] = map[string]playingTrack{
		testTrackID.String(): {
			FileID: testTrackID.String(), CurrentTime: 10, Duration: 60,
			IsPlaying: true, updatedAt: time.Now(),
		},
	}
	h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"currentTime":45}`, testTrackID)), clients["gm"])
	readPlayback(t, clients["gm"])
	h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"playback":{"startTime":0,"endTime":30}}`, testTrackID)), clients["gm"])
	state := readPlayback(t, clients["gm"]).Tracks[0]
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

func TestPlaybackCommandsAreAtomicAndAuthorized(t *testing.T) {
	h, clients := testHub()
	for _, client := range clients {
		client.Token.TableID = server.DefaultTableID.String()
	}
	var wg sync.WaitGroup
	for i, id := range []uuid.UUID{testTrackID, testOtherID} {
		wg.Add(1)
		go func(client *Client, id uuid.UUID) {
			defer wg.Done()
			h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"isPlaying":true,"duration":60}`, id)), client)
		}(clients[[]string{"gm", "otherGM"}[i]], id)
	}
	wg.Wait()
	for _, client := range clients {
		readPlayback(t, client)
		snapshot := readPlayback(t, client)
		playing := 0
		for _, track := range snapshot.Tracks {
			if track.IsPlaying {
				playing++
			}
		}
		if playing != 1 {
			t.Fatalf("exclusive commands diverged: %+v", snapshot)
		}
	}
	mix := &testMixer{}
	h.SetMixer(mix)
	for _, command := range []string{
		fmt.Sprintf(`{"fileID":%q,"volume":-1}`, testTrackID),
		fmt.Sprintf(`{"fileID":%q,"playback":{"startTime":20,"endTime":10}}`, testTrackID),
		`{"fileID":"00000000-0000-4000-8000-000000000099","isPlaying":true}`,
		`null`,
	} {
		h.handleSyncTrack(json.RawMessage(command), clients["gm"])
		var message Message
		if err := json.Unmarshal(<-clients["gm"].send, &message); err != nil || message.Method != "playbackError" {
			t.Fatalf("invalid command response = %+v, %v", message, err)
		}
	}
	h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"isPlaying":false}`, testTrackID)), clients["player"])
	if len(clients["gm"].send) != 0 {
		t.Fatal("player changed shared playback")
	}
	if len(mix.calls) != 0 {
		t.Fatalf("rejected playback commands reached the mixer: %v", mix.calls)
	}
	h.RemoveTrack(server.DefaultTableID.String(), testTrackID.String())
	if _, ok := h.playback[server.DefaultTableID.String()][testTrackID.String()]; ok {
		t.Fatal("removed track survives recovery")
	}
	h.CloseTable(server.DefaultTableID.String())
	if _, ok := h.playback[server.DefaultTableID.String()]; ok {
		t.Fatal("deleted table survives recovery")
	}
}

func TestPlaybackExclusivityFadesMixerAudio(t *testing.T) {
	for _, simultaneous := range []bool{false, true} {
		t.Run(fmt.Sprintf("simultaneous=%v", simultaneous), func(t *testing.T) {
			h, clients := testHub()
			h.store = playbackTypeTestStore{simultaneous: simultaneous}
			tableID := server.DefaultTableID.String()
			for _, client := range clients {
				client.Token.TableID = tableID
			}
			mix, err := livemixer.New(6, func(_ context.Context, _, fileID string) (livemixer.Clip, error) {
				sample := mixer.Frame{Left: 0.5}
				if fileID == testOtherID.String() {
					sample = mixer.Frame{Right: 0.5}
				}
				frames := make([]mixer.Frame, 360)
				for i := range frames {
					frames[i] = sample
				}
				return livemixer.Clip{Frames: frames, Type: "Music", Repeating: true}, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			h.SetMixer(mix)
			play := func(id uuid.UUID) {
				t.Helper()
				h.handleSyncTrack(json.RawMessage(fmt.Sprintf(
					`{"fileID":%q,"isPlaying":true,"isRepeating":true,"duration":60}`, id)), clients["gm"])
				for _, client := range clients {
					readPlayback(t, client)
				}
				var response Message
				if err := json.Unmarshal(<-clients["gm"].send, &response); err != nil || response.Method != "mixerAck" {
					t.Fatalf("mixer response = %+v, %v", response, err)
				}
			}
			play(testTrackID)
			mix.Render(tableID, 12)
			play(testOtherID)
			frames := mix.Render(tableID, 13)
			if frames[0] != (mixer.Frame{Left: 0.5}) {
				t.Fatalf("switch cut off outgoing audio: %+v", frames[0])
			}
			want := mixer.Frame{Left: 0.25, Right: 0.25}
			if simultaneous {
				want.Left = 0.5
			}
			if frames[6] != want {
				t.Fatalf("mid-transition audio = %+v; want %+v", frames[6], want)
			}
			want = mixer.Frame{Right: 0.5}
			if simultaneous {
				want.Left = 0.5
			}
			if frames[12] != want {
				t.Fatalf("settled audio = %+v; want %+v", frames[12], want)
			}
			for _, frame := range mix.Render(tableID, 12) {
				if frame != want {
					t.Fatalf("old track resumed after transition: %+v; want %+v", frame, want)
				}
			}
		})
	}
}

type playbackTypeTestStore struct {
	playbackTestStore
	simultaneous bool
}

func (s playbackTypeTestStore) GetTrackTypeByID(context.Context, uuid.UUID) (server.TrackType, error) {
	return server.TrackType{
		ID: testTypeID, Name: "Music", IsRepeating: true, AllowSimultaneousPlay: s.simultaneous,
	}, nil
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
				Duration: 60, Playback: server.PlaybackSettings{StartTime: 10, EndTime: &end, LoopBackTime: &loop},
				updatedAt: now.Add(-time.Duration(tc.elapsed * float64(time.Second)))}
			got := state.at(now)
			if got.CurrentTime != tc.want || got.IsPlaying != tc.playing {
				t.Fatalf("clock = %+v; want %v, playing %v", got, tc.want, tc.playing)
			}
		})
	}
}

func TestPlaybackRefreshAndSettings(t *testing.T) {
	h, clients := testHub()
	tableID := server.DefaultTableID.String()
	for _, client := range clients {
		client.Token.TableID = tableID
	}
	h.playback[tableID] = map[string]playingTrack{
		testTrackID.String(): {
			FileID: testTrackID.String(), IsPlaying: true, Duration: 1,
			updatedAt: time.Now().Add(-2 * time.Second), Volume: 50,
		},
	}
	h.refreshPlayback()
	for _, client := range clients {
		if readPlayback(t, client).Tracks[0].IsPlaying {
			t.Fatal("server did not notify clients of one-shot completion")
		}
	}
	h.refreshPlayback()
	if len(clients["gm"].send) != 0 {
		t.Fatal("idle playback was broadcast repeatedly")
	}
	h.store = playbackSettingsTestStore{settings: server.TableSettings{
		MasterVolume: 0, TrackVolumes: map[string]float64{testTrackID.String(): 30},
	}}
	state := h.playback[tableID][testTrackID.String()]
	state.IsPlaying = true
	state.updatedAt = time.Now()
	h.playback[tableID][testTrackID.String()] = state
	mix := &testMixer{}
	h.mixer = mix
	h.SyncSettings(tableID, server.TableSettingsPatch{
		TrackVolumes: map[string]float64{testTrackID.String(): 10},
	})
	if len(mix.calls) != 1 || mix.calls[0] != "update:"+tableID {
		t.Fatalf("mixer did not receive the track volume update: %v", mix.calls)
	}
	if readPlayback(t, clients["gm"]).Tracks[0].Volume != 30 {
		t.Fatal("GM did not receive the unscaled track mix")
	}
	if readPlayback(t, clients["player"]).Tracks[0].Volume != 0 {
		t.Fatal("player master mute was not applied")
	}
}

type playbackSettingsTestStore struct {
	playbackTestStore
	settings server.TableSettings
}

func (s playbackSettingsTestStore) GetTableSettings(context.Context, uuid.UUID) (server.TableSettings, error) {
	return s.settings, nil
}

type playbackSnapshot struct {
	Tracks   []playingTrack        `json:"tracks"`
	Settings *server.TableSettings `json:"settings"`
}

func readPlayback(t *testing.T, client *Client) playbackSnapshot {
	t.Helper()
	select {
	case raw := <-client.send:
		var message Message
		var snapshot playbackSnapshot
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatal(err)
		}
		if message.Method != "syncAll" {
			t.Fatalf("unexpected response: %s", raw)
		}
		if err := json.Unmarshal(message.Payload, &snapshot); err != nil {
			t.Fatal(err)
		}
		return snapshot
	default:
		t.Fatal("missing playback snapshot")
		return playbackSnapshot{}
	}
}
