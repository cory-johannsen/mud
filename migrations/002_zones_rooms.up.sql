CREATE TABLE zones (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL,
    start_room  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE rooms (
    id          TEXT PRIMARY KEY,
    zone_id     TEXT NOT NULL REFERENCES zones(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    properties  JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE room_exits (
    id          BIGSERIAL PRIMARY KEY,
    room_id     TEXT NOT NULL REFERENCES rooms(id),
    direction   TEXT NOT NULL,
    target_room TEXT NOT NULL REFERENCES rooms(id),
    locked      BOOLEAN NOT NULL DEFAULT FALSE,
    hidden      BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE(room_id, direction)
);
