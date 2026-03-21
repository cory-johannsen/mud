# Crafting — Design Spec

**Date:** 2026-03-20
**Status:** Draft
**Feature:** `crafting` (priority 236)
**Dependencies:** `non-combat-npcs` (merchant material stock), `exploration` (scavenge in exploration mode), `downtime` (downtime Craft activity), `wire-refactor` (ContentDeps/StorageDeps on GameServiceServer)

---

## Overview

Players craft weapons, armor, items, and consumables from typed individual materials. Recipes are defined in YAML and globally available to any player with sufficient Rigging (the Gunchete skill equivalent of PF2E Crafting) proficiency rank. Materials are scavenged from zones, looted from NPCs, or purchased from merchants. Crafting uses a hybrid quick/downtime model: simple recipes are resolved in a single check; complex recipes consume downtime days via the `downtime` feature.

---

## 1. Recipe Data Model

Recipes live in `content/recipes/` as YAML files. Each recipe defines:

```yaml
id: stimpack
name: Stimpack
output_item_id: stimpack          # must reference a valid item in the inventory registry
output_count: 1                   # number of items produced on success
category: consumable              # consumable | weapon | armor | item
complexity: 1                     # 1=basic, 2=standard, 3=advanced, 4=expert
dc: 14                            # Rigging check DC
quick_craft_min_rank: trained     # untrained | trained | expert | master | legendary
                                  # if omitted, complexity-tier default applies (see Table 1.1)
                                  # set to "never" to force downtime-only regardless of rank
materials:
  - id: hydrogen_peroxide
    quantity: 2
  - id: rubbing_alcohol
    quantity: 1
  - id: sugar_packets
    quantity: 1
description: "A field-mixed stimulant that restores 1d8 HP."
```

### 1.1 Complexity Tiers

| Complexity | Label | Default category | Default `quick_craft_min_rank` |
|---|---|---|---|
| 1 | Basic | Consumable, misc | untrained |
| 2 | Standard | Trap, ammo, simple item | trained |
| 3 | Advanced | Weapon, complex item | expert |
| 4 | Expert | Armor, rare weapon | master |

If `quick_craft_min_rank` is absent from a recipe's YAML, the complexity-tier default from this table applies. To force a recipe to downtime-only regardless of player rank, set `quick_craft_min_rank: never` explicitly. A player whose Rigging rank meets or exceeds `quick_craft_min_rank` may quick-craft; below that rank, the recipe requires downtime days.

### 1.2 Recipe Registry

`RecipeRegistry` is loaded at startup from `content/recipes/`. All recipes are globally available — no per-character unlocking. The registry validates:

- Every `materials[].id` exists in `MaterialRegistry` — fatal load error otherwise (REQ-CRAFT-6).
- Every `output_item_id` resolves against the inventory registry using the category-to-registry mapping in Section 1.3 — fatal load error otherwise (REQ-CRAFT-10).

### 1.3 `output_item_id` Validation

The inventory `Registry` stores different item types in separate maps. Validation looks up `output_item_id` using the recipe's `category` field:

| Recipe `category` | Registry lookup |
|---|---|
| `consumable` | `Registry.Item(id)` where `ItemDef.Kind == "consumable"` |
| `item` | `Registry.Item(id)` (any kind) |
| `weapon` | `Registry.Weapon(id)` |
| `armor` | `Registry.Armor(id)` |

A `category: weapon` recipe whose `output_item_id` resolves only in `Registry.Item()` (not `Registry.Weapon()`) is a fatal load error. The category and the registry lookup must agree.

- REQ-CRAFT-6: `content/materials.yaml` MUST be the single source of truth for all valid material IDs. Recipes referencing unknown material IDs MUST be a fatal load error.
- REQ-CRAFT-10: Recipes referencing unknown `output_item_id` values per the category/registry mapping in Section 1.3 MUST be a fatal load error.

---

## 2. Material Registry

All materials are declared in `content/materials.yaml` — a flat registry of ~100 material IDs with display name, category, and base credit value. The `value` field is cosmetic at this stage (used for display and future merchant pricing); it is not consumed by any crafting logic in this feature.

```yaml
materials:
  - id: hydrogen_peroxide
    name: Hydrogen Peroxide
    category: chemical
    value: 5

  - id: scrap_metal
    name: Scrap Metal
    category: mechanical
    value: 2
```

**Categories:** `mechanical`, `chemical`, `organic`, `electrical`, `misc`

### 2.1 Material List

