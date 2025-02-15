package sqlitedatastore

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/httpfs"
)

type Migrations struct {
	*migrate.Migrate
}

func NewMigration(migrationDir fs.FS, db *SQLiteDatastore) (*Migrations, error) {
	migrationSource, err := httpfs.New(http.FS(migrationDir), "/")
	if err != nil {
		return nil, fmt.Errorf("couldn't open migration source: %w", err)
	}

	driver, err := sqlite.WithInstance(db.DB, &sqlite.Config{})
	if err != nil {
		return nil, fmt.Errorf("couldn't open sqlite database: %w", err)
	}

	m, err := migrate.NewWithInstance("httpfs", migrationSource, "db", driver)
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize migration: %w", err)
	}

	return &Migrations{
		Migrate: m,
	}, nil
}

type MigrateVersion struct {
	Version uint
	Dirty   bool
}

func (m *Migrations) Version() (*MigrateVersion, error) {
	version, dirty, err := m.Migrate.Version()
	if err != nil {
		return nil, fmt.Errorf("couldn't get migration version: %w", err)
	}

	return &MigrateVersion{
		Version: version,
		Dirty:   dirty,
	}, nil
}
