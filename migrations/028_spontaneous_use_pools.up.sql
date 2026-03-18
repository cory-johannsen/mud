CREATE TABLE character_spontaneous_use_pools (
    character_id   BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    tech_level     INT    NOT NULL,
    uses_remaining INT    NOT NULL DEFAULT 0,
    max_uses       INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, tech_level)
);
