// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: track_type.sql

package sqlitedb

import (
	"context"
)

const getTrackTypeByID = `-- name: GetTrackTypeByID :one
select id, name, color, is_repeating, allow_simultaneous_play, created_at from track_types where id = ?1
`

func (q *Queries) GetTrackTypeByID(ctx context.Context, id []byte) (TrackType, error) {
	row := q.db.QueryRowContext(ctx, getTrackTypeByID, id)
	var i TrackType
	err := row.Scan(
		&i.ID,
		&i.Name,
		&i.Color,
		&i.IsRepeating,
		&i.AllowSimultaneousPlay,
		&i.CreatedAt,
	)
	return i, err
}

const getTrackTypes = `-- name: GetTrackTypes :many
select id, name, color, is_repeating, allow_simultaneous_play, created_at from track_types
`

func (q *Queries) GetTrackTypes(ctx context.Context) ([]TrackType, error) {
	rows, err := q.db.QueryContext(ctx, getTrackTypes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TrackType
	for rows.Next() {
		var i TrackType
		if err := rows.Scan(
			&i.ID,
			&i.Name,
			&i.Color,
			&i.IsRepeating,
			&i.AllowSimultaneousPlay,
			&i.CreatedAt,
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
