CREATE TABLE character_feats (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    feat_id      TEXT NOT NULL,
    PRIMARY KEY (character_id, feat_id)
);
