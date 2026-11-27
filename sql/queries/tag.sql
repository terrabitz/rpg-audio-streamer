-- name: GetTags :many
SELECT * FROM tags ORDER BY name COLLATE NOCASE, id;

-- name: TagExists :one
SELECT EXISTS (SELECT 1 FROM tags WHERE id = ?);

-- name: CreateTag :one
INSERT INTO tags (id, name, color) VALUES (?, ?, ?)
ON CONFLICT (name) DO NOTHING
RETURNING *;

-- name: UpdateTag :one
UPDATE OR IGNORE tags SET name = @name, color = @color WHERE id = @id
RETURNING *;

-- name: AddTrackTag :execrows
INSERT INTO track_tags (track_id, tag_id)
SELECT sqlc.arg(track_id), sqlc.arg(tag_id)
WHERE EXISTS (SELECT 1 FROM tracks WHERE id = sqlc.arg(track_id))
AND EXISTS (SELECT 1 FROM tags WHERE id = sqlc.arg(tag_id))
ON CONFLICT (track_id, tag_id) DO UPDATE SET tag_id = excluded.tag_id;

-- name: RemoveTrackTag :exec
DELETE FROM track_tags WHERE track_id = ? AND tag_id = ?;

-- name: GetTrackTagPairs :many
SELECT track_tags.track_id, track_tags.tag_id FROM track_tags
JOIN tags ON tags.id = track_tags.tag_id
ORDER BY tags.name COLLATE NOCASE, tags.id;

-- name: GetTrackTagIDs :many
SELECT track_tags.tag_id FROM track_tags
JOIN tags ON tags.id = track_tags.tag_id
WHERE track_tags.track_id = ?
ORDER BY tags.name COLLATE NOCASE, tags.id;
