# UI Display Improvements — Design Spec

**Date:** 2026-03-13

---

## Goal

Three targeted UI enhancements: show active conditions in the player prompt, show NPC conditions in the room view, and display the map grid and legend side-by-side.

---

## Feature 1: Player Prompt Mental State

### Problem

Active conditions (mental states, grabbed, prone, etc.) are invisible in the prompt. Players have no at-a-glance indicator of their current condition status.

### Design

`game_bridge.go` already processes `ConditionEvent` messages. It will maintain a `map[string]string` of `conditionID → displayName` for all active conditions on the `GameBridge` struct. `BuildPrompt` gains a `conditions []string` parameter (display names only). Each entry renders as `[Name]` in BrightMagenta after the HP segment.

**Prompt format:**
```
[Hero] [Morning 08:00] [10/10hp] [Panicked] [Grabbed]>
```

**Data flow:**
- `ConditionEvent` with `applied: true` → add `id → name` to `activeConditions` map
- `ConditionEvent` with `applied: false` → delete by `id`
- On each prompt refresh, sort the condition IDs (`slices.Sort(slices.Collect(maps.Keys(activeConditions)))`), then build `[]string` of display names in that order, and pass to `BuildPrompt`. Deterministic sort order by ID prevents display churn.

**Call sites — ALL of the following must be updated:**
- `BuildPrompt` function signature in `game_bridge.go`
- The internal `buildPrompt` closure is called at many sites throughout `game_bridge.go`. The implementer MUST grep for every invocation of `buildPrompt()` in that file and update each one to pass the `activeConditions` slice. Do not rely on a partial list; find all call sites exhaustively.

**Affected files:**
- `internal/frontend/handlers/game_bridge.go` — add `activeConditions map[string]string` field to `GameBridge`; update `BuildPrompt` signature; update all `buildPrompt` closure call sites; update `ConditionEvent` handler

---

## Feature 2: NPC Conditions in Room View

### Problem

NPCs in the room listing show health description and fighting target, but not conditions applied during combat (grabbed, prone, submerged, etc.).

### Design

`NpcInfo` proto gets a new `repeated string conditions = 5` field containing display names of active conditions. `buildRoomView` queries the combat handler for each NPC's condition set. `RenderRoomView` renders them as a comma-separated Yellow list after the health/fighting_target text.

**Room display example:**
```
[Goblin] lightly wounded, fighting Hero    prone, grabbed
```

**Data flow:**
- `buildRoomView` calls `combatHandler.GetCombatConditionSet(uid, npcInstanceID)` for each NPC in the room
  - **Precondition:** `uid` MUST be the viewing player's UID (the same player whose room this view is for). Passing a different player's UID would look up the wrong combat.
- If an `ActiveSet` is returned, iterate via `activeSet.All()` and collect `ac.Def.Name` for each active condition → `NpcInfo.Conditions`
- If no active combat or NPC not a combatant, `Conditions` is empty (zero-length slice)

**Affected files:**
- `api/proto/game/v1/game.proto` — add `repeated string conditions = 5` to `NpcInfo`; run `make proto`
- `internal/gameserver/world_handler.go` — populate `NpcInfo.Conditions` in `buildRoomView`
- `internal/frontend/handlers/text_renderer.go` — render `NpcInfo.Conditions` in `RenderRoomView`

---

## Feature 3: Map 2-Column Layout

### Problem

The map grid and legend are stacked vertically. On wide terminals, the bottom half is wasted whitespace.

### Design

`RenderMap` splits the terminal width into two equal halves when `width >= 100`. The left half renders the grid; the right half renders the legend as a single vertical list. Grid lines and legend lines are zipped together with a ` │ ` separator. If `width < 100`, falls back to the current stacked layout (existing behavior, no change).

**Layout (width ≥ 100):**
```
[01]-[02]   [04]  │  *1. Starting Room
 |         /      │   2. Dark Corridor
[03]  [05]-[06]   │   3. Guard Post
                  │   4. Armory
                  │   5. Storage Room
                  │   6. Exit Tunnel
```

**Algorithm:**
1. Compute `halfWidth = width / 2` (integer division; odd widths truncate — left column gets `halfWidth`, right gets the remainder)
2. Render grid lines into `[]string` using grid width `halfWidth - 3` (3 chars reserved for ` │ ` separator)
3. Render legend entries into `[]string` (existing format: ` *NN. RoomName`, one entry per line)
4. Zip: for each index `i`, output `padRight(gridLines[i], halfWidth-3) + " │ " + legendLines[i]`, using empty string when one side runs out
5. Lines where only legend remains: `strings.Repeat(" ", halfWidth-3) + " │ " + legendLine`
6. Lines where only grid remains: `gridLine` (no separator needed after legend is exhausted)

**Fallback:** `width < 100` → unchanged stacked layout (grid full width, legend below in 4 columns).

**Affected files:**
- `internal/frontend/handlers/text_renderer.go` — restructure `RenderMap` assembly section only; grid and legend rendering logic unchanged

---

## Testing

### Feature 1

- REQ-T1 (property): For any `conditions []string` (including empty), every entry appears in `BuildPrompt` output exactly once as `[entry]`.
- REQ-T2 (property): `BuildPrompt` with empty `conditions` produces output with no `[` segment after the HP segment (prompt format unchanged).
- REQ-T3 (example): `BuildPrompt` with `["Panicked", "Grabbed"]` contains both `[Panicked]` and `[Grabbed]` in BrightMagenta.
- REQ-T4 (example): After a `ConditionEvent` applied, the condition name appears in the next prompt. After the corresponding removed event, it does not.

### Feature 2

- REQ-T5 (example): `NpcInfo.Conditions` is empty when NPC is not in active combat.
- REQ-T6 (example): `NpcInfo.Conditions` contains condition display names (`ac.Def.Name`) when NPC is a combatant with active conditions.
- REQ-T7 (property): For any `NpcInfo` with non-empty `Conditions`, `RenderRoomView` output contains every condition name at least once.
- REQ-T8 (example): `RenderRoomView` renders no condition text when `NpcInfo.Conditions` is empty.

### Feature 3

- REQ-T9 (example): `RenderMap` at `width = 100` produces side-by-side layout with ` │ ` separator on every line that has content.
- REQ-T10 (example): `RenderMap` at `width = 99` produces stacked layout (current behavior, no ` │ ` separator).
- REQ-T11 (property): For any valid `MapResponse` and `width >= 100`, every legend entry appears exactly once in the output regardless of relative grid/legend height.
- REQ-T12 (property): For any valid `MapResponse` and `width < 100`, output is identical to the pre-change stacked layout.
