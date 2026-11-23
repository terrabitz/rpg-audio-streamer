UPDATE game_tables
SET settings = json_set(settings, '$.trackVolumes', json(COALESCE((
    SELECT json_group_object(
        lower(
            substr(hex(tracks.id), 1, 8) || '-' ||
            substr(hex(tracks.id), 9, 4) || '-' ||
            substr(hex(tracks.id), 13, 4) || '-' ||
            substr(hex(tracks.id), 17, 4) || '-' ||
            substr(hex(tracks.id), 21, 12)
        ),
        tracks.volume
    )
    FROM table_tracks
    JOIN tracks ON tracks.id = table_tracks.track_id
    WHERE table_tracks.table_id = game_tables.id
), '{}')));

ALTER TABLE tracks DROP COLUMN volume;
