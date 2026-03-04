CREATE TABLE character_skills (
    character_id BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    skill_id     TEXT    NOT NULL,
    proficiency  TEXT    NOT NULL DEFAULT 'untrained',
    PRIMARY KEY (character_id, skill_id)
);
