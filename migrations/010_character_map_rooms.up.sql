CREATE TABLE character_map_rooms (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    zone_id      TEXT   NOT NULL,
    room_id      TEXT   NOT NULL,
    PRIMARY KEY (character_id, zone_id, room_id)
);
