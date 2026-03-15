-- terminal_squatter.lua: HTN preconditions for terminal_squatter_combat domain.

function terminal_squatter_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function terminal_squatter_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
