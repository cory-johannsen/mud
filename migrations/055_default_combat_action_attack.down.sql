ALTER TABLE characters ALTER COLUMN default_combat_action SET DEFAULT 'pass';
UPDATE characters SET default_combat_action = 'pass' WHERE default_combat_action = 'attack';
