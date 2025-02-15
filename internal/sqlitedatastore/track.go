package sqlitedatastore

import (
	"context"
	"fmt"
	"time"

	"github.com/terrabitz/rpg-audio-streamer/internal/server"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (db *SQLiteDatastore) SaveTrack(ctx context.Context, track *server.Track) error {
	dbTrack := sqlitedb.SaveTrackParams{
		ID:        track.ID[:],
		CreatedAt: track.CreatedAt.Format(time.RFC3339),
		Name:      track.Name,
		Path:      track.Path,
		Type:      track.Type,
	}

	if err := sqlitedb.New(db.DB).SaveTrack(ctx, dbTrack); err != nil {
		return fmt.Errorf("couldn't save track to SQLite: %w", err)
	}

	return nil
}
