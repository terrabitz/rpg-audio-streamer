package sqlitedatastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (db *SQLiteDatastore) GetTags(ctx context.Context) ([]domain.Tag, error) {
	rows, err := sqlitedb.New(db.DB).GetTags(ctx)
	if err != nil {
		return nil, err
	}
	tags := make([]domain.Tag, 0, len(rows))
	for _, row := range rows {
		tag, err := convertDBTag(row)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func (db *SQLiteDatastore) CreateTag(ctx context.Context, tag domain.Tag) (domain.Tag, error) {
	row, err := sqlitedb.New(db.DB).CreateTag(ctx, sqlitedb.CreateTagParams{
		ID:    tag.ID[:],
		Name:  string(tag.Name),
		Color: string(tag.Color),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Tag{}, domain.ErrTagNameTaken
	}
	if err != nil {
		return domain.Tag{}, err
	}
	return convertDBTag(row)
}

func (db *SQLiteDatastore) UpdateTag(ctx context.Context, tag domain.Tag) (domain.Tag, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Tag{}, err
	}
	defer tx.Rollback()
	queries := sqlitedb.New(tx)
	row, err := queries.UpdateTag(ctx, sqlitedb.UpdateTagParams{
		ID:    tag.ID[:],
		Name:  string(tag.Name),
		Color: string(tag.Color),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Tag{}, missingTagError(ctx, queries, tag.ID)
	}
	if err != nil {
		return domain.Tag{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.Tag{}, err
	}
	return convertDBTag(row)
}

func (db *SQLiteDatastore) DeleteUnusedTags(ctx context.Context, tagIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(tagIDs) == 0 {
		return nil, nil
	}
	ids := make([][]byte, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		ids = append(ids, tagID[:])
	}
	rows, err := sqlitedb.New(db.DB).DeleteUnusedTags(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("couldn't delete unused tags: %w", err)
	}
	return convertDBIDs(rows)
}

func editTrackTags(ctx context.Context, queries *sqlitedb.Queries, trackID uuid.UUID, edit domain.TrackEdit) error {
	for _, tagID := range edit.AddTagIDs {
		affected, err := queries.AddTrackTag(ctx, sqlitedb.AddTrackTagParams{
			TrackID: trackID[:],
			TagID:   tagID[:],
		})
		if err != nil {
			return err
		}
		if affected == 0 {
			return sql.ErrNoRows
		}
	}
	for _, tagID := range edit.RemoveTagIDs {
		if err := queries.RemoveTrackTag(ctx, sqlitedb.RemoveTrackTagParams{
			TrackID: trackID[:],
			TagID:   tagID[:],
		}); err != nil {
			return err
		}
	}
	return nil
}

// UPDATE OR IGNORE returns no row for both a missing tag and a name collision.
func missingTagError(ctx context.Context, queries *sqlitedb.Queries, id uuid.UUID) error {
	exists, err := queries.TagExists(ctx, id[:])
	if err != nil {
		return err
	}
	if exists != 0 {
		return domain.ErrTagNameTaken
	}
	return sql.ErrNoRows
}

func trackTagIDs(ctx context.Context, queries *sqlitedb.Queries, trackID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := queries.GetTrackTagIDs(ctx, trackID[:])
	if err != nil {
		return nil, fmt.Errorf("couldn't get track tags: %w", err)
	}
	return convertDBIDs(rows)
}

func tagIDsByTrack(ctx context.Context, queries *sqlitedb.Queries) (map[uuid.UUID][]uuid.UUID, error) {
	rows, err := queries.GetTrackTagPairs(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get track tags: %w", err)
	}
	result := make(map[uuid.UUID][]uuid.UUID)
	for _, row := range rows {
		trackID, err := uuid.FromBytes(row.TrackID)
		if err != nil {
			return nil, err
		}
		tagID, err := uuid.FromBytes(row.TagID)
		if err != nil {
			return nil, err
		}
		result[trackID] = append(result[trackID], tagID)
	}
	return result, nil
}

func convertDBIDs(rows [][]byte) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for _, row := range rows {
		id, err := uuid.FromBytes(row)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func convertDBTag(row sqlitedb.Tag) (domain.Tag, error) {
	id, err := uuid.FromBytes(row.ID)
	if err != nil {
		return domain.Tag{}, err
	}
	return domain.Tag{
		ID:    id,
		Name:  domain.TagName(row.Name),
		Color: domain.TagColor(row.Color),
	}, nil
}
