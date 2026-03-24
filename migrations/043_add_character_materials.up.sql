CREATE TABLE character_materials (
    character_id  bigint NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    material_id   text   NOT NULL,
    quantity      int    NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (character_id, material_id)
);
