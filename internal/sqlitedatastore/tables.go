package sqlitedatastore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/terrabitz/rpg-audio-streamer/internal/server"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (db *SQLiteDatastore) GetTables(ctx context.Context) ([]server.Table, error) {
	dbTables, err := sqlitedb.New(db.DB).GetTables(ctx)
	if err != nil {
		return nil, err
	}

	var result []server.Table
	for _, dbTable := range dbTables {
		table, parseErr := convertDBTable(dbTable)
		if parseErr != nil {
			return nil, parseErr
		}

		result = append(result, table)
	}
	return result, nil
}

func convertDBTable(dbTable sqlitedb.Table) (server.Table, error) {
	id, err := uuid.FromBytes(dbTable.ID)
	if err != nil {
		return server.Table{}, err
	}

	return server.Table{
		ID:         id,
		Name:       dbTable.Name,
		InviteCode: dbTable.InviteCode,
		CreatedAt:  time.Unix(dbTable.CreatedAt, 0),
	}, nil
}
