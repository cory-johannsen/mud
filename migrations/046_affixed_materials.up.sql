-- migrations/046_affixed_materials.up.sql

ALTER TABLE character_inventory_instances
    ADD COLUMN affixed_materials              text[] NOT NULL DEFAULT '{}',
    ADD COLUMN material_max_durability_bonus  int    NOT NULL DEFAULT 0;

ALTER TABLE character_equipment
    ADD COLUMN affixed_materials              text[] NOT NULL DEFAULT '{}',
    ADD COLUMN material_max_durability_bonus  int    NOT NULL DEFAULT 0;

ALTER TABLE character_weapon_presets
    ADD COLUMN affixed_materials              text[] NOT NULL DEFAULT '{}',
    ADD COLUMN material_max_durability_bonus  int    NOT NULL DEFAULT 0;