**Mechanical (20):** scrap_metal, copper_tubing, machine_screws, hex_bolts, steel_plates, lenses, coil_springs, salvaged_gears, door_hinges, steel_brackets, pop_rivets, flat_washers, pipe_fittings, wire_mesh, sheet_metal_strips, rebar_sections, chain_links, cable_clamps, ball_bearings, valve_stems

**Chemical (20):** bleach, acetone, ammonia, rubbing_alcohol, hydrogen_peroxide, drain_cleaner, table_salt, baking_soda, vinegar, motor_oil, brake_fluid, lighter_fluid, paint_thinner, mineral_spirits, glycerin, borax, epoxy_resin, soldering_flux, battery_acid, isopropyl_alcohol

**Organic (20):** wild_mushrooms, walnuts, wildflowers, bark_strips, tree_sap, blackberries, acorns, dried_nettles, wild_garlic, cattail_fluff, pine_needles, rose_hips, dandelion_roots, moss_clumps, lichen, fern_fronds, cedar_bark, willow_bark, hemlock_seeds, chanterelles

**Electrical (20):** copper_wire, foam_padding, circuit_boards, capacitors, resistors, led_strips, aa_batteries, transistors, fuses, toggle_switches, relay_coils, heat_sinks, antenna_wire, solder_sticks, electrical_tape, microchips, potentiometer_dials, diodes, fiber_optic_strands, solar_cells

**Misc (20):** sugar_packets, instant_coffee, salt_packets, powdered_milk, cooking_oil, hot_sauce, dried_beans, rice, corn_syrup, protein_powder, multivitamins, energy_drink_powder, hard_candy, cigarettes, rolling_papers, tin_foil, plastic_wrap, rubber_bands, zip_ties, duct_tape

---

## 3. Material Inventory

Materials are stored as a quantity map, persisted in a new `character_materials` table:

```sql
character_materials (
    character_id  bigint       NOT NULL REFERENCES characters(id),
    material_id   text         NOT NULL,
    quantity      int          NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (character_id, material_id)
)
```

Materials stack — no per-item instance tracking. At login, `character_materials` rows are loaded into `PlayerSession.Materials map[string]int`. When a material quantity reaches zero, the row is deleted (the `CHECK (quantity > 0)` constraint enforces this — `Decrement` to zero MUST delete the row, not set quantity=0).

**Commands:**

```
materials                   — list all materials in inventory with quantities, grouped by category
materials <category>        — filter by category
```

Materials are weightless — bulk/encumbrance is deferred to the `equipment-mechanics` feature.

- REQ-CRAFT-7: `character_materials` MUST be loaded into `PlayerSession.Materials` at login and persisted on every craft transaction.
- REQ-CRAFT-12: All material deductions for a single craft transaction MUST be executed within a single database transaction. A partial deduction that fails mid-way MUST be rolled back entirely.

---

## 4. Material Sources

### 4.1 Merchants

Merchant NPC YAML gains an optional `material_stock` list alongside the existing item inventory:

```yaml
material_stock:
  - id: hydrogen_peroxide
    price: 8
    restock_quantity: 10
  - id: rubbing_alcohol
    price: 6
    restock_quantity: 10
```

The `buy` command is extended to detect whether the target item is an inventory item or a material:
- If `buy <name>` matches a material in the merchant's `material_stock`, the transaction deducts credits and increments `character_materials` (via `CharacterMaterialsRepository`).
- If `buy <name>` matches an inventory item in the merchant's item stock, the existing item purchase flow applies unchanged.
- If the name is ambiguous (matches both), the game prompts the player to disambiguate.

Merchant material stock uses the same replenishment timer as item stock (per `non-combat-npcs` spec).

### 4.2 Scavenging

A new `scavenge` command is available in exploration mode and during downtime. The player makes a Scavenging skill check vs the zone's `material_pool.dc`. Any attempt (regardless of outcome) exhausts scavenging for that room visit. The exhaustion flag is tracked in `PlayerSession.ScavengeExhaustedRoomID string` — set to the current `RoomID` on any scavenge attempt; cleared to empty string on room exit.

Zone YAML gains a `material_pool` block:

```yaml
material_pool:
  dc: 14
  drops:
    - id: scrap_metal
      weight: 30
    - id: copper_wire
      weight: 20
    - id: wild_mushrooms
      weight: 15
```

| Scavenge outcome | Result |
|---|---|
| Critical success | 3 materials drawn from pool |
| Success | 1–2 materials drawn from pool |
| Failure | Nothing |
| Critical failure | Nothing |

All four outcomes exhaust scavenging for the room visit.

