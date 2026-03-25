-- migrations/046_affixed_materials.down.sql

ALTER TABLE character_inventory_instances
    DROP COLUMN affixed_materials,
    DROP COLUMN material_max_durability_bonus;

ALTER TABLE character_equipment
    DROP COLUMN affixed_materials,
    DROP COLUMN material_max_durability_bonus;

ALTER TABLE character_weapon_presets
    DROP COLUMN affixed_materials,
    DROP COLUMN material_max_durability_bonus;
