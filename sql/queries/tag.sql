-- name: GetTags :many
SELECT * FROM tags ORDER BY name;

-- name: CreateTag :one
INSERT INTO tags (id, name, color) VALUES (@id, @name, @color)
ON CONFLICT(name) DO NOTHING
RETURNING *;

-- name: UpdateTag :one
UPDATE tags SET name = sqlc.arg(name), color = sqlc.arg(color) WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteTag :execrows
DELETE FROM tags WHERE id = @id;

-- name: GetTrackTags :many
SELECT tags.* FROM tags JOIN track_tags ON tags.id = track_tags.tag_id
WHERE track_tags.track_id = @track_id ORDER BY tags.name;

-- name: GetAllTrackTags :many
SELECT track_tags.track_id, tags.* FROM tags JOIN track_tags ON tags.id = track_tags.tag_id
ORDER BY tags.name;

-- name: ClearTrackTags :exec
DELETE FROM track_tags WHERE track_id = @track_id;

-- name: AddTrackTag :execrows
INSERT INTO track_tags (track_id, tag_id)
SELECT tracks.id, tags.id FROM tracks, tags WHERE tracks.id = @track_id AND tags.id = @tag_id;
