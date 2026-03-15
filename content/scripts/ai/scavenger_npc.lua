-- scavenger_npc.lua: HTN preconditions for scavenger_combat domain.

function scavenger_npc_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function scavenger_npc_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
