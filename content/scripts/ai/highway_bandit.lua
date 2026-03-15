-- highway_bandit.lua: HTN preconditions for highway_bandit_combat domain.

function highway_bandit_has_enemy(uid)
    return engine.combat.enemy_count(uid) > 0
end

function highway_bandit_not_outnumbered(uid)
    return engine.combat.ally_count(uid) >= engine.combat.enemy_count(uid)
end
