# Character Initial Inventory Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Grant every new character a full job-appropriate starting kit (weapon, armor, consumables, currency) on first login, auto-equipped, persisted via the DB.

**Architecture:** Layered YAML loadouts (archetype base → team override → job override) merged into a `StartingLoadout` struct. DB flag `has_received_starting_inventory` gates the one-time grant. Backpack persisted via new `character_inventory` table. Grant logic lives in `grpc_service.go` at login; save at disconnect alongside existing equipment save.

**Tech Stack:** Go 1.23, `gopkg.in/yaml.v3`, `pgregory.net/rapid` (property tests), `github.com/stretchr/testify`, PostgreSQL/pgx, gRPC

---

## Key File Locations

- `internal/game/inventory/starting_loadout.go` — **NEW** `StartingLoadout`, `ConsumableGrant`, loader, merge logic
- `internal/game/inventory/starting_loadout_test.go` — **NEW** unit + property tests
- `content/loadouts/*.yaml` — **NEW** 6 archetype loadout files
- `content/jobs/boot_gun.yaml`, `boot_machete.yaml` — **MODIFY** add `starting_inventory` block
- `internal/gameserver/grpc_service.go` — **MODIFY** `CharacterSaver` interface + grant/save logic
- `internal/storage/postgres/character.go` — **MODIFY** implement 4 new methods
- `internal/storage/postgres/character_test.go` — **MODIFY** add inventory round-trip tests
- `migrations/009_character_inventory.up.sql` — **NEW**
- `migrations/009_character_inventory.down.sql` — **NEW**

## Test Commands

```bash
# Run all non-postgres tests with race detector
go test -race $(go list ./... | grep -v postgres)

# Run inventory package tests
go test -race ./internal/game/inventory/... -run TestStartingLoadout

# Run postgres tests (requires live DB)
go test -tags postgres ./internal/storage/postgres/... -run TestCharacterRepository
```

---

## Task 1: DB migration — `character_inventory` table + `has_received_starting_inventory` flag

**Files:**
- Create: `migrations/009_character_inventory.up.sql`
- Create: `migrations/009_character_inventory.down.sql`

### Step 1: Write the up migration

```sql
-- migrations/009_character_inventory.up.sql
ALTER TABLE characters
    ADD COLUMN has_received_starting_inventory BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE character_inventory (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    item_def_id  TEXT   NOT NULL,
    quantity     INT    NOT NULL DEFAULT 1,
    PRIMARY KEY (character_id, item_def_id)
);
```

### Step 2: Write the down migration

```sql
-- migrations/009_character_inventory.down.sql
DROP TABLE IF EXISTS character_inventory;

ALTER TABLE characters
    DROP COLUMN IF EXISTS has_received_starting_inventory;
```

### Step 3: Commit

```bash
git add migrations/009_character_inventory.up.sql migrations/009_character_inventory.down.sql
git commit -m "feat: add character_inventory table and has_received_starting_inventory flag migration"
```

---

## Task 2: `StartingLoadout` struct, loader, and merge logic

**Files:**
- Create: `internal/game/inventory/starting_loadout.go`
- Create: `internal/game/inventory/starting_loadout_test.go`

### Step 1: Write failing tests

```go
// internal/game/inventory/starting_loadout_test.go
package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
}

func TestLoadStartingLoadout_BaseOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "aggressor.yaml", `
archetype: aggressor
base:
  weapon: combat_knife
  armor:
    torso: kevlar_vest
  consumables:
    - item: canadian_bacon
      quantity: 2
  currency: 50
`)
	sl, err := inventory.LoadStartingLoadout(dir, "aggressor", "", "")
	require.NoError(t, err)
	assert.Equal(t, "combat_knife", sl.Weapon)
	assert.Equal(t, "kevlar_vest", sl.Armor[inventory.SlotTorso])
	assert.Equal(t, 50, sl.Currency)
	require.Len(t, sl.Consumables, 1)
	assert.Equal(t, "canadian_bacon", sl.Consumables[0].ItemID)
	assert.Equal(t, 2, sl.Consumables[0].Quantity)
}

func TestLoadStartingLoadout_TeamGunOverridesWeapon(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "aggressor.yaml", `
archetype: aggressor
base:
  weapon: combat_knife
  armor:
    torso: kevlar_vest
  currency: 50
