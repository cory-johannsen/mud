CREATE TABLE IF NOT EXISTS weather_events (
    id               SERIAL PRIMARY KEY,
    weather_type     TEXT   NOT NULL,
    end_tick         BIGINT NOT NULL,
    cooldown_end_tick BIGINT NOT NULL DEFAULT 0,
    active           BOOL   NOT NULL DEFAULT TRUE
);
