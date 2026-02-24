-- Condition lifecycle hooks for the Dying condition.
-- Loaded into the __global__ VM via LoadGlobal.

function dying_on_apply(uid, cond_id, stacks, duration)
    engine.log.info(uid .. " is dying (stacks: " .. tostring(stacks) .. ")")
end

function dying_on_remove(uid, cond_id)
    engine.log.info(uid .. " recovers from dying")
end

function dying_on_tick(uid, cond_id, stacks, duration_remaining)
    engine.log.debug(uid .. " dying tick: stacks=" .. tostring(stacks))
end
