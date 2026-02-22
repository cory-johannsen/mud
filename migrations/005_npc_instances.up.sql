CREATE TABLE IF NOT EXISTS npc_instances (
    id          TEXT        NOT NULL PRIMARY KEY,
    template_id TEXT        NOT NULL,
    room_id     TEXT        NOT NULL,
    current_hp  INT         NOT NULL,
    conditions  JSONB       NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS npc_instances_room_id_idx ON npc_instances (room_id);
