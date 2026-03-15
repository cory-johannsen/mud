-- outlet_scavenger.lua: HTN preconditions for outlet_scavenger_combat domain.

function outlet_scavenger_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function outlet_scavenger_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
