---
title: Smarter NPC Movement in Combat
issue: https://github.com/cory-johannsen/mud/issues/251
date: 2026-04-24
status: spec
prefix: MOVE
depends_on: []
related:
  - "#247 Cover bonuses in combat (positional cover model)"
  - "#248 Terrain type penalties in combat (TERRAIN-* speed budget)"
  - "#249 Targeting system in combat"
  - "#250 AoE drawing in combat (clustering risk model)"
---

# Smarter NPC Movement in Combat

## 1. Summary

Replace the binary "stride toward / stride away" NPC movement heuristic with a deliberate **per-NPC tactical movement step** that picks a destination cell from the legal-move set using a small set of weighted goals: get into effective range, gain or preserve cover, spread out from allies (AoE clustering avoidance), and avoid difficult / hazardous terrain.

Today, `combat_handler.go:npcMovementStrideLocked` returns one of `"toward"` / `"away"` / `""` based purely on the NPC's `RangeIncrement` and the Chebyshev distance to its target. This produces robotic positioning: melee NPCs always close to adjacent, ranged NPCs always pull back to range 1, and nobody ever sidesteps into cover, around a hazard, or off the centerline of an obvious AoE setup. The fallback `legacyAutoQueueLocked` does no positional reasoning at all.

This spec adds a **`MoveDecision` step** that runs inside the existing per-NPC turn loop. It produces a destination cell (or no-move) that the existing `ActionStride` machinery then executes via the same AP economy that players use, preserving every existing test pinning current behavior at the *direction* layer.

## 2. Goals & Non-Goals

### 2.1 Goals

