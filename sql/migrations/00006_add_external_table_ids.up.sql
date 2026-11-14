ALTER TABLE game_tables ADD COLUMN external_id TEXT NOT NULL DEFAULT '';
UPDATE game_tables
SET external_id = lower(hex(randomblob(21))) || substr('048c', (random() & 3) + 1, 1);
CREATE UNIQUE INDEX game_tables_external_id_unique ON game_tables(external_id);
