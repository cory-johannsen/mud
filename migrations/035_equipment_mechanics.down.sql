-- Revert migration 035: Equipment mechanics

DROP TABLE IF EXISTS character_inventory_instances;

ALTER TABLE characters DROP COLUMN IF EXISTS team;

ALTER TABLE character_weapon_presets
    DROP COLUMN IF EXISTS modifier,
    DROP COLUMN IF EXISTS max_durability,
    DROP COLUMN IF EXISTS durability;

ALTER TABLE character_equipment
    DROP COLUMN IF EXISTS curse_revealed,
    DROP COLUMN IF EXISTS modifier,
    DROP COLUMN IF EXISTS max_durability,
    DROP COLUMN IF EXISTS durability;
