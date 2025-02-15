-- name: GetTracks :many
select * from tracks;

-- name: GetTrackByID :one
select * from tracks where id = :id;

-- name: DeleteTrackByID :exec
delete from tracks where id = :id;

-- name: SaveTrack :exec
insert into tracks (id, created_at, name, path, type) values (:id, :created_at, :name, :path, :type);
