-- name: GetTracks :many
select * from tracks;

-- name: GetTrackByID :one
select * from tracks where id = @id;

-- name: DeleteTrackByID :exec
delete from tracks where id = @id;

-- name: SaveTrack :exec
insert into tracks (id, created_at, name, path, type_id) values (@id, @created_at, @name, @path, @type_id);

-- name: UpdateTrack :one
update tracks
set
  name = coalesce(sqlc.narg('name'), name),
  type_id = coalesce(sqlc.narg('type_id'), type_id)
where id = @id
returning *