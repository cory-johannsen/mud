CREATE TABLE IF NOT EXISTS character_faction_rep (
    character_id BIGINT NOT NULL REFERENCES characters(id),
    faction_id   TEXT NOT NULL,
    rep          INT NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, faction_id)
);
