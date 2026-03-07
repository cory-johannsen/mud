CREATE TABLE character_proficiencies (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    category     TEXT   NOT NULL,
    rank         TEXT   NOT NULL DEFAULT 'untrained',
    PRIMARY KEY (character_id, category)
);
