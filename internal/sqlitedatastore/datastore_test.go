package sqlitedatastore

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
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

func TestConcurrentTrackProcessingAndPolling(t *testing.T) {
	db, _ := migratedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	types, err := db.GetTrackTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for range 24 {
		workers.Go(func() {
			track := server.Track{ID: uuid.New(), CreatedAt: time.Now().UTC().Truncate(time.Second),
				Name: "Queued", TypeID: types[0].ID, Path: "/uploads/queued", Status: "processing"}
			if err := db.SaveTrackToTable(ctx, &track, server.DefaultTableID); err != nil {
				t.Error(err)
				return
			}
			if err := db.SetTrackProcessing(ctx, track.ID, "ready", ""); err != nil {
				t.Error(err)
			}
		})
		workers.Go(func() {
			if _, err := db.GetTracks(ctx); err != nil {
				t.Error(err)
			}
			if _, err := db.GetTableTracks(ctx, server.DefaultTableID); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	tracks, err := db.GetTracks(ctx)
	if err != nil || len(tracks) != 24 {
		t.Fatalf("completed tracks = %d, %v", len(tracks), err)
	}
	for _, track := range tracks {
		if !track.Ready() {
			t.Errorf("track did not finish: %+v", track)
		}
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

func TestConcurrentTableReadsAndSettingsWrites(t *testing.T) {
	db, _ := migratedDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	table, err := db.GetTable(ctx, server.DefaultTableID)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 8)
	for worker := range cap(results) {
		go func() {
			<-start
			for range 25 {
				var err error
				if worker%2 == 0 {
					volume := 50.0
					_, err = db.UpdateTableSettings(ctx, table.ID, server.TableSettingsPatch{MasterVolume: &volume})
				} else {
					_, err = db.GetTableByExternalID(ctx, table.ExternalID)
				}
				if err != nil {
					results <- err
					return
				}
			}
			results <- nil
		}()
	}
	close(start)
	for range cap(results) {
		if err := <-results; err != nil {
			t.Errorf("concurrent table access: %v", err)
		}
	}
}

func TestTrackProcessingPersistence(t *testing.T) {
	db, path := migratedDatabase(t)
	ctx := context.Background()
	types, err := db.GetTrackTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	track := server.Track{ID: uuid.New(), CreatedAt: time.Now().UTC().Truncate(time.Second),
		Name: "Processing", TypeID: types[0].ID, Path: "/uploads/processing", Status: "processing"}
	if err := db.SaveTrackToTable(ctx, &track, server.DefaultTableID); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	tracks, err := reopened.GetTableTracks(ctx, server.DefaultTableID)
	if err != nil || len(tracks) != 1 || tracks[0].Status != "processing" {
		t.Fatalf("persisted pending association = %+v, %v", tracks, err)
	}
	if err := reopened.SetTrackProcessing(ctx, track.ID, "failed", "Conversion failed"); err != nil {
		t.Fatal(err)
	}
	got, err := reopened.GetTrackByID(ctx, track.ID)
	if err != nil || got.Status != "failed" || got.ProcessingError != "Conversion failed" {
		t.Fatalf("failed track = %+v, %v", got, err)
	}
	if err := reopened.SetTrackProcessing(ctx, track.ID, "ready", ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("terminal job changed status: %v", err)
	}
	track.ID, track.Status, track.ProcessingError = uuid.New(), "processing", ""
	if err := reopened.SaveTrack(ctx, &track); err != nil {
		t.Fatal(err)
	}
	if err := reopened.SetTrackProcessing(ctx, track.ID, "ready", ""); err != nil {
		t.Fatal(err)
	}
	got, err = reopened.GetTrackByID(ctx, track.ID)
	if err != nil || got.Status != "ready" || got.ProcessingError != "" {
		t.Fatalf("ready track = %+v, %v", got, err)
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
	if err != nil || version.Version != 10 || version.Dirty {
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

func TestTrackWaveformPersistence(t *testing.T) {
	db, path := migratedDatabase(t)
	ctx := t.Context()
	types, err := db.GetTrackTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	track := server.Track{
		ID:     uuid.New(),
		Name:   "Waveform",
		TypeID: types[0].ID,
	}
	if err := db.SaveTrack(ctx, &track); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetTrackWaveform(ctx, track.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("waveform for track without peaks: %v", err)
	}
	want := musicprocessing.Waveform{
		Peaks: []float64{0, 0.25, 1},
	}
	if err := db.SaveTrackWaveform(ctx, track.ID, want); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.GetTrackWaveform(ctx, track.ID)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened waveform = %+v, %v; want %+v", got, err, want)
	}
	if err := reopened.DeleteTrack(ctx, track.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.GetTrackWaveform(ctx, track.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("waveform survived track deletion: %v", err)
	}
	if err := reopened.SaveTrackWaveform(ctx, track.ID, want); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("save waveform for deleted track: %v", err)
	}
}
