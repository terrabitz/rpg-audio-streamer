// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: track.sql

package sqlitedb

import (
	"context"
)

const deleteTrackByID = `-- name: DeleteTrackByID :exec
delete from tracks where id = ?1
`

func (q *Queries) DeleteTrackByID(ctx context.Context, id []byte) error {
	_, err := q.db.ExecContext(ctx, deleteTrackByID, id)
	return err
}

const getTracks = `-- name: GetTracks :many
select id, created_at, name, path, type from tracks
`

func (q *Queries) GetTracks(ctx context.Context) ([]Track, error) {
	rows, err := q.db.QueryContext(ctx, getTracks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Track
	for rows.Next() {
		var i Track
		if err := rows.Scan(
			&i.ID,
			&i.CreatedAt,
			&i.Name,
			&i.Path,
			&i.Type,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
