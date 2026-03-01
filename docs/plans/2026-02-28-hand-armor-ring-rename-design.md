# Hand Armor Slot + Ring Rename + Human-Readable Display Design

**Date:** 2026-02-28

## Goal

Add a `hands` armor slot, rename ring slots to reflect which hand they belong to (`left_ring_1`–`left_ring_5`, `right_ring_1`–`right_ring_5`), and display all slot names using human-readable labels in the equipment command output.

## Constraints

- No proto changes required.
- No database migration required (no existing data).
- Slot identifiers (storage keys) and display names are kept separate (Option A).

## Architecture

Three changes in the inventory/equipment layer and display layer:

1. **New `hands` armor slot** — add `SlotHands ArmorSlot = "hands"` alongside the existing 7 armor slots.
2. **Ring slot rename** — replace `ring_1`–`ring_10` with `left_ring_1`–`left_ring_5` and `right_ring_1`–`right_ring_5`. All `AccessorySlot` constants and all references updated.
3. **Human-readable display names** — a `SlotDisplayName(slot string) string` function in the inventory package maps every slot identifier to its label. Equipment display calls this instead of printing raw slot strings. Unknown slots fall back to the raw string.

## Components

- `internal/game/inventory/equipment.go` — add `SlotHands`; rename ring constants; add `SlotDisplayName()` with complete map
- `internal/game/command/equipment.go` — call `SlotDisplayName()` for every label; add hands to armor section; show left/right ring groups
- `internal/game/command/unequip.go` — update `validUnequipSlots` with `hands` and new ring names
- `internal/game/command/equip.go` — update slot name references if present

## Display Format

```
Weapons
  Preset 1 [active]
    Main Hand:  Sword
    Off Hand:   Shield
  Preset 2
    Main Hand:  empty
    Off Hand:   empty

Armor
  Head:       empty
  Torso:      empty
  Left Arm:   empty
  Right Arm:  empty
  Hands:      empty
  Left Leg:   empty
  Right Leg:  empty
  Feet:       empty

Accessories
  Neck:              empty
  Left Hand Ring 1:  empty
  Left Hand Ring 2:  empty
  Left Hand Ring 3:  empty
  Left Hand Ring 4:  empty
  Left Hand Ring 5:  empty
  Right Hand Ring 1: empty
  Right Hand Ring 2: empty
  Right Hand Ring 3: empty
  Right Hand Ring 4: empty
  Right Hand Ring 5: empty
```

## Error Handling

- `SlotDisplayName()` returns the raw slot string as a fallback for unknown slots — no panics if a new slot is added without a display entry.

## Testing

- Property-based tests: `SlotDisplayName()` returns non-empty, non-identical-to-key string for every known slot constant.
- Unit tests: `validUnequipSlots` includes `hands` and all 10 new ring names; excludes all old `ring_1`–`ring_10` names.
- Unit tests: `HandleEquipment()` output contains human-readable labels (not raw slot keys).
