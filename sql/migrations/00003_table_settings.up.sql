ALTER TABLE game_tables ADD COLUMN settings TEXT NOT NULL
DEFAULT '{"masterVolume":100,"typeVolumes":{},"trackVolumes":{},"trackOrder":[]}';
