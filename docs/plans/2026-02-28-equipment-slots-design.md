# Equipment Slots Design

**Date:** 2026-02-28

## Goal

Add persistent equipment slots for armor, accessories, and weapons. Weapon slots are organized into swappable loadout presets. Armor and accessories are shared across all presets.

## Data Model

### WeaponKind

Add `Kind` field to `WeaponDef` (YAML + struct):

- `one_handed` — can go in main hand or off-hand
- `two_handed` — main hand only; locks off-hand empty
- `shield` — off-hand only; requires one-handed or empty main hand

### WeaponPreset

Replaces the existing `Loadout` struct in `internal/game/inventory/`:

```go
type WeaponPreset struct {
    MainHand *EquippedWeapon  // nil = empty
    OffHand  *EquippedWeapon  // nil = empty; locked if main is two-handed
}
```

Constraints enforced at equip time:
- Two-handed main hand → off-hand must be empty
- Shield in off-hand → main hand must be one-handed or empty
- One-handed in off-hand → main hand must be one-handed or empty (dual wield)

### LoadoutSet

```go
type LoadoutSet struct {
    Presets          []*WeaponPreset
    Active           int   // 0-based index of active preset
    SwappedThisRound bool  // reset at start of each combat round
}
```

Starts with 2 presets. Class features grow the `Presets` slice. Swap is a standard action, once per round.

### Equipment

Armor and accessories, shared across all presets:

```go
type ArmorSlot string
const (
    SlotHead     ArmorSlot = "head"
    SlotLeftArm  ArmorSlot = "left_arm"
    SlotRightArm ArmorSlot = "right_arm"
    SlotTorso    ArmorSlot = "torso"
    SlotLeftLeg  ArmorSlot = "left_leg"
    SlotRightLeg ArmorSlot = "right_leg"
    SlotFeet     ArmorSlot = "feet"
)

type AccessorySlot string
const (
    SlotNeck  AccessorySlot = "neck"
    SlotRing1 AccessorySlot = "ring_1"
    // ... ring_2 through ring_10
)

type Equipment struct {
    Armor       map[ArmorSlot]*EquippedItem
    Accessories map[AccessorySlot]*EquippedItem
}
```

### Session Integration

`PlayerSession` replaces its `Loadout *inventory.Loadout` field with:
- `LoadoutSet *inventory.LoadoutSet`
- `Equipment *inventory.Equipment`

Both are loaded from DB on session start and persisted on save-state.

## Persistence

### Migration

Two new tables:

```sql
CREATE TABLE character_weapon_presets (
    id           BIGSERIAL PRIMARY KEY,
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    preset_index INT NOT NULL,
    slot         TEXT NOT NULL,   -- "main_hand" or "off_hand"
    item_def_id  TEXT NOT NULL,
    ammo_count   INT NOT NULL DEFAULT 0,
    CONSTRAINT uq_character_preset_slot UNIQUE (character_id, preset_index, slot)
);

CREATE TABLE character_equipment (
    id           BIGSERIAL PRIMARY KEY,
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    slot         TEXT NOT NULL,   -- "head", "torso", "ring_1", etc.
    item_def_id  TEXT NOT NULL,
    CONSTRAINT uq_character_equipment_slot UNIQUE (character_id, slot)
);
```

### Repository

New methods on the postgres character repository:
- `LoadWeaponPresets(ctx, characterID) (*inventory.LoadoutSet, error)`
- `SaveWeaponPresets(ctx, characterID, loadoutSet) error`
- `LoadEquipment(ctx, characterID) (*inventory.Equipment, error)`
- `SaveEquipment(ctx, characterID, equipment) error`

Armor/accessory item definitions do not exist until feature #4 (weapon and armor library). `character_equipment` rows will be empty until then; the schema and load/save infrastructure is in place.

## Commands

### `loadout [1|2]`

No argument: display both presets, indicate active one.
With argument: swap to that preset (standard action, once per round). Error if already swapped this round or preset index invalid.

### `equip <item> [main|off]`

Equips an item from the backpack. Weapons require explicit slot argument (`main` or `off`). Armor and accessories auto-detect their slot from the item definition. Enforces all weapon constraints at equip time.

### `unequip <slot>`

Moves the item in that slot back to the backpack. Valid slot names: `main`, `off`, `head`, `torso`, `left_arm`, `right_arm`, `left_leg`, `right_leg`, `feet`, `neck`, `ring_1`…`ring_10`.

### `equipment`

Displays all equipped armor/accessories and the active weapon preset summary, with a secondary summary of the inactive preset(s).

## Combat Integration

`SwappedThisRound` is reset to `false` at the start of each combat round tick in the existing combat engine.

## Out of Scope

- Armor/accessory item definitions (feature #4: weapon and armor library)
- Stat bonuses from equipped items (feature #4)
- More than 2 loadout presets (granted by class features, future)
