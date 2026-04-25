---
title: Visibility / Line-of-Sight System
issue: https://github.com/cory-johannsen/mud/issues/267
date: 2026-04-25
status: spec
prefix: LOS
depends_on:
  - "#249 Targeting system in combat (validation pipeline reserves the line-of-fire stage)"
related:
  - "#247 Cover bonuses (uses positional cover routine that this spec extends with occlusion)"
  - "#250 AoE drawing (PostFilterAffectedCells extension point)"
  - "#251 Smarter NPC movement (terrain goal can read occlusion when authored)"
  - "#254 Detection states (OcclusionProvider interface from DETECT-NG1)"
  - "#264 Structured battle maps (cell grid bounds)"
```

# Visibility / Line-of-Sight System

## 1. Summary

Three already-shipped or in-flight specs declared a no-op extension point reserved for this work:

- Spec #254 (DETECT-NG1) exposes an `OcclusionProvider` interface that always returns "clear LoS" until #267 lands.
- Spec #250 (AOE-21) reserves a `PostFilterAffectedCells` resolver hook that filters occluded cells out of an AoE template; v1 is identity.
- Spec #249 (TGT-21) reserves a line-of-fire validation stage in the targeting pipeline that is currently a no-op.

This spec ships:

1. A **per-cell occlusion model** layered on the existing combat grid (`Combat.GridWidth/Height`).
2. **WallObject** as a new first-class grid entity for full LoS-blockers, distinct from `CoverObject` (which already partially blocks movement and grants cover bonuses).
3. A **LineOfSight Bresenham walker** that returns `Clear`, `Concealed` (passes through translucent cells), or `Blocked` (passes through any opaque cell).
4. A **dynamic environment overlay** for smoke / fog / darkness that contributes occlusion per cell with a duration, attached to the existing condition pipeline.
5. The **wired-up validation hooks**: `ValidateSingleTarget` and `ValidateAoE` consult LoS; the AoE `PostFilterAffectedCells` strips occluded cells; the detection `OcclusionProvider` consults the same machinery.
6. **UI surfacing**: occluded combatants are omitted from default target lists on telnet and web; concealed combatants surface a `(concealed)` annotation; AoE preview cells that are post-filtered out render dimmed.

The concealment miss-chance resolution stays out of scope (it's owned by the detection state's `Concealed` gate per spec #254 DETECT-7); this spec only sets the occlusion that drives a target into Concealed or Hidden in the first place.

## 2. Goals & Non-Goals

### 2.1 Goals

- LOS-G1: Every cell on the combat grid carries an opacity (`Transparent`, `Translucent`, `Opaque`).
- LOS-G2: A `WallObject` schema permits authoring full-line blockers in room YAML; walls occupy cells and are opaque.
- LOS-G3: Cover objects MAY occlude LoS based on a per-cover `occludes` flag (default `false` so existing cover keeps current behavior).
- LOS-G4: Environmental conditions (`smoke`, `fog`, `darkness`) MAY add cell-bound translucency / opacity for a duration.
- LOS-G5: A pure `LineOfSight(grid, from, to) Result` function returns `Clear` / `Concealed` / `Blocked` based on the Bresenham walk.
- LOS-G6: Targeting validation, AoE template post-filter, and detection-state transitions all consult the same LoS function — single source of truth.
- LOS-G7: Pinning tests for #247 / #250 / #254 / #257 continue to pass when no walls / smoke / opaque cover exist (back-compat).

### 2.2 Non-Goals

- LOS-NG1: Concealment miss-chance roll. That's the detection layer's `Concealed` gate (DETECT-7).
- LOS-NG2: Senses other than ordinary vision (darkvision, nightvision, blindsight). Future ticket.
- LOS-NG3: Light source modeling as first-class objects. Darkness is a cell condition only.
- LOS-NG4: Persistent fog-of-war that survives between rounds beyond the duration of the originating effect.
- LOS-NG5: Volumetric / 3D occlusion (z-axis). 2D Chebyshev grid only.
- LOS-NG6: Authoring UI for placing walls. YAML-only in v1; admin tab a future follow-on.
- LOS-NG7: Per-character "sight-blocking aware" line-of-fire weapons (e.g., guided missiles). All weapons obey LoF in v1.

## 3. Glossary

- **Opacity**: a per-cell attribute, one of `Transparent`, `Translucent`, `Opaque`.
- **Wall**: a static cell-based opaque object declared in room YAML.
- **Translucent cover**: a `CoverObject` with `occludes: true, partial: true` — passes light with concealment.
- **Smoke / Fog / Darkness**: environmental cell conditions adding occlusion for a duration.
- **LoS result**: `Clear` (no occlusion), `Concealed` (one or more translucent cells in the path), `Blocked` (one or more opaque cells in the path).
- **Bresenham walker**: an integer-grid line-traversal algorithm enumerating the cells crossed between two endpoints.

## 4. Requirements

### 4.1 Cell Opacity Model

- LOS-1: A new `OpacityLayer` MUST be added to `Combat`, holding a per-cell opacity. Default at combat-start: every cell `Transparent`. Population happens during `StartCombat` from room walls (LOS-2), cover objects (LOS-3), and active environmental conditions (LOS-4).
- LOS-2: `WallObject` MUST be a new room-level entity authored in YAML alongside `cover_objects:`. Schema: `{ id, cells: [{x, y}, ...], destructible: bool, hp?: int }`. `StartCombat` reads the room's walls and marks each cell `Opaque` in the layer.
- LOS-3: `CoverObject` schema MUST gain optional `occludes bool` (default `false`) and `partial bool` (default `false`). When `occludes: true` and `partial: false`, the cell is `Opaque`. When `occludes: true` and `partial: true`, the cell is `Translucent`. Existing cover with no `occludes` flag stays `Transparent` and behaves as today.
- LOS-4: Environmental occluders (smoke, fog, darkness) MUST be expressible as `EnvironmentalEffect` instances with `cells: [{x, y}, ...]`, `opacity` (`Translucent` | `Opaque`), and `duration_rounds`. The instances live in `Combat.Environment` and tick down per round; expired effects clear their cells back to baseline opacity.

### 4.2 Line-of-Sight Function

- LOS-5: A new function `combat.LineOfSight(layer OpacityLayer, from, to Cell) Result` MUST be added. Algorithm:
  - Walk Bresenham cells from `from` to `to`, exclusive of both endpoints.
  - If any cell is `Opaque` → return `Blocked`.
  - Else if any cell is `Translucent` → return `Concealed`.
  - Else → return `Clear`.
- LOS-6: The function MUST be pure (no I/O), deterministic, and side-effect free. Property tests under `internal/game/combat/testdata/rapid/TestLineOfSight_Property/` MUST verify symmetry (`LoS(a, b) == LoS(b, a)`), monotonicity in opacity (more opaque cells → never `Clear`-er result), and idempotence.
- LOS-7: When `from == to`, the function MUST return `Clear` (a combatant always sees their own cell).
- LOS-8: When `from` and `to` are adjacent (Chebyshev distance 1), the function MUST return `Clear` regardless of cell opacity (you can always see what's right next to you — the existing rule for melee in PF2E).

### 4.3 OcclusionProvider Implementation

- LOS-9: The `OcclusionProvider` interface from spec #254 (DETECT-NG1 / DETECT-21) MUST be implemented:
  - `LineOfSight(observerUID, targetUID string) detection.Result` — returns `Clear` / `Concealed` / `Blocked` translated from `LineOfSight(layer, observer.GridCell, target.GridCell)`.
- LOS-10: When the provider returns `Blocked` between two combatants, the detection layer MUST set their pair-state to `Undetected` (or `Hidden` if they were Observed last tick — preserving the "you remember roughly where it was" PF2E rule). The transition is owned by the detection layer; this spec just emits the `Result`.
- LOS-11: When the provider returns `Concealed`, the detection layer MAY set `Concealed` for that pair until the LoS clears. The transition rule lives in the detection layer per spec #254.

### 4.4 Targeting Validation

- LOS-12: The `ValidateSingleTarget` line-of-fire stage from spec #249 (TGT-21) MUST consult `combat.LineOfSight(layer, attackerCell, targetCell)`. When the result is `Blocked`, validation MUST reject the target. When `Concealed`, validation MUST allow but mark the target with a "concealed" warning that surfaces in the UI.
- LOS-13: The `ValidateAoE` line-of-fire stage MUST consult LoS from the attacker to the template's anchor cell (for burst) or apex cell (for cone) or origin (for line). Same `Blocked` rejection / `Concealed` warning rules apply.
- LOS-14: AoE templates MUST also have their cell list post-filtered via the `PostFilterAffectedCells` hook from spec #250 (AOE-21): for each cell in the template, if `combat.LineOfSight(layer, attackerCell, cell)` returns `Blocked`, drop the cell from the resolved set. `Concealed` cells stay in the set (the AoE blast still touches them).

### 4.5 UI Surfacing

- LOS-15: The targeting UI on telnet (target list per `attack`, `strike`, `use` commands) MUST omit combatants whose pair-state from the player is `Undetected`, `Unnoticed`, or otherwise hidden by the detection layer (which itself reads LoS via DETECT-G1).
- LOS-16: The web targeting picker MUST grey-out combatants the player cannot see, with a tooltip "no line of sight" when the player attempts to click them anyway.
- LOS-17: Combatants whose pair-state is `Concealed` due to translucent LoS cells (smoke, partial cover) MUST surface a `(concealed)` annotation next to their name in both UIs.
- LOS-18: AoE preview cells that are post-filtered out (LOS-14) MUST render dimmed on the web template preview so the player sees what the template will not actually hit.
- LOS-19: A new telnet command `look <direction>` (or extension to existing `look`) MUST report cells the player can see along that compass octant out to the player's vision range; concealed cells are noted, blocked cells terminate the look. (This is a quality-of-life addition that exercises the same LoS plumbing for non-combat eyes-on inspection.)

### 4.6 Environmental Effects

- LOS-20: A new YAML schema `EnvironmentalEffect` MUST live under `content/environment/` with shape:
  - `id`, `display_name`, `description`.
  - `cells_relative_to: { kind: "anchor" | "self_at_apply" }`.
  - `cells: [{x, y}, ...]` (relative to the anchor or self).
  - `opacity` (one of `Translucent`, `Opaque`).
  - `duration_rounds` (int).
- LOS-21: At least three exemplar environmental effects MUST be authored: `smoke_grenade` (Translucent, 5 rounds, 3×3 burst), `fog_bank` (Translucent, 10 rounds, 5×5 burst), `darkness_zone` (Opaque, 20 rounds, 7×7 burst).
- LOS-22: Environmental effects MUST be deployable via existing `ActionThrow` (for grenades — spec #245 / explosives content) or via NPC abilities. Authoring path is out of scope; the resolver dispatch is the integration point.
- LOS-23: At round-tick, `Combat.Environment` MUST decrement `duration_rounds` for each active effect; effects with `duration_rounds <= 0` MUST be removed and their cells cleared back to baseline opacity.

### 4.7 Tests

- LOS-24: Existing tests for #247 / #250 / #254 / #257 MUST pass unchanged when no walls, opaque cover, or environmental effects are present.
- LOS-25: New tests in `internal/game/combat/los_test.go` MUST cover:
  - Adjacent always Clear (LOS-8).
  - Self always Clear (LOS-7).
  - Wall on the path → Blocked.
  - Translucent cover on the path → Concealed.
  - Translucent + opaque on the path → Blocked.
  - Smoke environmental effect → Concealed for the duration; cleared after expiry.
  - Symmetry property (LOS-6).
- LOS-26: A new integration test MUST exercise the full pipeline: (1) NPC behind a wall is Undetected to the player; (2) NPC moves out, becomes Observed; (3) player throws smoke grenade between them, NPC becomes Concealed.

## 5. Architecture

### 5.1 Where the new code lives

```
internal/game/combat/
  opacity.go                       # NEW: OpacityLayer, opacity enum, helpers
  los.go                           # NEW: LineOfSight Bresenham walker
  los_test.go                      # NEW
  environment.go                   # NEW: EnvironmentalEffect runtime
  testdata/rapid/TestLineOfSight_Property/   # NEW
  combat.go                        # existing; Combat.OpacityLayer field, Combat.Environment slice
  engine.go                        # existing; StartCombat populates OpacityLayer from room

