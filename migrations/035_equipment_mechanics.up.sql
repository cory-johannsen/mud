-- Migration 035: Equipment mechanics — durability, modifiers, team, inventory instances

-- REQ-EM-17: character_equipment gains durability and modifier columns.
ALTER TABLE character_equipment
    ADD COLUMN IF NOT EXISTS durability     int  NOT NULL DEFAULT -1,
    ADD COLUMN IF NOT EXISTS max_durability int  NOT NULL DEFAULT -1,
    ADD COLUMN IF NOT EXISTS modifier       text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS curse_revealed bool NOT NULL DEFAULT false;

-- character_weapon_presets gains durability and modifier columns.
ALTER TABLE character_weapon_presets
    ADD COLUMN IF NOT EXISTS durability     int  NOT NULL DEFAULT -1,
    ADD COLUMN IF NOT EXISTS max_durability int  NOT NULL DEFAULT -1,
    ADD COLUMN IF NOT EXISTS modifier       text NOT NULL DEFAULT '';

-- characters gains team column for consumable team effectiveness (REQ-EM-39).
ALTER TABLE characters ADD COLUMN IF NOT EXISTS team text NOT NULL DEFAULT '';

-- New table for per-instance backpack item durability and modifier tracking.
CREATE TABLE IF NOT EXISTS character_inventory_instances (
    instance_id    text   PRIMARY KEY,
    character_id   bigint NOT NULL REFERENCES characters(id),
    item_def_id    text   NOT NULL,
    durability     int    NOT NULL DEFAULT -1,
    max_durability int    NOT NULL DEFAULT -1,
    modifier       text   NOT NULL DEFAULT '',
    curse_revealed bool   NOT NULL DEFAULT false
);
