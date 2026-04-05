# 2D Grid Combat Expansion

## Overview

Expand the combat system from 1D linear positioning to a 2D grid, enabling flanking,
area-of-effect targeting, and tactical positioning. The modal combat screen renders a
small ASCII grid with combatant positions. The web client MapPanel replaces the
horizontal ruler with a 2D grid view.

---

## Current State

- `Combatant.Position` is a single `int` (feet, 0–100).
- Stride/step commands take `"toward"` or `"away"` — direction along a 1D axis.
- Clients track positions via `combatPositions: Record<string, number>`.
- Web client renders a normalized horizontal ruler (`renderBattleMap`).
- Telnet shows position contextually in combat event narratives.
- No flanking, no AoE, no diagonal movement.

---

## Requirements

### REQ-2D-1: Grid Model

- REQ-2D-1a: Replace `Combatant.Position int` with `Combatant.GridX int` and `Combatant.GridY int`.
- REQ-2D-1b: Grid squares are 5 feet. Grid dimensions are 10 columns × 10 rows (50 ft × 50 ft).
- REQ-2D-1c: Players spawn at row 0; NPCs spawn at row 9. X positions are assigned sequentially left-to-right within each group.
- REQ-2D-1d: `Combat.GridWidth` and `Combat.GridHeight` are stored on the Combat struct (default 10×10).
- REQ-2D-1e: Grid coordinates are bounded: `GridX ∈ [0, GridWidth-1]`, `GridY ∈ [0, GridHeight-1]`. Movement that would exceed bounds is silently clamped.

### REQ-2D-2: Distance Calculation

- REQ-2D-2a: Distances use Chebyshev (chessboard) distance: `max(|dx|, |dy|)` squares × 5 ft.
- REQ-2D-2b: All existing range checks (`≤ 5 ft` for melee, weapon range for ranged) use the new Chebyshev distance.
- REQ-2D-2c: `CombatRange(a, b Combatant) int` replaces the existing `combatRange(a, b)` function; returns feet.

### REQ-2D-3: Movement Commands

- REQ-2D-3a: `stride <direction>` and `step <direction>` accept compass directions: `n`, `s`, `e`, `w`, `ne`, `nw`, `se`, `sw`. Legacy `"toward"` / `"away"` remain accepted: `"toward"` moves one step toward the closest enemy along the primary axis; `"away"` moves one step away.
- REQ-2D-3b: Stride moves 5 ft (1 square) per AP spent in the default implementation, consistent with existing stride cost. `Step` moves 5 ft and does not trigger reactive strikes.
- REQ-2D-3c: `StrideRequest` and `StepRequest` proto messages gain a `direction string` field that accepts all compass directions in addition to "toward"/"away".

### REQ-2D-4: Flanking

- REQ-2D-4a: A combatant is **flanked** when at least two enemies are on opposite sides: their line-of-sight vectors through the target's square differ by ≥135°.
- REQ-2D-4b: In practice: an attacker flanks the target if any allied combatant is in the opposite diagonal quadrant (row and column both differ by ≥1 in opposite directions from target).
- REQ-2D-4c: Flanking grants +2 to attack rolls. Flanking bonus is computed in `resolveAttack` and applied to `AttackTotal`.
- REQ-2D-4d: The flanking bonus is logged in the `CombatEvent` narrative as `"(flanking +2)"`.
- REQ-2D-4e: The `CombatEvent` proto gains a `bool flanking` field.

### REQ-2D-5: Area-of-Effect (AoE)

- REQ-2D-5a: Technologies and feats with `aoe_radius > 0` (in feet) target a grid coordinate rather than a single combatant.
- REQ-2D-5b: All combatants within Chebyshev distance ≤ `aoe_radius/5` squares of the target coordinate are hit.
- REQ-2D-5c: `UseRequest` gains an optional `target_x int32` and `target_y int32` for AoE targeting. If the feat/tech has no AoE, these fields are ignored.
- REQ-2D-5d: AoE technologies and feats specify `aoe_radius: <feet>` in their YAML. Existing content without this field has `aoe_radius: 0` (single-target).
- REQ-2D-5e: AoE does not distinguish friend from foe — all combatants in radius are affected.

### REQ-2D-6: Reactive Strikes on Movement

- REQ-2D-6a: Reactive strikes continue to fire when a combatant moves away from an adjacent enemy (Chebyshev distance ≤ 1 square / 5 ft before move, > 1 square after).
- REQ-2D-6b: No change to reactive strike rules beyond adapting the distance check to 2D.

### REQ-2D-7: NPC Auto-Movement

