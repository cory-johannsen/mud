-- migrations/009_character_inventory.down.sql
DROP TABLE IF EXISTS character_inventory;

ALTER TABLE characters
    DROP COLUMN IF EXISTS has_received_starting_inventory;
