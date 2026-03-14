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

`game_bridge.go` already processes `ConditionEvent` messages. It will maintain a `map[string]string` of `conditionID → displayName` for all active conditions. `BuildPrompt` gains a `conditions []string` parameter (display names only). Each entry renders as `[Name]` in BrightMagenta after the HP segment.

**Prompt format:**
```
[Hero] [Morning 08:00] [10/10hp] [Panicked] [Grabbed]>
```

**Data flow:**
- `ConditionEvent` with `applied: true` → add `id → name` to map
- `ConditionEvent` with `applied: false` → delete by `id`
- On each prompt refresh, pass `maps.Values(conditionMap)` sorted by ID to `BuildPrompt`

**Affected files:**
- `internal/frontend/handlers/game_bridge.go` — add `activeConditions map[string]string`; update `BuildPrompt` call; update `ConditionEvent` handler
- No proto changes required

---

## Feature 2: NPC Conditions in Room View

### Problem

NPCs in the room listing show health description and fighting target, but not conditions applied during combat (grabbed, prone, submerged, etc.).

### Design

`NpcInfo` proto gets a new `repeated string conditions` field containing display names of active conditions. `buildRoomView` queries the combat handler for each NPC's condition set. `RenderRoomView` renders them as a comma-separated Yellow list after the health/fighting_target text.

**Room display example:**
```
[Goblin] lightly wounded, fighting Hero    prone, grabbed
```

**Data flow:**
- `buildRoomView` calls `combatHandler.GetCombatConditionSet(uid, npcInstanceID)` for each NPC
- If found, iterates the `ActiveSet` to collect display names → `NpcInfo.Conditions`
- If no active combat or NPC not a combatant, `Conditions` is empty

**Affected files:**
- `api/proto/game/v1/game.proto` — add `repeated string conditions = 6` to `NpcInfo`; run `make proto`
- `internal/gameserver/world_handler.go` — populate `NpcInfo.Conditions` in `buildRoomView`
- `internal/frontend/handlers/text_renderer.go` — render `NpcInfo.Conditions` in `RenderRoomView`

**Note:** `GetCombatConditionSet` requires a player UID; `buildRoomView` already receives `uid`. The method already exists on `CombatHandler`.

---

## Feature 3: Map 2-Column Layout

### Problem

The map grid and legend are stacked vertically. On wide terminals, the bottom half is wasted whitespace.

### Design

`RenderMap` splits the terminal width into two equal halves when `width >= 100`. The left half renders the grid; the right half renders the legend as a single vertical list. Grid lines and legend lines are zipped together with a ` │ ` separator. If `width < 100`, falls back to the current stacked layout.

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
1. Render grid lines into `[]string` (existing logic, width = `width/2 - 2`)
2. Render legend entries into `[]string` (existing format: ` *NN. RoomName`)
3. Zip: `gridLine + padding + " │ " + legendLine` (pad shorter side with spaces)
4. Lines with no grid content but remaining legend entries: indent + `" │ " + legendLine`

**Width threshold:** `width >= 100` for 2-column layout; below that, stacked (current behavior).

**Affected files:**
- `internal/frontend/handlers/text_renderer.go` — restructure `RenderMap` assembly

---

## Testing

### Feature 1
- REQ-T1: `BuildPrompt` with empty conditions produces current format (no extra segment).
- REQ-T2: `BuildPrompt` with one condition appends `[ConditionName]` in BrightMagenta.
- REQ-T3: `BuildPrompt` with multiple conditions appends each as a separate segment.
- REQ-T4: `ConditionEvent` applied adds to active conditions; removed deletes by ID.

### Feature 2
- REQ-T5: `NpcInfo.Conditions` is empty when NPC is not in active combat.
- REQ-T6: `NpcInfo.Conditions` contains condition display names when NPC is a combatant with active conditions.
- REQ-T7: `RenderRoomView` renders conditions in Yellow after health/fighting text.
- REQ-T8: `RenderRoomView` renders no condition text when `NpcInfo.Conditions` is empty.

### Feature 3
- REQ-T9: `RenderMap` at `width >= 100` produces side-by-side layout with ` │ ` separator.
- REQ-T10: `RenderMap` at `width < 100` produces stacked layout (current behavior unchanged).
- REQ-T11: Property: all legend entries appear exactly once in the output regardless of grid height.

All tests MUST use property-based testing per SWENG-5a.
