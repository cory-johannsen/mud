-- Change the default combat action from 'pass' to 'attack' for all characters
-- that have never explicitly set their combat default (still have the old default).
UPDATE characters SET default_combat_action = 'attack' WHERE default_combat_action = 'pass';
ALTER TABLE characters ALTER COLUMN default_combat_action SET DEFAULT 'attack';
