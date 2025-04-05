-- name: GetTrackTypes :many
select * from track_types;

-- name: GetTrackTypeByID :one
select * from track_types where id = @id;