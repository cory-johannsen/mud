ALTER TABLE character_inventory_instances
    DROP COLUMN IF EXISTS charges_remaining,
    DROP COLUMN IF EXISTS expended;
