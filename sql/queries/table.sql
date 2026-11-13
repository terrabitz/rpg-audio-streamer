-- name: GetTables :many
SELECT * FROM game_tables ORDER BY name, id;

-- name: GetTable :one
SELECT * FROM game_tables WHERE id = ?;

-- name: GetTableByInvite :one
SELECT * FROM game_tables WHERE invite_token = ?;

-- name: CreateTable :one
INSERT INTO game_tables (id, name, invite_token) VALUES (?, ?, ?) RETURNING *;

-- name: UpdateTable :one
UPDATE game_tables SET name = ? WHERE id = ? RETURNING *;

-- name: GetTableSettings :one
SELECT settings FROM game_tables WHERE id = ?;

-- name: UpdateTableSettings :one
UPDATE game_tables SET settings = json_patch(settings, sqlc.arg(patch))
WHERE id = sqlc.arg(id) RETURNING settings;

-- name: DeleteTable :execrows
DELETE FROM game_tables WHERE id = ?;

-- name: GetTableTracks :many
SELECT tracks.* FROM tracks
JOIN table_tracks ON tracks.id = table_tracks.track_id
WHERE table_tracks.table_id = ? ORDER BY tracks.created_at, tracks.id;

-- name: AddTableTrack :one
INSERT INTO table_tracks (table_id, track_id)
SELECT sqlc.arg(table_id), sqlc.arg(track_id)
WHERE EXISTS (SELECT 1 FROM game_tables WHERE id = sqlc.arg(table_id))
AND EXISTS (SELECT 1 FROM tracks WHERE id = sqlc.arg(track_id))
ON CONFLICT (table_id, track_id) DO UPDATE SET track_id = excluded.track_id
RETURNING track_id;

-- name: RemoveTableTrack :exec
DELETE FROM table_tracks WHERE table_id = ? AND track_id = ?;

-- name: HasTableTrack :one
SELECT EXISTS (SELECT 1 FROM table_tracks WHERE table_id = ? AND track_id = ?);

-- name: CountTrackTables :one
SELECT COUNT(table_tracks.table_id) FROM tracks
LEFT JOIN table_tracks ON tracks.id = table_tracks.track_id
WHERE tracks.id = ? GROUP BY tracks.id;