team_gun:
  weapon: ganger_pistol
  armor:
    torso: tactical_vest
`)
	sl, err := inventory.LoadStartingLoadout(dir, "aggressor", "gun", "")
	require.NoError(t, err)
	assert.Equal(t, "ganger_pistol", sl.Weapon)
	assert.Equal(t, "tactical_vest", sl.Armor[inventory.SlotTorso])
	// Currency not overridden by team
	assert.Equal(t, 50, sl.Currency)
}

func TestLoadStartingLoadout_JobOverrideWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "aggressor.yaml", `
archetype: aggressor
base:
  weapon: combat_knife
  currency: 50
team_gun:
  weapon: ganger_pistol
`)
	jobOverride := &inventory.StartingLoadoutOverride{
		Weapon:   "heavy_revolver",
		Currency: 100,
	}
	sl, err := inventory.LoadStartingLoadoutWithOverride(dir, "aggressor", "gun", jobOverride)
	require.NoError(t, err)
	assert.Equal(t, "heavy_revolver", sl.Weapon)
	assert.Equal(t, 100, sl.Currency)
}

func TestLoadStartingLoadout_MissingArchetypeReturnsError(t *testing.T) {
	dir := t.TempDir()
	_, err := inventory.LoadStartingLoadout(dir, "unknown_archetype", "", "")
	assert.Error(t, err)
}

func TestProperty_LoadStartingLoadout_NeverPanics(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "aggressor.yaml", `
archetype: aggressor
base:
  weapon: combat_knife
  currency: 50
`)
	rapid.Check(t, func(rt *rapid.T) {
		archetype := rapid.SampledFrom([]string{"aggressor", "nonexistent"}).Draw(rt, "archetype")
		team := rapid.SampledFrom([]string{"", "gun", "machete"}).Draw(rt, "team")
		assert.NotPanics(rt, func() {
			inventory.LoadStartingLoadout(dir, archetype, team, "") //nolint:errcheck
		})
	})
}
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/game/inventory/ -run TestLoadStartingLoadout
```
Expected: FAIL — `LoadStartingLoadout undefined`

### Step 3: Implement `starting_loadout.go`

