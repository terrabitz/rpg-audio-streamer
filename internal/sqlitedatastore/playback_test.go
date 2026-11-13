package sqlitedatastore

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

func TestPlaybackPersistence(t *testing.T) {
	db, path := migratedDatabase(t)
	ctx := context.Background()
	types, err := db.GetTrackTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	end, loop := 50.5, 20.25
	settings := server.PlaybackSettings{StartTime: 10, EndTime: &end, LoopBackTime: &loop}
	track := server.Track{
		ID: uuid.New(), Name: "Boss", TypeID: types[0].ID, Path: "/uploads/boss",
		CreatedAt: time.Now().UTC().Truncate(time.Second), Playback: settings,
	}
	if err := db.SaveTrackToTable(ctx, &track, server.DefaultTableID); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetTrackByID(ctx, track.ID)
	if err != nil || !reflect.DeepEqual(got, track) {
		t.Fatalf("save = %+v, %v", got, err)
	}
	name := "Battle"
	got, err = db.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{Name: &name})
	if err != nil || !reflect.DeepEqual(got.Playback, settings) {
		t.Fatalf("omitted playback = %+v, %v", got, err)
	}
	tracks, err := db.GetTableTracks(ctx, server.DefaultTableID)
	if err != nil || len(tracks) != 1 || !reflect.DeepEqual(tracks[0].Playback, settings) {
		t.Fatalf("table playback = %+v, %v", tracks, err)
	}
	settings.StartTime = 12.5
	got, err = db.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{Playback: &settings})
	if err != nil || !reflect.DeepEqual(got.Playback, settings) {
		t.Fatalf("update = %+v, %v", got, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err = reopened.GetTrackByID(ctx, track.ID)
	if err != nil || !reflect.DeepEqual(got.Playback, settings) {
		t.Fatalf("reopened = %+v, %v", got, err)
	}
	defaults := server.PlaybackSettings{}
	got, err = reopened.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{Playback: &defaults})
	if err != nil || !reflect.DeepEqual(got.Playback, defaults) {
		t.Fatalf("clear = %+v, %v", got, err)
	}
	invalid := server.PlaybackSettings{StartTime: -1}
	if _, err := reopened.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{Playback: &invalid}); err == nil {
		t.Fatal("invalid update accepted")
	}
	track.ID = uuid.New()
	track.Playback = invalid
	if err := reopened.SaveTrack(ctx, &track); err == nil {
		t.Fatal("invalid save accepted")
	}
}
