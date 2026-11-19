package sqlitedatastore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/musicprocessing"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (db *SQLiteDatastore) GetTrackWaveform(ctx context.Context, id uuid.UUID) (musicprocessing.Waveform, error) {
	data, err := sqlitedb.New(db.DB).GetTrackWaveform(ctx, id[:])
	if err != nil {
		return musicprocessing.Waveform{}, err
	}
	var waveform musicprocessing.Waveform
	if err := json.Unmarshal([]byte(data), &waveform); err != nil {
		return musicprocessing.Waveform{}, fmt.Errorf("decode stored waveform: %w", err)
	}
	return waveform, nil
}

func (db *SQLiteDatastore) SaveTrackWaveform(ctx context.Context, id uuid.UUID, waveform musicprocessing.Waveform) error {
	data, err := json.Marshal(waveform)
	if err != nil {
		return fmt.Errorf("encode waveform: %w", err)
	}
	affected, err := sqlitedb.New(db.DB).SaveTrackWaveform(ctx, sqlitedb.SaveTrackWaveformParams{
		ID:   id[:],
		Data: string(data),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}
