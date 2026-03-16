CREATE TABLE character_hardwired_technologies (
    character_id BIGINT NOT NULL,
    tech_id      TEXT   NOT NULL,
    PRIMARY KEY (character_id, tech_id)
);

CREATE TABLE character_prepared_technologies (
    character_id BIGINT NOT NULL,
    slot_level   INT    NOT NULL,
    slot_index   INT    NOT NULL,
    tech_id      TEXT   NOT NULL,
    PRIMARY KEY (character_id, slot_level, slot_index)
);

CREATE TABLE character_spontaneous_technologies (
    character_id BIGINT NOT NULL,
    tech_id      TEXT   NOT NULL,
    level        INT    NOT NULL,
    PRIMARY KEY (character_id, tech_id)
);

CREATE TABLE character_innate_technologies (
    character_id BIGINT NOT NULL,
    tech_id      TEXT   NOT NULL,
    max_uses     INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, tech_id)
);
