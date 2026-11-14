package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
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
	h.handleSyncTrack(json.RawMessage(fmt.Sprintf(`{"fileID":%q,"playback":{"startTime":1}}`, testTrackID)), clients["otherGM"])
	snapshot = readPlayback(t, clients["otherGM"])
	if snapshot.Tracks[0].PositionVersion != 3 {
		t.Fatal("marker edits must not issue a seek command")
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
	h.RemoveTrack(server.DefaultTableID.String(), testTrackID.String())
	if _, ok := h.playback[server.DefaultTableID.String()][testTrackID.String()]; ok {
		t.Fatal("removed track survives recovery")
	}
	h.CloseTable(server.DefaultTableID.String())
	if _, ok := h.playback[server.DefaultTableID.String()]; ok {
		t.Fatal("deleted table survives recovery")
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
	h.SyncSettings(tableID, server.TableSettingsPatch{
		TrackVolumes: map[string]float64{testTrackID.String(): 10},
	})
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