internal/game/world/
  model.go                         # WallObject schema; CoverObject gains `occludes` and `partial`

internal/game/detection/
  occlusion.go                     # existing no-op v1 (#254); replaced by combat-backed impl

internal/gameserver/
  combat_handler.go                # validation hooks consult LineOfSight
  grpc_service.go                  # `look <direction>` command (LOS-19)

content/environment/
  smoke_grenade.yaml, fog_bank.yaml, darkness_zone.yaml

cmd/webclient/ui/src/game/panels/
  MapPanel.tsx                     # AoE preview dimming for filtered cells
  TargetPicker.tsx                 # grey-out blocked targets, concealed annotation
```

### 5.2 LoS evaluation flow

```
attack / target validation
   │
   ▼
LineOfSight(combat.OpacityLayer, attackerCell, targetCell)
   ├── walk Bresenham cells exclusive endpoints
   ├── if any opaque → Blocked
   ├── elif any translucent → Concealed
   ├── else → Clear
   ▼
validation:
   Blocked → reject "no line of sight"
   Concealed → allow with warning, set pair-state Concealed via DETECT layer
   Clear → allow

AoE post-filter:
   per cell c: if LineOfSight(layer, attackerCell, c) == Blocked → drop c
   Concealed cells stay in template

detection layer (#254):
   per pair (observer, target): consult OcclusionProvider → set pair-state
```

### 5.3 Single sources of truth

- LoS algorithm: `combat.LineOfSight` only.
- Opacity layer: `Combat.OpacityLayer` only.
- Environmental effect runtime: `Combat.Environment` only.
- Detection-pair state derived from LoS: detection layer (#254) consumes `OcclusionProvider`.

## 6. Open Questions

- LOS-Q1: When a wall is partially destroyed (`destructible: true, hp: <reduced>`), does opacity stepwise reduce, or stay opaque until destroyed? Recommendation: binary — opaque until destroyed, transparent when destroyed. Adds a future-extension comment for stepwise behavior.
- LOS-Q2: Bresenham line-walking for thick targets (a 2x2 NPC) — does any cell on the line need to be opaque, or all cells of *every* candidate line between any pair of source/target cells? Recommendation: pick a single representative line (cell-center to cell-center). Multi-cell creatures are out of scope until they ship.
- LOS-Q3: When a player Strides through their own thrown smoke, do they break their own LoS? Recommendation: yes — the LoS rule is symmetric and cell-based; the player's own smoke conceals them from outside observers and obscures their view outward, mirroring the PF2E rule.
- LOS-Q4: PF2E says "you can attempt to Seek even when target is Undetected"; does Seek need to consult LoS? Recommendation: yes — Seek through a wall fails (you'd need to listen, which is sound-based and out of scope for v1). The `OcclusionProvider` makes this clean.
- LOS-Q5: The `look <direction>` command (LOS-19) is a quality-of-life add. Is it worth shipping in v1, or defer? Recommendation: ship — it exercises the same plumbing and gives players an out-of-combat way to test the system.
- LOS-Q6: When environmental effects overlap (smoke + smoke = thicker smoke?), does the resolver compose? Recommendation: no — each cell takes the *most opaque* of all overlapping effects (Opaque > Translucent > Transparent).

## 7. Acceptance

- [ ] All existing combat / detection / AoE / cover tests pass when no walls, opaque cover, or environmental effects are present.
- [ ] New LoS unit tests pass.
- [ ] LoS property tests pass (symmetry, monotonicity).
- [ ] An NPC behind a wall is Undetected to the player and not in the player's target list.
- [ ] A player throws smoke between themselves and an NPC; the NPC's pair-state to the player becomes Concealed; LoS through the smoke is Concealed.
- [ ] An AoE template that crosses a wall has the post-wall cells dropped from the resolved set.
- [ ] `look <direction>` reports correct visible cells and stops at the first opaque cell.

## 8. Out-of-Scope Follow-Ons

- LOS-F1: Concealment miss-chance (covered by #254 DETECT-7).
- LOS-F2: Senses beyond vision (darkvision, nightvision, blindsight).
- LOS-F3: First-class light sources / illumination model.
- LOS-F4: Persistent fog-of-war beyond effect duration.
- LOS-F5: Volumetric / 3D occlusion.
- LOS-F6: Admin UI for placing walls.
- LOS-F7: Multi-cell creature representative-line selection (per LOS-Q2).
- LOS-F8: Stepwise destructible-wall opacity (per LOS-Q1).

## 9. References

- Issue: https://github.com/cory-johannsen/mud/issues/267
- Targeting validation hook (TGT-21): `docs/superpowers/specs/2026-04-22-targeting-system-in-combat.md`
- AoE post-filter hook (AOE-21): `docs/superpowers/specs/2026-04-24-aoe-drawing-in-combat.md`
- Detection-state OcclusionProvider (DETECT-NG1, DETECT-21): `docs/superpowers/specs/2026-04-24-detection-states.md`
- Cover spec (CoverObject schema extended here): `docs/superpowers/specs/2026-04-21-cover-bonuses-in-combat.md`
- Combat grid: `internal/game/combat/combat.go` (`Combat.GridWidth/Height`)
- Existing CoverObject: `internal/game/combat/combat.go` (CoverObjects slice)
- StartCombat (extended to populate OpacityLayer): `internal/game/combat/engine.go:541`
- Room model: `internal/game/world/model.go:161-215`
