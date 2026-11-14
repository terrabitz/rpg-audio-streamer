package sqlitedatastore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func (db *SQLiteDatastore) GetTags(ctx context.Context) ([]server.Tag, error) {
	rows, err := sqlitedb.New(db.DB).GetTags(ctx)
	if err != nil {
		return nil, err
	}
	tags := make([]server.Tag, 0, len(rows))
	for _, row := range rows {
		tag, err := convertDBTag(row)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func (db *SQLiteDatastore) CreateTag(ctx context.Context, tag server.Tag) (server.Tag, error) {
	row, err := sqlitedb.New(db.DB).CreateTag(ctx, sqlitedb.CreateTagParams{
		ID: tag.ID[:], Name: tag.Name, Color: tag.Color,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return server.Tag{}, server.ErrTagConflict
	}
	if err != nil {
		return server.Tag{}, err
	}
	return convertDBTag(row)
}

func (db *SQLiteDatastore) UpdateTag(ctx context.Context, tag server.Tag) (server.Tag, error) {
	queries := sqlitedb.New(db.DB)
	row, err := queries.UpdateTag(ctx, sqlitedb.UpdateTagParams{
		ID: tag.ID[:], Name: tag.Name, Color: tag.Color,
	})
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return server.Tag{}, server.ErrTagConflict
	}
	if err != nil {
		return server.Tag{}, err
	}
	return convertDBTag(row)
}

func (db *SQLiteDatastore) DeleteTag(ctx context.Context, id uuid.UUID) error {
	count, err := sqlitedb.New(db.DB).DeleteTag(ctx, id[:])
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func attachTrackTags(ctx context.Context, queries *sqlitedb.Queries, tracks []server.Track) error {
	rows, err := queries.GetAllTrackTags(ctx)
	if err != nil {
		return err
	}
	byTrack := make(map[uuid.UUID][]server.Tag)
	for _, row := range rows {
		trackID, err := uuid.FromBytes(row.TrackID)
		if err != nil {
			return err
		}
		tag, err := convertDBTag(sqlitedb.Tag{ID: row.ID, Name: row.Name, Color: row.Color})
		if err != nil {
			return err
		}
		byTrack[trackID] = append(byTrack[trackID], tag)
	}
	for i := range tracks {
		tracks[i].Tags = byTrack[tracks[i].ID]
	}
	return nil
}

func getTrackTags(ctx context.Context, queries *sqlitedb.Queries, trackID uuid.UUID) ([]server.Tag, error) {
	rows, err := queries.GetTrackTags(ctx, trackID[:])
	if err != nil {
		return nil, err
	}
	var tags []server.Tag
	for _, row := range rows {
		tag, err := convertDBTag(row)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func replaceTrackTags(ctx context.Context, queries *sqlitedb.Queries, trackID uuid.UUID, ids []uuid.UUID) error {
	if err := queries.ClearTrackTags(ctx, trackID[:]); err != nil {
		return err
	}
	seen := make(map[uuid.UUID]bool)
	for _, id := range ids {
		if id == uuid.Nil || seen[id] {
			return server.ErrInvalidTag
		}
		seen[id] = true
		count, err := queries.AddTrackTag(ctx, sqlitedb.AddTrackTagParams{TrackID: trackID[:], TagID: id[:]})
		if err != nil {
			return err
		}
		if count != 1 {
			return server.ErrInvalidTag
		}
	}
	return nil
}

func convertDBTag(row sqlitedb.Tag) (server.Tag, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return server.Tag{}, err
	}
	return server.Tag{ID: id, Name: row.Name, Color: row.Color}, nil
}
