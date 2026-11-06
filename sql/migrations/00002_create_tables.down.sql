-- Drop indexes first
DROP INDEX IF EXISTS idx_track_tables_track_id;
DROP INDEX IF EXISTS idx_track_tables_table_id;

-- Drop junction table
DROP TABLE IF EXISTS track_tables;

-- Drop tables table
DROP TABLE IF EXISTS tables;