```go
// internal/game/inventory/starting_loadout.go
package inventory

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConsumableGrant is an item+quantity pair for starting consumables.
type ConsumableGrant struct {
	ItemID   string
	Quantity int
}

// StartingLoadout is the fully-merged starting kit for a character.
//
// Postcondition: All string fields are item IDs referencing content/items/.
type StartingLoadout struct {
	Weapon      string
	Armor       map[ArmorSlot]string
	Consumables []ConsumableGrant
	Currency    int
}

// StartingLoadoutOverride holds fields from a job's starting_inventory block.
// Only non-zero fields override the base+team loadout.
type StartingLoadoutOverride struct {
	Weapon      string
	Armor       map[ArmorSlot]string
	Consumables []ConsumableGrant
	Currency    int
}

// archetypeLoadoutFile is the YAML structure for content/loadouts/<archetype>.yaml.
type archetypeLoadoutFile struct {
	Archetype string        `yaml:"archetype"`
	Base      loadoutBlock  `yaml:"base"`
	TeamGun   loadoutBlock  `yaml:"team_gun"`
	TeamMachete loadoutBlock `yaml:"team_machete"`
}

type loadoutBlock struct {
	Weapon      string              `yaml:"weapon"`
	Armor       map[string]string   `yaml:"armor"`
	Consumables []consumableEntry   `yaml:"consumables"`
	Currency    int                 `yaml:"currency"`
}

type consumableEntry struct {
	Item     string `yaml:"item"`
	Quantity int    `yaml:"quantity"`
}

// LoadStartingLoadout loads and merges the starting loadout for the given archetype and team.
//
// Precondition: dir must be a readable directory; archetype must be non-empty.
// Postcondition: Returns a merged StartingLoadout or an error if the archetype file is missing.
func LoadStartingLoadout(dir, archetype, team, _ string) (*StartingLoadout, error) {
	return LoadStartingLoadoutWithOverride(dir, archetype, team, nil)
}

// LoadStartingLoadoutWithOverride merges archetype base → team section → job override.
//
// Precondition: dir must be a readable directory; archetype must be non-empty.
// Postcondition: Returns a merged StartingLoadout or an error if the archetype file is missing.
func LoadStartingLoadoutWithOverride(dir, archetype, team string, override *StartingLoadoutOverride) (*StartingLoadout, error) {
	path := filepath.Join(dir, archetype+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("loading loadout for archetype %q: %w", archetype, err)
	}

	var af archetypeLoadoutFile
	if err := yaml.Unmarshal(data, &af); err != nil {
		return nil, fmt.Errorf("parsing loadout %q: %w", path, err)
	}

	// Start from base.
	sl := applyBlock(&StartingLoadout{Armor: make(map[ArmorSlot]string)}, af.Base)

	// Apply team section.
	switch team {
	case "gun":
		sl = applyBlock(sl, af.TeamGun)
	case "machete":
		sl = applyBlock(sl, af.TeamMachete)
	}

	// Apply job override.
	if override != nil {
		if override.Weapon != "" {
			sl.Weapon = override.Weapon
		}
		for slot, itemID := range override.Armor {
			sl.Armor[slot] = itemID
		}
		if len(override.Consumables) > 0 {
			sl.Consumables = override.Consumables
		}
		if override.Currency != 0 {
			sl.Currency = override.Currency
		}
	}

	return sl, nil
}

func applyBlock(sl *StartingLoadout, b loadoutBlock) *StartingLoadout {
	if b.Weapon != "" {
		sl.Weapon = b.Weapon
	}
	for slotStr, itemID := range b.Armor {
		sl.Armor[ArmorSlot(slotStr)] = itemID
	}
	if len(b.Consumables) > 0 {
		sl.Consumables = make([]ConsumableGrant, len(b.Consumables))
		for i, c := range b.Consumables {
			sl.Consumables[i] = ConsumableGrant{ItemID: c.Item, Quantity: c.Quantity}
		}
	}
	if b.Currency != 0 {
		sl.Currency = b.Currency
	}
	return sl
}
```

### Step 4: Run tests

```bash
go test -race ./internal/game/inventory/ -run TestLoadStartingLoadout
go test -race ./internal/game/inventory/ -run TestProperty_LoadStartingLoadout
```
Expected: PASS

### Step 5: Commit

```bash
git add internal/game/inventory/starting_loadout.go internal/game/inventory/starting_loadout_test.go
git commit -m "feat: add StartingLoadout struct, loader, and merge logic"
```

---

## Task 3: `CharacterSaver` interface additions + postgres implementation

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (lines 53–59, `CharacterSaver` interface)
- Modify: `internal/storage/postgres/character.go`
- Modify: `internal/storage/postgres/character_test.go`

### Step 1: Add new methods to the `CharacterSaver` interface

In `internal/gameserver/grpc_service.go`, extend the interface:

```go
type CharacterSaver interface {
	SaveState(ctx context.Context, id int64, location string, currentHP int) error
	LoadWeaponPresets(ctx context.Context, characterID int64) (*inventory.LoadoutSet, error)
	SaveWeaponPresets(ctx context.Context, characterID int64, ls *inventory.LoadoutSet) error
	LoadEquipment(ctx context.Context, characterID int64) (*inventory.Equipment, error)
	SaveEquipment(ctx context.Context, characterID int64, eq *inventory.Equipment) error
	// New:
	LoadInventory(ctx context.Context, characterID int64) ([]inventory.InventoryItem, error)
	SaveInventory(ctx context.Context, characterID int64, items []inventory.InventoryItem) error
	HasReceivedStartingInventory(ctx context.Context, characterID int64) (bool, error)
	MarkStartingInventoryGranted(ctx context.Context, characterID int64) error
}
```