- REQ-CRAFT-8: The `scavenge` command MUST use the Scavenging skill vs the zone's `material_pool.dc`.
- REQ-CRAFT-9: A zone with no `material_pool` defined MUST yield nothing on `scavenge` with no error.
- REQ-CRAFT-11: `scavenge` MUST be limited to one attempt per room entry per player. The exhaustion flag MUST be stored in `PlayerSession.ScavengeExhaustedRoomID` and cleared on room exit.

### 4.3 NPC Loot Drops

NPC YAML gains an optional `material_drops` list resolved on NPC death alongside existing item drops:

```yaml
material_drops:
  - id: scrap_metal
    quantity_min: 1
    quantity_max: 3
    chance: 0.6
  - id: copper_wire
    quantity_min: 1
    quantity_max: 2
    chance: 0.3
```

Each entry is resolved independently. Dropped materials are tracked by `FloorManager` via a new `MaterialDrops map[string]map[string]int` field (roomID → materialID → quantity), separate from the existing `ItemInstance` floor items. Players claim floor materials via the existing `take` command, which is extended to detect material names alongside item names. The room view proto gains a `floor_materials` repeated field listing material names and quantities visible in the room.

---

## 5. Crafting Commands

```
craft list                  — list all recipes the player can attempt, filtered by Rigging rank
craft list <category>       — filter by category (weapon, armor, consumable, item)
craft <recipe-id|name>      — display recipe details and prompt for confirmation
craft confirm               — execute quick craft or hand off to downtime
```

### 5.1 Pending Craft State

The two-step `craft` / `craft confirm` flow stores pending state in `PlayerSession`:

```go
PendingCraftRecipeID string  // empty = no pending craft
```

`PendingCraftRecipeID` is set when `craft <item>` is successfully validated. It is cleared by:
- `craft confirm` (regardless of outcome)
- Any command other than `craft confirm`
- Room exit

`craft confirm` MUST fail with a message if `PendingCraftRecipeID` is empty.

### 5.2 `craft list`

Displays all recipes where the player's Rigging rank meets `quick_craft_min_rank` (quick-craftable) OR the recipe is completable via downtime. Each entry shows:
- Recipe name and category
- Materials required with have/need counts
- `[missing: N]` indicator where N is the count of material types the player is short on
- `[downtime only]` tag when quick craft is not available at the player's rank

- REQ-CRAFT-1: `craft list` MUST show all recipes accessible to the player's rank. Recipes above the player's quick-craft threshold MUST show `[downtime only]`. Recipes for which the player lacks sufficient materials MUST show `[missing: N]` where N is the count of material types the player is short on.

### 5.3 `craft <item>` Flow

1. `craft <item>` — validates materials, displays recipe details (materials required with have/need per line, DC, quick vs downtime, expected output). Sets `PendingCraftRecipeID`. Fails with missing materials list if insufficient.
2. `craft confirm` — executes quick craft or initiates downtime Craft activity.

- REQ-CRAFT-2: `craft <item>` MUST fail with a message listing missing materials if the player has insufficient materials.
- REQ-CRAFT-3: Materials MUST be deducted at `craft confirm` time, not at completion time.
- REQ-CRAFT-13: `craft confirm` MUST fail with a message if `PlayerSession.PendingCraftRecipeID` is empty.

### 5.4 Quick Craft Outcomes

| Outcome | Effect |
|---|---|
| Critical success | `output_count + 1` copies of `output_item_id` produced; all materials consumed |
| Success | `output_count` copies of `output_item_id` produced; all materials consumed |
| Failure | No items produced; half of each required material quantity consumed (rounded down per material) |
| Critical failure | No items produced; all required materials consumed |

- REQ-CRAFT-4: Quick craft critical failure MUST consume all required materials with no output.
- REQ-CRAFT-5: Quick craft failure MUST consume half of each required material quantity (rounded down per material line), with no output.

### 5.5 Downtime Craft Handoff

When the player's Rigging rank is below `quick_craft_min_rank`, `craft confirm` initiates a downtime Craft activity. Materials are deducted immediately at confirm time (REQ-CRAFT-3). The `downtime` feature implements against this interface:

```go
// DowntimeCraftStarter is implemented by the downtime feature and injected into the crafting engine.
type DowntimeCraftStarter interface {
    BeginCraftActivity(ctx context.Context, characterID int64, recipeID string, daysRequired int) error
}
```

`daysRequired` is computed from recipe complexity:

| Complexity | Downtime days required |
|---|---|
| 2 — Standard | 1 day |
| 3 — Advanced | 2 days |
| 4 — Expert | 4 days |

