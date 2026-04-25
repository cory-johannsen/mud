---
title: Move Trait for Weapons
issue: https://github.com/cory-johannsen/mud/issues/253
date: 2026-04-24
status: spec
prefix: WMOVE
depends_on: []
related:
  - "#244 Reactions and Ready actions (TriggerOnEnemyMoveAdjacent — must be suppressed)"
  - "#251 Smarter NPC movement in combat (NPC must opt-in to free Stride)"
  - "#262 Agile weapons / MAP (sister-trait surfacing)"
---

# Move Trait for Weapons

## 1. Summary

Add a weapon-level **`move`** trait that grants the wielder a single free Stride (up to their full Speed) tied to a Strike with that weapon. The free Stride does **not** trigger reactions, does **not** count against the 2-AP movement cap, and does **not** consume an action slot from the 3-AP turn budget. The Stride may be taken **immediately before or after** the Strike; the choice is the wielder's.

Today, weapons already carry a `Traits []string` field (`internal/game/inventory/weapon.go:48`) populated from YAML, but no traits drive runtime behavior — they are display-only. The Strike pipeline (`grpc_service.go:handleStrike` → `combatH.Strike` → `ActionStrike` queued → `round.go:ResolveAttack`) and the reactive-strike fire path (`round.go:CheckReactiveStrikes`, fires on `TriggerOnEnemyMoveAdjacent` from spec #244) both run unaware of weapon traits. There is also no "free action" concept in the action economy — closest is `ActionPass` (cost 0, but explicitly forfeits the rest of the turn).

This spec adds:
- A trait registry recognizing `move` (with room to grow as the surface that drives sister traits like `agile`, `reach`, `propulsive`, etc., come online).
- A free-action concept, narrowly scoped to Stride-from-Strike for v1, that bypasses the 3-AP and 2-AP caps and the AP queue.
- A reaction-suppression flag that propagates from the trait-driven Stride into the existing `CheckReactiveStrikes` path so threatened-cell movement does not provoke.
- Player UX: an explicit prompt or auto-flow on web, a `move-strike` / `strike-move` command shape on telnet, with an "always auto-prompt" preference.
- NPC integration: when #251 lands, the smarter-movement step recognizes the free Stride as a bonus opportunity worth scoring.
- Content: tag at least one weapon with `move` as the canonical exemplar so the trait exercises end-to-end.

## 2. Goals & Non-Goals

### 2.1 Goals

- WMOVE-G1: A `move` trait declared on a weapon's `traits` list grants its wielder a single free Stride tied to each Strike with that weapon.
- WMOVE-G2: The free Stride does not trigger reactions, does not count against `MaxMovementAP`, and does not consume one of the wielder's 3 turn AP.
- WMOVE-G3: The Stride may occur immediately before *or* after the Strike at the wielder's choice.
- WMOVE-G4: Trait recognition lives in a registry, not a string-comparison spread across the codebase.
- WMOVE-G5: Both player surfaces (telnet, web) can opt the trait in or out per Strike with one click / one command.
- WMOVE-G6: NPCs equipped with a `move`-trait weapon can take the free Stride via the existing HTN planner and the (pending) `chooseMoveDestination` step from #251.
- WMOVE-G7: Existing combat tests pass unchanged.

### 2.2 Non-Goals

- WMOVE-NG1: Implementing every PF2E weapon trait. This spec only ships `move` plus the trait-registry plumbing that future traits will reuse.
- WMOVE-NG2: Step-only variants. PF2E's strict reading allows Step (5-ft non-provoking) or Stride; v1 ships Stride only because Step and Stride share the same reaction-suppression on this trait. A future spec can split if needed.
- WMOVE-NG3: Multi-Strike chaining (e.g., a `move`-trait weapon used during `ActionStrike` which is two attacks). v1 grants the free Stride **once per strike action**, even when the action contains multiple attacks. PF2E rules allow once per Strike; the implementer MUST confirm the user's preferred reading before locking it in.
- WMOVE-NG4: Diagonal-cost geometry changes. Movement still uses Chebyshev distance and `SpeedFt / 5` cells per stride.
- WMOVE-NG5: Free-action generalization beyond Stride-from-Strike. The free-action plumbing introduced here is narrowly scoped; broader free-action support (e.g., interact, drop) is a follow-on.
- WMOVE-NG6: Authoring UI for tagging weapons with traits. YAML is the source of truth.

## 3. Glossary

- **Move trait**: the string `move` in a weapon's `traits` list.
- **Free Stride**: a Stride that bypasses AP accounting and reaction triggers, granted by a Move-trait Strike.
- **Strike action**: the existing `combat.ActionStrike` (or `ActionAttack` for single-attack variants) that consumes 2 AP (or 1 AP) and resolves an attack against a target.
- **Trait registry**: a single Go map (`map[string]TraitBehavior`) declaring the runtime behavior associated with each recognized weapon trait.
- **Suppress-reactions flag**: a boolean carried on a `MoveContext` struct that flows into `CheckReactiveStrikes` and gates the trigger fire.

## 4. Requirements

### 4.1 Trait Registry

- WMOVE-1: A new package `internal/game/inventory/traits/` MUST provide `Trait` constants (string-typed) and a `Registry` that maps each trait id to its behavior metadata.
- WMOVE-2: The constant `traits.Move = "move"` MUST be added.
- WMOVE-3: The registry MUST initially list `move` only. A `Behavior` struct MUST include at least: `id string`, `display_name string`, `description string`, `grants_free_action ActionType` (zero value = none), `suppresses_reactions bool`.
- WMOVE-4: Every trait listed in any `WeaponDef.Traits` slice MUST be validated against the registry at server startup. Unknown traits MUST log a warning (not error) so authoring can continue while traits are added incrementally.
- WMOVE-5: Helper `func (w *WeaponDef) HasTrait(id string) bool` MUST be added on `WeaponDef`. All trait-driven runtime checks MUST use this helper, not `slices.Contains` ad-hoc.

### 4.2 Free Action Plumbing

- WMOVE-6: A new `ActionType` constant `ActionMoveTraitStride` MUST be added in `internal/game/combat/action.go` between the existing constants. Its `Cost()` MUST return 0.
- WMOVE-7: `ActionMoveTraitStride` MUST NOT consume from `ActionQueue.remaining` and MUST NOT increment `ActionQueue.movementAPSpent`. A new method `ActionQueue.QueueFreeAction(QueuedAction)` MUST be added that bypasses the existing AP-deduction logic for this action type only.
- WMOVE-8: Calling `QueueFreeAction` with any `ActionType` other than the explicit free-action allowlist MUST return an error. The v1 allowlist is `{ActionMoveTraitStride}`.
- WMOVE-9: A `QueuedAction` of type `ActionMoveTraitStride` MUST carry the same `Direction` / `TargetX` / `TargetY` fields used by `ActionStride` and `MoveToRequest` so the existing per-cell stride loop in `round.go:ResolveRound` can execute it without modification.
- WMOVE-10: Once per Strike, only one free-action Stride MAY be queued. Attempting to queue a second free Stride from the same Strike MUST be rejected by the handler with a clear error.

### 4.3 Reaction Suppression

- WMOVE-11: A new struct `MoveContext` MUST be introduced (likely in `internal/game/combat/`) carrying `actor *Combatant`, `from Cell`, `to Cell`, `cause MoveCause` (enum: `Stride`, `MoveTo`, `MoveTrait`, `Forced`).
- WMOVE-12: `CheckReactiveStrikes` (`internal/game/combat/round.go:32-100`) MUST be refactored to take a `MoveContext` parameter. When `ctx.cause == MoveTrait`, `CheckReactiveStrikes` MUST be a no-op for the entire move and return immediately.
- WMOVE-13: All other movement causes (`Stride`, `MoveTo`, `Forced`) MUST behave exactly as today — no test for those paths regresses.
- WMOVE-14: Any other reaction trigger that keys on movement (e.g., a future `TriggerOnLeaveCell`) MUST also consult `MoveContext.cause` and skip when `MoveTrait`. The trait-registry `Behavior.suppresses_reactions` flag is the single source of truth.

### 4.4 Strike Pipeline Integration

- WMOVE-15: `combatH.Strike(uid, target)` and the parallel `combatH.handleAttack` paths MUST inspect the wielder's equipped weapon and, when `weapon.HasTrait(traits.Move)`, mark the resulting `QueuedAction` with a `MoveTraitWielder bool = true` flag.
- WMOVE-16: After a Strike-action resolves in `round.go:ResolveRound`, if `MoveTraitWielder` is true and a free Stride has been queued via WMOVE-7, that Stride MUST execute under a `MoveContext{cause: MoveTrait}`.
- WMOVE-17: When the wielder requests the free Stride **before** the Strike, the Stride executes first; the Strike then resolves from the new position. When requested **after**, the Strike resolves first; the Stride executes from the post-Strike position. Both orderings MUST be supported.
- WMOVE-18: If the Strike misses, fumbles, or critically fails, the wielder MUST still be permitted the free Stride. The trait grant is action-coupled, not result-coupled.
- WMOVE-19: If the wielder is incapacitated (down, paralyzed, etc.) between the Strike and the queued free Stride, the Stride MUST NOT execute and the queued action MUST be discarded silently.

### 4.5 Player UX — Telnet

- WMOVE-20: New telnet commands MUST exist:
  - `move-strike <target> <dx,dy>` — perform a free Stride to cell `(dx,dy)` then Strike `target`.
  - `strike-move <target> <dx,dy>` — Strike `target` then perform a free Stride to cell `(dx,dy)`.
  - `strike <target>` — existing Strike command, unchanged. When the wielder's weapon has the Move trait and the player has set the auto-prompt preference, the server MUST emit a follow-up prompt: `Move-trait Stride available. Specify destination cell or "skip".`
- WMOVE-21: A new player setting `move_trait_auto_prompt` (default `true`) MUST control whether plain `strike` triggers the post-Strike prompt. The setting is per-account and persists across sessions.
- WMOVE-22: The free-Stride prompt MUST time out after 30 seconds of no input and auto-`skip`. Combat resolution MUST NOT block on the prompt indefinitely.

### 4.6 Player UX — Web

- WMOVE-23: When the player selects an action button for a Strike with a Move-trait weapon, the web action bar MUST display a small `MOVE` badge on the button (sourced from the trait registry's `display_name`).
- WMOVE-24: Clicking a Move-trait Strike button MUST open a two-stage placement UI:
  - Stage 1 — Stride placement: highlight legal Stride-destination cells using the existing AoE-preview cell-highlight pattern from spec #250 (different tint to distinguish from AoE), and a "Skip Stride" button.
  - Stage 2 — Target selection: existing target picker.
  Players MAY swap stages with a "Stride after Strike" toggle.
- WMOVE-25: Each placement stage MUST be cancellable independently (Esc on Stage 1 returns to Stage 2 with no Stride; Esc on Stage 2 cancels the entire action and refunds nothing — no AP has been spent).
- WMOVE-26: The web client MUST mirror the server's "legal Stride cells" computation (= `SpeedSquares()` Chebyshev radius from current cell, minus blocked / occupied cells) for preview only. Server is authoritative.

### 4.7 NPC Integration

- WMOVE-27: HTN planner integration: a new HTN action `move_trait_stride{x, y}` MUST be exposed alongside `move_to_position` (spec #251). HTN domains MAY queue this as a free Stride before/after a Strike when the active weapon has the trait.
- WMOVE-28: The default NPC fallback (legacy `legacyAutoQueueLocked`) MUST detect a Move-trait wielder and, when `chooseMoveDestination` (#251) returns a destination cell, prefer to spend that movement as a free Stride (saving AP for additional Strikes) instead of as a normal `ActionStride`.
- WMOVE-29: When an NPC's free Stride and normal Stride both want different destinations in the same turn, the NPC MUST take the free Stride first to its preferred cell, then evaluate whether normal Stride is still useful. The implementation MUST NOT spend movement AP twice for the same target cell.

### 4.8 Content

- WMOVE-30: At least one existing weapon in `content/weapons/` MUST be tagged with `move` as the canonical exemplar. The implementer MUST confirm the choice with the user before tagging; suggested candidates are reach polearms or whip-class weapons that historically embody "step in, hit, step out".
- WMOVE-31: A tagged weapon MUST also surface its trait in its `description` field so the in-game inventory and shop UI document the behavior to players. A future trait-tooltip system can replace this manual step.

### 4.9 Tests

- WMOVE-32: Existing tests under `internal/gameserver/` and `internal/game/combat/` MUST pass unchanged.
- WMOVE-33: A new test file `internal/game/inventory/traits/registry_test.go` MUST cover registry validation: known trait → behavior present; unknown trait → warning logged; `WeaponDef.HasTrait` true/false.
- WMOVE-34: A new test file `internal/game/combat/move_trait_test.go` MUST cover scenarios:
  - Strike with Move-trait weapon then free Stride into a previously threatened cell — no Reactive Strike fires.
  - Strike with non-Move weapon then `ActionStride` into the same cell — Reactive Strike fires (regression guard).
  - Free Stride distance limited to `SpeedSquares()`; attempting longer is rejected.
  - Free Stride does NOT decrement remaining AP; a follow-up `ActionAttack` succeeds at correct AP cost.
  - Once-per-Strike enforcement — second free-Stride attempt rejected.
  - Stride-before-Strike vs Stride-after-Strike both produce correct final position and damage resolution.
  - Wielder downed between Strike and free Stride — Stride is silently discarded.
- WMOVE-35: Property tests under `internal/game/combat/testdata/rapid/TestMoveTrait_Property/` MUST cover:
  - Determinism: same combat state + same input → same final positions.
  - Boundedness: free Stride destination is always within `SpeedSquares()` cells.
  - AP-respect: total AP spent on a Move-trait turn ≤ 3.
  - Reaction-suppression invariant: no `Reactive Strike` event is emitted for a `MoveContext{cause: MoveTrait}` move.

## 5. Architecture

### 5.1 Where the new code lives

```
internal/game/inventory/
  weapon.go                    # existing; HasTrait helper added
  traits/
    registry.go                # NEW: Trait constants, Registry, Behavior struct
    registry_test.go           # NEW

internal/game/combat/
  action.go                    # existing; ActionMoveTraitStride const + QueueFreeAction
  move_context.go              # NEW: MoveContext struct + MoveCause enum
  round.go                     # existing; CheckReactiveStrikes refactored to take MoveContext
  move_trait_test.go           # NEW
  testdata/rapid/TestMoveTrait_Property/   # NEW

internal/gameserver/
  combat_handler.go            # existing; Strike marks MoveTraitWielder; queues free Stride per request
  grpc_service.go              # existing; handleStrike + new handleMoveStrike / handleStrikeMove
                               # ListCombatActions (from #252) reports the Move badge metadata

internal/game/ai/
  domain.go                    # existing; add `move_trait_stride{x,y}` action shape
  build_state.go               # existing; surface MoveTraitWielder in WorldState

cmd/webclient/ui/src/game/
  panels/MapPanel.tsx          # two-stage placement UI for Move-trait Strikes
  combat/                      # ActionBar entries get MOVE badge

content/weapons/
  <chosen weapon>.yaml         # add `move` to traits + description text

api/proto/game/v1/game.proto
  StrikeRequest gains optional `move_trait_stride` message: { destination Cell, ordering enum BEFORE/AFTER, skip bool }
```

### 5.2 Strike + Free Stride flow

```
client request: StrikeRequest{ target, move_trait_stride: {dest, ordering, skip} }
   │
   ▼
handleStrike (grpc_service.go)
   │
   ├── validate target, weapon equipped
   ├── weapon.HasTrait("move")? → set MoveTraitWielder=true
   │
   ├── if move_trait_stride present and !skip:
   │       queue.QueueFreeAction(QueuedAction{Type: ActionMoveTraitStride, TargetX, TargetY, ordering})
   │       (rejects unless wielder has Move trait — defense in depth)
   │
   ├── queue.Enqueue(QueuedAction{Type: ActionStrike, Target, MoveTraitWielder})
   │
   └── resolveAndAdvanceLocked → round.go:ResolveRound
                │
                ▼
        for each queued action in turn order:
          if Type == ActionMoveTraitStride and ordering == BEFORE → execute Stride under MoveContext{cause:MoveTrait}
          execute Strike (resolveAttack)
          if ActionMoveTraitStride and ordering == AFTER       → execute Stride under MoveContext{cause:MoveTrait}
                │
                ▼
        CheckReactiveStrikes(ctx) — early return when ctx.cause == MoveTrait
```

### 5.3 Single sources of truth

- Trait runtime behavior: `internal/game/inventory/traits/registry.go`.
- Free-action allowlist and AP-bypass logic: `ActionQueue.QueueFreeAction` only.
- Reaction suppression: `MoveContext.cause` + trait registry `Behavior.suppresses_reactions`.
- Speed and stride cell-count: existing `Combatant.SpeedSquares()`.

## 6. Open Questions

- WMOVE-Q1: PF2E's `move` trait actually marks **actions** that involve movement (used so other rules can key on movement, e.g., "this triggers when an enemy uses a move action"), not weapons. The issue text reinterprets it as a weapon-level free Stride. Should the trait id be `move` (collides with PF2E semantics) or something Mud-native like `mobile_strike` / `kiting` / `mobility`? Recommendation: rename the trait id to `mobile` to avoid collision; keep YAML migration trivial and add an alias in the registry that maps `move` → `mobile` for any content already authored with the old name.
- WMOVE-Q2: Does the free Stride trigger AoE-template "you-have-moved" effects (e.g., a hypothetical *Web* zone)? PF2E says non-reaction effects still fire. Recommendation: yes — only `MoveContext` reaction-trigger consumers are suppressed; AoE persistent effects evaluate independently.
- WMOVE-Q3: When a multi-Strike action like `ActionStrike` (two attacks at MAP) uses a Move-trait weapon, does the wielder get one free Stride or two? PF2E says one per Strike "action" (the umbrella action). Recommendation: one. Locked in WMOVE-NG3 unless the user prefers per-attack semantics.
- WMOVE-Q4: When a wielder dual-wields a Move-trait weapon and a non-Move weapon and uses an off-hand attack, does the Strike confer a free Stride? Recommendation: only when the *attacking* weapon has the trait.
- WMOVE-Q5: Telnet's two-stage prompt for `strike` requires a synchronous reply within 30 s. If the player AFKs mid-prompt, does the Strike still resolve at the end of the round, or wait until the prompt times out? Recommendation: queue the Strike immediately, fold a "skip Stride" outcome into the queued action so the round can resolve while the player is offline. The Stride opportunity is forfeited if the prompt times out.

## 7. Acceptance

- [ ] All existing tests pass without modification.
- [ ] `WeaponDef.HasTrait("move")` returns true for the tagged exemplar weapon and false for every other weapon in `content/weapons/`.
- [ ] A Strike with the tagged weapon followed by a free Stride into a threatened cell produces no Reactive Strike event in the combat log.
- [ ] An identical Strike with a non-Move weapon produces a Reactive Strike event when followed by `ActionStride` into the same cell.
- [ ] The free Stride distance is bounded by `SpeedSquares()`; an over-distance request is rejected with a clear error and consumes no AP.
- [ ] After a Move-trait Strike, the wielder's remaining AP equals the AP they would have had with a non-Move Strike.
- [ ] Telnet `strike` command emits the Stride prompt when `move_trait_auto_prompt = true` and skips it when `false`.
- [ ] Web action-bar Strike with a Move-trait weapon shows the `MOVE` badge and presents the two-stage placement UI on click.
- [ ] HTN domains can queue `move_trait_stride{x,y}` and the handler honors it.
- [ ] At least one combat scenario test demonstrates an NPC with a tagged weapon executing the free Stride end-to-end.

## 8. Out-of-Scope Follow-Ons

- WMOVE-F1: Other PF2E weapon traits (`agile`, `finesse`, `reach`, `sweep`, `forceful`, `propulsive`, `shove`, `trip`, `disarm`, etc.) — each gets its own ticket but reuses the registry.
- WMOVE-F2: General free-action support (free Interact, free Drop, free Speak) — generalize `QueueFreeAction`'s allowlist when more callers exist.
- WMOVE-F3: A trait-tooltip system that surfaces every weapon trait's behavior in the inventory UI from the registry instead of from manually authored description text.
- WMOVE-F4: Multi-Strike per-attack free Stride variant (per WMOVE-Q3 if user prefers).
- WMOVE-F5: A "wielder-of-Move-trait gains opportunity to react to enemy movement with a free Strike" inversion — separate design, not implied by the PF2E rule.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/253
- Weapon model and traits field: `internal/game/inventory/weapon.go:38-67` (`Traits []string` at line 48)
- Weapon load: `internal/game/inventory/weapon.go:149-180`
- Strike entry: `internal/gameserver/grpc_service.go:3885-3909` (`handleStrike`)
- Combat handler Strike: `internal/gameserver/combat_handler.go:721-761`
- Round resolution: `internal/game/combat/round.go` (`ResolveRound`, `CheckReactiveStrikes:32-100`)
- Reaction trigger: `internal/game/reaction/trigger.go:18` (`TriggerOnEnemyMoveAdjacent`)
- Action types: `internal/game/combat/action.go:13-29`
- AP economy: `internal/game/combat/action.go:120` (`MaxMovementAP`), `:141-150` (`DeductAP`)
- Speed: `internal/game/combat/combat.go:178-188` (`SpeedSquares`)
- Reactions spec: `docs/superpowers/specs/2026-04-21-reactions-and-ready-actions.md`
- Smarter NPC movement spec: `docs/superpowers/specs/2026-04-24-smarter-npc-movement-in-combat.md`
- AoE-preview pattern (cell highlight): `docs/superpowers/specs/2026-04-24-aoe-drawing-in-combat.md`
- PF2E trait reference: `vendor/pf2e-data/src/scripts/config/traits.ts` (search `move:`)