### Step 2: Add `InventoryItem` type to inventory package

In `internal/game/inventory/starting_loadout.go`, add:

```go
// InventoryItem represents a persisted backpack item (item def ID + quantity).
type InventoryItem struct {
	ItemDefID string
	Quantity  int
}
```

### Step 3: Write failing postgres tests (postgres-tagged)

Add to `internal/storage/postgres/character_test.go`:

```go
//go:build postgres

func TestCharacterRepository_Inventory_RoundTrip(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()
	char := createTestCharacter(t, repo, ctx)

	items := []inventory.InventoryItem{
		{ItemDefID: "combat_knife", Quantity: 1},
		{ItemDefID: "canadian_bacon", Quantity: 2},
	}
	require.NoError(t, repo.SaveInventory(ctx, char.ID, items))

	loaded, err := repo.LoadInventory(ctx, char.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, items, loaded)
}

func TestCharacterRepository_HasReceivedStartingInventory_DefaultFalse(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()
	char := createTestCharacter(t, repo, ctx)

	got, err := repo.HasReceivedStartingInventory(ctx, char.ID)
	require.NoError(t, err)
	assert.False(t, got)
}

func TestCharacterRepository_MarkStartingInventoryGranted(t *testing.T) {
	repo := setupTestRepo(t)
	ctx := context.Background()
	char := createTestCharacter(t, repo, ctx)

	require.NoError(t, repo.MarkStartingInventoryGranted(ctx, char.ID))

	got, err := repo.HasReceivedStartingInventory(ctx, char.ID)
	require.NoError(t, err)
	assert.True(t, got)
}
```

### Step 4: Implement the four new methods in `internal/storage/postgres/character.go`

```go
// LoadInventory fetches all backpack items for characterID.
//
// Precondition: characterID must be >= 0.
// Postcondition: Returns nil slice and nil error when no rows exist.
func (r *CharacterRepository) LoadInventory(ctx context.Context, characterID int64) ([]inventory.InventoryItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT item_def_id, quantity
		FROM character_inventory
		WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("loading inventory for character %d: %w", characterID, err)
	}
	defer rows.Close()

	var items []inventory.InventoryItem
	for rows.Next() {
		var it inventory.InventoryItem
		if err := rows.Scan(&it.ItemDefID, &it.Quantity); err != nil {
			return nil, fmt.Errorf("scanning inventory row: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// SaveInventory replaces all backpack rows for characterID.
//
// Precondition: characterID must be > 0.
// Postcondition: DB rows reflect items exactly; returns nil on success.
func (r *CharacterRepository) SaveInventory(ctx context.Context, characterID int64, items []inventory.InventoryItem) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM character_inventory WHERE character_id = $1`, characterID); err != nil {
		return fmt.Errorf("clearing inventory for character %d: %w", characterID, err)
	}
	for _, it := range items {
		if _, err := r.db.Exec(ctx, `
			INSERT INTO character_inventory (character_id, item_def_id, quantity)
			VALUES ($1, $2, $3)
			ON CONFLICT (character_id, item_def_id) DO UPDATE SET quantity = EXCLUDED.quantity`,
			characterID, it.ItemDefID, it.Quantity,
		); err != nil {
			return fmt.Errorf("saving inventory item %q for character %d: %w", it.ItemDefID, characterID, err)
		}
	}
	return nil
}

// HasReceivedStartingInventory returns whether the character has already received their starting kit.
//
// Precondition: characterID must be > 0.
// Postcondition: Returns false and nil error if the character has not yet received the kit.
func (r *CharacterRepository) HasReceivedStartingInventory(ctx context.Context, characterID int64) (bool, error) {
	var received bool
	err := r.db.QueryRow(ctx, `
		SELECT has_received_starting_inventory
		FROM characters WHERE id = $1`,
		characterID,
	).Scan(&received)
	if err != nil {
		return false, fmt.Errorf("checking starting inventory flag for character %d: %w", characterID, err)
	}
	return received, nil
}

