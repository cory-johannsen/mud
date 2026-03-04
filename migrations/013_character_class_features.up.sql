CREATE TABLE character_class_features (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    feature_id   TEXT NOT NULL,
    PRIMARY KEY (character_id, feature_id)
);
