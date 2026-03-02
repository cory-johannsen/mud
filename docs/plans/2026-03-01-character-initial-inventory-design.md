# Character Initial Inventory — Design

**Date:** 2026-03-01
**Feature:** Character initial inventory — each new character receives a job-appropriate starting kit on first login.

---

## Summary

On a character's first login, the server grants a full starting kit (weapon, armor, consumables, currency) appropriate to the character's job. The kit is auto-equipped. A DB flag prevents re-granting on subsequent logins. Inventory is persisted across sessions.

---

## Architecture

Three layers:

1. **Content layer** — `content/loadouts/<archetype>.yaml` defines the base kit per archetype with optional `team_gun` and `team_machete` override sections. Each job YAML may add a `starting_inventory` block to override specific slots.

2. **LoadoutDef loader** — `internal/game/inventory/loadout.go` reads and merges YAML into a `StartingLoadout` struct. Merge order (later wins): archetype base → team section → job override.

3. **Grant logic** — at login, the server checks `has_received_starting_inventory` in the DB. If false: resolve the `StartingLoadout` for the character's job/archetype/team, populate backpack, auto-equip weapon + armor slots, set currency, save inventory, flip the flag.

---

## Data Schema

### Archetype loadout YAML (`content/loadouts/<archetype>.yaml`)

```yaml
archetype: aggressor
base:
  weapon: combat_knife          # item ID from content/items/
  armor:
    torso: kevlar_vest          # slot: item ID
    hands: tactical_gloves
  consumables:
    - item: canadian_bacon
      quantity: 2
  currency: 50

team_gun:
  weapon: ganger_pistol
  armor:
    torso: tactical_vest

team_machete:
  weapon: cheap_blade
  armor:
    torso: leather_jacket
```

### Job YAML override (added to `content/jobs/<job>.yaml`)

```yaml
starting_inventory:
  weapon: heavy_revolver
  armor:
    head: combat_helmet
  consumables:
    - item: canadian_bacon
      quantity: 3
  currency: 100
```

### Merge rules

1. Start with archetype `base`
2. If job has `team: gun` → overlay `team_gun` section (only specified fields override)
3. If job YAML has `starting_inventory` → overlay that (only specified fields override)

### Go structs

```go
// StartingLoadout is the fully-merged starting kit for a character.
type StartingLoadout struct {
    Weapon      string              // item ID; empty means no weapon granted
    Armor       map[ArmorSlot]string // slot → item ID; only populated slots granted
    Consumables []ConsumableGrant
    Currency    int
}

type ConsumableGrant struct {
    ItemID   string
    Quantity int
}
```

---

## Database & Persistence

### Migration

```sql
-- Flag: has this character already received their starting inventory?
ALTER TABLE characters
    ADD COLUMN has_received_starting_inventory BOOLEAN NOT NULL DEFAULT FALSE;

-- Backpack persistence
CREATE TABLE character_inventory (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    item_def_id  TEXT   NOT NULL,
    quantity     INT    NOT NULL DEFAULT 1,
    PRIMARY KEY (character_id, item_def_id)
);
```

### CharacterSaver interface additions

```go
LoadInventory(ctx context.Context, characterID int64) ([]InventoryItem, error)
SaveInventory(ctx context.Context, characterID int64, items []InventoryItem) error
HasReceivedStartingInventory(ctx context.Context, characterID int64) (bool, error)
MarkStartingInventoryGranted(ctx context.Context, characterID int64) error
```

Where `InventoryItem` is `struct { ItemDefID string; Quantity int }`.

### Login flow (updated)

1. Load equipment + weapon presets (existing)
2. `LoadInventory` → populate backpack
3. `HasReceivedStartingInventory` → if false:
   a. Resolve `StartingLoadout` for job/archetype/team
   b. Add items to backpack
   c. Auto-equip: weapon → main hand, armor → correct slots
   d. Set currency
   e. `SaveInventory`
   f. `MarkStartingInventoryGranted`

### Disconnect flow (updated)

- `SaveInventory` called alongside existing `SaveEquipment` / `SaveWeaponPresets`

---

## Content Plan

### Archetype loadouts (6 files)

| Archetype | Base Weapon | Gun-team Weapon | Machete-team Weapon | Torso Armor | Consumables | Currency |
|---|---|---|---|---|---|---|
| aggressor | combat_knife | ganger_pistol | cheap_blade | kevlar_vest | 2× canadian_bacon | 50 |
| criminal | ceramic_shiv | holdout_derringer | ceramic_shiv | corp_suit_liner | 2× canadian_bacon | 100 |
| drifter | rebar_club | ganger_pistol | rebar_club | leather_jacket | 3× canadian_bacon | 25 |
| influencer | stun_baton | emp_pistol | stun_baton | corp_suit_liner | 2× canadian_bacon | 75 |
| nerd | stun_baton | smartgun_pistol | stun_baton | kevlar_vest | 2× canadian_bacon | 75 |
| normie | rebar_club | ganger_pistol | rebar_club | leather_jacket | 3× canadian_bacon | 30 |

### Job overrides (4 team-specific jobs)

- `boot_gun` → heavy_revolver, head: combat_helmet, currency: 100
- `boot_machete` → vibroblade, head: ballistic_cap, currency: 75
- `(other gun job)` → follows team_gun base
- `(other machete job)` → follows team_machete base

---

## Testing

### Unit tests

- `TestLoadLoadout_ArchetypeOnly` — base loadout loads correctly
- `TestLoadLoadout_TeamGunOverride` — gun-team weapon replaces base, non-specified fields unchanged
- `TestLoadLoadout_JobOverride` — job block overrides team block
- `TestLoadLoadout_MissingArchetypeReturnsError` — unknown archetype returns error
- `TestGrantStartingInventory_PopulatesBackpack` — after grant, backpack contains all items
- `TestGrantStartingInventory_AutoEquips` — weapon in main hand, armor in correct slots
- `TestGrantStartingInventory_IdempotentIfFlagSet` — second call with flag true is a no-op

### Property-based tests (`pgregory.net/rapid`)

- Every archetype loadout resolves all item refs against the registry (no dangling refs)
- Merge order is deterministic — job override always wins over team, team always wins over base
- `StartingLoadout` for any valid job ID never panics

### Integration test (postgres-tagged)

- `TestCharacterSaver_InventoryRoundTrip` — save → load returns identical items

---

## Key Files

| File | Change |
|---|---|
| `content/loadouts/*.yaml` | NEW — 6 archetype loadout files |
| `content/jobs/*.yaml` | MODIFY — add `starting_inventory` to 4 team jobs |
| `internal/game/inventory/loadout.go` | NEW — `StartingLoadout`, loader, merge logic |
| `internal/game/inventory/loadout_test.go` | NEW — unit + property tests |
| `internal/gameserver/grpc_service.go` | MODIFY — grant logic at login, save at disconnect |
| `internal/gameserver/grpc_service.go` | MODIFY — `CharacterSaver` interface additions |
| `migrations/NNNN_character_inventory.sql` | NEW — DB migration |
| `internal/storage/postgres/character_saver.go` | MODIFY — implement new interface methods |
