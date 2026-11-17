ALTER TABLE tracks ADD COLUMN status TEXT NOT NULL DEFAULT 'ready'
    CHECK (status IN ('ready', 'processing', 'failed'));
ALTER TABLE tracks ADD COLUMN processing_error TEXT NOT NULL DEFAULT '';