// MarkStartingInventoryGranted sets has_received_starting_inventory = true for characterID.
//
// Precondition: characterID must be > 0.
// Postcondition: Flag is set; subsequent HasReceivedStartingInventory calls return true.
func (r *CharacterRepository) MarkStartingInventoryGranted(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx, `
		UPDATE characters SET has_received_starting_inventory = TRUE WHERE id = $1`,
		characterID,
	)
	if err != nil {
		return fmt.Errorf("marking starting inventory granted for character %d: %w", characterID, err)
	}
	return nil
}
```

### Step 5: Run non-postgres tests to verify compile

```bash
go test -race $(go list ./... | grep -v postgres)
```
Expected: PASS (postgres tests skipped)

### Step 6: Commit

```bash
git add internal/gameserver/grpc_service.go internal/game/inventory/starting_loadout.go \
        internal/storage/postgres/character.go internal/storage/postgres/character_test.go
git commit -m "feat: add inventory persistence methods to CharacterSaver and CharacterRepository"
```

---

## Task 4: Archetype loadout YAML content (6 files)

**Files:**
- Create: `content/loadouts/aggressor.yaml`
- Create: `content/loadouts/criminal.yaml`
- Create: `content/loadouts/drifter.yaml`
- Create: `content/loadouts/influencer.yaml`
- Create: `content/loadouts/nerd.yaml`
- Create: `content/loadouts/normie.yaml`

### Step 1: Create `content/loadouts/aggressor.yaml`

```yaml
archetype: aggressor
base:
  weapon: combat_knife
  armor:
    torso: kevlar_vest
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

### Step 2: Create `content/loadouts/criminal.yaml`

```yaml
archetype: criminal
base:
  weapon: ceramic_shiv
  armor:
    torso: corp_suit_liner
  consumables:
    - item: canadian_bacon
      quantity: 2
  currency: 100
team_gun:
  weapon: holdout_derringer
team_machete:
  weapon: ceramic_shiv
```

### Step 3: Create `content/loadouts/drifter.yaml`

```yaml
archetype: drifter
base:
  weapon: rebar_club
  armor:
    torso: leather_jacket
  consumables:
    - item: canadian_bacon
      quantity: 3
  currency: 25
team_gun:
  weapon: ganger_pistol
team_machete:
  weapon: rebar_club
```

### Step 4: Create `content/loadouts/influencer.yaml`

```yaml
archetype: influencer
base:
  weapon: stun_baton
  armor:
    torso: corp_suit_liner
  consumables:
    - item: canadian_bacon
      quantity: 2
  currency: 75
team_gun:
  weapon: emp_pistol
team_machete:
  weapon: stun_baton
```

### Step 5: Create `content/loadouts/nerd.yaml`

```yaml
archetype: nerd
base:
  weapon: stun_baton
  armor:
    torso: kevlar_vest
  consumables:
    - item: canadian_bacon
      quantity: 2
  currency: 75
team_gun:
  weapon: smartgun_pistol
team_machete:
  weapon: stun_baton
```

### Step 6: Create `content/loadouts/normie.yaml`

```yaml
archetype: normie
base:
  weapon: rebar_club
  armor:
    torso: leather_jacket
  consumables:
    - item: canadian_bacon
      quantity: 3
  currency: 30
team_gun:
  weapon: ganger_pistol
team_machete:
  weapon: rebar_club
```

### Step 7: Commit

```bash
git add content/loadouts/
git commit -m "content: add 6 archetype starting loadout YAML files"
```

---

## Task 5: Job override `starting_inventory` blocks (4 team jobs)

**Files:**
- Modify: `content/jobs/boot_gun.yaml`
- Modify: `content/jobs/boot_machete.yaml`

First, check what the other two team jobs are:

```bash
grep -l "team: gun\|team: machete" content/jobs/*.yaml
```

For each team job found, add a `starting_inventory` block. The job YAML parser must be updated to read this field (see Task 6 note — check if `Job` struct already has `StartingInventory` field; if not, add it in Task 6).

### Step 1: Add override to `boot_gun.yaml`

Open `content/jobs/boot_gun.yaml` and add at the end:

```yaml
starting_inventory:
  weapon: heavy_revolver
  armor:
    head: combat_helmet
  currency: 100
```

