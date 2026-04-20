ALTER TABLE characters
  DROP COLUMN IF EXISTS hotbars,
  DROP COLUMN IF EXISTS active_hotbar_idx;
