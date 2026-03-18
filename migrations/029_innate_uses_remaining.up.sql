ALTER TABLE character_innate_technologies
    ADD COLUMN uses_remaining INT NOT NULL DEFAULT 0;

UPDATE character_innate_technologies
    SET uses_remaining = max_uses;
