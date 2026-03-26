CREATE TABLE character_downtime (
    character_id      bigint PRIMARY KEY REFERENCES characters(id) ON DELETE CASCADE,
    activity_id       text        NOT NULL,
    completes_at      timestamptz NOT NULL,
    room_id           text        NOT NULL,
    activity_metadata jsonb
);
