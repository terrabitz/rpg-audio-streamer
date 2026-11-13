ALTER TABLE game_tables ADD COLUMN invite_token TEXT NOT NULL DEFAULT '';
UPDATE game_tables SET invite_token = lower(hex(id));
CREATE UNIQUE INDEX game_tables_invite_token_unique ON game_tables(invite_token);
