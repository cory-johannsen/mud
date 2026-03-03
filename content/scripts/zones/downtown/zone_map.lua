-- zone_map_use is called when a player uses a zone map equipment item.
-- uid: the player's UID string
-- Calls engine.map.reveal_zone to bulk-reveal all rooms in the downtown zone.
function zone_map_use(uid)
    engine.map.reveal_zone(uid, "downtown")
    return "You study the district map carefully. Your knowledge of the area expands."
end
