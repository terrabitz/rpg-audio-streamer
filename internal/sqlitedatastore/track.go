package sqlitedatastore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (db *SQLiteDatastore) SaveTrack(ctx context.Context, track *server.Track) error {
	dbTrack := sqlitedb.SaveTrackParams{
		ID:        track.ID[:],
		CreatedAt: track.CreatedAt.Format(time.RFC3339),
		Name:      track.Name,
		Path:      track.Path,
		TypeID:    track.TypeID[:],
	}

	if err := sqlitedb.New(db.DB).SaveTrack(ctx, dbTrack); err != nil {
		return fmt.Errorf("couldn't save track to SQLite: %w", err)
	}

	return nil
}

func (db *SQLiteDatastore) GetTracks(ctx context.Context) ([]server.Track, error) {
	dbTracks, err := sqlitedb.New(db.DB).GetTracks(ctx)
	if err != nil {
		return nil, err
	}

	var result []server.Track
	for _, dbTrack := range dbTracks {
		track, parseErr := convertDBTrack(dbTrack)
		if parseErr != nil {
			return nil, parseErr
		}
		result = append(result, track)
	}
	return result, nil
}

func (db *SQLiteDatastore) GetTrackByID(ctx context.Context, trackID uuid.UUID) (server.Track, error) {
	dbTrack, err := sqlitedb.New(db.DB).GetTrackByID(ctx, trackID[:])
	if err != nil {
		return server.Track{}, fmt.Errorf("couldn't get track by ID: %w", err)
	}

	return convertDBTrack(dbTrack)
}

func (db *SQLiteDatastore) DeleteTrack(ctx context.Context, trackID uuid.UUID) error {
	return sqlitedb.New(db.DB).DeleteTrackByID(ctx, trackID[:])
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
		ID:        id,
		CreatedAt: createdAt,
		Name:      dbTrack.Name,
		Path:      dbTrack.Path,
		TypeID:    typeID,
	}, nil
}
