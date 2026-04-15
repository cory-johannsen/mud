-- REQ-TTA-12: tracks pending L2+ technology slots awaiting trainer resolution.
-- char_level: character level at which this grant was issued (for PendingTechGrants lookup).
-- remaining: slots still to be filled by a trainer (decremented per training session).
CREATE TABLE character_pending_tech_slots (
    character_id  BIGINT       NOT NULL,
    char_level    INT          NOT NULL,
    tech_level    INT          NOT NULL,
    tradition     TEXT         NOT NULL,
    usage_type    TEXT         NOT NULL,
    remaining     INT          NOT NULL DEFAULT 1,
    granted_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (character_id, char_level, tech_level, tradition, usage_type)
);