### Step 2: Add override to `boot_machete.yaml`

Open `content/jobs/boot_machete.yaml` and add at the end:

```yaml
starting_inventory:
  weapon: vibroblade
  armor:
    head: ballistic_cap
  currency: 75
```

### Step 3: For any other gun/machete team jobs found, add appropriate overrides following the same pattern.

### Step 4: Commit

```bash
git add content/jobs/
git commit -m "content: add starting_inventory overrides to team jobs"
```

---

## Task 6: Extend `Job` struct to parse `starting_inventory` + wire grant logic at login

**Files:**
- Modify: `internal/game/ruleset/job.go` — add `StartingInventory *StartingLoadoutOverride`
- Modify: `internal/gameserver/grpc_service.go` — grant logic at login + save at disconnect

### Step 1: Check `Job` struct in `internal/game/ruleset/job.go`

Read the file. Add the field if missing:

```go
// In Job struct, add:
StartingInventory *inventory.StartingLoadoutOverride `yaml:"starting_inventory"`
```

The `StartingLoadoutOverride` type is in `internal/game/inventory/starting_loadout.go`. Add the import.

### Step 2: Write a test for job YAML loading with starting_inventory

Add to `internal/game/ruleset/job_registry_test.go` (or a new `job_test.go`):

```go
func TestJob_ParsesStartingInventory(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: boot_gun
name: Boot (Gun)
archetype: aggressor
team: gun
starting_inventory:
  weapon: heavy_revolver
  currency: 100
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "boot_gun.yaml"), []byte(yaml), 0644))
	jobs, err := ruleset.LoadJobs(dir)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NotNil(t, jobs[0].StartingInventory)
	assert.Equal(t, "heavy_revolver", jobs[0].StartingInventory.Weapon)
	assert.Equal(t, 100, jobs[0].StartingInventory.Currency)
}
```

### Step 3: Implement grant helper in `grpc_service.go`

Add a `grantStartingInventory` method to `GameServiceServer`:

```go
// grantStartingInventory grants the starting kit for a new character and auto-equips it.
//
// Precondition: sess must be non-nil; characterID must be > 0; archetype and team may be empty.
// Postcondition: Backpack populated, weapon equipped in main hand, armor equipped in slots,
// currency set, inventory saved, flag marked. Returns nil on success.
func (s *GameServiceServer) grantStartingInventory(ctx context.Context, sess *session.PlayerSession, characterID int64, archetype, team string, jobOverride *inventory.StartingLoadoutOverride) error {
	sl, err := inventory.LoadStartingLoadoutWithOverride(s.loadoutsDir, archetype, team, jobOverride)
	if err != nil {
		return fmt.Errorf("resolving starting loadout: %w", err)
	}

	// Add weapon to backpack then equip in main hand.
	if sl.Weapon != "" {
		if _, err := sess.Backpack.Add(sl.Weapon, 1, s.invRegistry); err != nil {
			s.logger.Warn("failed to add starting weapon", zap.String("item", sl.Weapon), zap.Error(err))
		} else {
			command.HandleEquip(sess, s.invRegistry, sl.Weapon+" main")
		}
	}

	// Add and wear armor.
	for slot, itemID := range sl.Armor {
		if _, err := sess.Backpack.Add(itemID, 1, s.invRegistry); err != nil {
			s.logger.Warn("failed to add starting armor", zap.String("item", itemID), zap.Error(err))
			continue
		}
		command.HandleWear(sess, s.invRegistry, itemID+" "+string(slot))
	}

	// Add consumables.
	for _, cg := range sl.Consumables {
		if _, err := sess.Backpack.Add(cg.ItemID, cg.Quantity, s.invRegistry); err != nil {
			s.logger.Warn("failed to add starting consumable", zap.String("item", cg.ItemID), zap.Error(err))
		}
	}

	// Set currency.
	sess.Currency = cg.Currency  // NOTE: use sl.Currency, not cg

	// Persist inventory.
	items := backpackToInventoryItems(sess.Backpack)
	if err := s.charSaver.SaveInventory(ctx, characterID, items); err != nil {
		return fmt.Errorf("saving starting inventory: %w", err)
	}

	// Mark flag.
	if err := s.charSaver.MarkStartingInventoryGranted(ctx, characterID); err != nil {
		return fmt.Errorf("marking starting inventory granted: %w", err)
	}

	return nil
}

func backpackToInventoryItems(bp *inventory.Backpack) []inventory.InventoryItem {
	instances := bp.Items()
	// Aggregate by ItemDefID.
	counts := make(map[string]int)
	for _, inst := range instances {
		counts[inst.ItemDefID] += inst.Quantity
	}
	out := make([]inventory.InventoryItem, 0, len(counts))
	for id, qty := range counts {
		out = append(out, inventory.InventoryItem{ItemDefID: id, Quantity: qty})
	}
	return out
}
```

