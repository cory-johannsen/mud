CREATE TABLE characters (
    id              BIGSERIAL    PRIMARY KEY,
    account_id      BIGINT       NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    name            VARCHAR(64)  NOT NULL,
    region          TEXT         NOT NULL,
    class           TEXT         NOT NULL,
    level           INT          NOT NULL DEFAULT 1,
    experience      INT          NOT NULL DEFAULT 0,
    location        TEXT         NOT NULL DEFAULT 'grinders_row',
    strength        INT          NOT NULL DEFAULT 10,
    dexterity       INT          NOT NULL DEFAULT 10,
    constitution    INT          NOT NULL DEFAULT 10,
    intelligence    INT          NOT NULL DEFAULT 10,
    wisdom          INT          NOT NULL DEFAULT 10,
    charisma        INT          NOT NULL DEFAULT 10,
    max_hp          INT          NOT NULL DEFAULT 8,
    current_hp      INT          NOT NULL DEFAULT 8,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_characters_account_name UNIQUE (account_id, name)
);

CREATE INDEX idx_characters_account_id ON characters (account_id);
