CREATE TABLE IF NOT EXISTS character_wanted_levels (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    zone_id      VARCHAR(64) NOT NULL,
    wanted_level INTEGER NOT NULL CHECK (wanted_level BETWEEN 1 AND 4),
    PRIMARY KEY (character_id, zone_id)
);