**NOTE:** Fix the bug in the code above — `cg.Currency` should be `sl.Currency`. The implementation above has a typo — use `sl.Currency` when setting `sess.Currency`.

### Step 4: Add `loadoutsDir` field to `GameServiceServer`

In `GameServiceServer` struct, add:
```go
loadoutsDir string
```

In `NewGameServiceServer`, accept and store it. In `cmd/gameserver/main.go`, add flag:
```go
loadoutsDir := flag.String("loadouts-dir", "content/loadouts", "path to archetype loadout YAML directory")
```
Pass to `NewGameServiceServer`.

### Step 5: Wire at login in `grpc_service.go`

After loading equipment (around line 232), add:

```go
// Load persisted inventory.
if characterID > 0 && s.charSaver != nil {
    invCtx, invCancel := context.WithTimeout(stream.Context(), 5*time.Second)
    invItems, invErr := s.charSaver.LoadInventory(invCtx, characterID)
    invCancel()
    if invErr != nil {
        s.logger.Warn("failed to load inventory on login", zap.Error(invErr))
    } else {
        for _, it := range invItems {
            if _, err := sess.Backpack.Add(it.ItemDefID, it.Quantity, s.invRegistry); err != nil {
                s.logger.Warn("failed to restore inventory item", zap.String("item", it.ItemDefID), zap.Error(err))
            }
        }
    }

    // Grant starting kit on first login.
    flagCtx, flagCancel := context.WithTimeout(stream.Context(), 5*time.Second)
    received, flagErr := s.charSaver.HasReceivedStartingInventory(flagCtx, characterID)
    flagCancel()
    if flagErr != nil {
        s.logger.Warn("failed to check starting inventory flag", zap.Error(flagErr))
    } else if !received {
        // Resolve job for archetype + team + override.
        archetype := joinReq.Archetype  // NOTE: must add Archetype field to JoinWorldRequest proto
        grantCtx, grantCancel := context.WithTimeout(stream.Context(), 10*time.Second)
        if err := s.grantStartingInventory(grantCtx, sess, characterID, archetype, sess.Team, nil); err != nil {
            s.logger.Error("failed to grant starting inventory", zap.Error(err))
        }
        grantCancel()
    }
}
```

**Important:** `JoinWorldRequest` needs an `archetype` field and the frontend needs to send it. Add `string archetype = 11;` to the proto and run `make proto`. The frontend sends the archetype from the character record at login. Also add `Team string` to `PlayerSession` (or derive it from `JobRegistry.TeamFor(sess.Class)`). Use `s.jobRegistry.TeamFor(sess.Class)` instead of `sess.Team`.

### Step 6: Wire save at disconnect

In `cleanupPlayer` (around line 940), after `SaveEquipment`, add:

```go
items := backpackToInventoryItems(sess.Backpack)
if err := s.charSaver.SaveInventory(ctx, characterID, items); err != nil {
    s.logger.Error("failed to save inventory on disconnect", zap.Error(err))
}
```

### Step 7: Run all non-postgres tests

```bash
go test -race $(go list ./... | grep -v postgres)
```
Expected: PASS

### Step 8: Commit

