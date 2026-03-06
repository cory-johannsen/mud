CREATE TABLE character_ability_boosts (
    character_id  BIGINT  NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    source        TEXT    NOT NULL,  -- "archetype" or "region"
    ability       TEXT    NOT NULL,
    PRIMARY KEY (character_id, source, ability)
);
