# Initiative Display on Character Sheet — Design Spec

## Overview

This feature adds an Initiative entry to the Saves section of the character sheet left column. Initiative is displayed as a signed modifier so players can immediately see their combat turn order bonus without performing mental arithmetic. No new proto fields, no database changes, and no new data are required — Quickness (the raw ability score) is already present on `CharacterSheetView` and the modifier is derived from it.

## Requirements

- REQ-INIT-1: The character sheet Saves section MUST display Initiative as a dedicated row immediately below the Awareness row.
- REQ-INIT-2: The Initiative value MUST be calculated as `(quickness - 10) / 2` using the `quickness` field already present on `CharacterSheetView`, matching the DexMod component of the combat initiative roll (`d20 + DexMod`).
- REQ-INIT-3: The Initiative value MUST be rendered as a signed integer (e.g. `+3`, `-1`, `+0`) using the existing `signedInt()` helper, consistent with Toughness, Hustle, Cool, and Awareness.
- REQ-INIT-4: The label MUST be `Initiative`.
- REQ-INIT-5: The layout MUST follow the same single-row label-left / value-right pattern used by the Awareness row.
- REQ-INIT-6: New proto fields, schema migrations, and database changes MUST NOT be introduced.

## Design

### Formula

Initiative modifier = `(quickness - 10) / 2`. This is the deterministic addend applied to every initiative roll (`d20 + DexMod`). Because the d20 roll is stochastic and not stored, only the modifier is meaningful to display as a standing stat. `Quickness` is already present on `CharacterSheetView` via `csv.GetQuickness()`; no data plumbing is required.

### Placement

Initiative is appended as a dedicated row immediately below the Awareness row in the Saves section of the character sheet left column. It does not share a row with other stats (unlike Toughness/Hustle/Cool which share one row). The format matches the Awareness row pattern.

### Implementation Notes

- File to change: `internal/frontend/handlers/text_renderer.go`
- Function to change: `RenderCharacterSheet`
- Locate the block that appends the Awareness row in the Saves section and append one additional row for Initiative immediately after it.
- Value: `signedInt((int(csv.GetQuickness()) - 10) / 2)`
- No changes to proto definitions, generated files, database schema, or any other file are required.

## Out of Scope

- Dynamic initiative bonuses from feats, equipment, or other sources are not included.
- Displaying initiative in the combat log, room view, or any UI surface other than the character sheet Saves section is not included.
- A new proto field for a precomputed initiative modifier is not introduced.
- Any server-side or database-side change is not included.
