-- name: GetTracks :many
select * from tracks;

-- name: GetTracksByTableID :many
select t.* from tracks t
join track_tables tt on tt.track_id = t.id
where tt.table_id = @table_id
order by tt.created_at DESC;

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