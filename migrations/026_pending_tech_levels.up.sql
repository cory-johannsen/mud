CREATE TABLE character_pending_tech_levels (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    level        INT    NOT NULL,
    PRIMARY KEY (character_id, level)
);