- REQ-2D-7a: NPC stride logic continues to move toward (melee) or away (ranged) from their combat target.
- REQ-2D-7b: Preferred approach direction is the shortest Chebyshev path. Ties resolved by preferring to reduce Y distance first, then X.

### REQ-2D-8: Proto Changes

- REQ-2D-8a: `RoundStartEvent` gains `repeated CombatantPosition initial_positions` where `CombatantPosition { string name = 1; int32 x = 2; int32 y = 3; }`.
- REQ-2D-8b: `CombatEvent` replaces `attacker_position int32` with `attacker_x int32` and `attacker_y int32`.
- REQ-2D-8c: `GameState.combatPositions: Record<string, number>` becomes `Record<string, {x: number, y: number}>` in the web client.

### REQ-2D-9: Telnet Battlefield Rendering

- REQ-2D-9a: The telnet room region (rows 1–8) renders a 10×10 ASCII grid during combat showing each combatant's token (first letter of name, uppercase) at their grid coordinate.
- REQ-2D-9b: Empty squares render as `.`. Grid borders use `+`, `-`, `|`.
- REQ-2D-9c: A legend below the grid shows `P=PlayerName`, `A=AllyName`, `E=EnemyName`.
- REQ-2D-9d: Grid is re-rendered after every `CombatEvent` that changes a position.

### REQ-2D-10: Web Client Battlefield Rendering

- REQ-2D-10a: `renderBattleMap` in `MapPanel.tsx` is replaced with `renderBattleGrid` that renders a 10×10 CSS grid.
- REQ-2D-10b: Each cell is 28×28 px. Players shown in blue, enemies in red, allies in green.
- REQ-2D-10c: Hovering a cell shows its coordinates. Clicking a cell with AoE tech active sends a targeted `UseRequest`.
- REQ-2D-10d: Grid is updated on every `UPDATE_COMBAT_POSITION` dispatch.

### REQ-2D-11: Backward Compatibility

- REQ-2D-11a: The `Position` field on `Combatant` is removed. All callers updated to use `GridX`/`GridY`.
- REQ-2D-11b: `COMBAT_EVENT_TYPE_POSITION` enum value is preserved; `attacker_position` field is deprecated (kept at field number, defaulting to 0) to avoid breaking old clients.
- REQ-2D-11c: All existing tests referencing `Combatant.Position` must be updated.

---

## Data Model Changes

```go
// combat/combat.go
type Combatant struct {
    // ... existing fields ...
    GridX int  // replaces Position
    GridY int
}

type Combat struct {
    // ... existing fields ...
    GridWidth  int // default 10
    GridHeight int // default 10
}
```

```go
// New helper
func CombatRange(a, b Combatant) int {
    dx := a.GridX - b.GridX
    dy := a.GridY - b.GridY
    if dx < 0 { dx = -dx }
    if dy < 0 { dy = -dy }
    if dx > dy { return dx * 5 }
    return dy * 5
}

func IsFlanked(target Combatant, attackers []Combatant) bool {
    // at least two attackers in opposite quadrants relative to target
}
```

---

## Out of Scope

- Line-of-sight blocking by terrain or obstacles.
- Multi-square (large/huge) combatants.
- Diagonal movement cost differentiation.
- Cover from grid position (cover remains equipment-based per existing system).
- Zone-of-control rules.

---

## Files Affected

| File | Change |
|------|--------|
| `internal/game/combat/combat.go` | Replace `Position` with `GridX`, `GridY`; add `GridWidth`, `GridHeight`; add `CombatRange`, `IsFlanked` |
| `internal/game/combat/round.go` | Update all range checks; add flanking bonus in `resolveAttack`; update reactive strike trigger |
| `internal/game/combat/engine.go` | Update spawn placement to assign grid coords |
| `internal/gameserver/combat_handler.go` | Update stride/step handling for compass directions; NPC auto-move |
| `api/proto/game/v1/game.proto` | `CombatantPosition` message; `RoundStartEvent.initial_positions`; `CombatEvent.attacker_x/y`; `UseRequest.target_x/y`; `CombatEvent.flanking` |
| `internal/gameserver/gamev1/game.pb.go` | Regenerate |
| `internal/frontend/telnet/` | ASCII grid renderer for room region during combat |
| `cmd/webclient/ui/src/game/GameContext.tsx` | `combatPositions` type → `{x,y}` |
| `cmd/webclient/ui/src/game/panels/MapPanel.tsx` | Replace `renderBattleMap` with `renderBattleGrid` |
| `cmd/webclient/ui/src/proto/index.ts` | New/updated proto TS types |
| All combat tests | Update `Position` → `GridX`/`GridY` |