- MOVE-G1: NPCs choose movement *destinations*, not just *directions*, on each turn.
- MOVE-G2: Movement decisions are scored against a small, transparent set of goals — `range`, `cover`, `spread`, `terrain` — so behavior is debuggable and tunable.
- MOVE-G3: Per-NPC tuning is data-driven via existing `CombatStrategy` fields on `NPCTemplate`; new fields default to values that reproduce today's behavior.
- MOVE-G4: HTN-driven NPCs continue to be controlled by their HTN domain; the new movement step only fires when the planner does not specify a destination.
- MOVE-G5: Movement obeys the same AP/speed rules as players (`MaxMovementAP`, terrain speed budget per #248 once shipped).
- MOVE-G6: All existing NPC movement tests continue to pass without modification.

### 2.2 Non-Goals

- MOVE-NG1: Full A* pathfinding across complex obstacle graphs. Movement chooses a *destination cell* via direct scoring, then strides toward it; multi-cell pathing reuses the existing per-cell stride loop in `round.go`.
- MOVE-NG2: Multi-turn planning, lookahead, or learning. Each turn's decision is myopic.
- MOVE-NG3: Faction coordination (squad maneuvers, flanking pairs, fire-and-maneuver). Each NPC scores independently. Coordination is a follow-on.
- MOVE-NG4: Player-visible AI tuning UI. Tuning is YAML-only.
- MOVE-NG5: Replacing the HTN planner. HTN domains keep ownership of NPCs that declare `AIDomain`.
- MOVE-NG6: Reactive movement on enemy turns (e.g., shifting in response to an aimed AoE). Reactions are owned by #244 / Reactions and are out of scope here.

## 3. Glossary

- **Move step**: the per-turn decision phase that picks an NPC's destination cell.
- **Goal**: a single positional concern (range, cover, spread, terrain). Each is a pure function `Cell → float64`.
- **Score**: weighted sum of goal scores at a candidate cell; higher is better.
- **Candidate set**: cells reachable this turn given remaining AP, speed, and terrain budget; excludes occupied cells and out-of-bounds cells.
- **Threat target**: the combatant the NPC is currently trying to engage / disengage from. Inherited from existing target selection (HTN, faction, or first-living-player fallback).

## 4. Requirements

### 4.1 Decision Step Placement

- MOVE-1: A new function `chooseMoveDestination(cbt *combat.Combat, c *combat.Combatant) *Cell` MUST be added in a new file `internal/gameserver/combat_handler_move.go`. Returning `nil` means "no movement this turn" and MUST behave identically to today's empty-string return from `npcMovementStrideLocked`.
- MOVE-2: `npcMovementStrideLocked` MUST be retained as a thin wrapper that calls `chooseMoveDestination` and converts the destination into the existing `"toward"` / `"away"` direction string for the legacy direction-based stride path. Existing call sites MUST NOT change.
- MOVE-3: When an NPC has `AIDomain` set and the HTN planner returns a queued `ActionStride` with an explicit destination, `chooseMoveDestination` MUST NOT run — HTN intent wins.
- MOVE-4: When an NPC has `AIDomain` set and the HTN planner returns no movement at all, `chooseMoveDestination` MUST run as a default behavior (preserves current behavior where HTN-only NPCs sit still unless their domain says otherwise).
- MOVE-5: For NPCs with no `AIDomain` (legacy fallback path at `combat_handler.go:3979`), `chooseMoveDestination` MUST always run.

### 4.2 Candidate Set

- MOVE-6: `candidateCells(cbt, c)` MUST enumerate every cell reachable from the NPC's current position within the NPC's remaining movement AP budget, using the same speed rules as players (`SpeedSquares()` today, replaced by `SpeedBudget()` once #248 lands).
- MOVE-7: Candidate cells MUST exclude occupied cells (any combatant or `CoverObject`) per the existing `CellOccupied` / `CellBlocked` predicates in `internal/game/combat/combat.go`.
- MOVE-8: Candidate cells MUST exclude out-of-bounds cells per the grid bounds used by `round.go:ResolveRound` (10×10 web, 20×20 telnet — single source of truth via existing constants).
- MOVE-9: The current cell MUST always be a candidate (the "stand still" option).
- MOVE-10: When the candidate set has only the current cell, `chooseMoveDestination` MUST return `nil`.

### 4.3 Goal Functions

Each goal returns a normalized score in `[0.0, 1.0]` for a candidate cell. Higher is better.

- MOVE-11: **`rangeGoal(c, target, cell)`** MUST score cells by how close they are to the NPC's *effective range* against the target.
  - Effective range: `5` ft for melee NPCs (`RangeIncrement == 0`), `RangeIncrement` ft for ranged NPCs.
  - Score: `1.0` when the cell-to-target Chebyshev distance equals effective range; falls off linearly to `0.0` at the worst-case grid distance.
  - Ranged NPCs MUST NOT prefer cells closer than `5` ft to the target (avoids ranged NPCs running into melee).
- MOVE-12: **`coverGoal(c, target, cell)`** MUST score cells by the cover tier the NPC would receive against `target` from that cell, using the existing positional cover model from #247.
  - Score: `0.0` no cover, `0.33` lesser, `0.66` standard, `1.0` greater.
  - When the NPC has `CombatStrategy.UseCover == false`, `coverGoal` MUST return `0.0` for every cell (disables the goal).
- MOVE-13: **`spreadGoal(c, allies, cell)`** MUST score cells by how far the candidate is from the *nearest* friendly combatant.
  - Score: `0.0` when adjacent (≤ 5 ft) to an ally; rises linearly to `1.0` at distance equal to the NPC's effective range or 30 ft, whichever is greater.
  - Friendly = same `FactionID`, alive, not the NPC itself.
  - Spreads NPCs out to reduce AoE exposure (see #250) without scattering them so far that they can't support each other.
- MOVE-14: **`terrainGoal(cbt, cell)`** MUST score cells by how friendly the destination terrain is.
  - Score: `1.0` for `normal`, `0.5` for `difficult`, `0.0` for `hazardous`, and the cell MUST be excluded from the candidate set entirely if `greater_difficult` (impassable).
  - Until #248 lands, terrain MUST default to `normal` for every cell, making `terrainGoal` return `1.0` uniformly. The function MUST be wired now so #248 plugs in without further AI changes.

### 4.4 Scoring & Selection

- MOVE-15: Total score MUST be the weighted sum:
  `total = wRange * rangeGoal + wCover * coverGoal + wSpread * spreadGoal + wTerrain * terrainGoal`
- MOVE-16: Default weights MUST be `wRange=1.0`, `wCover=0.7`, `wSpread=0.4`, `wTerrain=0.6`. These reproduce a sensible bias toward correct positioning while keeping range as the dominant pull.
- MOVE-17: Weights MUST be overridable per-template via new `CombatStrategy.MoveWeights` fields (`RangeWeight`, `CoverWeight`, `SpreadWeight`, `TerrainWeight`). Missing fields use the defaults from MOVE-16.
- MOVE-18: When the NPC's HP percentage is at or below `CombatStrategy.WoundedHPPct` (default `50`), `wCover` MUST be doubled. Implements the issue's "seek cover when damaged" goal.
- MOVE-19: Selection MUST pick the highest-scoring candidate cell. Ties MUST break deterministically: lower `(GridY, GridX)` wins. Determinism is required for property tests.
- MOVE-20: When the highest-scoring candidate is the NPC's current cell (with score >= the best move-cell score by less than an `epsilon` of `0.01`), the NPC MUST stay put (`chooseMoveDestination` returns `nil`). Avoids twitchy single-cell shuffles.

### 4.5 AP Economy & Stride Conversion

- MOVE-21: Once a destination cell is chosen, the NPC MUST reach it via the existing `ActionStride` machinery. The handler MUST decompose the path into the minimum number of strides given the AP budget; if the destination is not reachable in one stride, additional strides MUST be queued up to `MaxMovementAP`.
- MOVE-22: Stride direction at each step MUST be inferred by comparing the NPC's current cell to the destination cell along Chebyshev neighbors; the existing per-cell clamping in `round.go:ResolveRound` is reused.
- MOVE-23: NPCs MUST NOT exceed `MaxMovementAP` movement actions per round, mirroring the player rule in `internal/game/combat/action.go:120`.
- MOVE-24: When the destination is within one stride and the NPC is melee, the existing direct `ActionStride{Direction: "toward"}` queue MUST continue to be used so that the legacy melee-closes-distance test (`grpc_service_stride_test.go:337`) keeps passing under the new code path.

### 4.6 Fleeing & Threat

- MOVE-25: When the NPC's HP percentage is at or below `CombatStrategy.FleeHPPct`, the move step MUST flip the sign of `rangeGoal` (NPCs prefer cells *farther* from the target), preserving the existing flee semantics in code form. Cover and terrain goals continue normally.
- MOVE-26: When `combatThreat(cbt, c) > CombatStrategy.CourageThreshold`, the same sign-flip MUST apply (NPC disengages).

### 4.7 HTN Plumbing

- MOVE-27: A new HTN action `move_to_position{x, y}` MUST be exposed in `internal/game/ai` so HTN domains can override the new movement step with explicit destinations.
- MOVE-28: When an HTN plan returns `move_to_position`, the handler MUST short-circuit `chooseMoveDestination` and queue strides directly to the named cell (subject to MOVE-21..MOVE-23).
- MOVE-29: HTN domain authors MUST be able to read goal-score values via a new `WorldState.MoveScores` map (`Cell → float64`) for advanced tactical decisions. This is a read-only debugging / advanced-AI affordance; no domain in v1 is required to use it.

### 4.8 Tests

- MOVE-30: Existing tests `TestNPCAutoStride_MeleeNPC_ClosesDistance` and `TestNPCAutoStride_RangedNPC_DoesNotStride` MUST continue to pass without modification.
- MOVE-31: New property tests under `internal/gameserver/testdata/rapid/` MUST verify:
  - Determinism: same `Combat` state ⇒ same destination across runs.
  - Boundedness: returned destination is always in the candidate set.
  - AP-respect: number of strides queued ≤ `MaxMovementAP`.
  - Spread monotonicity: a candidate adjacent to an ally is never preferred over an otherwise-equal candidate not adjacent to an ally.
- MOVE-32: New scenario tests MUST cover:
  - Melee NPC with cover-bearing cell at +1 distance vs. cover-less adjacent cell — chooses the +1 cover cell when `wCover` is default.
  - Ranged NPC at point-blank with target — moves to a cell at `RangeIncrement` distance.
  - Wounded NPC near `FleeHPPct` — moves away from target while staying near cover.
  - Three same-faction NPCs adjacent — at least one moves to a non-adjacent cell on its turn (spread).
  - Hazardous-terrain cell exists — never chosen.
  - Greater-difficult terrain cell exists — excluded from candidate set entirely.

## 5. Architecture

### 5.1 Where the new code lives

```
internal/gameserver/
  combat_handler.go             # existing; npcMovementStrideLocked becomes a thin wrapper
  combat_handler_move.go        # NEW: chooseMoveDestination, candidateCells, goal fns, scoring

internal/game/ai/
  domain.go                     # existing; add `move_to_position` action shape
  build_state.go                # existing; add MoveScores map

internal/game/npc/
  template.go                   # existing; add CombatStrategy.MoveWeights and WoundedHPPct

internal/gameserver/testdata/rapid/
  TestChooseMoveDestination_Property/   # NEW: property tests

internal/gameserver/
  combat_handler_move_test.go    # NEW: scenario tests
```

### 5.2 Decision flow

```
NPC turn
  │
  ▼
HTN planner runs (if AIDomain)
  │
  ├── plan includes ActionStride or move_to_position?
  │       │
  │       └── yes → use planner destination → stride loop (MOVE-21..23)
  │
  └── no movement in plan / no AIDomain
          │
          ▼
      chooseMoveDestination(cbt, c)
          │
          ├── candidateCells (MOVE-6..10)
          │
          ├── for each cell:
          │     score = wRange*range + wCover*cover + wSpread*spread + wTerrain*terrain
          │     (modifiers per MOVE-18, MOVE-25, MOVE-26)
          │
          ├── pick highest (MOVE-19); epsilon-stay (MOVE-20)
          │
          └── nil → no-move; else → stride loop (MOVE-21..23)
```

### 5.3 Single sources of truth

- Cover tier per cell vs. target: the function shipped by #247.
- Terrain type per cell: the function shipped by #248 (no-op default until then per MOVE-14).
- AP / speed: `combat.MaxMovementAP`, `Combatant.SpeedSquares()` / future `SpeedBudget()`.
- Friendly identification: `Combatant.FactionID`.

## 6. Open Questions

- MOVE-Q1: Should `coverGoal` be evaluated against *the current* threat target only, or against the maximum cover the NPC would have against any living enemy? Recommendation: current target only. Multi-enemy cover is too coupled to plan-quality and would dominate the score in messy fights.
- MOVE-Q2: Should `spreadGoal` consider only living allies in the same combat, or also account for cover objects as soft-blockers (since allies can't pass through them anyway)? Recommendation: allies only in v1 — cover objects already factor into the candidate set via `CellBlocked`.
- MOVE-Q3: Default `wSpread = 0.4` is conservative. Should bosses (NPCs with `Tier == "boss"`) get `wSpread = 0.0` so they don't pace away from their own minions? Recommendation: yes, but implement as a one-line override in the boss handler rather than a new template field.
- MOVE-Q4: Should the move step react to *aimed* AoE templates from #250 (player has placed but not confirmed a burst over the NPC)? Recommendation: defer to a follow-on; AoE-reactive movement crosses into reaction-economy territory (#244) and deserves its own design.
- MOVE-Q5: Performance — at worst case, a candidate set on a 20×20 grid is 400 cells × 4 goals × O(combatants) per goal. For combats with 5-10 NPCs per round on a 20×20 grid this is fine, but should we pre-compute per-target cover and terrain once per round? Recommendation: don't pre-optimize; revisit if profile shows hotspots.

## 7. Acceptance

- [ ] All existing combat tests pass.
- [ ] New property tests under `testdata/rapid/` pass.
- [ ] New scenario tests in `combat_handler_move_test.go` pass.
- [ ] `npcMovementStrideLocked` returns the same direction for the cases pinned by `TestNPCAutoStride_MeleeNPC_ClosesDistance` and `TestNPCAutoStride_RangedNPC_DoesNotStride`.
- [ ] Default-weight playthrough on a stock encounter shows visibly different positioning vs. main: at least one NPC should sidestep into cover or away from a clustered group during a 4-round combat.
- [ ] HTN domains can issue `move_to_position` and the handler honors it.

## 8. Out-of-Scope Follow-Ons

- MOVE-F1: Faction coordination — flanking pairs, fire-and-maneuver, suppressing-fire pickups.
- MOVE-F2: Reactive movement to aimed AoEs from #250 (per MOVE-Q4).
- MOVE-F3: Multi-turn lookahead / planning beyond the current round.
- MOVE-F4: Pathfinding around irregular obstacle layouts beyond the per-cell stride loop (only relevant if `CoverObject` topology becomes more interesting).
- MOVE-F5: Per-archetype `MoveWeights` presets in YAML (e.g., a `skirmisher` preset, a `bruiser` preset). v1 only allows per-template overrides.

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/251
- Current NPC movement heuristic: `internal/gameserver/combat_handler.go:4379` (`npcMovementStrideLocked`)
- Current NPC turn entry: `internal/gameserver/combat_handler.go:3795` (`autoQueueNPCsLocked`)
- HTN planner integration: `internal/gameserver/combat_handler.go:3961`, `internal/game/ai/`
- Existing stride mechanics: `internal/game/combat/round.go:ResolveRound`
- AP economy: `internal/game/combat/action.go:120` (`MaxMovementAP`), `internal/gameserver/grpc_service.go:10804` (`handleMoveTo`)
- Cover model: `docs/superpowers/specs/2026-04-21-cover-bonuses-in-combat.md`
- Terrain model: `docs/superpowers/specs/2026-04-21-terrain-type-penalties.md` (referenced as TERRAIN-*)
- AoE templates (clustering rationale): `docs/superpowers/specs/2026-04-24-aoe-drawing-in-combat.md`
- Threat function: `internal/game/npc/behavior/threat.go:19`
- Existing tests: `internal/gameserver/grpc_service_stride_test.go:337`, `:389`
