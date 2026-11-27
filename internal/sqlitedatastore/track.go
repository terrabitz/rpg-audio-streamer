package sqlitedatastore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (db *SQLiteDatastore) SaveTrack(ctx context.Context, track *server.Track) error {
	return saveTrack(ctx, sqlitedb.New(db.DB), track)
}

func saveTrack(ctx context.Context, queries *sqlitedb.Queries, track *server.Track) error {
	if err := track.Playback.Validate(); err != nil {
		return err
	}
	if track.Volume < 0 || track.Volume > 100 {
		return fmt.Errorf("invalid track volume")
	}
	if track.Status == "" {
		track.Status = "ready"
	}
	dbTrack := sqlitedb.SaveTrackParams{
		ID:              track.ID[:],
		CreatedAt:       track.CreatedAt.Format(time.RFC3339),
		Name:            track.Name,
		Path:            track.Path,
		TypeID:          track.TypeID[:],
		StartTime:       track.Playback.StartTime,
		EndTime:         nullableTime(track.Playback.EndTime),
		LoopBackTime:    nullableTime(track.Playback.LoopBackTime),
		Volume:          track.Volume,
		Status:          track.Status,
		ProcessingError: track.ProcessingError,
	}

	if err := queries.SaveTrack(ctx, dbTrack); err != nil {
		return fmt.Errorf("couldn't save track to SQLite: %w", err)
	}

	return nil
}

func (db *SQLiteDatastore) SetTrackProcessing(ctx context.Context, id uuid.UUID, status, message string) error {
	affected, err := sqlitedb.New(db.DB).SetTrackProcessing(ctx, sqlitedb.SetTrackProcessingParams{
		ID: id[:], Status: status, ProcessingError: message,
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (db *SQLiteDatastore) GetTracks(ctx context.Context) ([]server.Track, error) {
	queries := sqlitedb.New(db.DB)
	dbTracks, err := queries.GetTracks(ctx)
	if err != nil {
		return nil, err
	}
	return convertDBTracksWithTags(ctx, queries, dbTracks)
}

func (db *SQLiteDatastore) GetTrackByID(ctx context.Context, trackID uuid.UUID) (server.Track, error) {
	queries := sqlitedb.New(db.DB)
	dbTrack, err := queries.GetTrackByID(ctx, trackID[:])
	if err != nil {
		return server.Track{}, fmt.Errorf("couldn't get track by ID: %w", err)
	}

	return convertDBTrackWithTags(ctx, queries, dbTrack)
}

func (db *SQLiteDatastore) DeleteTrack(ctx context.Context, trackID uuid.UUID) error {
	return sqlitedb.New(db.DB).DeleteTrackByID(ctx, trackID[:])
}

func (db *SQLiteDatastore) DeleteTrackIfTableCount(ctx context.Context, trackID uuid.UUID, expectedCount int64) (bool, error) {
	affected, err := sqlitedb.New(db.DB).DeleteTrackByIDIfTableCount(ctx, sqlitedb.DeleteTrackByIDIfTableCountParams{
		ID: trackID[:], ExpectedTableCount: expectedCount,
	})
	return affected == 1, err
}

func (db *SQLiteDatastore) UpdateTrack(ctx context.Context, trackID uuid.UUID, update server.UpdateTrackRequest) (server.Track, error) {
	if update.Volume != nil && (*update.Volume < 0 || *update.Volume > 100) {
		return server.Track{}, fmt.Errorf("invalid track volume")
	}
	params := sqlitedb.UpdateTrackParams{
		ID: trackID[:],
	}

	if update.Name != nil {
		params.Name.String = *update.Name
		params.Name.Valid = true
	}

	if update.TypeID != nil {
		params.TypeID = update.TypeID[:]
	}
	if update.Volume != nil {
		params.SetVolume = 1
		params.Volume = *update.Volume
	}
	if update.Playback != nil {
		if err := update.Playback.Validate(); err != nil {
			return server.Track{}, err
		}
		params.SetPlayback = 1
		params.StartTime = update.Playback.StartTime
		params.EndTime = nullableTime(update.Playback.EndTime)
		params.LoopBackTime = nullableTime(update.Playback.LoopBackTime)
	}

	queries := sqlitedb.New(db.DB)
	dbTrack, err := queries.UpdateTrack(ctx, params)
	if err != nil {
		return server.Track{}, fmt.Errorf("couldn't update track in SQLite: %w", err)
	}

	return convertDBTrackWithTags(ctx, queries, dbTrack)
}

func convertDBTracksWithTags(ctx context.Context, queries *sqlitedb.Queries, dbTracks []sqlitedb.Track) ([]server.Track, error) {
	tagIDs, err := tagIDsByTrack(ctx, queries)
	if err != nil {
		return nil, err
	}
	tracks := make([]server.Track, 0, len(dbTracks))
	for _, dbTrack := range dbTracks {
		track, err := convertDBTrack(dbTrack)
		if err != nil {
			return nil, err
		}
		track.TagIDs = tagIDs[track.ID]
		tracks = append(tracks, track)
	}
	return tracks, nil
}

func convertDBTrackWithTags(ctx context.Context, queries *sqlitedb.Queries, dbTrack sqlitedb.Track) (server.Track, error) {
	track, err := convertDBTrack(dbTrack)
	if err != nil {
		return server.Track{}, err
	}
	track.TagIDs, err = trackTagIDs(ctx, queries, track.ID)
	return track, err
}

func convertDBTrack(dbTrack sqlitedb.Track) (server.Track, error) {
	id, err := uuid.FromBytes(dbTrack.ID)
	if err != nil {
		return server.Track{}, fmt.Errorf("invalid ID: %w", err)
	}

	createdAt, err := time.Parse(time.RFC3339, dbTrack.CreatedAt)
	if err != nil {
		return server.Track{}, fmt.Errorf("invalid CreatedAt: %w", err)
	}

	typeID, err := uuid.FromBytes(dbTrack.TypeID)
	if err != nil {
		return server.Track{}, fmt.Errorf("error converting track type ID to UUID: %w", err)
	}

	return server.Track{
		ID:              id,
		CreatedAt:       createdAt,
		Name:            dbTrack.Name,
		Path:            dbTrack.Path,
		TypeID:          typeID,
		Volume:          dbTrack.Volume,
		Status:          dbTrack.Status,
		ProcessingError: dbTrack.ProcessingError,
		Playback: server.PlaybackSettings{
			StartTime:    dbTrack.StartTime,
			EndTime:      optionalTime(dbTrack.EndTime),
			LoopBackTime: optionalTime(dbTrack.LoopBackTime),
		},
	}, nil
}

func nullableTime(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}

func optionalTime(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}
