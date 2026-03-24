-- Migration 037: Add charge state to item instances (REQ-ACT-14).
-- Charges belong to the item instance, not the equipped slot.

ALTER TABLE character_inventory_instances
    ADD COLUMN IF NOT EXISTS charges_remaining INTEGER NOT NULL DEFAULT -1,
    ADD COLUMN IF NOT EXISTS expended          BOOLEAN NOT NULL DEFAULT FALSE;
