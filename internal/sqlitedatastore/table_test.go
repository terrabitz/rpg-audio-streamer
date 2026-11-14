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

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

func TestTableSettingsPersistence(t *testing.T) {
	db, path := migratedDatabase(t)
	ctx := context.Background()
	defaults := server.TableSettings{
		MasterVolume: 100, TypeVolumes: map[string]float64{},
		TrackVolumes: map[string]float64{}, TrackOrder: []uuid.UUID{},
	}
	settings, err := db.GetTableSettings(ctx, server.DefaultTableID)
	if err != nil || !reflect.DeepEqual(settings, defaults) {
		t.Fatalf("default settings = %+v, %v", settings, err)
	}
	other := server.Table{ID: uuid.New(), Name: "Other", InviteToken: uuid.NewString()}
	if _, err := db.CreateTable(ctx, other); err != nil {
		t.Fatal(err)
	}
	trackID := uuid.New()
	order := []uuid.UUID{trackID, uuid.New()}
	master := 0.0
	want := server.TableSettings{
		MasterVolume: 0, TypeVolumes: map[string]float64{"Music": 0, "Ambiance": 42},
		TrackVolumes: map[string]float64{trackID.String(): 0}, TrackOrder: order,
	}
	settings, err = db.UpdateTableSettings(ctx, server.DefaultTableID, server.TableSettingsPatch{
		MasterVolume: &master, TypeVolumes: want.TypeVolumes,
		TrackVolumes: want.TrackVolumes, TrackOrder: &order,
	})
	if err != nil || !reflect.DeepEqual(settings, want) {
		t.Fatalf("save settings = %+v, %v", settings, err)
	}
	want.TypeVolumes["Ambiance"] = 65
	settings, err = db.UpdateTableSettings(ctx, server.DefaultTableID, server.TableSettingsPatch{
		TypeVolumes: map[string]float64{"Ambiance": 65},
	})
	if err != nil || !reflect.DeepEqual(settings, want) {
		t.Fatalf("partial settings = %+v, %v", settings, err)
	}
	if _, err := db.UpdateTable(ctx, server.DefaultTableID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	settings, err = db.GetTableSettings(ctx, server.DefaultTableID)
	if err != nil || !reflect.DeepEqual(settings, want) {
		t.Fatalf("reopened settings = %+v, %v", settings, err)
	}
	settings, err = db.GetTableSettings(ctx, other.ID)
	if err != nil || !reflect.DeepEqual(settings, defaults) {
		t.Fatalf("other table settings = %+v, %v", settings, err)
	}
	empty := []uuid.UUID{}
	settings, err = db.UpdateTableSettings(ctx, server.DefaultTableID, server.TableSettingsPatch{TrackOrder: &empty})
	if err != nil || len(settings.TrackOrder) != 0 || settings.TrackOrder == nil {
		t.Fatalf("empty order = %+v, %v", settings, err)
	}
	for _, id := range []uuid.UUID{uuid.New(), other.ID} {
		if id == other.ID {
			if err := db.DeleteTable(ctx, id); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.GetTableSettings(ctx, id); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("missing settings = %v", err)
		}
		if _, err := db.UpdateTableSettings(ctx, id, server.TableSettingsPatch{}); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("missing settings update = %v", err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := db.GetTableSettings(canceled, server.DefaultTableID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled settings lookup = %v", err)
	}
	if _, err := db.UpdateTableSettings(canceled, server.DefaultTableID, server.TableSettingsPatch{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled settings update = %v", err)
	}
}

func TestTableLifecycle(t *testing.T) {
	db, path := migratedDatabase(t)
	ctx := context.Background()
	tables, err := db.GetTables(ctx)
	if err != nil || len(tables) != 1 || tables[0].ID != server.DefaultTableID {
		t.Fatalf("seeded tables = %v, %v", tables, err)
	}
	first := server.Table{ID: uuid.New(), Name: "Friday", InviteToken: uuid.NewString()}
	second := server.Table{ID: uuid.New(), Name: "Sunday", InviteToken: uuid.NewString()}
	for _, table := range []server.Table{first, second} {
		if got, err := db.CreateTable(ctx, table); err != nil || got != table {
			t.Fatalf("create = %v, %v", got, err)
		}
	}
	if _, err := db.CreateTable(ctx, first); err == nil {
		t.Fatal("duplicate table should fail")
	}
	if got, err := db.GetTableByInvite(ctx, first.InviteToken); err != nil || got != first {
		t.Fatalf("invite lookup = %v, %v", got, err)
	}
	first.Name = "Friday group"
	if got, err := db.UpdateTable(ctx, first.ID, first.Name); err != nil || got != first {
		t.Fatalf("rename = %v, %v", got, err)
	}
	tracks, err := db.GetTableTracks(ctx, first.ID)
	if err != nil || tracks == nil || len(tracks) != 0 {
		t.Fatalf("empty tracks = %v, %v", tracks, err)
	}
	types, err := db.GetTrackTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	track := server.Track{ID: uuid.New(), Name: "Forest", TypeID: types[0].ID,
		CreatedAt: time.Now().UTC().Truncate(time.Second), Path: "/uploads/forest"}
	assertCount := func(want int64) {
		t.Helper()
		if got, err := db.CountTrackTables(ctx, track.ID); err != nil || got != want {
			t.Fatalf("table count = %d, %v; want %d", got, err, want)
		}
	}
	if _, err := db.CountTrackTables(ctx, uuid.New()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing track count = %v", err)
	}
	unused := track
	unused.ID = uuid.New()
	if err := db.SaveTrack(ctx, &unused); err != nil {
		t.Fatal(err)
	}
	if count, err := db.CountTrackTables(ctx, unused.ID); err != nil || count != 0 {
		t.Fatalf("unused track count = %d, %v", count, err)
	}
	if err := db.SaveTrackToTable(ctx, &track, first.ID); err != nil {
		t.Fatal(err)
	}
	assertCount(1)
	for range 2 {
		if err := db.AddTableTrack(ctx, second.ID, track.ID); err != nil {
			t.Fatal(err)
		}
	}
	assertCount(2)
	if deleted, err := db.DeleteTrackIfTableCount(ctx, track.ID, 1); err != nil || deleted {
		t.Fatalf("stale conditional delete = %v, %v", deleted, err)
	}
	assertCount(2)
	for _, id := range []uuid.UUID{first.ID, second.ID} {
		tracks, err := db.GetTableTracks(ctx, id)
		if err != nil || len(tracks) != 1 || !reflect.DeepEqual(tracks[0], track) {
			t.Fatalf("associated tracks = %v, %v", tracks, err)
		}
		if has, err := db.HasTableTrack(ctx, id, track.ID); err != nil || !has {
			t.Fatalf("membership = %v, %v", has, err)
		}
	}
	if err := db.RemoveTableTrack(ctx, first.ID, track.ID); err != nil {
		t.Fatal(err)
	}
	assertCount(1)
	if has, err := db.HasTableTrack(ctx, first.ID, track.ID); err != nil || has {
		t.Fatalf("removed membership = %v, %v", has, err)
	}
	if _, err := db.GetTrackByID(ctx, track.ID); err != nil {
		t.Fatalf("removing association deleted library track: %v", err)
	}
	missing := uuid.New()
	if err := db.AddTableTrack(ctx, missing, track.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing table association = %v", err)
	}
	if err := db.AddTableTrack(ctx, first.ID, missing); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing track association = %v", err)
	}
	rollback := track
	rollback.ID = uuid.New()
	if err := db.SaveTrackToTable(ctx, &rollback, missing); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("failed association = %v", err)
	}
	if _, err := db.GetTrackByID(ctx, rollback.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("upload was not rolled back: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if got, err := db.GetTable(ctx, first.ID); err != nil || got != first {
		t.Fatalf("persisted table = %v, %v", got, err)
	}
	if has, err := db.HasTableTrack(ctx, second.ID, track.ID); err != nil || !has {
		t.Fatalf("persisted association = %v, %v", has, err)
	}
	if err := db.DeleteTable(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	assertCount(0)
	if has, err := db.HasTableTrack(ctx, second.ID, track.ID); err != nil || has {
		t.Fatalf("table deletion did not clean associations = %v, %v", has, err)
	}
	if _, err := db.GetTableByInvite(ctx, second.InviteToken); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted invite = %v", err)
	}
	if _, err := db.GetTrackByID(ctx, track.ID); err != nil {
		t.Fatalf("table deletion deleted library track: %v", err)
	}
	if err := db.AddTableTrack(ctx, first.ID, track.ID); err != nil {
		t.Fatal(err)
	}
	if deleted, err := db.DeleteTrackIfTableCount(ctx, track.ID, 1); err != nil || !deleted {
		t.Fatalf("confirmed conditional delete = %v, %v", deleted, err)
	}
	if _, err := db.CountTrackTables(ctx, track.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted track count = %v", err)
	}
	if deleted, err := db.DeleteTrackIfTableCount(ctx, track.ID, 0); err != nil || deleted {
		t.Fatalf("missing conditional delete = %v, %v", deleted, err)
	}
	if has, err := db.HasTableTrack(ctx, first.ID, track.ID); err != nil || has {
		t.Fatalf("library deletion did not clean associations = %v, %v", has, err)
	}
	if err := db.DeleteTable(ctx, second.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing table delete = %v", err)
	}
	if _, err := db.UpdateTable(ctx, second.ID, "Missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing table rename = %v", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := db.CountTrackTables(canceled, unused.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled count = %v", err)
	}
}

func TestExistingLibraryMigratesToDefaultTable(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m, err := NewMigration(os.DirFS("../../sql/migrations"), db)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	types, err := db.GetTrackTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	track := server.Track{ID: uuid.New(), Name: "Legacy", TypeID: types[0].ID,
		CreatedAt: time.Now().UTC().Truncate(time.Second), Path: "/uploads/legacy"}
	if err := db.SaveTrack(ctx, &track); err != nil {
		t.Fatal(err)
	}
	if err := m.Steps(-4); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	tracks, err := db.GetTableTracks(ctx, server.DefaultTableID)
	if err != nil || len(tracks) != 1 || !reflect.DeepEqual(tracks[0], track) {
		t.Fatalf("migrated tracks = %v, %v", tracks, err)
	}
	settings, err := db.GetTableSettings(ctx, server.DefaultTableID)
	if err != nil || settings.MasterVolume != 100 || len(settings.TrackOrder) != 0 {
		t.Fatalf("migrated settings = %+v, %v", settings, err)
	}
	if err := m.Steps(-4); err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetTrackByID(ctx, track.ID); err != nil {
		t.Fatalf("table migration rollback lost library: %v", err)
	}
	if has, err := db.HasTableTrack(ctx, server.DefaultTableID, track.ID); err != nil || !has {
		t.Fatalf("reapplied membership = %v, %v", has, err)
	}
}
