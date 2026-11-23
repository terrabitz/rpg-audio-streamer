ALTER TABLE tracks ADD COLUMN volume REAL NOT NULL DEFAULT 100
    CHECK (volume >= 0 AND volume <= 100);

UPDATE tracks
SET volume = COALESCE((
    SELECT CAST(json_extract(game_tables.settings, '$.trackVolumes."' ||
        lower(
            substr(hex(tracks.id), 1, 8) || '-' ||
            substr(hex(tracks.id), 9, 4) || '-' ||
            substr(hex(tracks.id), 13, 4) || '-' ||
            substr(hex(tracks.id), 17, 4) || '-' ||
            substr(hex(tracks.id), 21, 12)
        ) || '"') AS REAL)
    FROM game_tables
    WHERE json_type(game_tables.settings, '$.trackVolumes."' ||
        lower(
            substr(hex(tracks.id), 1, 8) || '-' ||
            substr(hex(tracks.id), 9, 4) || '-' ||
            substr(hex(tracks.id), 13, 4) || '-' ||
            substr(hex(tracks.id), 17, 4) || '-' ||
            substr(hex(tracks.id), 21, 12)
        ) || '"') IN ('integer', 'real')
    ORDER BY game_tables.name COLLATE NOCASE, game_tables.id
    LIMIT 1
), 100);

UPDATE game_tables
SET settings = json_remove(settings, '$.trackVolumes');
