package sqlitedatastore

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
)

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
	if err := db.SaveTrackToTable(ctx, &track, first.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.AddTableTrack(ctx, second.ID, track.ID); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []uuid.UUID{first.ID, second.ID} {
		tracks, err := db.GetTableTracks(ctx, id)
		if err != nil || len(tracks) != 1 || tracks[0] != track {
			t.Fatalf("associated tracks = %v, %v", tracks, err)
		}
		if has, err := db.HasTableTrack(ctx, id, track.ID); err != nil || !has {
			t.Fatalf("membership = %v, %v", has, err)
		}
	}
	if err := db.RemoveTableTrack(ctx, first.ID, track.ID); err != nil {
		t.Fatal(err)
	}
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
	if err := db.DeleteTrack(ctx, track.ID); err != nil {
		t.Fatal(err)
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
	if err := m.Steps(1); err != nil {
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
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	tracks, err := db.GetTableTracks(ctx, server.DefaultTableID)
	if err != nil || len(tracks) != 1 || tracks[0] != track {
		t.Fatalf("migrated tracks = %v, %v", tracks, err)
	}
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetTrackByID(ctx, track.ID); err != nil {
		t.Fatalf("table migration rollback lost library: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	if has, err := db.HasTableTrack(ctx, server.DefaultTableID, track.ID); err != nil || !has {
		t.Fatalf("reapplied membership = %v, %v", has, err)
	}
}
