package sqlitedatastore

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/discord"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (db *SQLiteDatastore) GetTableDiscord(ctx context.Context, id uuid.UUID) (*discord.Destination, error) {
	value, err := sqlitedb.New(db.DB).GetTableDiscord(ctx, id[:])
	if err != nil || value == "" {
		return nil, translateError(err)
	}
	var target discord.Destination
	if err := json.Unmarshal([]byte(value), &target); err != nil {
		return nil, err
	}
	return &target, nil
}

func (db *SQLiteDatastore) SaveTableDiscord(ctx context.Context, id uuid.UUID, target discord.Destination) error {
	value, err := json.Marshal(target)
	if err != nil {
		return err
	}
	count, err := sqlitedb.New(db.DB).SaveTableDiscord(ctx, sqlitedb.SaveTableDiscordParams{
		ID: id[:], DiscordDestination: string(value),
	})
	if err != nil {
		return translateError(err)
	}
	if count == 0 {
		return errNotFound
	}
	return nil
}
func (db *SQLiteDatastore) GetTables(ctx context.Context) ([]domain.Table, error) {
	rows, err := sqlitedb.New(db.DB).GetTables(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	tables := make([]domain.Table, 0, len(rows))
	for _, row := range rows {
		table, err := convertDBTable(row)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func (db *SQLiteDatastore) GetTable(ctx context.Context, id uuid.UUID) (domain.Table, error) {
	row, err := sqlitedb.New(db.DB).GetTable(ctx, id[:])
	if err != nil {
		return domain.Table{}, translateError(err)
	}
	return convertDBTable(row)
}

func (db *SQLiteDatastore) GetTableByExternalID(ctx context.Context, externalID string) (domain.Table, error) {
	row, err := sqlitedb.New(db.DB).GetTableByExternalID(ctx, externalID)
	if err != nil {
		return domain.Table{}, translateError(err)
	}
	return convertDBTable(row)
}

func (db *SQLiteDatastore) CreateTable(ctx context.Context, table domain.Table) (domain.Table, error) {
	externalID, err := generateExternalID()
	if err != nil {
		return domain.Table{}, err
	}
	row, err := sqlitedb.New(db.DB).CreateTable(ctx, sqlitedb.CreateTableParams{
		ID: table.ID[:], Name: table.Name, ExternalID: externalID,
	})
	if err != nil {
		return domain.Table{}, translateError(err)
	}
	return convertDBTable(row)
}

func (db *SQLiteDatastore) RotateTableExternalID(ctx context.Context, id uuid.UUID) (domain.Table, error) {
	externalID, err := generateExternalID()
	if err != nil {
		return domain.Table{}, err
	}
	row, err := sqlitedb.New(db.DB).RotateTableExternalID(ctx, sqlitedb.RotateTableExternalIDParams{
		ExternalID: externalID,
		ID:         id[:],
	})
	if err != nil {
		return domain.Table{}, translateError(err)
	}
	return convertDBTable(row)
}

func (db *SQLiteDatastore) UpdateTable(ctx context.Context, id uuid.UUID, name string) (domain.Table, error) {
	row, err := sqlitedb.New(db.DB).UpdateTable(ctx, sqlitedb.UpdateTableParams{ID: id[:], Name: name})
	if err != nil {
		return domain.Table{}, translateError(err)
	}
	return convertDBTable(row)
}

func (db *SQLiteDatastore) DeleteTable(ctx context.Context, id uuid.UUID) error {
	count, err := sqlitedb.New(db.DB).DeleteTable(ctx, id[:])
	if err != nil {
		return translateError(err)
	}
	if count == 0 {
		return errNotFound
	}
	return nil
}

func (db *SQLiteDatastore) GetTableSettings(ctx context.Context, id uuid.UUID) (domain.TableSettings, error) {
	value, err := sqlitedb.New(db.DB).GetTableSettings(ctx, id[:])
	if err != nil {
		return domain.TableSettings{}, translateError(err)
	}
	var settings domain.TableSettings
	err = json.Unmarshal([]byte(value), &settings)
	return settings, err
}

func (db *SQLiteDatastore) UpdateTableSettings(ctx context.Context, id uuid.UUID, patch domain.TableSettingsPatch) (domain.TableSettings, error) {
	value, err := json.Marshal(patch)
	if err != nil {
		return domain.TableSettings{}, err
	}
	updated, err := sqlitedb.New(db.DB).UpdateTableSettings(ctx, sqlitedb.UpdateTableSettingsParams{
		ID: id[:], Patch: string(value),
	})
	if err != nil {
		return domain.TableSettings{}, translateError(err)
	}
	var settings domain.TableSettings
	err = json.Unmarshal([]byte(updated), &settings)
	return settings, err
}

func (db *SQLiteDatastore) GetTableTracks(ctx context.Context, id uuid.UUID) ([]domain.Track, error) {
	rows, err := sqlitedb.New(db.DB).GetTableTracks(ctx, id[:])
	if err != nil {
		return nil, translateError(err)
	}
	tracks := make([]domain.Track, 0, len(rows))
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
	return translateError(err)
}

func (db *SQLiteDatastore) RemoveTableTrack(ctx context.Context, tableID, trackID uuid.UUID) error {
	err := sqlitedb.New(db.DB).RemoveTableTrack(ctx, sqlitedb.RemoveTableTrackParams{TableID: tableID[:], TrackID: trackID[:]})
	return translateError(err)
}

func (db *SQLiteDatastore) HasTableTrack(ctx context.Context, tableID, trackID uuid.UUID) (bool, error) {
	has, err := sqlitedb.New(db.DB).HasTableTrack(ctx, sqlitedb.HasTableTrackParams{TableID: tableID[:], TrackID: trackID[:]})
	return has != 0, err
}

func (db *SQLiteDatastore) CountTrackTables(ctx context.Context, trackID uuid.UUID) (int64, error) {
	count, err := sqlitedb.New(db.DB).CountTrackTables(ctx, trackID[:])
	return count, translateError(err)
}

func (db *SQLiteDatastore) SaveTrackToTable(ctx context.Context, track *domain.Track, tableID uuid.UUID) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return translateError(err)
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

func convertDBTable(row sqlitedb.GameTable) (domain.Table, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return domain.Table{}, err
	}
	return domain.Table{ID: id, ExternalID: row.ExternalID, Name: row.Name}, nil
}

func generateExternalID() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
