CREATE TABLE table_tracks_backup AS SELECT table_id, track_id FROM table_tracks;
DROP TRIGGER delete_track_tables;
DROP TABLE table_tracks;

CREATE TABLE game_tables_new (
    id BLOB PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    settings TEXT NOT NULL
        DEFAULT '{"masterVolume":100,"typeVolumes":{},"trackVolumes":{},"trackOrder":[]}'
);

INSERT INTO game_tables_new (id, name, settings)
SELECT id, name, settings FROM game_tables;

DROP TABLE game_tables;
ALTER TABLE game_tables_new RENAME TO game_tables;

CREATE TABLE table_tracks (
    table_id BLOB NOT NULL REFERENCES game_tables(id) ON DELETE CASCADE,
    track_id BLOB NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    PRIMARY KEY (table_id, track_id)
);

INSERT INTO table_tracks (table_id, track_id)
SELECT table_id, track_id FROM table_tracks_backup;
DROP TABLE table_tracks_backup;

CREATE INDEX table_tracks_track_id ON table_tracks(track_id);

CREATE TRIGGER delete_table_tracks AFTER DELETE ON game_tables
BEGIN
    DELETE FROM table_tracks WHERE table_id = OLD.id;
END;

CREATE TRIGGER delete_track_tables AFTER DELETE ON tracks
BEGIN
    DELETE FROM table_tracks WHERE track_id = OLD.id;
END;
