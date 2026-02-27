# Stage 11 — Inventory & Loot System Design

## Goal

Add a complete item system: generic item definitions, player backpack with slot+weight limits, NPC loot tables with currency drops, room floor item tracking, and pickup/drop/inventory commands.

## Currency

Ammunition-as-currency with three display tiers:

- **Round** — base unit (1x)
- **Clip** — 25 Rounds
- **Crate** — 500 Rounds (20 Clips)

Stored internally as a single `int` (total rounds). Decomposed into Crates/Clips/Rounds for display only. Currency is separate from inventory — weightless, does not consume slots.

## Item Definition

```go
type ItemDef struct {
    ID           string  // "pistol_9mm", "medkit"
    Name         string  // "9mm Pistol"
    Description  string
    Kind         string  // "weapon", "explosive", "consumable", "junk"
    Weight       float64 // kg; 0 for weightless
    WeaponRef    string  // WeaponDef.ID when Kind=="weapon"
    ExplosiveRef string  // ExplosiveDef.ID when Kind=="explosive"
    Stackable    bool    // consumables/junk may stack; weapons do not
    MaxStack     int     // max per stack (1 for non-stackable)
    Value        int     // base value in rounds for selling
}
```

- Loaded from `content/items/*.yaml`.
- Weapon and explosive items reference existing `WeaponDef`/`ExplosiveDef` by ID.
- The existing `WeaponDef` and `ExplosiveDef` types remain unchanged.

## Item Instance

```go
type ItemInstance struct {
    InstanceID string // unique UUID
    ItemDefID  string // references ItemDef.ID
    Quantity   int    // 1 for non-stackable; >1 for stackable
}
```

Represents a concrete item in a player's backpack or on a room floor.

## Player Backpack

```go
type Backpack struct {
    MaxSlots  int
    MaxWeight float64
    Items     []ItemInstance
}
```

- Lives on `PlayerSession` alongside a `Currency int` field.
- Each `ItemInstance` consumes one slot (stacks count as one slot).
- Weight check: `sum(instance.Quantity * itemDef.Weight) <= MaxWeight`.
- Slot check: `len(Items) <= MaxSlots`.
- Adding a stackable item to an existing stack does not consume a new slot.
- In-memory only; no DB persistence in this stage.

## NPC Loot Tables

Defined inline in NPC template YAML:

```yaml
loot:
  currency:
    min: 10
    max: 75
  items:
    - item: medkit
      chance: 0.25
      quantity: [1, 1]
    - item: pistol_9mm
      chance: 0.10
      quantity: [1, 1]
```

On NPC death:
1. Roll currency uniformly in `[min, max]` and award to the killing player.
2. For each item entry, roll against `chance`. On success, roll quantity in `[min, max]` and place on the room floor as an `ItemInstance`.

## Room Floor

A `FloorManager` tracks dropped items per room: `map[roomID][]ItemInstance`.

- Items placed on the floor when NPCs die or players drop them.
- Items removed from the floor when players pick them up.
- No decay timer in this stage — items persist until picked up.
- Thread-safe via `sync.RWMutex`.

## Commands

| Command | Aliases | Description |
|---------|---------|-------------|
| `inventory` | `inv`, `i` | Show backpack contents and currency |
| `get <item>` | `take` | Pick up item from room floor into backpack |
| `get all` | `take all` | Pick up all items from floor |
| `drop <item>` | — | Drop item from backpack to room floor |
| `balance` | `bal` | Show currency (Crates/Clips/Rounds) |

## Integration Points

- **`removeDeadNPCsLocked`** in `combat_handler.go`: after removing dead NPC, generate loot and place items on floor, award currency to killer.
- **`look` handler**: append floor items to room description output.
- **Proto messages**: add `InventoryView`, `FloorItem` messages; extend `RoomView` with floor items field.
- **`ItemRegistry`**: new registry in the inventory package for `ItemDef` lookup; loaded at startup alongside weapons/explosives.

## Key Invariants

- Backpack slot count never exceeds `MaxSlots`.
- Backpack total weight never exceeds `MaxWeight`.
- Non-stackable items always have `Quantity == 1`.
- Stackable items never exceed `MaxStack` per instance.
- Currency is always >= 0.
- Floor items are removed atomically on pickup (no double-pickup).
