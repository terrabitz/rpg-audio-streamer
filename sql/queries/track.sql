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
insert into tracks (id, created_at, name, path, type_id, start_time, end_time, loop_back_time)
values (@id, @created_at, @name, @path, @type_id, @start_time, sqlc.narg('end_time'), sqlc.narg('loop_back_time'));

-- name: UpdateTrack :one
update tracks
set
  name = coalesce(sqlc.narg('name'), name),
  type_id = coalesce(sqlc.narg('type_id'), type_id),
  start_time = CASE WHEN @set_playback THEN @start_time ELSE start_time END,
  end_time = CASE WHEN @set_playback THEN sqlc.narg('end_time') ELSE end_time END,
  loop_back_time = CASE WHEN @set_playback THEN sqlc.narg('loop_back_time') ELSE loop_back_time END
where id = @id
returning *