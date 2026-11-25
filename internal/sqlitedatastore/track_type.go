package sqlitedatastore

import (
	"context"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (db *SQLiteDatastore) GetTrackTypes(ctx context.Context) ([]domain.TrackType, error) {
	dbTrackTypes, err := sqlitedb.New(db.DB).GetTrackTypes(ctx)
	if err != nil {
		return nil, translateError(err)
	}

	result := make([]domain.TrackType, 0, len(dbTrackTypes))
	for _, dbTrackType := range dbTrackTypes {
		trackType, parseErr := convertDBTrackType(dbTrackType)
		if parseErr != nil {
			return nil, parseErr
		}
		result = append(result, trackType)
	}
	return result, nil
}

func (db *SQLiteDatastore) GetTrackTypeByID(ctx context.Context, id uuid.UUID) (domain.TrackType, error) {
	dbTrackType, err := sqlitedb.New(db.DB).GetTrackTypeByID(ctx, id[:])
	if err != nil {
		return domain.TrackType{}, translateError(err)
	}

	return convertDBTrackType(dbTrackType)
}

func convertDBTrackType(dbTrackType sqlitedb.TrackType) (domain.TrackType, error) {
	id, err := uuid.FromBytes(dbTrackType.ID)
	if err != nil {
		return domain.TrackType{}, err
	}

	return domain.TrackType{
		ID:                    id,
		Name:                  dbTrackType.Name,
		Color:                 dbTrackType.Color,
		IsRepeating:           dbTrackType.IsRepeating,
		AllowSimultaneousPlay: dbTrackType.AllowSimultaneousPlay,
	}, nil
}
