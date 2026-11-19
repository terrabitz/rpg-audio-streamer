package sqlitedatastore

import (
	"context"
	"database/sql"
	"errors"

	"github.com/terrabitz/rpg-audio-streamer/internal/auth"
	"github.com/terrabitz/rpg-audio-streamer/internal/sqlitedatastore/sqlitedb"
)

func (s *SQLiteDatastore) LoadAdmin(ctx context.Context) (auth.Admin, error) {
	row, err := sqlitedb.New(s.DB).GetAdmin(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Admin{}, nil
	}
	return auth.Admin{
		Username:     row.Username,
		PasswordHash: row.PasswordHash,
	}, err
}

func (s *SQLiteDatastore) CreateAdmin(ctx context.Context, admin auth.Admin) (bool, error) {
	count, err := sqlitedb.New(s.DB).CreateAdmin(ctx, sqlitedb.CreateAdminParams{
		Username:     admin.Username,
		PasswordHash: admin.PasswordHash,
	})
	return count == 1, err
}

func (s *SQLiteDatastore) UpdateAdminPassword(ctx context.Context, currentHash, newHash string) (bool, error) {
	count, err := sqlitedb.New(s.DB).UpdateAdminPassword(ctx, sqlitedb.UpdateAdminPasswordParams{
		CurrentHash: currentHash,
		NewHash:     newHash,
	})
	return count == 1, err
}

func (s *SQLiteDatastore) LoadSigningSecret(ctx context.Context) (string, error) {
	return sqlitedb.New(s.DB).GetSigningSecret(ctx)
}

func (s *SQLiteDatastore) InitializeSigningSecret(ctx context.Context, secret string) (string, error) {
	return sqlitedb.New(s.DB).InitializeSigningSecret(ctx, secret)
}
