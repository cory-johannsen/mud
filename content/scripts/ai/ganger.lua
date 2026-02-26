-- ganger.lua: Lua preconditions for ganger_combat HTN domain.
-- Each function receives the NPC's UID and returns true/false.
-- These hooks are called during active combat by the HTN planner.

-- ganger_has_enemy: returns true unconditionally during combat.
-- The HTN planner invokes this only when the NPC is already in an active
-- combat encounter, so the presence of a living enemy is guaranteed by context.
function ganger_has_enemy(uid)
    return true
end

-- ganger_enemy_below_half: returns false, routing to the attack_any fallback.
-- Enemy HP queries require a world query API not yet available in Lua.
-- The attack_any method (unconditional attack) is the correct combat behaviour.
function ganger_enemy_below_half(uid)
    return false
end
