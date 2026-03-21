# Map POI — Design Spec

**Date:** 2026-03-21
**Status:** Draft
**Feature:** `map-poi` (priority 300)
**Dependencies:** none

---

## Overview

Adds color-coded Points of Interest (POI) symbols to the map grid and legend. POIs represent non-combat NPCs and room equipment. Symbols appear only for rooms the player has already explored (using the existing `AutomapCache`).

---

## 1. POI Types and Symbols

| POI Type ID | Symbol | ANSI Color | Trigger |
|-------------|--------|------------|---------|
| `merchant` | `$` | Cyan (`\033[36m`) | Room has a live, non-dead NPC with `npc_role: merchant` |
| `healer` | `+` | Green (`\033[32m`) | Room has a live, non-dead NPC with `npc_role: healer` |
| `trainer` | `T` | Blue (`\033[34m`) | Room has a live, non-dead NPC with `npc_role: job_trainer` |
| `guard` | `G` | Yellow (`\033[33m`) | Room has a live, non-dead NPC with `npc_role: guard` |
| `npc` | `N` | White (`\033[37m`) | Room has a live, non-dead NPC with any other non-empty `npc_role` |
| `equipment` | `E` | Magenta (`\033[35m`) | Room has at least one `RoomEquipmentConfig` entry |

### 1.1 npc_role Field

`npc.Template` gains a new optional field:

```go
NpcRole string `yaml:"npc_role,omitempty"` // "merchant"|"healer"|"job_trainer"|"guard"|"chip_doc"|... empty for combat NPCs
```

`npc.Instance` gains the same field, copied from the template at spawn time in `NewInstanceWithResolver`:

```go
NpcRole string // copied from Template.NpcRole at spawn
```

- REQ-POI-1: `npc_role` MUST be optional on `npc.Template`. An absent or empty value indicates a combat NPC and MUST contribute no POI.
- REQ-POI-2: `npc_role` values not in the explicit POI type table MUST map to the `npc` POI type (`N`, White) if non-empty.
- REQ-POI-3: `npc.Instance.NpcRole` MUST be populated from `Template.NpcRole` in `NewInstanceWithResolver`. This MUST be implemented before the `handleMap` POI population logic.

### 1.2 POI Evaluation Order

POI symbols are rendered in fixed table order: `merchant`, `healer`, `trainer`, `guard`, `npc`, `equipment`. This order is immutable.

- REQ-POI-4: At most 4 POI symbols MUST be rendered per room cell. If 5 or more POI types are present, symbols 1–3 are rendered normally and the 4th slot MUST display `…` (U+2026, plain, no color). The `…` character counts as 1 display column.
- REQ-POI-5: POI suffix row padding MUST use visible column width (not byte length), because `…` (U+2026) is 3 bytes but 1 display column.

---

## 2. Map Grid Display

The `RenderMap` loop currently emits three row types per y-coordinate band:
1. Room row (cells across all xs)
2. South connector row (if any cell has a south exit)

This spec adds a new row type between them:

1. Room row (cells across all xs) — **existing**
2. **POI suffix row (cells across all xs) — NEW**
3. South connector row — **existing**

Each room cell occupies `cellW = 4` display columns. The POI suffix row for a given cell is a string of up to 4 colored symbols (or blank spaces), padded to exactly `cellW` display columns.

Example output for a y-row containing one explored room with merchant + equipment POIs at x=0, and one unexplored room at x=1:

```
[3]  [4]       ← room row
$E             ← POI suffix row (x=0 has POIs; x=1 is unexplored, blank)
|              ← south connector row
```

- REQ-POI-6: The POI suffix row MUST be emitted after the room row and before the south connector row for each y-coordinate band.
- REQ-POI-7: POI symbols MUST only appear in the suffix row for explored rooms (`AutomapCache[zoneID][roomID] == true`). Unexplored room slots in the suffix row MUST be rendered as `cellW` blank spaces.
- REQ-POI-8: The current room (`current == true`) MUST follow the same POI suffix rules as other explored rooms.
- REQ-POI-9: Each POI symbol character MUST be wrapped in its ANSI color from Section 1, with `\033[0m` reset after each symbol character.

### 2.1 Two-Column Layout Adjustment

`RenderMap` supports a side-by-side grid/legend layout for `width >= 100`. This layout zips grid lines with legend lines using `\r\n` as the line separator. Adding the POI suffix row doubles the number of grid lines per y-band. The zip logic MUST account for the increased line count by treating each y-band as producing 2 or 3 output lines (room row + POI suffix row + optional south connector row), not 1 or 2.

- REQ-POI-10: The two-column grid/legend zip logic MUST be updated to correctly interleave legend lines with the expanded grid line count after adding POI suffix rows.

---

## 3. Legend Display

The map legend gains a "Points of Interest" section. It is inserted directly after the existing `Legend:` header line and before the numbered room list.

