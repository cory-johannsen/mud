# NPC Behaviors

Adds per-NPC custom behaviors: HTN-driven dialog, threat-assessment aggressiveness, time-of-day schedules, expanded combat AI operators, and map movement fencing.

Design spec: `docs/superpowers/specs/2026-03-21-npc-behaviors-design.md`

## Requirements

- [ ] REQ-NB-1: The `say` HTN operator MUST select a random string from its `strings` parameter and broadcast it to the NPC's room as a speech event.
- [ ] REQ-NB-2: The `say` operator MUST enforce its `cooldown` parameter via `Instance.AbilityCooldowns` keyed by operator ID. The `cooldown` value MUST be a Go duration string parsed via `time.ParseDuration`.
- [ ] REQ-NB-3: The HTN evaluator MUST support the `in_combat`, `player_entered_room`, `hp_pct_below`, and `on_damage_taken` preconditions.
- [ ] REQ-NB-4: The `player_entered_room` and `on_damage_taken` preconditions MUST be true for exactly one evaluation tick after the triggering event.
- [ ] REQ-NB-5: All existing NPC templates using `taunts`/`taunt_chance`/`taunt_cooldown` MUST be migrated to `say` operator HTN domain entries.
- [ ] REQ-NB-6: `Template.Taunts`, `Template.TauntChance`, `Template.TauntCooldown`, `Instance.Taunts`, `Instance.TauntChance`, `Instance.TauntCooldown`, `Instance.LastTauntTime`, and `Instance.TryTaunt()` MUST be removed. `Template.Validate()` MUST be updated to remove the corresponding checks.
- [ ] REQ-NB-7: Threat assessment MUST fire on each idle tick when a hostile-disposition NPC shares a room with one or more players and is not in combat.
- [ ] REQ-NB-7A: When threat assessment determines the NPC should not engage and the NPC is not in combat, the NPC MUST remain passive and MUST NOT initiate combat.
- [ ] REQ-NB-7B: When threat assessment determines the NPC should not engage and the NPC is already in combat and `flee_hp_pct > 0` and the threshold is met, the NPC MUST execute the `flee` operator.
- [ ] REQ-NB-7C: When the NPC is already in combat and no flee threshold applies, engagement MUST NOT be re-evaluated mid-combat.
- [ ] REQ-NB-8: Threat assessment MUST fire when a player enters a room containing a hostile-disposition NPC not in combat.
- [ ] REQ-NB-9: The threat score formula MUST incorporate player average level, NPC level, party size, and player average HP% as a `float64` in `[0.0, 1.0]`.
- [ ] REQ-NB-10: `Template.CourageThreshold` MUST default to `999` to preserve current always-engage behavior.
- [ ] REQ-NB-11: `Template.FleeHPPct` MUST default to `0` (never flee).
- [ ] REQ-NB-12: `Instance` MUST gain a `GrudgePlayerID string` field with zero value `""`. It MUST be cleared to `""` on NPC respawn.
- [ ] REQ-NB-13: When an NPC takes damage, it MUST set `GrudgePlayerID` to the attacking player's ID; last damage event in processing order wins.
- [ ] REQ-NB-14: `flee_hp_pct` MUST be evaluated at the end of each round in which the NPC took damage.
- [ ] REQ-NB-15: The HTN evaluator MUST support the `has_grudge_target` precondition.
- [ ] REQ-NB-16: The NPC manager MUST receive the current game hour via an injected accessor to the calendar service.
- [ ] REQ-NB-17: `Template.Schedule` MUST be an optional field; templates without it MUST behave as before.
- [ ] REQ-NB-18: Schedule `hours` MUST support range format (`"6-18"`) and comma-separated format (`"8,12,20"`). When end hour < start hour, the range MUST wrap midnight.
- [ ] REQ-NB-19: On each idle tick, the manager MUST read the current game hour and find the first matching schedule entry.
- [ ] REQ-NB-20: When a matching entry is found and the NPC is not in `preferred_room`, the NPC MUST move one BFS step toward `preferred_room` per idle tick.
- [ ] REQ-NB-21: When a matching entry is found, the NPC MUST apply the entry's `behavior_mode` for that tick.
- [ ] REQ-NB-21A: When `behavior_mode` is `idle`, the NPC MUST remain in its `preferred_room` without movement and MUST fire idle-context `say` operator entries.
- [ ] REQ-NB-21B: When `behavior_mode` is `patrol`, the NPC MUST wander within `wander_radius` hops of its `preferred_room` using the `move_random` operator.
- [ ] REQ-NB-21C: When `behavior_mode` is `aggressive`, the NPC's effective `courage_threshold` MUST be `0` for the duration of the matching schedule window.
- [ ] REQ-NB-22: When no entry matches, the NPC MUST apply its default template behavior.
- [ ] REQ-NB-24: Schedule evaluation MUST NOT fire while the NPC is in active combat.
- [ ] REQ-NB-25: The `flee` operator MUST require the NPC to be in active combat as a precondition.
- [ ] REQ-NB-26: On success, `flee` MUST remove the NPC from the active combat round before relocating it via a randomly selected visible exit.
- [ ] REQ-NB-27: After a successful `flee`, combat MUST continue among remaining participants without the fled NPC.
- [ ] REQ-NB-28: When no exits are available, `flee` MUST fail and allow HTN fallback.
- [ ] REQ-NB-29: `target_weakest` MUST set the NPC's combat target to the living player with the lowest current HP% in the room.
- [ ] REQ-NB-30: `target_weakest` MUST require two or more living players in the room as a precondition.
- [ ] REQ-NB-31: When fewer than two living players are present, `target_weakest` MUST fail silently and retain the existing target.
- [ ] REQ-NB-32A: For the purposes of `call_for_help`, "adjacent room" MUST mean a room reachable via exactly one exit (BFS distance 1) from the caller's room.
- [ ] REQ-NB-32: `call_for_help` MUST require the NPC to be in active combat as a precondition.
- [ ] REQ-NB-33: `call_for_help` MUST require at least one adjacent room (BFS distance 1) containing a qualifying idle NPC as a precondition.
- [ ] REQ-NB-34: On success, `call_for_help` MUST cause all qualifying idle NPCs in adjacent rooms to join combat in the caller's room on the following tick.
- [ ] REQ-NB-35: `call_for_help` MUST fire at most once per combat instance per NPC.
- [ ] REQ-NB-36: `Template.HomeRoom` MUST default to the NPC's spawn room when not set.
- [ ] REQ-NB-37: `Template.WanderRadius` MUST default to `0`; templates without it MUST NOT move during patrol.
- [ ] REQ-NB-38: At zone load, the BFS distance map from `home_room` MUST be computed and cached on the NPC instance. If `home_room` is not in the same zone, the zone loader MUST return an error.
- [ ] REQ-NB-39: The `move_random` operator MUST exclude exits beyond `wander_radius` hops from `home_room`.
- [ ] REQ-NB-40: When `move_random`'s filtered pool is empty, the operator MUST fail and allow HTN fallback.
- [ ] REQ-NB-41: `Instance` MUST gain a `ReturningHome bool` field. After combat ends with no players remaining in the NPC's room, if the NPC is not in `home_room`, the NPC MUST set `Instance.ReturningHome = true`.
- [ ] REQ-NB-42: On each idle tick while `ReturningHome` is true, the NPC MUST move one BFS step toward `home_room`.
- [ ] REQ-NB-43: When the NPC arrives at `home_room`, `Instance.ReturningHome` MUST be cleared.
- [ ] REQ-NB-44: Home-room return movement MUST NOT fire while the NPC is in active combat.
- [ ] REQ-NB-45: When a schedule entry is active, its `preferred_room` MUST replace `home_room` as the fencing anchor. When the window ends, the template `home_room` MUST resume.
