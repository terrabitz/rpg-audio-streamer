package sqlitedatastore

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/terrabitz/rpg-audio-streamer/internal/domain"
)

var errNotFound = fmt.Errorf("%w: %w", domain.ErrNotFound, sql.ErrNoRows)

// translateError lets callers recognize missing rows as domain.ErrNotFound.
func translateError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %w", domain.ErrNotFound, err)
	}
	return err
}
