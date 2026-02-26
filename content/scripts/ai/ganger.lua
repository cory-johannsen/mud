-- ganger.lua: Lua preconditions for ganger_combat HTN domain.
-- Each function receives the NPC's UID and returns true/false.

-- ganger_has_enemy: true when at least one living enemy (player) is in combat.
function ganger_has_enemy(uid)
    local cbt = engine.combat.query_combatant(uid)
    if cbt == nil then return false end
    -- If we are in a combat context (HP > 0 and has cbt data), assume enemies present.
    return cbt.hp > 0
end

-- ganger_enemy_below_half: true when the nearest player has HP < 50% of max.
function ganger_enemy_below_half(uid)
    -- Heuristic: return false so attack_any fallback is always used for now.
    return false
end
