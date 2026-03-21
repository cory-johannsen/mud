# Exploration Mode

Persistent exploration stances that modify player behavior between combat encounters. A player has at most one active mode; it persists across room transitions until changed. Seven modes fire effects on room entry or combat start via hooks on `PlayerSession`. See `docs/superpowers/specs/2026-03-20-exploration-mode-design.md` for the full design spec.

## Requirements

- [ ] Core data model and command
  - REQ-EXP-1: `explore <mode>` MUST set `ExploreMode` and confirm to the player.
  - REQ-EXP-2: `explore off` MUST clear `ExploreMode` and confirm to the player.
  - REQ-EXP-3: Setting a new mode MUST replace the old mode without requiring `explore off` first.
  - REQ-EXP-4: `explore` MUST be rejected with an error message when the player is in active combat.
  - REQ-EXP-5: `active_sensors` and `case_it` MUST fire their room-entry hooks immediately upon being set.
  - REQ-EXP-6: `explore shadow` without a valid player name in the same room MUST fail with an error message.
- [ ] Lay Low (`lay_low`)
  - REQ-EXP-7: MUST make a secret Ghosting check vs. the highest NPC Awareness DC (`10 + instance.Awareness`) on room entry.
  - REQ-EXP-8: Critical success MUST apply `hidden` + `undetected`; success MUST apply `hidden` only.
  - REQ-EXP-8a: Failure MUST have no effect. Critical failure MUST prevent the player from gaining `hidden` or `undetected` in the current room for the duration of the visit.
  - REQ-EXP-9: MUST be cleared automatically when combat starts.
  - REQ-EXP-10: If no NPCs are present, MUST make no check and apply no conditions.
- [ ] Hold Ground (`hold_ground`)
  - REQ-EXP-11: MUST apply `shield_raised` when AP is granted for the player's first initiative slot in a new combat, at no AP cost.
  - REQ-EXP-12: If no shield is equipped, MUST have no effect and MUST NOT produce an error.
  - REQ-EXP-13: The applied `shield_raised` MUST follow normal shield rules and expire when AP is next granted to the player.
- [ ] Active Sensors (`active_sensors`)
  - REQ-EXP-14: MUST make a secret Tech Lore check on room entry and immediately on mode set.
  - REQ-EXP-15: Success MUST send a console message listing detected technology. The room view struct MUST NOT be modified.
  - REQ-EXP-16: Critical success MUST additionally reveal concealed technology.
  - REQ-EXP-17: Failure MUST send no message and reveal nothing.
  - REQ-EXP-18: DC MUST be determined by the room's current danger level; unset defaults to 16 (Sketchy).
- [ ] Case It (`case_it`) — implements "Search mode" from the `traps` feature (REQ-TR-3, REQ-TR-4)
  - REQ-EXP-19: MUST make a secret Awareness check on room entry and immediately on mode set.
  - REQ-EXP-20: Success MUST reveal hidden exits, concealed items, and trap triggers via a console message. The room view struct MUST NOT be modified.
  - REQ-EXP-21: Critical success MUST additionally reveal trap type and rough DC range.
  - REQ-EXP-22: Failure MUST reveal nothing.
  - REQ-EXP-23: DC MUST be determined by the room's current danger level; unset defaults to 16 (Sketchy).
  - REQ-EXP-24: MUST satisfy REQ-TR-3 and REQ-TR-4 from the `traps` feature spec. The traps system MUST check for mode ID `"case_it"`.
- [ ] Run Point (`run_point`)
  - REQ-EXP-25: MUST grant +1 circumstance bonus to Initiative for all other players in the same room at combat start.
  - REQ-EXP-26: The Run Point player MUST NOT receive the bonus themselves.
  - REQ-EXP-27: Room co-location MUST be evaluated at Initiative roll time, not at mode-set time.
- [ ] Shadow (`shadow`)
  - REQ-EXP-28: MUST require a valid player name in the same room at nomination time ("valid player" = any other connected player in the same room).
  - REQ-EXP-29: MUST use the ally's skill rank when higher than the player's rank.
  - REQ-EXP-30: MUST use the player's own rank when the ally's rank is equal or lower.
  - REQ-EXP-31: MUST suspend silently when the target player is not in the same room, including when disconnected.
  - REQ-EXP-32: MUST resume automatically when the target player re-enters the player's room.
- [ ] Poke Around (`poke_around`)
  - REQ-EXP-33: MUST make a secret Recall Knowledge check on room entry.
  - REQ-EXP-34: Skill and DC MUST be selected based on `world.Room.Properties["context"]` (values: `"history"`→Intel/15, `"faction"`→Conspiracy or Factions/17, `"technology"`→Tech Lore/16, `"creature"`→Wasteland/14, unset→Intel/15).
  - REQ-EXP-35: Success MUST reveal one lore fact; critical success MUST reveal two facts.
  - REQ-EXP-36: Failure MUST reveal nothing; false information MUST NOT be generated.
  - REQ-EXP-37: Conspiracy vs Factions tie MUST use Conspiracy.
- [ ] Hook integration
  - REQ-EXP-38: Room-entry hook MUST fire after the room description is sent. Results MUST be console messages; the room view struct MUST NOT be modified.
  - REQ-EXP-39: Combat-start hook MUST fire before Initiative order is finalized.
  - REQ-EXP-40: Lay Low MUST be cleared from `ExploreMode` when combat starts, before other hooks are processed.
