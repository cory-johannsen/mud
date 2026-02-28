CREATE TABLE character_weapon_presets (
    id           BIGSERIAL PRIMARY KEY,
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    preset_index INT NOT NULL,
    slot         TEXT NOT NULL,
    item_def_id  TEXT NOT NULL,
    ammo_count   INT NOT NULL DEFAULT 0,
    CONSTRAINT uq_character_preset_slot UNIQUE (character_id, preset_index, slot)
);

CREATE TABLE character_equipment (
    id           BIGSERIAL PRIMARY KEY,
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    slot         TEXT NOT NULL,
    item_def_id  TEXT NOT NULL,
    CONSTRAINT uq_character_equipment_slot UNIQUE (character_id, slot)
);
