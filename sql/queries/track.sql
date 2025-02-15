-- name: GetTracks :many
select * from tracks;

-- name: DeleteTrackByID :exec
delete from tracks where id = :id;