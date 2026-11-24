package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/events"
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

func routeSyncTrack(h *Hub, payload json.RawMessage, client *Client) error {
	var patch *PlaybackPatch
	if err := json.Unmarshal(payload, &patch); err != nil {
		return err
	}
	if patch == nil {
		return fmt.Errorf("missing patch")
	}
	id, err := uuid.Parse(patch.FileID)
	if err != nil {
		return err
	}
	return h.playback.Update(context.Background(), uuid.MustParse(client.Token.TableID), id, *patch)
}

func (playbackTestStore) HasTableTrack(_ context.Context, _ uuid.UUID, id uuid.UUID) (bool, error) {
	return id == testTrackID || id == testOtherID, nil
}

func (playbackTestStore) GetTrackByID(_ context.Context, id uuid.UUID) (server.Track, error) {
	volume := 100.0
	if id == testTrackID {
		volume = 50
	}
	return server.Track{ID: id, Name: "Music", TypeID: testTypeID, Volume: volume}, nil
}

func (playbackTestStore) GetTrackTypeByID(context.Context, uuid.UUID) (server.TrackType, error) {
	return server.TrackType{ID: testTypeID, Name: "Music", IsRepeating: true}, nil
}

func (playbackTestStore) GetTableSettings(context.Context, uuid.UUID) (server.TableSettings, error) {
	return server.TableSettings{MasterVolume: 40}, nil
}

