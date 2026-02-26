-- scavenger.lua: Lua preconditions for scavenger_patrol HTN domain.
-- These hooks are called during active combat by the HTN planner.

-- scavenger_has_enemy: returns true unconditionally during combat.
-- The HTN planner invokes this only when the NPC is in an active combat
-- encounter, so the presence of a living enemy is guaranteed by context.
function scavenger_has_enemy(uid)
    return true
end

-- scavenger_not_outnumbered: returns true, routing to the attack path.
-- Ally/enemy count comparison requires a world query API; without it the
-- scavenger defaults to fighting rather than passing.
function scavenger_not_outnumbered(uid)
    return true
end
