---
title: Take Cover Action
issue: https://github.com/cory-johannsen/mud/issues/266
date: 2026-04-25
status: spec
prefix: TKCV
depends_on:
  - "#247 Cover bonuses in combat (positional cover tier per attack)"
related:
  - "#252 Non-combat actions (action invocation pattern)"
  - "#244 Reactions and Ready actions (action expiry on triggers)"
---

# Take Cover Action

## 1. Summary

A `TakeCover` handler already exists at `internal/gameserver/grpc_service.go:10086-10176`. It picks the best room-cover equipment, applies a tier-named condition (`lesser_cover` / `standard_cover` / `greater_cover`), spends 1 AP, tracks cover HP, and clears on stride. This pre-dates the PF2E-aligned cover model from #247 and uses *room-equipment-derived* tiers rather than *positional* tiers.

The PF2E **Take Cover** action is different:

- It requires the character to *already* be receiving at least lesser cover from something (positional or equipment).
- It *elevates* the cover tier by one rank (Lesser → Standard → Greater; Greater unchanged).
- It expires when the character moves OR attacks (today's implementation only expires on move).
- It composes with the per-attack positional cover model from #247 — the cover routine returns the base tier, the elevation condition adds one rung, and the resulting bonus flows through the typed-bonus pipeline (#245 / #259).

This spec migrates the existing handler to the PF2E semantics, integrates with #247's positional tier, adds the expire-on-attack trigger, and adjusts UX so the action is gated on having actual cover. Existing tier-named conditions (`lesser_cover`, etc.) become legacy and are replaced by a single `taking_cover` condition that holds the elevation delta.

## 2. Goals & Non-Goals

### 2.1 Goals

- TKCV-G1: `TakeCover` requires the character to be receiving at least lesser cover from any source at action time; otherwise it fails with a clear narrative ("There's nothing to take cover behind.").
- TKCV-G2: `TakeCover` applies a `taking_cover` condition that adds +1 to the effective cover tier, capped at Greater.
- TKCV-G3: The `taking_cover` condition expires when the character moves (any movement action) OR attacks (any Strike, Attack, throw, ranged action), or when combat ends.
- TKCV-G4: The elevation composes with #247's positional cover at attack-resolution time: `effectiveTier = min(Greater, base + (taking_cover ? 1 : 0))`.
- TKCV-G5: Existing `lesser_cover` / `standard_cover` / `greater_cover` conditions are deprecated; the migration replaces them with `taking_cover` and reads positional cover from #247.
- TKCV-G6: Telnet `take cover` command and a web action-bar button surface the action; the button is greyed out / the command rejects when no qualifying cover is present.

### 2.2 Non-Goals

- TKCV-NG1: Implementing the positional cover model itself — that's #247.
- TKCV-NG2: A separate "improve cover" action when already at Greater (PF2E says no further improvement is possible).
- TKCV-NG3: Cover-from-cover (cover that grants additional cover when destroyed). Out of scope.
- TKCV-NG4: An "uncertainty cover" mechanic for darkness or smoke — that's #267 (visibility / LOS).
- TKCV-NG5: Cover-related feats (`Diehard`, `Reactive Cover`, etc.). Future tickets.
- TKCV-NG6: Auto-take-cover when the character first enters a position with cover — explicit player action only.

## 3. Glossary

- **Take Cover**: the 1-AP action this spec specifies.
- **`taking_cover` condition**: the per-character condition applied by Take Cover; carries the +1 elevation delta.
- **Base cover tier**: the tier returned by #247's positional routine for the (attacker, target) pair at attack time.
- **Effective cover tier**: `min(Greater, base + 1 if taking_cover else base)`.
- **Cover-bearing source**: any positional cover (per #247) or any cover-emitting condition the target currently has.

## 4. Requirements

### 4.1 Action Definition

- TKCV-1: A new combat `ActionType` constant `ActionTakeCover` MUST be added in `internal/game/combat/action.go` between the existing constants. Its `Cost()` MUST return `1`.
- TKCV-2: The existing `handleTakeCover` (`grpc_service.go:10086-10176`) MUST be migrated to:
  - Validate the character is currently receiving at least `Lesser` cover from any source. The check MUST consult #247's positional tier function for the (most-recent-attacker, this-character) pair. When no recent attacker exists (start of round), the check considers the *worst-case* tier (i.e., requires cover against at least one potential attacker).
  - Reject when no qualifying cover with the narrative "There's nothing to take cover behind." and consume no AP.
  - Spend 1 AP via the existing `SpendAP` helper.
  - Apply the `taking_cover` condition (TKCV-3) to the character.
  - Emit narrative "Kira takes cover behind <source-name>." on success, naming the highest-tier cover source for flavor.
- TKCV-3: A new condition `taking_cover` MUST be authored in `content/conditions/taking_cover.yaml` per spec #252's NCA-2 catalog shape, declaring no direct stat bonuses (the elevation logic is handled in code at TKCV-7). The condition's effects MAY include a `tag: detection_state_neutral` so the detection layer (#254) treats Take Cover-induced cover as opaque-to-attacker per the standard cover rules.

### 4.2 Expiry Triggers

- TKCV-4: When the character moves (any of `ActionStride`, `ActionMoveTraitStride` per #253, `MoveToRequest`, forced movement), the `taking_cover` condition MUST be removed and the narrative "Kira's cover is broken." emitted.
- TKCV-5: When the character takes any attack action (`ActionAttack`, `ActionStrike`, `ActionFireBurst`, `ActionFireAutomatic`, `ActionThrow`, `ActionUseAbility` if it includes an attack roll, `ActionUseTech` if it includes an attack roll), the `taking_cover` condition MUST be removed *after* the attack resolves. (After the attack benefits from non-elevation, but a follow-on attack in the same turn does not.) The attack itself MUST NOT benefit from the elevation since the act of attacking ends the cover; this is a deliberate PF2E-aligned simplification.
  - **Alternative interpretation reserved**: PF2E says "until you move, attack, or end your turn" — the *first* attack still benefits, then ends the condition. The implementer MUST confirm with the user which reading to ship before locking in TKCV-5; recommendation is the PF2E-strict reading (first attack benefits, then condition ends).
- TKCV-6: When combat ends, the `taking_cover` condition MUST be removed silently.

### 4.3 Cover Tier Composition

- TKCV-7: `getEffectiveCoverTier(target, attacker)` MUST be added next to #247's positional cover routine. Implementation:
  ```
  base := positional_cover_tier(target, attacker)   // from #247
  if base == None and target has taking_cover:
      // elevation requires base cover; return None
      return None
  if target has taking_cover:
      return min(Greater, base + 1)
  return base
  ```
- TKCV-8: The attack resolver MUST call `getEffectiveCoverTier` instead of consulting the legacy `Combatant.CoverTier` field. The legacy field MUST remain for one release cycle as a back-compat mirror set by `getEffectiveCoverTier` for any non-migrated reader.
- TKCV-9: The cover bonus emitted via the typed-bonus pipeline MUST use the `circumstance` type per spec #259 BTYPE-16 contract.

### 4.4 Legacy Condition Migration

- TKCV-10: The existing tier-named conditions (`lesser_cover`, `standard_cover`, `greater_cover`) MUST be deprecated. The migration plan:
  - When the loader sees one of the legacy condition ids on a character at session-start, it MUST translate it to nothing — the positional cover model from #247 takes over for tier computation; nothing persists.
  - A migration script in the load path MUST clear these conditions from in-flight saves silently. No user-facing migration prompt.
- TKCV-11: After the migration ships, the legacy conditions MAY be removed from `content/conditions/` once no save data references them. The removal MUST be a separate PR after one release cycle.
- TKCV-12: `Combatant.CoverEquipmentID` and `Combatant.CoverTier` fields MAY remain readable for one cycle but MUST NOT be written by the new code path. They become derived from `getEffectiveCoverTier` for any legacy reader.

### 4.5 UX

- TKCV-13: Telnet `take cover` command (existing) MUST continue to work; the handler now applies the new semantics.
- TKCV-14: Web action-bar button for Take Cover MUST appear *only when* the player is receiving at least lesser cover; otherwise it MUST be greyed out with a tooltip "no cover here".
- TKCV-15: The combat status panel (telnet and web) MUST surface the `taking_cover` condition badge so the player knows the elevation is active. The badge clears when expiry fires.
- TKCV-16: Combat-log narrative on apply ("takes cover behind X") and expiry ("cover is broken") MUST be emitted as structured events for clarity.

### 4.6 Tests

- TKCV-17: Existing `handleTakeCover` tests MUST be updated to expect the new "requires existing cover" semantics.
- TKCV-18: New tests in `internal/game/combat/take_cover_test.go` MUST cover:
  - Take Cover with no cover present rejects with narrative and consumes no AP.
  - Take Cover with lesser cover applies `taking_cover`; effective tier becomes Standard for the next attack against the character.
  - Take Cover with Standard cover yields Greater; with Greater stays Greater.
  - Stride after Take Cover removes the condition with narrative.
  - First attack after Take Cover benefits from elevation, then condition is removed (per TKCV-5 PF2E-strict reading; adjust if user prefers the other reading).
  - Combat end silently removes the condition.
- TKCV-19: A property test under `internal/game/combat/testdata/rapid/TestTakeCover_Property/` MUST verify that elevation never produces a tier above Greater and that the resolver always reads `getEffectiveCoverTier` (no direct `CoverTier` read).

## 5. Architecture

### 5.1 Where the new code lives

```
internal/game/combat/
  action.go                       # ActionTakeCover constant
  cover.go                        # NEW: getEffectiveCoverTier (next to #247 positional routine)
  take_cover.go                   # NEW: helper for apply + expire wiring
  take_cover_test.go              # NEW
  testdata/rapid/TestTakeCover_Property/   # NEW

internal/gameserver/
  grpc_service.go                 # handleTakeCover migrated to new semantics
  combat_handler.go               # remove SetCombatantCover writes from new path

content/conditions/
  taking_cover.yaml               # NEW
  lesser_cover.yaml, standard_cover.yaml, greater_cover.yaml  # deprecated (one release cycle)
```

### 5.2 Apply / expire flow

```
player: take cover
   │
   ▼
handleTakeCover
   ├── base := positional_cover_tier(player, attacker_or_worst_case)
   ├── if base == None: reject "nothing to take cover behind"; no AP
   ├── SpendAP(player, 1)
   ├── ApplyCondition(player, "taking_cover", duration: encounter)
   ├── narrative "Kira takes cover behind <source>"
   ▼
attack against player (later)
   ├── effective := getEffectiveCoverTier(player, attacker)
   ├── attack resolver uses effective tier; emits circumstance bonus
   ▼
player: any move OR any attack
   ├── RemoveCondition(player, "taking_cover")
   ├── narrative "Kira's cover is broken"
   ▼
combat end
   └── RemoveCondition(player, "taking_cover") silently
```

### 5.3 Single sources of truth

- Effective cover tier: `combat.getEffectiveCoverTier` only.
- Apply / expire: `combat.takeCoverApply` and `combat.takeCoverExpire` helpers.
- Condition catalog: `content/conditions/taking_cover.yaml`.

## 6. Open Questions

- TKCV-Q1: Per TKCV-5, does the *first* attack after Take Cover benefit from the elevation (PF2E-strict) or not (simplified)? Recommendation: PF2E-strict — first attack benefits, then condition ends. Locked pending user confirmation.
- TKCV-Q2: When Take Cover requires "at least one potential attacker has line-of-fire that grants cover", the worst-case computation may be expensive in large fights. Recommendation: in v1, evaluate against the *most recent* attacker; if none in the current combat, allow Take Cover when the player has cover from any cell within `MaxRange` of any enemy. Defer perf if a profile shows a hot path.
- TKCV-Q3: NPCs MUST also be able to Take Cover via the HTN planner. The existing combat-strategy `UseCover` flag (`/home/cjohannsen/src/mud/internal/game/npc/template.go:128`) auto-applies the *legacy* take-cover at turn start. Recommendation: rewire the HTN planner to call the new path; preserve the auto-apply on turn start by inserting a `TakeCover` action at the start of the NPC's queue when `UseCover == true` AND the NPC is in cover. Out-of-scope for this ticket; capture as TKCV-F1.
- TKCV-Q4: The deprecated tier-named conditions are referenced by content. Should the migration script (TKCV-10) write the migration into the database itself, or rely on the load-time translation only? Recommendation: load-time only — keeps the data path simple and lets a future cleanup remove the rows.
- TKCV-Q5: `getEffectiveCoverTier` introduces a circular dependency between cover and the resolver if not placed carefully. Recommendation: place it in the `combat` package alongside #247's positional routine; resolver calls it as a leaf function.

## 7. Acceptance

- [ ] All existing combat tests pass after handler migration.
- [ ] Take Cover with no qualifying cover rejects without consuming AP.
- [ ] Take Cover with lesser cover yields Standard effective tier; Standard yields Greater; Greater unchanged.
- [ ] Stride and any attack action remove the `taking_cover` condition with narrative.
- [ ] First attack after Take Cover benefits from elevation per TKCV-5 (assuming PF2E-strict reading).
- [ ] Combat end removes the condition silently.
- [ ] Telnet `take cover` works; web button greyed out without cover.
- [ ] Legacy tier-named conditions in saves migrate silently to the new system.

## 8. Out-of-Scope Follow-Ons

- TKCV-F1: HTN planner integration for NPC TakeCover (per TKCV-Q3).
- TKCV-F2: Cover feats (`Diehard`, `Reactive Cover`, `Cover Up`).
- TKCV-F3: Cover-from-cover mechanic.
- TKCV-F4: Cover during exploration mode (currently combat-only).
- TKCV-F5: Removal of legacy tier-named conditions one release cycle after this ships.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/266
- Existing handler: `internal/gameserver/grpc_service.go:10086-10176`
- Existing helper: `internal/gameserver/combat_handler.go:1428` (`SetCombatantCover`)
- Cover-clear on stride: `internal/gameserver/grpc_service.go:10803` (`clearPlayerCover` call)
- Cover bonus type contract: `docs/superpowers/specs/2026-04-25-bonus-types.md` BTYPE-16
- Predecessor cover spec: `docs/superpowers/specs/2026-04-21-cover-bonuses-in-combat.md`
- Condition catalog shape: `docs/superpowers/specs/2026-04-24-noncombat-actions-vs-combat-npcs.md` NCA-2
- NPC use-cover flag: `internal/game/npc/template.go:128`
