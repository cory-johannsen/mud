-- river_pirate.lua: HTN preconditions for river_pirate_combat domain.

function river_pirate_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function river_pirate_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
