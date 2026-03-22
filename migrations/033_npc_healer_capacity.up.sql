CREATE TABLE IF NOT EXISTS npc_healer_capacity (
    npc_template_id VARCHAR(64) PRIMARY KEY,
    capacity_used   INTEGER NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
