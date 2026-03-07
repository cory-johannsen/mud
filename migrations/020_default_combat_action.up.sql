ALTER TABLE characters ADD COLUMN IF NOT EXISTS default_combat_action TEXT NOT NULL DEFAULT 'pass';
