CREATE TABLE character_favored_target (
    character_id  BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    target_type   TEXT   NOT NULL,
    PRIMARY KEY (character_id)
);

INSERT INTO character_favored_target (character_id, target_type)
SELECT character_id, value
FROM character_feature_choices
WHERE feature_id = 'predators_eye' AND choice_key = 'favored_target';

DROP TABLE character_feature_choices;