func TestSharedPlaybackAndRecovery(t *testing.T) {
	h, clients := testHub()
	for _, client := range clients {
		client.Token.TableID = server.DefaultTableID.String()
	}
	event := fmt.Sprintf(`{"fileID":%q,"isPlaying":true,"duration":60}`, testTrackID.String())
	routeSyncTrack(h, json.RawMessage(event), clients["gm"])
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
	snapshot, err := h.playback.Snapshot(t.Context(), server.DefaultTableID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Tracks[0].CurrentTime < 0 || !snapshot.Tracks[0].IsPlaying {
		t.Fatalf("recovered state = %+v", snapshot)
	}
	if len(clients["player"].send) != 0 {
		t.Fatal("targeted recovery broadcast to other clients")
	}
	routeSyncTrack(h, json.RawMessage(fmt.Sprintf(`{"fileID":%q,"isPlaying":false}`, testTrackID)), clients["otherGM"])
	snapshot = readPlayback(t, clients["otherGM"])
	if snapshot.Tracks[0].IsPlaying || snapshot.Tracks[0].CurrentTime < 0 {
		t.Fatalf("pause did not preserve position: %+v", snapshot)
	}
	routeSyncTrack(h, json.RawMessage(fmt.Sprintf(`{"fileID":%q,"currentTime":12}`, testTrackID)), clients["otherGM"])
	snapshot = readPlayback(t, clients["otherGM"])
	if snapshot.Tracks[0].CurrentTime != 12 || snapshot.Tracks[0].IsPlaying {
		t.Fatalf("paused seek = %+v", snapshot)
	}
	if snapshot.Tracks[0].PositionVersion != 3 {
		t.Fatalf("seek event version = %d", snapshot.Tracks[0].PositionVersion)
	}
	if snapshot.Tracks[0].SeekVersion != 1 {
		t.Fatalf("seek version = %d; play/pause must not count as seeks", snapshot.Tracks[0].SeekVersion)
	}
	routeSyncTrack(h, json.RawMessage(fmt.Sprintf(`{"fileID":%q,"playback":{"startTime":1,"endTime":null,"loopBackTime":null}}`, testTrackID)), clients["otherGM"])
	snapshot = readPlayback(t, clients["otherGM"])
	if snapshot.Tracks[0].PositionVersion != 3 || snapshot.Tracks[0].SeekVersion != 1 {
		t.Fatal("marker edits must not issue a seek event")
	}
}

func TestPlaybackCommandsAreAtomic(t *testing.T) {
	h, clients := testHub()
	for _, client := range clients {
		client.Token.TableID = server.DefaultTableID.String()
	}
	var wg sync.WaitGroup
	for i, id := range []uuid.UUID{testTrackID, testOtherID} {
		wg.Add(1)
		go func(client *Client, id uuid.UUID) {
			defer wg.Done()
			routeSyncTrack(h, json.RawMessage(fmt.Sprintf(`{"fileID":%q,"isPlaying":true,"duration":60}`, id)), client)
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
			t.Fatalf("exclusive events diverged: %+v", snapshot)
		}
	}
	mix := &testMixer{}
	t.Cleanup(livemixer.Subscribe(h, mix))
	for _, event := range []string{
		fmt.Sprintf(`{"fileID":%q,"volume":-1}`, testTrackID),
		fmt.Sprintf(`{"fileID":%q,"playback":{"startTime":20,"endTime":10,"loopBackTime":null}}`, testTrackID),
		`{"fileID":"00000000-0000-4000-8000-000000000099","isPlaying":true}`,
		`null`,
	} {
		if err := routeSyncTrack(h, json.RawMessage(event), clients["gm"]); err == nil {
			t.Fatal("invalid patch accepted")
		}
	}
	if len(mix.calls) != 0 {
		t.Fatalf("rejected playback events reached the mixer: %v", mix.calls)
	}
	sendEvent(t, h, events.TrackRemoved, events.TableEvent[events.TrackReference]{
		TableID: server.DefaultTableID.String(), Data: events.TrackReference{FileID: testTrackID.String()},
	})
	snapshot, err := h.playback.Snapshot(t.Context(), server.DefaultTableID)
	if err != nil || len(snapshot.Tracks) != 1 || snapshot.Tracks[0].FileID == testTrackID.String() {
		t.Fatalf("removed track survives recovery: %+v, %v", snapshot, err)
	}
	sendEvent(t, h, events.TableDeleted, server.DefaultTableID.String())
	snapshot, err = h.playback.Snapshot(t.Context(), server.DefaultTableID)
	if err != nil || len(snapshot.Tracks) != 0 {
		t.Fatalf("deleted table survives recovery: %+v, %v", snapshot, err)
	}
}

func TestPlaybackExclusivityFadesMixerAudio(t *testing.T) {
	for _, simultaneous := range []bool{false, true} {
		t.Run(fmt.Sprintf("simultaneous=%v", simultaneous), func(t *testing.T) {
			h, clients := testHubWithStore(playbackTypeTestStore{simultaneous: simultaneous})
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
			t.Cleanup(livemixer.Subscribe(h, mix))
			play := func(id uuid.UUID) {
				t.Helper()
				routeSyncTrack(h, json.RawMessage(fmt.Sprintf(
					`{"fileID":%q,"isPlaying":true,"isRepeating":true,"duration":60}`, id)), clients["gm"])
				for _, client := range clients {
					readPlayback(t, client)
				}
				<-clients["otherGM"].send
				var response Message
				if err := json.Unmarshal(<-clients["gm"].send, &response); err != nil || response.Event != "mixerApplied" {
					t.Fatalf("mixer response = %+v, %v", response, err)
				}
			}
			play(testTrackID)
			mix.Render(tableID, 12)
			play(testOtherID)
			frames := mix.Render(tableID, 13)
			if frames[0] != (mixer.Frame{Left: 0.25}) {
				t.Fatalf("switch cut off outgoing audio: %+v", frames[0])
			}
			want := mixer.Frame{Left: 0.125, Right: 0.25}
			if simultaneous {
				want.Left = 0.25
			}
			if frames[6] != want {
				t.Fatalf("mid-transition audio = %+v; want %+v", frames[6], want)
			}
			want = mixer.Frame{Right: 0.5}
			if simultaneous {
				want.Left = 0.25
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

func readPlayback(t *testing.T, client *Client) events.Snapshot {
	t.Helper()
	select {
	case raw := <-client.send:
		var message Message
		var snapshot events.Snapshot
		if err := json.Unmarshal(raw, &message); err != nil {
			t.Fatal(err)
		}
		if message.Event != "playbackStateChanged" {
			t.Fatalf("unexpected response: %s", raw)
		}
		if err := json.Unmarshal(message.Payload, &snapshot); err != nil {
			t.Fatal(err)
		}
		return snapshot
	default:
		t.Fatal("missing playback snapshot")
		return events.Snapshot{}
	}
}

func TestSavedTrackUpdatesReachGMsAndAllPlayingTables(t *testing.T) {
	tableID := server.DefaultTableID.String()
	otherTableID := uuid.NewString()
	fileID := testTrackID.String()
	end, loop := 50.0, 20.0
	store := &playbackTrackTestStore{
		track: server.Track{
			ID:       testTrackID,
			Name:     "Updated music",
			TypeID:   testTypeID,
			Volume:   25,
			Playback: server.PlaybackSettings{StartTime: 10, EndTime: &end, LoopBackTime: &loop},
		},
	}
	saved := store.track
	store.track = server.Track{ID: testTrackID, Name: "Music", TypeID: testTypeID, Volume: 50}
	h, clients := testHubWithStore(store)
	for _, client := range clients {
		client.Token.TableID = tableID
	}
	clients["otherGM"].Token.TableID = otherTableID
	for _, name := range []string{"gm", "otherGM"} {
		if err := routeSyncTrack(h, json.RawMessage(fmt.Sprintf(`{"fileID":%q,"isPlaying":true,"currentTime":25,"duration":60}`, testTrackID)), clients[name]); err != nil {
			t.Fatal(err)
		}
	}
	for _, client := range clients {
		readPlayback(t, client)
	}
	store.track = saved
	changes := make(map[string]events.PlaybackPatch)
	stop := events.OnPlaybackChanged(h, func(change events.PlaybackChanges[events.PlaybackPatch], metadata events.Metadata) error {
		changes[metadata.TableID] = change.Tracks[0]
		return nil
	})
	defer stop()
	if err := h.SendEvent(events.TrackUpdated, fileID); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gm", "otherGM"} {
		client := clients[name]
		var event Message
		if err := json.Unmarshal(<-client.send, &event); err != nil {
			t.Fatal(err)
		}
		var track server.Track
		if err := json.Unmarshal(event.Payload, &track); err != nil || event.Event != events.TrackUpdated ||
			track.Name != store.track.Name || track.Playback.StartTime != 10 || track.Volume != 25 {
			t.Fatalf("%s metadata = %+v, %v", name, track, err)
		}
	}
	for name, client := range clients {
		snapshot := readPlayback(t, client)
		state := snapshot.Tracks[0]
		wantVolume := 25.0
		if name == "player" || name == "otherPlayer" {
			wantVolume = 10
		}
		if state.Playback.StartTime != 10 || state.Playback.EndTime == nil || *state.Playback.EndTime != end ||
			state.Playback.LoopBackTime == nil || *state.Playback.LoopBackTime != loop ||
			state.CurrentTime < 25 || state.CurrentTime > 26 || !state.IsPlaying ||
			state.PositionVersion != 1 || state.SeekVersion != 1 || state.Volume != wantVolume {
			t.Fatalf("%s live state = %+v", name, state)
		}
		if len(client.send) != 0 {
			t.Fatalf("%s received unexpected extra events", name)
		}
	}
	if len(changes) != 2 {
		t.Fatalf("mixer updates = %+v", changes)
	}
	for _, patch := range changes {
		if patch.Playback == nil || patch.Playback.StartTime != 10 || patch.Volume == nil || *patch.Volume != 25 ||
			patch.CurrentTime != nil || patch.IsPlaying != nil {
			t.Fatalf("metadata update must not seek or toggle playback: %+v", patch)
		}
	}
}

type playbackTrackTestStore struct {
	playbackTestStore
	track server.Track
}

func (s *playbackTrackTestStore) GetTrackByID(context.Context, uuid.UUID) (server.Track, error) {
	return s.track, nil
}
