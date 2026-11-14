package sqlitedatastore

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

func TestTrackLifecycle(t *testing.T) {
	db, path := migratedDatabase(t)
	ctx := context.Background()

	tracks, err := db.GetTracks(ctx)
	if err != nil || tracks == nil || len(tracks) != 0 {
		t.Fatalf("new database tracks = %v, %v", tracks, err)
	}
	types, err := db.GetTrackTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	track := server.Track{
		ID: uuid.New(), CreatedAt: time.Now().UTC().Truncate(time.Second),
		Name: "Forest", Path: "/uploads/forest", TypeID: types[0].ID,
	}
	if err := db.SaveTrack(ctx, &track); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveTrack(ctx, &track); err == nil {
		t.Fatal("duplicate primary key should fail")
	}
	got, err := db.GetTrackByID(ctx, track.ID)
	if err != nil || !reflect.DeepEqual(got, track) {
		t.Fatalf("round trip = %#v, %v; want %#v", got, err, track)
	}
	tracks, err = db.GetTracks(ctx)
	if err != nil || !reflect.DeepEqual(tracks, []server.Track{track}) {
		t.Fatalf("list = %#v, %v", tracks, err)
	}

	name := "Rain"
	got, err = db.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{Name: &name})
	track.Name = name
	if err != nil || !reflect.DeepEqual(got, track) {
		t.Fatalf("name-only update = %#v, %v; want %#v", got, err, track)
	}
	got, err = db.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{TypeID: &types[1].ID})
	track.TypeID = types[1].ID
	if err != nil || !reflect.DeepEqual(got, track) {
		t.Fatalf("type-only update = %#v, %v; want %#v", got, err, track)
	}
	got, err = db.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{})
	if err != nil || !reflect.DeepEqual(got, track) {
		t.Fatalf("empty patch changed track = %#v, %v", got, err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	got, err = reopened.GetTrackByID(ctx, track.ID)
	if err != nil || !reflect.DeepEqual(got, track) {
		t.Fatalf("persisted track = %#v, %v", got, err)
	}
	if err := reopened.DeleteTrack(ctx, track.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.GetTrackByID(ctx, track.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted track error = %v; want sql.ErrNoRows", err)
	}
	if _, err := reopened.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{Name: &name}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing update error = %v; want sql.ErrNoRows", err)
	}
	if err := reopened.DeleteTrack(ctx, track.ID); err != nil {
		t.Fatalf("repeated delete should be idempotent: %v", err)
	}
}

func TestSeededTrackTypes(t *testing.T) {
	db, _ := migratedDatabase(t)
	types, err := db.GetTrackTypes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		repeating, simultaneous bool
	}{
		"Ambiance": {true, true},
		"Music":    {true, false},
		"One-Shot": {false, true},
	}
	if len(types) != len(want) {
		t.Fatalf("track types = %v", types)
	}
	for _, trackType := range types {
		flags, ok := want[trackType.Name]
		if !ok || trackType.IsRepeating != flags.repeating || trackType.AllowSimultaneousPlay != flags.simultaneous || trackType.Color == "" {
			t.Errorf("unexpected seeded type: %#v", trackType)
		}
		got, err := db.GetTrackTypeByID(context.Background(), trackType.ID)
		if err != nil || got != trackType {
			t.Errorf("type lookup = %#v, %v; want %#v", got, err, trackType)
		}
	}
	if _, err := db.GetTrackTypeByID(context.Background(), uuid.New()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing type error = %v", err)
	}
}

func TestDatastoreCancellation(t *testing.T) {
	db, _ := migratedDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	id := uuid.New()
	checks := []struct {
		name string
		run  func() error
	}{
		{"save", func() error { return db.SaveTrack(ctx, &server.Track{ID: id}) }},
		{"list", func() error { _, err := db.GetTracks(ctx); return err }},
		{"get", func() error { _, err := db.GetTrackByID(ctx, id); return err }},
		{"update", func() error { _, err := db.UpdateTrack(ctx, id, server.UpdateTrackRequest{}); return err }},
		{"delete", func() error { return db.DeleteTrack(ctx, id) }},
		{"types", func() error { _, err := db.GetTrackTypes(ctx); return err }},
		{"type", func() error { _, err := db.GetTrackTypeByID(ctx, id); return err }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v; want context.Canceled", err)
			}
		})
	}
}

