-- gravel_pit_boss.lua: HTN preconditions for gravel_pit_boss_combat domain.

function gravel_pit_boss_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function gravel_pit_boss_enemy_below_half(uid)
    local enemies = engine.combat.get_enemies(uid)
    if enemies == nil or #enemies == 0 then return false end
    local e = enemies[1]
    return e.hp < (e.max_hp * 0.5)
end
