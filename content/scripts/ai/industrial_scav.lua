-- industrial_scav.lua: HTN preconditions for industrial_scav_combat domain.

function industrial_scav_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function industrial_scav_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
