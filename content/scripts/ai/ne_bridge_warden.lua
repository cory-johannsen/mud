-- ne_bridge_warden.lua: HTN preconditions for ne_bridge_warden_combat domain.

function ne_bridge_warden_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function ne_bridge_warden_enemy_below_half(uid)
    local enemies = engine.combat.get_enemies(uid)
    if enemies == nil or #enemies == 0 then return false end
    local e = enemies[1]
    return e.hp < (e.max_hp * 0.5)
end