```bash
git add internal/game/ruleset/job.go internal/gameserver/grpc_service.go \
        internal/game/inventory/starting_loadout.go cmd/gameserver/main.go \
        api/proto/game/v1/game.proto
git commit -m "feat: wire starting inventory grant at login and save at disconnect"
```

---

## Task 7: Content completeness test for loadouts

**Files:**
- Create: `internal/game/inventory/loadout_content_test.go`

### Step 1: Write the test

```go
// internal/game/inventory/loadout_content_test.go
package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

var archetypes = []string{"aggressor", "criminal", "drifter", "influencer", "nerd", "normie"}

func TestContent_AllArchetypeLoadoutsLoad(t *testing.T) {
	for _, arch := range archetypes {
		sl, err := inventory.LoadStartingLoadout("../../../content/loadouts", arch, "", "")
		require.NoError(t, err, "archetype %q should load without error", arch)
		assert.NotEmpty(t, sl.Weapon, "archetype %q should have a base weapon", arch)
		assert.Greater(t, sl.Currency, 0, "archetype %q should have currency > 0", arch)
	}
}

func TestContent_AllLoadoutItemRefsResolve(t *testing.T) {
	reg := inventory.NewRegistry()
	weapons, err := inventory.LoadWeapons("../../../content/weapons")
	require.NoError(t, err)
	for _, w := range weapons {
		require.NoError(t, reg.RegisterWeapon(w))
	}
	armors, err := inventory.LoadArmors("../../../content/armor")
	require.NoError(t, err)
	for _, a := range armors {
		require.NoError(t, reg.RegisterArmor(a))
	}
	items, err := inventory.LoadItems("../../../content/items")
	require.NoError(t, err)
	for _, it := range items {
		require.NoError(t, reg.RegisterItem(it))
	}

	teams := []string{"", "gun", "machete"}
	for _, arch := range archetypes {
		for _, team := range teams {
			sl, err := inventory.LoadStartingLoadout("../../../content/loadouts", arch, team, "")
			require.NoError(t, err)
			if sl.Weapon != "" {
				_, ok := reg.Item(sl.Weapon)
				assert.True(t, ok, "loadout %s/%s weapon %q not in item registry", arch, team, sl.Weapon)
			}
			for slot, itemID := range sl.Armor {
				_, ok := reg.Item(itemID)
				assert.True(t, ok, "loadout %s/%s armor slot %s item %q not in item registry", arch, team, slot, itemID)
			}
			for _, cg := range sl.Consumables {
				_, ok := reg.Item(cg.ItemID)
				assert.True(t, ok, "loadout %s/%s consumable %q not in item registry", arch, team, cg.ItemID)
			}
		}
	}
}

func TestProperty_StartingLoadout_TeamOverrideIsDeterministic(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		arch := rapid.SampledFrom(archetypes).Draw(rt, "arch")
		team := rapid.SampledFrom([]string{"", "gun", "machete"}).Draw(rt, "team")
		sl1, err1 := inventory.LoadStartingLoadout("../../../content/loadouts", arch, team, "")
		sl2, err2 := inventory.LoadStartingLoadout("../../../content/loadouts", arch, team, "")
		assert.Equal(rt, err1 == nil, err2 == nil)
		if err1 == nil {
			assert.Equal(rt, sl1.Weapon, sl2.Weapon)
			assert.Equal(rt, sl1.Currency, sl2.Currency)
		}
	})
}
```

### Step 2: Run the tests

```bash
go test -race ./internal/game/inventory/ -run TestContent_AllArchetype
go test -race ./internal/game/inventory/ -run TestContent_AllLoadout
go test -race ./internal/game/inventory/ -run TestProperty_StartingLoadout
```
Expected: PASS

### Step 3: Run all tests

```bash
go test -race $(go list ./... | grep -v postgres)
```

### Step 4: Commit

```bash
git add internal/game/inventory/loadout_content_test.go
git commit -m "test: add content completeness tests for archetype starting loadouts"
```

---

## Final Verification

```bash
go test -race $(go list ./... | grep -v postgres)
```

Expected: all PASS, 0 failures.

Then use `superpowers:finishing-a-development-branch` to merge and deploy.
