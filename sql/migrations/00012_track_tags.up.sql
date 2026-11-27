CREATE TABLE tags (
    id BLOB PRIMARY KEY NOT NULL,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    color TEXT NOT NULL
);

CREATE TABLE track_tags (
    track_id BLOB NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    tag_id BLOB NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (track_id, tag_id)
);

CREATE INDEX track_tags_tag_id ON track_tags(tag_id);

-- SQLite connections do not all enable foreign keys.
CREATE TRIGGER delete_track_tags AFTER DELETE ON tracks
BEGIN
    DELETE FROM track_tags WHERE track_id = OLD.id;
END;
