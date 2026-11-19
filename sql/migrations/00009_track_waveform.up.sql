ALTER TABLE tracks ADD COLUMN waveform JSON CHECK (waveform IS NULL OR json_valid(waveform));
