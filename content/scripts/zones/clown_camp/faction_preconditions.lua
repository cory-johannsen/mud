-- has_faction_enemy: returns true iff the NPC has at least one faction enemy in the room.
function has_faction_enemy(uid)
  local enemies = engine.combat.get_faction_enemies(uid)
  local count = 0
  for _ in pairs(enemies) do count = count + 1 end
  return count > 0
end

-- has_player_enemy: returns true iff the NPC has at least one player enemy in the room.
function has_player_enemy(uid)
  return engine.combat.enemy_count(uid) > 0
end
