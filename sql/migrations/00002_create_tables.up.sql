CREATE TABLE game_tables (
    id BLOB PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    invite_token TEXT NOT NULL UNIQUE
);

CREATE TABLE table_tracks (
    table_id BLOB NOT NULL REFERENCES game_tables(id) ON DELETE CASCADE,
    track_id BLOB NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    PRIMARY KEY (table_id, track_id)
);

CREATE INDEX table_tracks_track_id ON table_tracks(track_id);

-- The empty invite uses the configured legacy token without storing configuration.
INSERT INTO game_tables (id, name, invite_token)
VALUES (X'00000000000040008000000000000001', 'My Table', '');

INSERT INTO table_tracks (table_id, track_id)
SELECT X'00000000000040008000000000000001', id FROM tracks;

-- SQLite connections do not all enable foreign keys.
CREATE TRIGGER delete_table_tracks AFTER DELETE ON game_tables
BEGIN
    DELETE FROM table_tracks WHERE table_id = OLD.id;
END;

CREATE TRIGGER delete_track_tables AFTER DELETE ON tracks
BEGIN
    DELETE FROM table_tracks WHERE track_id = OLD.id;
END;
