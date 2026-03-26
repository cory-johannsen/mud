ALTER TABLE character_map_rooms ADD COLUMN IF NOT EXISTS explored boolean NOT NULL DEFAULT TRUE;
