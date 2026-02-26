-- scavenger.lua: Lua preconditions for scavenger_patrol HTN domain.

-- scavenger_has_enemy: true when scavenger can sense a player (HP > 0 in combat).
function scavenger_has_enemy(uid)
    local cbt = engine.combat.query_combatant(uid)
    if cbt == nil then return false end
    return cbt.hp > 0
end

-- scavenger_not_outnumbered: true when NPC allies >= living players.
-- Simplified: always return true for now (full implementation requires world query API).
function scavenger_not_outnumbered(uid)
    return true
end
