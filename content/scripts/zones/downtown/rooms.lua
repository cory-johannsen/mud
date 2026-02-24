-- Room event hooks for downtown zone.

function on_enter(uid, room_id, from_room_id)
    engine.log.debug(uid .. " entered " .. room_id)
end

function on_exit(uid, room_id, to_room_id)
    -- no-op for stage 6
end

function on_look(uid, room_id)
    -- no-op for stage 6
end
