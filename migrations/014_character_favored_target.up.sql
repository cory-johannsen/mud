CREATE TABLE character_favored_target (
    character_id  BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    target_type   TEXT   NOT NULL,
    PRIMARY KEY (character_id)
);
