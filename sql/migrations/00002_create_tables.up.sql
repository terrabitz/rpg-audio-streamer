-- Create tables table
CREATE TABLE tables (
    id BLOB PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    invite_code TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL
);

-- Create junction table for many-to-many relationship between tracks and tables
CREATE TABLE track_tables (
    track_id BLOB NOT NULL,
    table_id BLOB NOT NULL,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (track_id, table_id),
    FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE,
    FOREIGN KEY (table_id) REFERENCES tables(id) ON DELETE CASCADE
);

-- Create indexes for faster lookups
CREATE INDEX idx_track_tables_table_id ON track_tables(table_id);
CREATE INDEX idx_track_tables_track_id ON track_tables(track_id);

-- Insert default table with a generated invite code
INSERT INTO tables (id, name, invite_code, created_at)
VALUES (
    X'1EC000A2A7C911EEA0E50242AC120005',
    'My First Table',
    lower(hex(randomblob(16))),
    strftime('%s','now')
);

-- Migrate all existing tracks to the default table
INSERT INTO track_tables (track_id, table_id, created_at)
SELECT id, X'1EC000A2A7C911EEA0E50242AC120005', strftime('%s','now')
FROM tracks;
