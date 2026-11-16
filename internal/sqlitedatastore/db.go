package sqlitedatastore

import (
	"database/sql"
	"fmt"
)

type SQLiteDatastore struct {
	*sql.DB
}

func New(path string) (*SQLiteDatastore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("couldn't open sqlite DB at '%s': %v", path, err)
	}
	// Serialize access so settings writes cannot lock out table and stream reads.
	db.SetMaxOpenConns(1)

	return &SQLiteDatastore{DB: db}, nil
}
