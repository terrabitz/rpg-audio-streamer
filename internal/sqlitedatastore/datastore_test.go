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
	if err != nil || version.Version != 2 || version.Dirty {
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
