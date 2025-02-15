CREATE TABLE track_types (
    id BLOB PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    color TEXT NOT NULL,
    is_repeating BOOLEAN NOT NULL DEFAULT 0,
    allow_simultaneous_play BOOLEAN NOT NULL DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE tracks (
    id BLOB PRIMARY KEY NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP NOT NULL,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    type_id BLOB NOT NULL,
    FOREIGN KEY (type_id) REFERENCES track_types(id)
);

-- Insert some default track types
INSERT INTO track_types (id, name, color, is_repeating, allow_simultaneous_play)
VALUES
    (X'1EC000A2A7C911EEA0E50242AC120002', 'Ambiance', '#B39DDB', TRUE, TRUE),
    (X'1EC000A2A7C911EEA0E50242AC120003', 'Music', '#80DEEA', TRUE, FALSE),
    (X'1EC000A2A7C911EEA0E50242AC120004', 'One-Shot', '#A5D6A7', FALSE, TRUE);

