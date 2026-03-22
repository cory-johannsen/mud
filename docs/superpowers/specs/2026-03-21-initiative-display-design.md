# Initiative Display on Character Sheet — Design Spec

## Overview

This feature adds an Initiative entry to the Saves section of the character sheet left column. Initiative is displayed as a signed modifier (d20 + DexMod) so players can immediately see their combat turn order bonus without performing mental arithmetic. No new proto fields, no database changes, and no new data are required — DexMod is already present on `CharacterSheetView`.

## Requirements

- REQ-INIT-1: The character sheet Saves section MUST display Initiative as a fifth entry, below Awareness.
- REQ-INIT-2: The Initiative value MUST be calculated as DexMod sourced from the existing `CharacterSheetView` proto field.
- REQ-INIT-3: The Initiative value MUST be rendered as a signed integer (e.g. `+3`, `-1`, `+0`) matching the format used by Toughness, Hustle, Cool, and Awareness.
- REQ-INIT-4: The label MUST be `Initiative`.
- REQ-INIT-5: The layout MUST follow the existing label-left / value-right pattern used by all other Saves entries.
- REQ-INIT-6: No new proto fields, schema migrations, or database changes MUST be introduced.

## Design

### Formula

Initiative modifier = DexMod. This is the deterministic addend applied to every initiative roll (d20 + DexMod). Because the d20 roll is stochastic and not stored, only the modifier is meaningful to display as a standing stat. DexMod is already available on `CharacterSheetView`, so no data plumbing is required.

### Placement

The entry is appended as the fifth row in the Saves block of the character sheet left column, immediately after Awareness. It uses the same label-left / value-right two-column layout and signed-integer formatting (`fmt.Sprintf("%+d", value)`) as all adjacent entries.

### Implementation Notes

- File to change: `internal/frontend/handlers/text_renderer.go`
- Function to change: `RenderCharacterSheet`
- Locate the block that appends Toughness, Hustle, Cool, and Awareness rows to the Saves section and append one additional row for Initiative using `csv.DexMod` as the value source.
- No changes to proto definitions, generated files, database schema, or any other file are required.

## Out of Scope

- Dynamic initiative (rolls, tiebreakers, or bonus sources other than DexMod) are not included.
- Displaying initiative in the combat log, room view, or any UI surface other than the character sheet Saves section is not included.
- A new proto field for initiative is not introduced.
- Any server-side or database-side change is not included.
