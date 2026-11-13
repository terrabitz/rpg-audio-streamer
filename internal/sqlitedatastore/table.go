package sqlitedatastore

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (db *SQLiteDatastore) GetTables(ctx context.Context) ([]server.Table, error) {
	rows, err := sqlitedb.New(db.DB).GetTables(ctx)
	if err != nil {
		return nil, err
	}
	tables := make([]server.Table, 0, len(rows))
	for _, row := range rows {
		table, err := convertDBTable(row)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func (db *SQLiteDatastore) GetTable(ctx context.Context, id uuid.UUID) (server.Table, error) {
	row, err := sqlitedb.New(db.DB).GetTable(ctx, id[:])
	if err != nil {
		return server.Table{}, err
	}
	return convertDBTable(row)
}

func (db *SQLiteDatastore) GetTableByInvite(ctx context.Context, invite string) (server.Table, error) {
	row, err := sqlitedb.New(db.DB).GetTableByInvite(ctx, invite)
	if err != nil {
		return server.Table{}, err
	}
	return convertDBTable(row)
}

func (db *SQLiteDatastore) CreateTable(ctx context.Context, table server.Table) (server.Table, error) {
	row, err := sqlitedb.New(db.DB).CreateTable(ctx, sqlitedb.CreateTableParams{
		ID: table.ID[:], Name: table.Name, InviteToken: table.InviteToken,
	})
	if err != nil {
		return server.Table{}, err
	}
	return convertDBTable(row)
}

func (db *SQLiteDatastore) UpdateTable(ctx context.Context, id uuid.UUID, name string) (server.Table, error) {
	row, err := sqlitedb.New(db.DB).UpdateTable(ctx, sqlitedb.UpdateTableParams{ID: id[:], Name: name})
	if err != nil {
		return server.Table{}, err
	}
	return convertDBTable(row)
}

func (db *SQLiteDatastore) DeleteTable(ctx context.Context, id uuid.UUID) error {
	count, err := sqlitedb.New(db.DB).DeleteTable(ctx, id[:])
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (db *SQLiteDatastore) GetTableSettings(ctx context.Context, id uuid.UUID) (server.TableSettings, error) {
	value, err := sqlitedb.New(db.DB).GetTableSettings(ctx, id[:])
	if err != nil {
		return server.TableSettings{}, err
	}
	var settings server.TableSettings
	err = json.Unmarshal([]byte(value), &settings)
	return settings, err
}

func (db *SQLiteDatastore) UpdateTableSettings(ctx context.Context, id uuid.UUID, patch server.TableSettingsPatch) (server.TableSettings, error) {
	value, err := json.Marshal(patch)
	if err != nil {
		return server.TableSettings{}, err
	}
	updated, err := sqlitedb.New(db.DB).UpdateTableSettings(ctx, sqlitedb.UpdateTableSettingsParams{
		ID: id[:], Patch: string(value),
	})
	if err != nil {
		return server.TableSettings{}, err
	}
	var settings server.TableSettings
	err = json.Unmarshal([]byte(updated), &settings)
	return settings, err
}

func (db *SQLiteDatastore) GetTableTracks(ctx context.Context, id uuid.UUID) ([]server.Track, error) {
	rows, err := sqlitedb.New(db.DB).GetTableTracks(ctx, id[:])
	if err != nil {
		return nil, err
	}
	tracks := make([]server.Track, 0, len(rows))
	for _, row := range rows {
		track, err := convertDBTrack(row)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, nil
}

func (db *SQLiteDatastore) AddTableTrack(ctx context.Context, tableID, trackID uuid.UUID) error {
	_, err := sqlitedb.New(db.DB).AddTableTrack(ctx, sqlitedb.AddTableTrackParams{TableID: tableID[:], TrackID: trackID[:]})
	return err
}

func (db *SQLiteDatastore) RemoveTableTrack(ctx context.Context, tableID, trackID uuid.UUID) error {
	return sqlitedb.New(db.DB).RemoveTableTrack(ctx, sqlitedb.RemoveTableTrackParams{TableID: tableID[:], TrackID: trackID[:]})
}

func (db *SQLiteDatastore) HasTableTrack(ctx context.Context, tableID, trackID uuid.UUID) (bool, error) {
	has, err := sqlitedb.New(db.DB).HasTableTrack(ctx, sqlitedb.HasTableTrackParams{TableID: tableID[:], TrackID: trackID[:]})
	return has != 0, err
}

func (db *SQLiteDatastore) CountTrackTables(ctx context.Context, trackID uuid.UUID) (int64, error) {
	return sqlitedb.New(db.DB).CountTrackTables(ctx, trackID[:])
}

func (db *SQLiteDatastore) SaveTrackToTable(ctx context.Context, track *server.Track, tableID uuid.UUID) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	queries := sqlitedb.New(tx)
	if err := saveTrack(ctx, queries, track); err != nil {
		return err
	}
	if _, err := queries.AddTableTrack(ctx, sqlitedb.AddTableTrackParams{TableID: tableID[:], TrackID: track.ID[:]}); err != nil {
		return err
	}
	return tx.Commit()
}

func convertDBTable(row sqlitedb.GameTable) (server.Table, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return server.Table{}, err
	}
	return server.Table{ID: id, Name: row.Name, InviteToken: row.InviteToken}, nil
}
