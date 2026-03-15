-- strip_mall_scav.lua: HTN preconditions for strip_mall_scav_combat domain.

function strip_mall_scav_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function strip_mall_scav_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