Example:
```
Legend:
Points of Interest
  $ Merchant
  E Equipment
[1] Pioneer Square (current)
[2] Morrison Bridge
```

- REQ-POI-11: The POI legend section MUST appear immediately after the `Legend:` header and before the first numbered room entry.
- REQ-POI-12: The POI legend section MUST only list POI types present on at least one tile in the current `MapResponse`. POI types absent from all tiles MUST NOT appear.
- REQ-POI-13: Each legend entry MUST render the symbol in its ANSI color followed by a plain-text label: `<colored-symbol> <Label>`.
- REQ-POI-14: POI legend entries MUST be listed in Section 1 table order.

POI type ID to label:

| POI Type ID | Label |
|-------------|-------|
| `merchant` | `Merchant` |
| `healer` | `Healer` |
| `trainer` | `Trainer` |
| `guard` | `Guard` |
| `npc` | `NPC` |
| `equipment` | `Equipment` |

---

## 4. Proto and Data Flow

### 4.1 MapTile Proto Extension

`api/proto/game/v1/game.proto` — add field 8 to the existing `MapTile` message (field 8 is currently unoccupied):

```protobuf
// ADD to existing MapTile message:
repeated string pois = 8; // POI type IDs present in this room e.g. ["merchant", "equipment"]
```

- REQ-POI-15: `pois` MUST be populated by the server only for tiles where `AutomapCache[zoneID][roomID] == true`. Unexplored tiles MUST have an empty `pois` slice.

### 4.2 Server-Side POI Population

In `handleMap` (`internal/gameserver/grpc_service.go`), for each tile in the map response:

1. If the room is not in `AutomapCache`, set `tile.Pois = nil` and skip POI evaluation.
2. Collect POI type IDs:
   a. Call `s.npcMgr.InstancesInRoom(roomID)` to get live NPC instances.
   b. For each instance where `instance.IsDead() == false` and `instance.NpcRole != ""`, call `maputil.NpcRoleToPOIID(instance.NpcRole)` and add the result to a set.
   c. If `room.Equipment` has length > 0, add `"equipment"` to the set.
3. Convert the set to a slice; sort using `maputil.SortPOIs`.
4. Assign to `tile.Pois`.

- REQ-POI-16: NPC instances where `IsDead() == true` MUST be excluded from POI evaluation.
- REQ-POI-17: POI type IDs in `tile.Pois` MUST be deduplicated (set semantics).
- REQ-POI-18: POI type IDs MUST be sorted in Section 1 table order before assignment.

### 4.3 Client-Side Rendering

`RenderMap` in `internal/frontend/handlers/text_renderer.go`:

1. For each map tile, read `tile.Pois`.
2. Map each POI type ID to its symbol character and ANSI color via the `maputil.POITypes` table.
3. Build the suffix string: up to 3 colored symbol characters; if `len(pois) > 3`, symbol 4 is `…` (U+2026, no color). Pad to `cellW` display columns using visible-width padding.
4. Emit the suffix row as a separate line between the room row and the south connector row (per Section 2).
5. For the legend, collect the union of all POI type IDs across all `MapResponse.Tiles`, sort in table order, and render the POI section immediately after the `Legend:` header (per Section 3).

- REQ-POI-19: `RenderMap` MUST NOT make any network calls or access game state. All POI data MUST come from `MapResponse.Tiles[i].Pois`.

---

## 5. Architecture

### 5.1 POI Registry

New file: `internal/game/maputil/poi.go`, package `maputil`.

