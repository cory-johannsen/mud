# Map POI

Non-combat NPCs and room equipment appear as color-coded POI symbols on the map after a room is explored. See `docs/superpowers/specs/2026-03-21-map-poi-design.md` for full design spec.

## Requirements

### NPC Role Field

- [ ] REQ-POI-1: `npc_role` optional on `npc.Template`; absent/empty = combat NPC, no POI
- [ ] REQ-POI-2: Unknown `npc_role` values map to `npc` POI type
- [ ] REQ-POI-3: `npc.Instance.NpcRole` populated from template in `NewInstanceWithResolver`; prerequisite for handleMap POI logic

### Map Grid Display

- [ ] REQ-POI-4: At most 4 display columns in POI suffix row; 4th slot is `…` when 5+ types present
- [ ] REQ-POI-5: POI suffix row padding uses visible column width, not byte length
- [ ] REQ-POI-6: POI suffix row emitted after room row and before south connector row
- [ ] REQ-POI-7: Unexplored room slots in suffix row rendered as blank spaces
- [ ] REQ-POI-8: Current room follows same POI suffix rules as explored rooms
- [ ] REQ-POI-9: Each POI symbol wrapped in ANSI color with `\033[0m` reset after each character
- [ ] REQ-POI-10: Two-column grid/legend zip logic updated for expanded line count

### Legend

- [ ] REQ-POI-11: POI legend section appears immediately after `Legend:` header, before room list
- [ ] REQ-POI-12: Legend only lists POI types present on at least one tile
- [ ] REQ-POI-13: Legend entries render symbol in color followed by plain-text label
- [ ] REQ-POI-14: Legend entries in Section 1 table order

### Proto and Server

- [ ] REQ-POI-15: `pois` field (field 8) added to `MapTile` proto; populated only for explored tiles
- [ ] REQ-POI-16: Dead NPC instances excluded from POI evaluation
- [ ] REQ-POI-17: `tile.Pois` deduplicated
- [ ] REQ-POI-18: `tile.Pois` sorted in table order

### Client Rendering

- [ ] REQ-POI-19: `RenderMap` reads only from `MapResponse`; no direct state access

### Architecture

- [ ] REQ-POI-20: `NpcRoleToPOIID`, `SortPOIs`, `POISuffixRow` are pure functions
- [ ] REQ-POI-21: `SortPOIs` sorts unknown POI type IDs after all known IDs
