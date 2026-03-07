CREATE TABLE IF NOT EXISTS character_pending_boosts (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    count        INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id)
);