```go
package maputil

import (
    "sort"
    "strings"
)

// POIType describes a single Point of Interest type.
type POIType struct {
    ID     string
    Symbol rune
    Color  string // ANSI escape sequence e.g. "\033[36m"
    Label  string
}

// POITypes is the ordered, immutable POI type table.
// Rendering and sorting use this order.
var POITypes = []POIType{
    {ID: "merchant",  Symbol: '$', Color: "\033[36m", Label: "Merchant"},
    {ID: "healer",    Symbol: '+', Color: "\033[32m", Label: "Healer"},
    {ID: "trainer",   Symbol: 'T', Color: "\033[34m", Label: "Trainer"},
    {ID: "guard",     Symbol: 'G', Color: "\033[33m", Label: "Guard"},
    {ID: "npc",       Symbol: 'N', Color: "\033[37m", Label: "NPC"},
    {ID: "equipment", Symbol: 'E', Color: "\033[35m", Label: "Equipment"},
}

// poiOrder maps POI type ID to its index in POITypes for sort comparisons.
var poiOrder = func() map[string]int {
    m := make(map[string]int, len(POITypes))
    for i, p := range POITypes {
        m[p.ID] = i
    }
    return m
}()

// NpcRoleToPOIID maps an npc_role string to a POI type ID.
// Returns "" for empty npcRole (combat NPC; no POI contribution).
// Returns "npc" for any non-empty role not in the explicit mapping.
func NpcRoleToPOIID(npcRole string) string {
    switch strings.ToLower(npcRole) {
    case "":
        return ""
    case "merchant":
        return "merchant"
    case "healer":
        return "healer"
    case "job_trainer":
        return "trainer"
    case "guard":
        return "guard"
    default:
        return "npc"
    }
}

// SortPOIs returns a new slice of POI type IDs sorted in POITypes table order.
// Unknown IDs are sorted last.
func SortPOIs(pois []string) []string {
    out := make([]string, len(pois))
    copy(out, pois)
    sort.Slice(out, func(i, j int) bool {
        oi, okI := poiOrder[out[i]]
        oj, okJ := poiOrder[out[j]]
        if !okI { oi = len(POITypes) }
        if !okJ { oj = len(POITypes) }
        return oi < oj
    })
    return out
}

// POISuffixRow builds a padded POI suffix string for a room cell of width cellW.
// Precondition: cellW >= 1.
// Up to 3 colored symbols are shown; a 4th slot shows … if more exist.
// Returns a blank string of cellW spaces if pois is empty.
func POISuffixRow(pois []string, cellW int) string {
    if len(pois) == 0 {
        return strings.Repeat(" ", cellW)
    }
    var sb strings.Builder
    displayCols := 0
    for i, id := range pois {
        if displayCols >= cellW {
            break
        }
        if i == 3 && len(pois) > 4 {
            sb.WriteString("…") // U+2026, 1 display column
            displayCols++
            break
        }
        var sym rune = '?'
        var color string
        for _, pt := range POITypes {
            if pt.ID == id {
                sym = pt.Symbol
                color = pt.Color
                break
            }
        }
        sb.WriteString(color)
        sb.WriteRune(sym)
        sb.WriteString("\033[0m")
        displayCols++
    }
    // Pad to cellW display columns
    for displayCols < cellW {
        sb.WriteByte(' ')
        displayCols++
    }
    return sb.String()
}
```

- REQ-POI-20: `NpcRoleToPOIID`, `SortPOIs`, and `POISuffixRow` MUST be pure functions with no side effects.
- REQ-POI-21: `SortPOIs` MUST sort unknown POI type IDs (not in the Section 1 table) after all known IDs.

### 5.2 Files Modified

| File | Change |
|------|--------|
| `api/proto/game/v1/game.proto` | Add `repeated string pois = 8` to `MapTile` |
| `internal/game/npc/template.go` | Add `NpcRole string \`yaml:"npc_role,omitempty"\`` |
| `internal/game/npc/instance.go` | Add `NpcRole string`; populate in `NewInstanceWithResolver` |
| `internal/gameserver/grpc_service.go` | Populate `tile.Pois` in `handleMap` per §4.2 |
| `internal/frontend/handlers/text_renderer.go` | Add POI suffix row and legend section per §2 and §3 |
| `internal/game/maputil/poi.go` | New file: POI type table and helper functions |

---

## 6. Requirements Summary

- REQ-POI-1: `npc_role` optional on `npc.Template`; absent/empty = combat NPC, no POI.
- REQ-POI-2: Unknown `npc_role` values map to `npc` POI type.
- REQ-POI-3: `npc.Instance.NpcRole` populated from template in `NewInstanceWithResolver`; required before `handleMap` POI logic.
- REQ-POI-4: At most 4 display columns in POI suffix row; 4th slot is `…` when 5+ types present.
- REQ-POI-5: POI suffix row padding uses visible column width, not byte length.
- REQ-POI-6: POI suffix row emitted after room row and before south connector row.
- REQ-POI-7: Unexplored room slots in suffix row rendered as blank spaces.
- REQ-POI-8: Current room follows same POI suffix rules as explored rooms.
- REQ-POI-9: Each POI symbol wrapped in ANSI color with `\033[0m` reset after each character.
- REQ-POI-10: Two-column grid/legend zip logic updated for expanded line count.
- REQ-POI-11: POI legend section appears immediately after `Legend:` header, before room list.
- REQ-POI-12: Legend only lists POI types present on at least one tile.
- REQ-POI-13: Legend entries render symbol in color followed by plain-text label.
- REQ-POI-14: Legend entries in Section 1 table order.
- REQ-POI-15: `pois` populated only for explored tiles; unexplored = empty slice.
- REQ-POI-16: Dead NPC instances excluded from POI evaluation.
- REQ-POI-17: `tile.Pois` deduplicated.
- REQ-POI-18: `tile.Pois` sorted in Section 1 table order.
- REQ-POI-19: `RenderMap` reads only from `MapResponse`; no direct state access.
- REQ-POI-20: `NpcRoleToPOIID`, `SortPOIs`, `POISuffixRow` are pure functions.
