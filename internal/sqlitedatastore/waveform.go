package sqlitedatastore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (db *SQLiteDatastore) GetTrackWaveform(ctx context.Context, id uuid.UUID) (domain.Waveform, error) {
	data, err := sqlitedb.New(db.DB).GetTrackWaveform(ctx, id[:])
	if err != nil {
		return domain.Waveform{}, translateError(err)
	}
	var waveform domain.Waveform
	if err := json.Unmarshal([]byte(data), &waveform); err != nil {
		return domain.Waveform{}, fmt.Errorf("decode stored waveform: %w", err)
	}
	return waveform, nil
}

func (db *SQLiteDatastore) SaveTrackWaveform(ctx context.Context, id uuid.UUID, waveform domain.Waveform) error {
	data, err := json.Marshal(waveform)
	if err != nil {
		return fmt.Errorf("encode waveform: %w", err)
	}
	affected, err := sqlitedb.New(db.DB).SaveTrackWaveform(ctx, sqlitedb.SaveTrackWaveformParams{
		ID:   id[:],
		Data: string(data),
	})
	if err != nil {
		return translateError(err)
	}
	if affected != 1 {
		return errNotFound
	}
	return nil
}
