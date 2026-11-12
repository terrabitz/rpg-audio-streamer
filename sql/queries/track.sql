-- name: GetTracks :many
select * from tracks;

-- name: GetTrackByID :one
select * from tracks where id = @id;

-- name: DeleteTrackByID :exec
delete from tracks where id = @id;

-- name: DeleteTrackByIDIfTableCount :execrows
DELETE FROM tracks
WHERE id = sqlc.arg(id)
AND (SELECT COUNT(*) FROM table_tracks WHERE track_id = sqlc.arg(id)) =
    CAST(sqlc.arg(expected_table_count) AS INTEGER);

-- name: SaveTrack :exec
insert into tracks (id, created_at, name, path, type_id) values (@id, @created_at, @name, @path, @type_id);

-- name: UpdateTrack :one
update tracks
set
  name = coalesce(sqlc.narg('name'), name),
  type_id = coalesce(sqlc.narg('type_id'), type_id)
where id = @id
returning *