func TestMigrationLifecycle(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m, err := NewMigration(os.DirFS("../../sql/migrations"), db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Version(); !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatalf("unmigrated version error = %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	version, err := m.Version()
	if err != nil || version.Version != 5 || version.Dirty {
		t.Fatalf("version = %#v, %v", version, err)
	}
	if err := m.Up(); !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("second up = %v; want ErrNoChange", err)
	}
	if err := m.Down(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetTracks(context.Background()); err == nil {
		t.Fatal("tracks table still exists after down migration")
	}
	if err := m.Up(); err != nil {
		t.Fatalf("reapply: %v", err)
	}
	types, err := db.GetTrackTypes(context.Background())
	if err != nil || len(types) != 3 {
		t.Fatalf("reapplied seeds = %v, %v", types, err)
	}
}

func TestTagLifecycle(t *testing.T) {
	db, path := migratedDatabase(t)
	ctx := context.Background()
	tags, err := db.GetTags(ctx)
	if err != nil || tags == nil || len(tags) != 0 {
		t.Fatalf("empty tags = %v, %v", tags, err)
	}
	intensity := server.Tag{ID: uuid.New(), Name: "High intensity", Color: "#ff0000"}
	calm := server.Tag{ID: uuid.New(), Name: "Calm", Color: "#00ff00"}
	for _, tag := range []server.Tag{intensity, calm} {
		if got, err := db.CreateTag(ctx, tag); err != nil || got != tag {
			t.Fatalf("create tag = %v, %v", got, err)
		}
	}
	if _, err := db.CreateTag(ctx, server.Tag{ID: uuid.New(), Name: "CALM", Color: "#ffffff"}); !errors.Is(err, server.ErrTagConflict) {
		t.Fatalf("duplicate tag = %v", err)
	}
	types, err := db.GetTrackTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	track := server.Track{ID: uuid.New(), Name: "Battle", Path: "/battle", TypeID: types[0].ID,
		CreatedAt: time.Now().UTC().Truncate(time.Second)}
	if err := db.SaveTrackToTable(ctx, &track, server.DefaultTableID); err != nil {
		t.Fatal(err)
	}
	ids := []uuid.UUID{intensity.ID, calm.ID}
	updated, err := db.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{TagIDs: &ids})
	if err != nil || !reflect.DeepEqual(updated.Tags, []server.Tag{calm, intensity}) {
		t.Fatalf("assigned tags = %v, %v", updated.Tags, err)
	}
	name := "Must not persist"
	for _, invalid := range [][]uuid.UUID{{uuid.New()}, {intensity.ID, intensity.ID}, {uuid.Nil}} {
		if _, err := db.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{Name: &name, TagIDs: &invalid}); !errors.Is(err, server.ErrInvalidTag) {
			t.Fatalf("invalid assignment = %v", err)
		}
		got, err := db.GetTrackByID(ctx, track.ID)
		if err != nil || !reflect.DeepEqual(got, updated) {
			t.Fatalf("failed assignment changed track = %v, %v", got, err)
		}
	}
	if _, err := db.UpdateTag(ctx, server.Tag{ID: intensity.ID, Name: "calm", Color: "#ffffff"}); !errors.Is(err, server.ErrTagConflict) {
		t.Fatalf("duplicate rename = %v", err)
	}
	intensity.Color = "#123456"
	if _, err := db.UpdateTag(ctx, intensity); err != nil {
		t.Fatal(err)
	}
	for _, list := range []func(context.Context) ([]server.Track, error){
		db.GetTracks, func(ctx context.Context) ([]server.Track, error) {
			return db.GetTableTracks(ctx, server.DefaultTableID)
		},
	} {
		tracks, err := list(ctx)
		if err != nil || len(tracks) != 1 || !reflect.DeepEqual(tracks[0].Tags, []server.Tag{calm, intensity}) {
			t.Fatalf("shared tags = %v, %v", tracks, err)
		}
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{})
	if err != nil || !reflect.DeepEqual(got.Tags, []server.Tag{calm, intensity}) {
		t.Fatalf("reopened omitted tags = %v, %v", got.Tags, err)
	}
	if err := db.DeleteTag(ctx, calm.ID); err != nil {
		t.Fatal(err)
	}
	got, err = db.GetTrackByID(ctx, track.ID)
	if err != nil || !reflect.DeepEqual(got.Tags, []server.Tag{intensity}) {
		t.Fatalf("delete tag kept audio and other tags = %v, %v", got, err)
	}
	empty := []uuid.UUID{}
	got, err = db.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{TagIDs: &empty})
	if err != nil || len(got.Tags) != 0 {
		t.Fatalf("clear tags = %v, %v", got.Tags, err)
	}
	ids = []uuid.UUID{intensity.ID}
	if _, err := db.UpdateTrack(ctx, track.ID, server.UpdateTrackRequest{TagIDs: &ids}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteTrack(ctx, track.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveTrack(ctx, &track); err != nil {
		t.Fatal(err)
	}
	got, err = db.GetTrackByID(ctx, track.ID)
	if err != nil || len(got.Tags) != 0 {
		t.Fatalf("track deletion did not clear assignments = %v, %v", got, err)
	}
	missing := uuid.New()
	if _, err := db.UpdateTag(ctx, server.Tag{ID: missing, Name: "Missing", Color: "#ffffff"}); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing update = %v", err)
	}
	if err := db.DeleteTag(ctx, missing); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing delete = %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := db.GetTags(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled tags = %v", err)
	}
}

func migratedDatabase(t *testing.T) (*SQLiteDatastore, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m, err := NewMigration(os.DirFS("../../sql/migrations"), db)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	return db, path
}
