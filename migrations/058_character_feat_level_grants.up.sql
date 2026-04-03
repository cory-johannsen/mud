CREATE TABLE IF NOT EXISTS character_feat_level_grants (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    level        INT    NOT NULL,
    PRIMARY KEY  (character_id, level)
);
