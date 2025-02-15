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
	migrations *migrate.Migrate
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
		migrations: m,
	}, nil
}

func (m *Migrations) Up() error {
	if err := m.migrations.Up(); err != nil {
		return fmt.Errorf("error applying migrations: %w", err)
	}

	return nil
}

func (m *Migrations) Down() error {
	if err := m.migrations.Steps(-1); err != nil {
		return fmt.Errorf("error applying downwards migrations: %w", err)
	}

	return nil
}

func (m *Migrations) Version() (version uint, dirty bool, err error) {
	version, dirty, err = m.migrations.Version()
	if err != nil {
		err = fmt.Errorf("couldn't get migration version: %w", err)
	}

	return version, dirty, err
}
