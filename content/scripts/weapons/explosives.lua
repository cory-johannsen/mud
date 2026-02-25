-- explosives.lua: Lua hooks for explosive mechanics.

-- on_explosive_throw: called before explosive damage is resolved.
-- Return nil to use Go-computed area effect.
function on_explosive_throw(uid, explosive_id)
    engine.log.info(uid .. " throws " .. explosive_id)
end
