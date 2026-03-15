-- lieutenant.lua: HTN preconditions for lieutenant_combat domain.

function lieutenant_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function lieutenant_enemy_below_half(uid)
    local enemies = engine.combat.get_enemies(uid)
    if enemies == nil or #enemies == 0 then return false end
    local e = enemies[1]
    return e.hp < (e.max_hp * 0.5)
end
