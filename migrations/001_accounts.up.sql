CREATE TABLE IF NOT EXISTS accounts (
    id         BIGSERIAL    PRIMARY KEY,
    username   VARCHAR(64)  NOT NULL UNIQUE,
    password_hash TEXT      NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_username ON accounts (username);