Complexity 1 recipes always have `quick_craft_min_rank: untrained` — they are never downtime-only. On completion the downtime feature emits a `CraftResultEvent` and delivers the output item to the player's inventory.

---

## 6. Architecture

### 6.1 New Package

```
internal/game/crafting/
  recipe.go        — Recipe struct, RecipeRegistry, YAML loader (content/recipes/)
  materials.go     — Material struct, MaterialRegistry, YAML loader (content/materials.yaml)
  engine.go        — CraftingEngine: ExecuteQuickCraft(), BeginDowntimeCraft(starter DowntimeCraftStarter)
```

### 6.2 New Content Files

```
content/materials.yaml          — flat material registry (~100 materials)
content/recipes/                — one YAML file per recipe (or grouped by category)
```

### 6.3 Proto Messages

New messages added to `ClientMessage` oneof:
- `MaterialsRequest { category string }` — empty category = unfiltered; response via existing text console event
- `CraftListRequest { category string }` — empty category = unfiltered
- `CraftRequest { recipe_id string }`
- `CraftConfirmRequest {}`

New event added to `ServerEvent` oneof:
- `CraftResultEvent { success bool, item_id string, quantity int32, materials_lost repeated MaterialLoss }`

### 6.4 Storage

New repository: `CharacterMaterialsRepository` in `internal/storage/postgres/` with methods:

```go
Load(ctx context.Context, characterID int64) (map[string]int, error)
DeductMany(ctx context.Context, characterID int64, deductions map[string]int) error
// DeductMany executes all deductions in a single DB transaction.
// Rows where quantity reaches zero are deleted (not set to 0).
Add(ctx context.Context, characterID int64, materialID string, amount int) error
```

### 6.5 FloorManager Extension

`FloorManager` gains a new `MaterialDrops map[string]map[string]int` field (roomID → materialID → quantity) for NPC material loot drops. This is separate from the existing `ItemInstance` floor items. The room view proto gains a `floor_materials` repeated field. The `take` command is extended to handle material names alongside item names — taking a material increments `character_materials` and decrements `FloorManager.MaterialDrops`.

### 6.6 GameServiceServer Integration

`GameServiceServer` gains:
- `craftingReg *crafting.RecipeRegistry` — added to `ContentDeps`
- `materialReg *crafting.MaterialRegistry` — added to `ContentDeps`
- `materialRepo CharacterMaterialsRepository` — added to `StorageDeps`

Per the `wire-refactor` spec, `StorageDeps` includes a `wire.Bind` for `CharacterMaterialsRepository`.

### 6.7 Command Pattern

`scavenge`, `craft`, and `materials` each follow CMD-1 through CMD-7: `Handler*` constant, `BuiltinCommands()` entry, proto message in `ClientMessage` oneof, bridge handler in `bridgeHandlerMap`, `handle*` case in `grpc_service.go`.

---

## 7. Requirements Summary

- REQ-CRAFT-1: `craft list` MUST show all accessible recipes. Recipes above quick-craft threshold MUST show `[downtime only]`. Recipes with insufficient materials MUST show `[missing: N]`.
- REQ-CRAFT-2: `craft <item>` MUST fail with a message listing missing materials if the player has insufficient materials.
- REQ-CRAFT-3: Materials MUST be deducted at `craft confirm` time, not at completion time.
- REQ-CRAFT-4: Quick craft critical failure MUST consume all required materials with no output.
- REQ-CRAFT-5: Quick craft failure MUST consume half of each required material quantity (rounded down per material), with no output.
- REQ-CRAFT-6: `content/materials.yaml` MUST be the single source of truth for all valid material IDs; recipes referencing unknown material IDs MUST be a fatal load error.
- REQ-CRAFT-7: `character_materials` MUST be loaded into `PlayerSession.Materials` at login and persisted on every craft transaction.
- REQ-CRAFT-8: The `scavenge` command MUST use the Scavenging skill vs the zone's `material_pool.dc`.
- REQ-CRAFT-9: A zone with no `material_pool` MUST yield nothing on `scavenge` with no error.
- REQ-CRAFT-10: Recipes referencing unknown `output_item_id` values per the category/registry mapping in Section 1.3 MUST be a fatal load error.
- REQ-CRAFT-11: `scavenge` MUST be limited to one attempt per room entry per player, tracked in `PlayerSession.ScavengeExhaustedRoomID`, cleared on room exit.
- REQ-CRAFT-12: All material deductions for a single craft transaction MUST execute within a single database transaction and roll back entirely on failure.
- REQ-CRAFT-13: `craft confirm` MUST fail with a message if `PlayerSession.PendingCraftRecipeID` is empty.
