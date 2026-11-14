CREATE TABLE tags (
    id BLOB PRIMARY KEY NOT NULL,
    name TEXT NOT NULL COLLATE NOCASE UNIQUE,
    color TEXT NOT NULL
);

CREATE TABLE track_tags (
    track_id BLOB NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    tag_id BLOB NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (track_id, tag_id)
);

CREATE INDEX track_tags_tag_id ON track_tags(tag_id);

CREATE TRIGGER delete_track_tags AFTER DELETE ON tracks
BEGIN
    DELETE FROM track_tags WHERE track_id = OLD.id;
END;

CREATE TRIGGER delete_tag_tracks AFTER DELETE ON tags
BEGIN
    DELETE FROM track_tags WHERE tag_id = OLD.id;
END;
