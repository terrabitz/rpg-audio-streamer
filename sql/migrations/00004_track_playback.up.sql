ALTER TABLE tracks ADD COLUMN start_time REAL NOT NULL DEFAULT 0 CHECK (start_time >= 0);
ALTER TABLE tracks ADD COLUMN end_time REAL CHECK (end_time > start_time);
ALTER TABLE tracks ADD COLUMN loop_back_time REAL CHECK (
  loop_back_time >= start_time AND (end_time IS NULL OR loop_back_time < end_time)
);
