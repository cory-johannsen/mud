-- alberta_drifter.lua: HTN preconditions for alberta_drifter_combat domain.

function alberta_drifter_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function alberta_drifter_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
