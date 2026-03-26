-- migrations/047_character_quests.up.sql
CREATE TABLE character_quests (
    character_id BIGINT NOT NULL REFERENCES characters(id),
    quest_id     TEXT    NOT NULL,
    status       TEXT    NOT NULL,
    completed_at TIMESTAMPTZ,
    PRIMARY KEY (character_id, quest_id)
);

CREATE TABLE character_quest_progress (
    character_id BIGINT NOT NULL REFERENCES characters(id),
    quest_id     TEXT   NOT NULL,
    objective_id TEXT   NOT NULL,
    progress     INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, quest_id, objective_id)
);
