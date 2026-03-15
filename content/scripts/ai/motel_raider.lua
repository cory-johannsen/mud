-- motel_raider.lua: HTN preconditions for motel_raider_combat domain.

function motel_raider_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function motel_raider_enemy_below_half(uid)
    local enemies = engine.combat.get_enemies(uid)
    if enemies == nil or #enemies == 0 then return false end
    local e = enemies[1]
    return e.hp < (e.max_hp * 0.5)
end
