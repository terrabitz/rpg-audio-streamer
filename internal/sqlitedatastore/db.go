package sqlitedatastore

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
)

type SQLiteDatastore struct {
	*sql.DB
}

func New(path string) (*SQLiteDatastore, error) {
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("open private SQLite file: %w", err)
		}
		if err := file.Chmod(0600); err != nil {
			file.Close()
			return nil, fmt.Errorf("restrict SQLite file permissions: %w", err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close SQLite file: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("couldn't open sqlite DB at '%s': %v", path, err)
	}
	// Serialize access so concurrent reads and background writes cannot contend for SQLite locks.
	db.SetMaxOpenConns(1)

	return &SQLiteDatastore{DB: db}, nil
}
