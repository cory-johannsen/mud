-- mill_plain_thug.lua: HTN preconditions for mill_plain_thug_combat domain.

function mill_plain_thug_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function mill_plain_thug_enemy_below_half(uid)
    local enemies = engine.combat.get_enemies(uid)
    if enemies == nil or #enemies == 0 then return false end
    local e = enemies[1]
    return e.hp < (e.max_hp * 0.5)
end
