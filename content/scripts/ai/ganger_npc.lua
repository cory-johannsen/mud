-- ganger_npc.lua: HTN preconditions for ganger_npc_combat domain.

function ganger_npc_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function ganger_npc_enemy_below_half(uid)
    local enemies = engine.combat.get_enemies(uid)
    if enemies == nil or #enemies == 0 then return false end
    local e = enemies[1]
    return e.hp < (e.max_hp * 0.5)
end
