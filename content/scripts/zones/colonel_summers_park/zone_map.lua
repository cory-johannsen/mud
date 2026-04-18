-- zone_map_use is called when a player uses a zone map equipment item.
-- uid: the player's UID string
--
-- The zone ID is hardcoded to "colonel_summers_park" because this script is zone-specific.
-- Each zone that provides a zone map item must have its own copy of this script
-- with the appropriate zone ID embedded.
function zone_map_use(uid)
    engine.map.reveal_zone(uid, "colonel_summers_park")
    return "You study the park map carefully. Your knowledge of the area expands."
end
