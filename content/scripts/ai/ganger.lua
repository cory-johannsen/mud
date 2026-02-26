-- ganger.lua: Lua preconditions for ganger_combat HTN domain.
-- Each function receives the NPC's UID and returns true/false.
-- These hooks are called during active combat by the HTN planner.

-- ganger_has_enemy: returns true when at least one living enemy is present.
function ganger_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

-- ganger_enemy_below_half: returns true when the first living enemy's HP is
-- strictly below 50% of its maximum, routing to the strike_weakest method.
-- Falls through to attack_any when no enemies are found or none are below half.
function ganger_enemy_below_half(uid)
    local enemies = engine.combat.get_enemies(uid)
    if enemies == nil or #enemies == 0 then return false end
    local e = enemies[1]
    return e.hp < (e.max_hp * 0.5)
end
