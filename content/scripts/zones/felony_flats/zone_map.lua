-- zone_map_use is called when a player uses a zone map equipment item.
-- uid: the player's UID string
--
-- The zone ID is hardcoded to "felony_flats" because this script is zone-specific.
-- Each zone that provides a zone map item must have its own copy of this script
-- with the appropriate zone ID embedded.
function zone_map_use(uid)
    engine.map.reveal_zone(uid, "felony_flats")
    return "You study the district map carefully. Your knowledge of the area expands."
end
