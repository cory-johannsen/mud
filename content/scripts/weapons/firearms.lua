-- firearms.lua: Lua hooks for firearm mechanics.

-- on_firearm_attack: called before damage is applied for a ranged attack.
-- Return a number to override the attack total; return nil to use Go-computed value.
function on_firearm_attack(uid, target_uid, weapon_id, attack_total)
    engine.log.debug(uid .. " fires " .. weapon_id .. " at " .. target_uid ..
        " (roll: " .. tostring(attack_total) .. ")")
    return nil
end

-- on_reload: called when a player or NPC reloads a firearm.
function on_reload(uid, weapon_id)
    engine.log.debug(uid .. " reloads " .. weapon_id)
end
