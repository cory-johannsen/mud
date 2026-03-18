# Passive Tech Mechanics Design

**Date:** 2026-03-18
**Sub-project:** Passive Tech Mechanics

---

## Goal

Make `seismic_sense` fire automatically whenever room state changes (player enters/exits, NPC repopulates, item respawns), notifying every player in the room who has that innate tech. `moisture_reclaim` remains explicit-activation only (its PF2E source, Create Water, is a 2-action spell, not a passive sense).

---

## Context

`seismic_sense` maps to PF2E **Tremorsense** — a passive, always-on imprecise sense that detects ground vibrations. It should require no player action. Currently it is an innate tech with `action_cost: 1`, fired only via the `use` command.

`moisture_reclaim` maps to PF2E **Create Water (Spell 1)** — an explicit 2-action activation. It stays as-is.

---

## Design

### REQ-PTM1
`TechnologyDef` MUST have a `Passive bool` field, serialized as `passive: true/false` in YAML. Default (absent) is `false`.

### REQ-PTM2
If `Passive` is `true`, `TechnologyDef.Validate()` MUST reject any definition where `ActionCost != 0`. A passive tech costs no actions.

### REQ-PTM3
`seismic_sense.yaml` MUST set `passive: true` and `action_cost: 0`. All other technology YAML files are unchanged.

### REQ-PTM4
A `triggerPassiveTechsForRoom(roomID string)` method MUST be added to `GameServiceServer`. It MUST:
- Find all players currently in `roomID` via the session manager
- For each player, iterate their `InnateTechs`
- For each innate tech where `TechnologyDef.Passive == true`, call `activateTechWithEffects` using that player's own session stream
- Never decrement innate tech uses (passive firing is not a use)

### REQ-PTM5
`handleMove` MUST call `triggerPassiveTechsForRoom` for the **destination** room after the player arrives and room broadcast completes, and for the **source** room after the player departs.

### REQ-PTM6
NPC repopulation and item/equipment respawn hooks MUST call `triggerPassiveTechsForRoom` for the affected room when they fire.

### REQ-PTM7
Each player's passive output MUST be streamed to their own session stream (`sess.Entity`), not to the triggering player's stream.

---

## Files Changed

| File | Change |
|------|--------|
| `internal/game/technology/model.go` | Add `Passive bool` to `TechnologyDef`; update `Validate()` |
| `internal/game/technology/model_test.go` | Add REQ-PTM1/PTM2 tests; YAML round-trip for `seismic_sense` |
| `content/technologies/innate/seismic_sense.yaml` | Add `passive: true`, set `action_cost: 0` |
| `internal/gameserver/grpc_service.go` | Add `triggerPassiveTechsForRoom`; call from `handleMove`; call from NPC repop and item respawn hooks |
| `internal/gameserver/grpc_service_passive_test.go` | New: REQ-PTM4–PTM7 tests |

No schema changes. No new commands. No new proto messages.

---

## Testing

- **REQ-PTM1/PTM2 (unit)**: `Passive: true` + `action_cost > 0` fails validation; `Passive: true` + `action_cost == 0` passes; YAML round-trip preserves `passive: true` on `seismic_sense`
- **REQ-PTM4 (unit)**: `triggerPassiveTechsForRoom` fires passive techs for all players with passive innate tech; players without passive techs unaffected
- **REQ-PTM5 (integration)**: Player with `seismic_sense` moves into a room — receives utility message; second player with `seismic_sense` already in room — also receives message
- **REQ-PTM7 (unit)**: Output goes to each player's own stream, not the triggering stream

---

## Non-Goals

- `moisture_reclaim` passive behavior (PF2E source is explicit-activation)
- Tick-based periodic passive evaluation (event-driven is sufficient)
- Passive prepared or spontaneous techs (innate only for now)
- Passive feat or class feature mechanics (separate concern)
