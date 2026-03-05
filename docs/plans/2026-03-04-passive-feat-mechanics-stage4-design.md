# Passive Feat Mechanics — Stage 4 Design

**Date:** 2026-03-04

## Goal

Implement mechanical effects for the four passive class features: `sucker_punch`, `predators_eye`, `street_brawler`, and `zone_awareness`. Add the `on_passive_feat_check` Lua hook. Add favored target type selection to character creation.

---

## Shared Infrastructure

### A) Passive feat cache on `PlayerSession`

`PlayerSession` gains `PassiveFeats map[string]bool` populated at login from `character_class_features` (and `character_feats`) filtered to `active: false` entries. Combat code checks `sess.PassiveFeats["sucker_punch"]` directly, avoiding per-attack DB queries.

### B) `DamageBonus` wired into `ResolveRound`

`condition.DamageBonus(actor.conditions)` exists in `internal/game/condition/modifiers.go` but is never called from `internal/game/combat/round.go`. It is added to the per-attack damage calculation alongside the existing `AttackBonus`/`ACBonus` calls.

---

## The Four Passives

### `sucker_punch` (criminal archetype)

- **Trigger:** attacker has `sucker_punch` passive AND target has `flat_footed` condition
- **Effect:** +1d6 bonus damage on the attack
- **`flat_footed` lifecycle:**
  - Applied to every NPC when they enter combat (`startCombatLocked`)
  - Cleared from an NPC after their first action resolves in `ResolveRound`
  - `duration_type: rounds`, `max_stacks: 0`
  - New condition YAML: `content/conditions/flat_footed.yaml`

### `predators_eye` (drifter archetype)

- **Trigger:** attacker has `predators_eye` passive AND `target.NPCType == sess.FavoredTarget`
- **Effect:** +1d8 precision damage on the attack
- **Favored target storage:**
  - New DB table: `character_favored_target (character_id BIGINT PK, target_type TEXT NOT NULL)`
  - `PlayerSession.FavoredTarget string` loaded at login
  - Valid types: `human`, `robot`, `animal`, `mutant`
- **Character creation:** if `predators_eye` is in the character's class features, player is prompted to select from the list before finishing creation
- **Existing characters:** if `predators_eye` is in features but no favored target exists, prompt at login

### `street_brawler` (aggressor archetype)

- **Trigger:** an NPC executes the flee action in `CombatHandler`
- **Effect:** before the NPC is removed from combat, each player in the room with `street_brawler` gets one free `ResolveAttack` against the fleeing NPC; result is broadcast
- **Implementation site:** `CombatHandler` flee path (where `h.engine.EndCombat` or flee resolution fires), after flee is confirmed

### `zone_awareness` (drifter archetype)

- **Trigger:** player moves into a room with `Properties["terrain"] == "difficult"`
- **Effect:**
  - Without `zone_awareness`: player receives flavor message ("The difficult terrain slows your movement.")
  - With `zone_awareness`: message is suppressed; no mechanical penalty in Stage 4
- **Implementation site:** movement handler in `grpc_service.go` after `MoveWithContext` succeeds

---

## Lua Hook

### `on_passive_feat_check(uid, feat_id, context)`

- **Fired:** once per passive feat evaluation in `ResolveRound` (whether or not the condition is met)
- **`context` table keys:** `target_uid`, `damage_bonus`, `outcome` (met/not_met), `target_type`
- **Return value:** integer to override `damage_bonus`, or `nil` to accept the default
- **Call site:** `internal/game/combat/round.go`, after each passive bonus is calculated, via `scripting.Manager.CallHook`

---

## Data Model Changes

| Change | Location |
|--------|----------|
| `PlayerSession.PassiveFeats map[string]bool` | `internal/game/session/manager.go` |
| `PlayerSession.FavoredTarget string` | `internal/game/session/manager.go` |
| `character_favored_target` DB table | new migration |
| `flat_footed` condition YAML | `content/conditions/flat_footed.yaml` |
| `DamageBonus` called in `ResolveRound` | `internal/game/combat/round.go` |
| Passive feat cache populated at login | `internal/gameserver/grpc_service.go` |
| Favored target loaded at login | `internal/gameserver/grpc_service.go` |
| Favored target prompt at character creation | `internal/gameserver/grpc_service.go` |

---

## Out of Scope (Stage 4)

- Mechanical speed penalty for difficult terrain (message only)
- `street_brawler` AoO on room-exit movement (flee action only)
- Stealth/hidden player state
- `sucker_punch` and `predators_eye` stacking with each other (each is independent)
