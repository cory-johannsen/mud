CREATE TABLE character_feature_choices (
    character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    feature_id    TEXT    NOT NULL,
    choice_key    TEXT    NOT NULL,
    value         TEXT    NOT NULL,
    PRIMARY KEY (character_id, feature_id, choice_key)
);

INSERT INTO character_feature_choices (character_id, feature_id, choice_key, value)
SELECT character_id, 'predators_eye', 'favored_target', target_type
FROM character_favored_target;

DROP TABLE character_favored_target;
