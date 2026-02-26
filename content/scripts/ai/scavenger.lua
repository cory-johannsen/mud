-- scavenger.lua: Lua preconditions for scavenger_patrol HTN domain.
-- These hooks are called during active combat by the HTN planner.

-- scavenger_has_enemy: returns true when at least one living enemy is present.
function scavenger_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

-- scavenger_not_outnumbered: returns true when ally count >= enemy count,
-- routing to the attack path. Returns false when the scavenger is outnumbered,
-- routing to the pass action.
function scavenger_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
