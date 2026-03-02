-- migrations/009_character_inventory.up.sql
ALTER TABLE characters
    ADD COLUMN has_received_starting_inventory BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE character_inventory (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    item_def_id  TEXT   NOT NULL,
    quantity     INT    NOT NULL DEFAULT 1,
    PRIMARY KEY (character_id, item_def_id)
);
