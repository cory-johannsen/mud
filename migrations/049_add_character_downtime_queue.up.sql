-- activity_metadata jsonb column is intentionally omitted: no downstream code references it
-- in the current implementation. Add a follow-on migration if structured metadata is needed.
CREATE TABLE character_downtime_queue (
    id            bigserial PRIMARY KEY,
    character_id  bigint NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    position      int    NOT NULL,
    activity_id   text   NOT NULL,
    activity_args text,
    UNIQUE (character_id, position)
);
