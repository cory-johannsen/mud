# Stage 11 — Inventory & Loot System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add generic item definitions, player backpack (slot+weight), NPC loot tables, room floor item tracking, currency (Rounds/Clips/Crates), and pickup/drop/inventory commands.

**Architecture:** A new `ItemDef` type wraps all item kinds (weapon, explosive, consumable, junk) with a `Kind` field and optional refs to existing `WeaponDef`/`ExplosiveDef`. An `ItemRegistry` manages definitions loaded from YAML. Player sessions gain a `Backpack` (slot+weight limited) and `Currency` (stored as total rounds). A `FloorManager` tracks dropped items per room. NPC templates gain inline loot tables. On NPC death, loot is generated and placed on the floor / awarded as currency.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`, `pgregory.net/rapid` for property tests, `github.com/google/uuid` for instance IDs.

---

## Task 1: `ItemDef` + `ItemRegistry` + YAML Loader

**Files:**
- Create: `internal/game/inventory/item.go`
- Create: `internal/game/inventory/item_test.go`
- Modify: `internal/game/inventory/registry.go`

### Step 1: Write failing tests

Create `internal/game/inventory/item_test.go`:

```go
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

func TestItemDef_Validate_RejectsEmptyID(t *testing.T) {
	d := inventory.ItemDef{Name: "X", Kind: "junk", MaxStack: 1}
	assert.Error(t, d.Validate())
}

func TestItemDef_Validate_RejectsEmptyKind(t *testing.T) {
	d := inventory.ItemDef{ID: "x", Name: "X", MaxStack: 1}
	assert.Error(t, d.Validate())
}

func TestItemDef_Validate_RejectsInvalidKind(t *testing.T) {
	d := inventory.ItemDef{ID: "x", Name: "X", Kind: "magic", MaxStack: 1}
	assert.Error(t, d.Validate())
}

func TestItemDef_Validate_RejectsZeroMaxStack(t *testing.T) {
	d := inventory.ItemDef{ID: "x", Name: "X", Kind: "junk", MaxStack: 0}
	assert.Error(t, d.Validate())
}

func TestItemDef_Validate_RejectsNegativeWeight(t *testing.T) {
	d := inventory.ItemDef{ID: "x", Name: "X", Kind: "junk", MaxStack: 1, Weight: -1}
	assert.Error(t, d.Validate())
}

func TestItemDef_Validate_AcceptsMinimalJunk(t *testing.T) {
	d := inventory.ItemDef{ID: "junk1", Name: "Scrap", Kind: "junk", MaxStack: 1}
	assert.NoError(t, d.Validate())
}

func TestItemDef_Validate_AcceptsWeaponRef(t *testing.T) {
	d := inventory.ItemDef{
		ID: "pistol", Name: "Pistol", Kind: "weapon",
		WeaponRef: "ganger_pistol", MaxStack: 1, Weight: 1.5,
	}
	assert.NoError(t, d.Validate())
}

func TestItemDef_Validate_AcceptsStackable(t *testing.T) {
	d := inventory.ItemDef{
		ID: "medkit", Name: "Medkit", Kind: "consumable",
		Stackable: true, MaxStack: 10, Weight: 0.5,
	}
	assert.NoError(t, d.Validate())
}

func TestLoadItems_LoadsYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: test_item
name: Test Item
description: A test.
kind: junk
weight: 0.5
stackable: false
max_stack: 1
value: 10
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(yaml), 0644))

	items, err := inventory.LoadItems(dir)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "test_item", items[0].ID)
	assert.Equal(t, "junk", items[0].Kind)
	assert.Equal(t, 0.5, items[0].Weight)
	assert.Equal(t, 10, items[0].Value)
}

func TestRegistry_Item_Lookup(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ItemDef{ID: "x", Name: "X", Kind: "junk", MaxStack: 1}
	require.NoError(t, reg.RegisterItem(def))

	got, ok := reg.Item("x")
	require.True(t, ok)
	assert.Equal(t, "X", got.Name)

	_, ok = reg.Item("missing")
	assert.False(t, ok)
}

func TestRegistry_RegisterItem_RejectsDuplicate(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ItemDef{ID: "x", Name: "X", Kind: "junk", MaxStack: 1}
	require.NoError(t, reg.RegisterItem(def))
	assert.Error(t, reg.RegisterItem(def))
}

func TestProperty_ItemDef_ValidKind_AcceptsAll(t *testing.T) {
	kinds := []string{"weapon", "explosive", "consumable", "junk"}
	rapid.Check(t, func(rt *rapid.T) {
		kind := kinds[rapid.IntRange(0, len(kinds)-1).Draw(rt, "kind")]
		d := inventory.ItemDef{
			ID:       rapid.StringMatching(`[a-z][a-z0-9_]{2,15}`).Draw(rt, "id"),
			Name:     "Test",
			Kind:     kind,
			MaxStack: rapid.IntRange(1, 99).Draw(rt, "maxStack"),
			Weight:   float64(rapid.IntRange(0, 100).Draw(rt, "weight")),
		}
		if kind == "weapon" {
			d.WeaponRef = "some_weapon"
		}
		if kind == "explosive" {
			d.ExplosiveRef = "some_explosive"
		}
		assert.NoError(rt, d.Validate())
	})
}
```

### Step 2: Run tests to verify they fail

```bash
cd /home/cjohannsen/src/mud
mise exec -- go test ./internal/game/inventory/... -run TestItemDef -run TestLoadItems -run TestRegistry_Item -run TestProperty_ItemDef 2>&1 | head -10
```

Expected: compile errors — `ItemDef`, `LoadItems`, `RegisterItem`, `Item` do not exist.

### Step 3: Implement `internal/game/inventory/item.go`

```go
package inventory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Valid item kinds.
const (
	KindWeapon    = "weapon"
	KindExplosive = "explosive"
	KindConsumable = "consumable"
	KindJunk      = "junk"
)

// ItemDef defines a generic item in the game world.
//
// Invariant: ID, Name, Kind are non-empty; Kind is one of the valid kinds;
// MaxStack >= 1; Weight >= 0; WeaponRef set iff Kind == "weapon";
// ExplosiveRef set iff Kind == "explosive".
type ItemDef struct {
	ID           string  `yaml:"id"`
	Name         string  `yaml:"name"`
	Description  string  `yaml:"description"`
	Kind         string  `yaml:"kind"`
	Weight       float64 `yaml:"weight"`
	WeaponRef    string  `yaml:"weapon_ref"`
	ExplosiveRef string  `yaml:"explosive_ref"`
	Stackable    bool    `yaml:"stackable"`
	MaxStack     int     `yaml:"max_stack"`
	Value        int     `yaml:"value"`
}

// Validate checks that the ItemDef satisfies all invariants.
//
// Precondition: none.
// Postcondition: returns nil iff all invariants hold.
func (d *ItemDef) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("item def: id is required")
	}
	if d.Name == "" {
		return fmt.Errorf("item def %q: name is required", d.ID)
	}
	switch d.Kind {
	case KindWeapon, KindExplosive, KindConsumable, KindJunk:
		// valid
	default:
		return fmt.Errorf("item def %q: invalid kind %q", d.ID, d.Kind)
	}
	if d.MaxStack < 1 {
		return fmt.Errorf("item def %q: max_stack must be >= 1", d.ID)
	}
	if d.Weight < 0 {
		return fmt.Errorf("item def %q: weight must be >= 0", d.ID)
	}
	if d.Kind == KindWeapon && d.WeaponRef == "" {
		return fmt.Errorf("item def %q: weapon_ref required when kind is weapon", d.ID)
	}
	if d.Kind == KindExplosive && d.ExplosiveRef == "" {
		return fmt.Errorf("item def %q: explosive_ref required when kind is explosive", d.ID)
	}
	return nil
}

// LoadItems loads all ItemDef YAML files from the given directory.
//
// Precondition: dir must be a valid directory path.
// Postcondition: returns validated ItemDef slice, or error on first failure.
func LoadItems(dir string) ([]*ItemDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading item directory %q: %w", dir, err)
	}
	var items []*ItemDef
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading item file %q: %w", entry.Name(), err)
		}
		var item ItemDef
		if err := yaml.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("parsing item %q: %w", entry.Name(), err)
		}
		if err := item.Validate(); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	return items, nil
}
```

### Step 4: Add `RegisterItem` and `Item` to Registry

In `internal/game/inventory/registry.go`, add an `items` map field and methods:

```go
// Add to Registry struct:
items map[string]*ItemDef

// Add to NewRegistry():
items: make(map[string]*ItemDef),

// Add methods:

// RegisterItem registers an item definition.
//
// Precondition: def must not be nil; def.ID must be unique.
// Postcondition: def is retrievable via Item(def.ID).
func (r *Registry) RegisterItem(def *ItemDef) error {
	if _, exists := r.items[def.ID]; exists {
		return fmt.Errorf("duplicate item ID %q", def.ID)
	}
	r.items[def.ID] = def
	return nil
}

// Item returns the ItemDef for the given ID.
//
// Postcondition: returns (def, true) if found, (nil, false) otherwise.
func (r *Registry) Item(id string) (*ItemDef, bool) {
	def, ok := r.items[id]
	return def, ok
}
```

### Step 5: Run tests

```bash
mise exec -- go test ./internal/game/inventory/... -race -count=1 -v 2>&1 | tail -20
mise exec -- go build ./... 2>&1
```

### Step 6: gofmt + commit

```bash
gofmt -w internal/game/inventory/item.go internal/game/inventory/item_test.go internal/game/inventory/registry.go
git add internal/game/inventory/item.go internal/game/inventory/item_test.go internal/game/inventory/registry.go
git commit -m "feat(inventory): add ItemDef, ItemRegistry, and YAML loader"
```

---

## Task 2: Currency Model + Display

**Files:**
- Create: `internal/game/inventory/currency.go`
- Create: `internal/game/inventory/currency_test.go`

### Step 1: Write failing tests

Create `internal/game/inventory/currency_test.go`:

```go
package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestCurrency_Decompose_Zero(t *testing.T) {
	c, cl, r := inventory.DecomposeRounds(0)
	assert.Equal(t, 0, c)
	assert.Equal(t, 0, cl)
	assert.Equal(t, 0, r)
}

func TestCurrency_Decompose_ExactCrate(t *testing.T) {
	c, cl, r := inventory.DecomposeRounds(500)
	assert.Equal(t, 1, c)
	assert.Equal(t, 0, cl)
	assert.Equal(t, 0, r)
}

func TestCurrency_Decompose_Mixed(t *testing.T) {
	// 1042 = 2 crates (1000) + 1 clip (25) + 17 rounds
	c, cl, r := inventory.DecomposeRounds(1042)
	assert.Equal(t, 2, c)
	assert.Equal(t, 1, cl)
	assert.Equal(t, 17, r)
}

func TestCurrency_FormatRounds_Zero(t *testing.T) {
	assert.Equal(t, "0 Rounds", inventory.FormatRounds(0))
}

func TestCurrency_FormatRounds_OnlyRounds(t *testing.T) {
	assert.Equal(t, "17 Rounds", inventory.FormatRounds(17))
}

func TestCurrency_FormatRounds_Mixed(t *testing.T) {
	assert.Equal(t, "2 Crates, 1 Clip, 17 Rounds", inventory.FormatRounds(1042))
}

func TestCurrency_FormatRounds_NoCrates(t *testing.T) {
	assert.Equal(t, "3 Clips, 5 Rounds", inventory.FormatRounds(80))
}

func TestProperty_Decompose_Roundtrips(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		total := rapid.IntRange(0, 100000).Draw(rt, "total")
		c, cl, r := inventory.DecomposeRounds(total)
		reconstructed := c*500 + cl*25 + r
		assert.Equal(rt, total, reconstructed)
		assert.True(rt, cl < 20, "clips should be < 20")
		assert.True(rt, r < 25, "rounds should be < 25")
	})
}
```

### Step 2: Implement `internal/game/inventory/currency.go`

```go
package inventory

import (
	"fmt"
	"strings"
)

// Currency conversion rates.
const (
	RoundsPerClip  = 25
	RoundsPerCrate = 500 // 20 Clips
)

// DecomposeRounds converts a total round count into Crates, Clips, Rounds.
//
// Precondition: total >= 0.
// Postcondition: crates*500 + clips*25 + rounds == total; clips < 20; rounds < 25.
func DecomposeRounds(total int) (crates, clips, rounds int) {
	crates = total / RoundsPerCrate
	remainder := total % RoundsPerCrate
	clips = remainder / RoundsPerClip
	rounds = remainder % RoundsPerClip
	return
}

// FormatRounds returns a human-readable currency string.
//
// Postcondition: returns a non-empty string.
func FormatRounds(total int) string {
	if total == 0 {
		return "0 Rounds"
	}
	crates, clips, rounds := DecomposeRounds(total)
	var parts []string
	if crates > 0 {
		parts = append(parts, fmt.Sprintf("%d Crate%s", crates, plural(crates)))
	}
	if clips > 0 {
		parts = append(parts, fmt.Sprintf("%d Clip%s", clips, plural(clips)))
	}
	if rounds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d Round%s", rounds, plural(rounds)))
	}
	return strings.Join(parts, ", ")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
```

### Step 3: Run tests

```bash
mise exec -- go test ./internal/game/inventory/... -race -count=1 -run TestCurrency -run TestProperty_Decompose -v 2>&1
```

### Step 4: Commit

```bash
gofmt -w internal/game/inventory/currency.go internal/game/inventory/currency_test.go
git add internal/game/inventory/currency.go internal/game/inventory/currency_test.go
git commit -m "feat(inventory): currency model with Rounds/Clips/Crates decomposition"
```

---

## Task 3: `ItemInstance` + `Backpack`

**Files:**
- Create: `internal/game/inventory/backpack.go`
- Create: `internal/game/inventory/backpack_test.go`

### Step 1: Write failing tests

Create `internal/game/inventory/backpack_test.go`:

```go
package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func junkDef(id string, weight float64) *inventory.ItemDef {
	return &inventory.ItemDef{ID: id, Name: id, Kind: "junk", MaxStack: 1, Weight: weight}
}

func stackDef(id string, weight float64, maxStack int) *inventory.ItemDef {
	return &inventory.ItemDef{ID: id, Name: id, Kind: "consumable", Stackable: true, MaxStack: maxStack, Weight: weight}
}

func TestBackpack_Add_SingleItem(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(junkDef("scrap", 1.0)))

	bp := inventory.NewBackpack(10, 50.0)
	inst, err := bp.Add("scrap", 1, reg)
	require.NoError(t, err)
	assert.Equal(t, "scrap", inst.ItemDefID)
	assert.Equal(t, 1, inst.Quantity)
	assert.Equal(t, 1, bp.UsedSlots())
}

func TestBackpack_Add_StackableItem_MergesIntoExisting(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(stackDef("medkit", 0.5, 10)))

	bp := inventory.NewBackpack(10, 50.0)
	_, err := bp.Add("medkit", 3, reg)
	require.NoError(t, err)
	inst, err := bp.Add("medkit", 2, reg)
	require.NoError(t, err)

	assert.Equal(t, 5, inst.Quantity)
	assert.Equal(t, 1, bp.UsedSlots()) // stacked into one slot
}

func TestBackpack_Add_ExceedsMaxStack_NewSlot(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(stackDef("ammo", 0.1, 5)))

	bp := inventory.NewBackpack(10, 50.0)
	_, err := bp.Add("ammo", 5, reg)
	require.NoError(t, err)
	_, err = bp.Add("ammo", 3, reg)
	require.NoError(t, err)

	assert.Equal(t, 2, bp.UsedSlots())
}

func TestBackpack_Add_RejectsSlotOverflow(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(junkDef("scrap", 0.1)))

	bp := inventory.NewBackpack(2, 50.0)
	_, err := bp.Add("scrap", 1, reg)
	require.NoError(t, err)
	_, err = bp.Add("scrap", 1, reg)
	require.NoError(t, err)
	_, err = bp.Add("scrap", 1, reg)
	assert.Error(t, err, "expected slot overflow error")
}

func TestBackpack_Add_RejectsWeightOverflow(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(junkDef("brick", 10.0)))

	bp := inventory.NewBackpack(10, 15.0)
	_, err := bp.Add("brick", 1, reg)
	require.NoError(t, err)
	_, err = bp.Add("brick", 1, reg)
	assert.Error(t, err, "expected weight overflow error")
}

func TestBackpack_Remove_ByInstanceID(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(junkDef("scrap", 1.0)))

	bp := inventory.NewBackpack(10, 50.0)
	inst, _ := bp.Add("scrap", 1, reg)
	err := bp.Remove(inst.InstanceID, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, bp.UsedSlots())
}

func TestBackpack_Remove_PartialStack(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(stackDef("medkit", 0.5, 10)))

	bp := inventory.NewBackpack(10, 50.0)
	inst, _ := bp.Add("medkit", 5, reg)
	err := bp.Remove(inst.InstanceID, 2)
	require.NoError(t, err)

	items := bp.Items()
	require.Len(t, items, 1)
	assert.Equal(t, 3, items[0].Quantity)
}

func TestBackpack_TotalWeight(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(junkDef("a", 2.0)))
	require.NoError(t, reg.RegisterItem(stackDef("b", 0.5, 10)))

	bp := inventory.NewBackpack(10, 50.0)
	bp.Add("a", 1, reg)
	bp.Add("b", 4, reg)

	assert.InDelta(t, 4.0, bp.TotalWeight(reg), 0.001) // 2.0 + 4*0.5
}

func TestBackpack_FindByItemDefID(t *testing.T) {
	reg := inventory.NewRegistry()
	require.NoError(t, reg.RegisterItem(junkDef("scrap", 1.0)))

	bp := inventory.NewBackpack(10, 50.0)
	bp.Add("scrap", 1, reg)

	found := bp.FindByItemDefID("scrap")
	require.Len(t, found, 1)
	assert.Equal(t, "scrap", found[0].ItemDefID)
}

func TestProperty_Backpack_NeverExceedsSlots(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxSlots := rapid.IntRange(1, 10).Draw(rt, "maxSlots")
		adds := rapid.IntRange(1, 20).Draw(rt, "adds")

		reg := inventory.NewRegistry()
		_ = reg.RegisterItem(junkDef("x", 0.0))

		bp := inventory.NewBackpack(maxSlots, 1000.0)
		for i := 0; i < adds; i++ {
			bp.Add("x", 1, reg)
		}
		assert.LessOrEqual(rt, bp.UsedSlots(), maxSlots)
	})
}

func TestProperty_Backpack_NeverExceedsWeight(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxWeight := float64(rapid.IntRange(1, 50).Draw(rt, "maxWeight"))
		adds := rapid.IntRange(1, 20).Draw(rt, "adds")

		reg := inventory.NewRegistry()
		_ = reg.RegisterItem(junkDef("x", 1.0))

		bp := inventory.NewBackpack(100, maxWeight)
		for i := 0; i < adds; i++ {
			bp.Add("x", 1, reg)
		}
		assert.LessOrEqual(rt, bp.TotalWeight(reg), maxWeight)
	})
}
```

### Step 2: Implement `internal/game/inventory/backpack.go`

```go
package inventory

import (
	"fmt"

	"github.com/google/uuid"
)

// ItemInstance represents a concrete item in a player's backpack or on a room floor.
//
// Invariant: InstanceID is globally unique; Quantity >= 1.
type ItemInstance struct {
	InstanceID string
	ItemDefID  string
	Quantity   int
}

// Backpack is a slot-and-weight-limited container for items.
//
// Invariant: len(items) <= MaxSlots; TotalWeight <= MaxWeight.
type Backpack struct {
	MaxSlots  int
	MaxWeight float64
	items     []ItemInstance
}

// NewBackpack creates a backpack with the given slot and weight limits.
//
// Precondition: maxSlots >= 1; maxWeight > 0.
// Postcondition: returns an empty backpack.
func NewBackpack(maxSlots int, maxWeight float64) *Backpack {
	return &Backpack{
		MaxSlots:  maxSlots,
		MaxWeight: maxWeight,
	}
}

// Add places quantity units of itemDefID into the backpack. Stackable items
// merge into an existing stack if one exists and has room; otherwise a new
// slot is consumed.
//
// Precondition: itemDefID must exist in reg; quantity >= 1.
// Postcondition: returns the ItemInstance that was added to or created, or an
// error if slot or weight limits would be exceeded.
func (bp *Backpack) Add(itemDefID string, quantity int, reg *Registry) (*ItemInstance, error) {
	def, ok := reg.Item(itemDefID)
	if !ok {
		return nil, fmt.Errorf("unknown item %q", itemDefID)
	}

	addedWeight := float64(quantity) * def.Weight
	if bp.totalWeight(reg)+addedWeight > bp.MaxWeight {
		return nil, fmt.Errorf("backpack weight limit exceeded")
	}

	// Try to merge into existing stack.
	if def.Stackable {
		for i := range bp.items {
			if bp.items[i].ItemDefID == itemDefID && bp.items[i].Quantity+quantity <= def.MaxStack {
				bp.items[i].Quantity += quantity
				return &bp.items[i], nil
			}
		}
		// Try to fill partial stacks first, then overflow to new slots.
		remaining := quantity
		for i := range bp.items {
			if bp.items[i].ItemDefID == itemDefID && bp.items[i].Quantity < def.MaxStack {
				space := def.MaxStack - bp.items[i].Quantity
				add := remaining
				if add > space {
					add = space
				}
				bp.items[i].Quantity += add
				remaining -= add
				if remaining == 0 {
					return &bp.items[i], nil
				}
			}
		}
		if remaining > 0 {
			if len(bp.items) >= bp.MaxSlots {
				// Rollback partial additions — undo by recalculating.
				// Simpler: check slot availability up front.
				// Reset: remove what we added above.
				added := quantity - remaining
				for i := range bp.items {
					if bp.items[i].ItemDefID == itemDefID && added > 0 {
						revert := added
						if revert > bp.items[i].Quantity {
							revert = bp.items[i].Quantity
						}
						bp.items[i].Quantity -= revert
						added -= revert
					}
				}
				return nil, fmt.Errorf("backpack slot limit exceeded")
			}
			inst := ItemInstance{
				InstanceID: uuid.NewString(),
				ItemDefID:  itemDefID,
				Quantity:   remaining,
			}
			bp.items = append(bp.items, inst)
			return &bp.items[len(bp.items)-1], nil
		}
	}

	// Non-stackable: each unit takes a slot.
	if len(bp.items) >= bp.MaxSlots {
		return nil, fmt.Errorf("backpack slot limit exceeded")
	}
	inst := ItemInstance{
		InstanceID: uuid.NewString(),
		ItemDefID:  itemDefID,
		Quantity:   quantity,
	}
	bp.items = append(bp.items, inst)
	return &bp.items[len(bp.items)-1], nil
}

// Remove removes quantity units from the item instance identified by instanceID.
// If quantity equals the instance's quantity, the instance is removed entirely.
//
// Precondition: instanceID must exist in the backpack; quantity >= 1 and <= instance.Quantity.
// Postcondition: instance quantity reduced or instance removed.
func (bp *Backpack) Remove(instanceID string, quantity int) error {
	for i, inst := range bp.items {
		if inst.InstanceID == instanceID {
			if quantity > inst.Quantity {
				return fmt.Errorf("cannot remove %d from stack of %d", quantity, inst.Quantity)
			}
			if quantity == inst.Quantity {
				bp.items = append(bp.items[:i], bp.items[i+1:]...)
			} else {
				bp.items[i].Quantity -= quantity
			}
			return nil
		}
	}
	return fmt.Errorf("item instance %q not found", instanceID)
}

// Items returns a snapshot of all items in the backpack.
func (bp *Backpack) Items() []ItemInstance {
	out := make([]ItemInstance, len(bp.items))
	copy(out, bp.items)
	return out
}

// UsedSlots returns the number of occupied slots.
func (bp *Backpack) UsedSlots() int {
	return len(bp.items)
}

// TotalWeight returns the total weight of all items.
func (bp *Backpack) TotalWeight(reg *Registry) float64 {
	return bp.totalWeight(reg)
}

func (bp *Backpack) totalWeight(reg *Registry) float64 {
	var total float64
	for _, inst := range bp.items {
		if def, ok := reg.Item(inst.ItemDefID); ok {
			total += float64(inst.Quantity) * def.Weight
		}
	}
	return total
}

// FindByItemDefID returns all instances matching the given item def ID.
func (bp *Backpack) FindByItemDefID(itemDefID string) []ItemInstance {
	var out []ItemInstance
	for _, inst := range bp.items {
		if inst.ItemDefID == itemDefID {
			out = append(out, inst)
		}
	}
	return out
}
```

### Step 3: Add `github.com/google/uuid` dependency

```bash
cd /home/cjohannsen/src/mud
mise exec -- go get github.com/google/uuid
```

### Step 4: Run tests

```bash
mise exec -- go test ./internal/game/inventory/... -race -count=1 -v 2>&1 | tail -30
mise exec -- go build ./... 2>&1
```

### Step 5: Commit

```bash
gofmt -w internal/game/inventory/backpack.go internal/game/inventory/backpack_test.go
git add internal/game/inventory/backpack.go internal/game/inventory/backpack_test.go go.mod go.sum
git commit -m "feat(inventory): Backpack with slot+weight limits and ItemInstance model"
```

---

## Task 4: `FloorManager` — Room Item Tracking

**Files:**
- Create: `internal/game/inventory/floor.go`
- Create: `internal/game/inventory/floor_test.go`

### Step 1: Write failing tests

Create `internal/game/inventory/floor_test.go`:

```go
package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestFloorManager_Drop_And_ItemsInRoom(t *testing.T) {
	fm := inventory.NewFloorManager()
	inst := inventory.ItemInstance{InstanceID: "i1", ItemDefID: "scrap", Quantity: 1}
	fm.Drop("r1", inst)

	items := fm.ItemsInRoom("r1")
	require.Len(t, items, 1)
	assert.Equal(t, "scrap", items[0].ItemDefID)
}

func TestFloorManager_Pickup_RemovesItem(t *testing.T) {
	fm := inventory.NewFloorManager()
	inst := inventory.ItemInstance{InstanceID: "i1", ItemDefID: "scrap", Quantity: 1}
	fm.Drop("r1", inst)

	got, ok := fm.Pickup("r1", "i1")
	require.True(t, ok)
	assert.Equal(t, "scrap", got.ItemDefID)
	assert.Empty(t, fm.ItemsInRoom("r1"))
}

func TestFloorManager_Pickup_NotFound(t *testing.T) {
	fm := inventory.NewFloorManager()
	_, ok := fm.Pickup("r1", "missing")
	assert.False(t, ok)
}

func TestFloorManager_PickupAll_ReturnsAndClears(t *testing.T) {
	fm := inventory.NewFloorManager()
	fm.Drop("r1", inventory.ItemInstance{InstanceID: "i1", ItemDefID: "a", Quantity: 1})
	fm.Drop("r1", inventory.ItemInstance{InstanceID: "i2", ItemDefID: "b", Quantity: 1})

	items := fm.PickupAll("r1")
	assert.Len(t, items, 2)
	assert.Empty(t, fm.ItemsInRoom("r1"))
}

func TestFloorManager_EmptyRoom(t *testing.T) {
	fm := inventory.NewFloorManager()
	assert.Empty(t, fm.ItemsInRoom("nonexistent"))
}

func TestProperty_FloorManager_DropPickup_Roundtrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 10).Draw(rt, "count")
		fm := inventory.NewFloorManager()
		var ids []string
		for i := 0; i < n; i++ {
			id := rapid.StringMatching(`[a-z]{8}`).Draw(rt, "id")
			fm.Drop("r1", inventory.ItemInstance{InstanceID: id, ItemDefID: "x", Quantity: 1})
			ids = append(ids, id)
		}
		assert.Equal(rt, n, len(fm.ItemsInRoom("r1")))

		for _, id := range ids {
			_, ok := fm.Pickup("r1", id)
			assert.True(rt, ok)
		}
		assert.Empty(rt, fm.ItemsInRoom("r1"))
	})
}
```

### Step 2: Implement `internal/game/inventory/floor.go`

```go
package inventory

import "sync"

// FloorManager tracks items dropped on room floors.
//
// Safe for concurrent use.
//
// Invariant: items are uniquely identified by InstanceID within a room.
type FloorManager struct {
	mu    sync.RWMutex
	rooms map[string][]ItemInstance // roomID → items on floor
}

// NewFloorManager creates a new FloorManager.
//
// Postcondition: returns a non-nil, empty FloorManager.
func NewFloorManager() *FloorManager {
	return &FloorManager{rooms: make(map[string][]ItemInstance)}
}

// Drop places an item on the floor of the given room.
//
// Precondition: roomID must be non-empty; inst.InstanceID must be unique within the room.
// Postcondition: item is retrievable via ItemsInRoom or Pickup.
func (fm *FloorManager) Drop(roomID string, inst ItemInstance) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.rooms[roomID] = append(fm.rooms[roomID], inst)
}

// Pickup removes and returns the item with the given instanceID from the room floor.
//
// Postcondition: returns (item, true) if found and removed; (zero, false) otherwise.
func (fm *FloorManager) Pickup(roomID, instanceID string) (ItemInstance, bool) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	items := fm.rooms[roomID]
	for i, inst := range items {
		if inst.InstanceID == instanceID {
			fm.rooms[roomID] = append(items[:i], items[i+1:]...)
			return inst, true
		}
	}
	return ItemInstance{}, false
}

// PickupAll removes and returns all items from the room floor.
//
// Postcondition: room floor is empty after call.
func (fm *FloorManager) PickupAll(roomID string) []ItemInstance {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	items := fm.rooms[roomID]
	delete(fm.rooms, roomID)
	return items
}

// ItemsInRoom returns a snapshot of items on the floor of the given room.
//
// Postcondition: returns a copy; modifications do not affect the floor.
func (fm *FloorManager) ItemsInRoom(roomID string) []ItemInstance {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	items := fm.rooms[roomID]
	out := make([]ItemInstance, len(items))
	copy(out, items)
	return out
}
```

### Step 3: Run tests + commit

```bash
mise exec -- go test ./internal/game/inventory/... -race -count=1 -v 2>&1 | tail -20
gofmt -w internal/game/inventory/floor.go internal/game/inventory/floor_test.go
git add internal/game/inventory/floor.go internal/game/inventory/floor_test.go
git commit -m "feat(inventory): FloorManager for room item tracking"
```

---

## Task 5: NPC Loot Table Schema + Loot Generation

**Files:**
- Modify: `internal/game/npc/template.go`
- Create: `internal/game/npc/loot.go`
- Create: `internal/game/npc/loot_test.go`
- Modify: `internal/game/npc/template_test.go`

### Step 1: Write failing tests

Create `internal/game/npc/loot_test.go`:

```go
package npc_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestLootTable_Validate_AcceptsValid(t *testing.T) {
	lt := npc.LootTable{
		Currency: &npc.CurrencyDrop{Min: 10, Max: 50},
		Items: []npc.ItemDrop{
			{ItemID: "medkit", Chance: 0.5, MinQty: 1, MaxQty: 2},
		},
	}
	assert.NoError(t, lt.Validate())
}

func TestLootTable_Validate_RejectsNegativeMinCurrency(t *testing.T) {
	lt := npc.LootTable{Currency: &npc.CurrencyDrop{Min: -1, Max: 10}}
	assert.Error(t, lt.Validate())
}

func TestLootTable_Validate_RejectsMinGreaterThanMax(t *testing.T) {
	lt := npc.LootTable{Currency: &npc.CurrencyDrop{Min: 50, Max: 10}}
	assert.Error(t, lt.Validate())
}

func TestLootTable_Validate_RejectsInvalidChance(t *testing.T) {
	lt := npc.LootTable{Items: []npc.ItemDrop{{ItemID: "x", Chance: 1.5, MinQty: 1, MaxQty: 1}}}
	assert.Error(t, lt.Validate())
}

func TestLootTable_Validate_RejectsZeroChance(t *testing.T) {
	lt := npc.LootTable{Items: []npc.ItemDrop{{ItemID: "x", Chance: 0.0, MinQty: 1, MaxQty: 1}}}
	assert.Error(t, lt.Validate())
}

func TestLootTable_Validate_RejectsMinQtyGreaterThanMaxQty(t *testing.T) {
	lt := npc.LootTable{Items: []npc.ItemDrop{{ItemID: "x", Chance: 0.5, MinQty: 5, MaxQty: 1}}}
	assert.Error(t, lt.Validate())
}

func TestLootTable_Validate_Empty(t *testing.T) {
	lt := npc.LootTable{}
	assert.NoError(t, lt.Validate())
}

func TestGenerateLoot_CurrencyInRange(t *testing.T) {
	lt := npc.LootTable{Currency: &npc.CurrencyDrop{Min: 10, Max: 20}}
	for i := 0; i < 100; i++ {
		result := npc.GenerateLoot(lt)
		assert.GreaterOrEqual(t, result.Currency, 10)
		assert.LessOrEqual(t, result.Currency, 20)
	}
}

func TestGenerateLoot_NoCurrency(t *testing.T) {
	lt := npc.LootTable{}
	result := npc.GenerateLoot(lt)
	assert.Equal(t, 0, result.Currency)
}

func TestGenerateLoot_GuaranteedItem(t *testing.T) {
	lt := npc.LootTable{Items: []npc.ItemDrop{{ItemID: "x", Chance: 1.0, MinQty: 1, MaxQty: 1}}}
	result := npc.GenerateLoot(lt)
	require.Len(t, result.Items, 1)
	assert.Equal(t, "x", result.Items[0].ItemDefID)
}

func TestGenerateLoot_ZeroChance_NeverDrops(t *testing.T) {
	// This case is prevented by Validate, but GenerateLoot should handle it gracefully.
	lt := npc.LootTable{Items: []npc.ItemDrop{{ItemID: "x", Chance: 0.0, MinQty: 1, MaxQty: 1}}}
	for i := 0; i < 100; i++ {
		result := npc.GenerateLoot(lt)
		assert.Empty(t, result.Items)
	}
}

func TestProperty_GenerateLoot_CurrencyAlwaysInRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		min := rapid.IntRange(0, 100).Draw(rt, "min")
		max := rapid.IntRange(min, min+100).Draw(rt, "max")
		lt := npc.LootTable{Currency: &npc.CurrencyDrop{Min: min, Max: max}}
		result := npc.GenerateLoot(lt)
		assert.GreaterOrEqual(rt, result.Currency, min)
		assert.LessOrEqual(rt, result.Currency, max)
	})
}

func TestProperty_GenerateLoot_ItemQuantityInRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		minQty := rapid.IntRange(1, 5).Draw(rt, "minQty")
		maxQty := rapid.IntRange(minQty, minQty+5).Draw(rt, "maxQty")
		lt := npc.LootTable{Items: []npc.ItemDrop{{ItemID: "x", Chance: 1.0, MinQty: minQty, MaxQty: maxQty}}}
		result := npc.GenerateLoot(lt)
		require.Len(rt, result.Items, 1)
		assert.GreaterOrEqual(rt, result.Items[0].Quantity, minQty)
		assert.LessOrEqual(rt, result.Items[0].Quantity, maxQty)
	})
}
```

In `internal/game/npc/template_test.go`, add a test that a template with a loot section parses correctly:

```go
func TestTemplate_LootTable_ParsesFromYAML(t *testing.T) {
	data := []byte(`
id: ganger
name: Ganger
description: A tough.
level: 1
max_hp: 18
ac: 14
perception: 5
loot:
  currency:
    min: 10
    max: 50
  items:
    - item: medkit
      chance: 0.25
      min_qty: 1
      max_qty: 1
`)
	tmpl, err := npc.LoadTemplateFromBytes(data)
	require.NoError(t, err)
	require.NotNil(t, tmpl.Loot)
	require.NotNil(t, tmpl.Loot.Currency)
	assert.Equal(t, 10, tmpl.Loot.Currency.Min)
	assert.Equal(t, 50, tmpl.Loot.Currency.Max)
	require.Len(t, tmpl.Loot.Items, 1)
	assert.Equal(t, "medkit", tmpl.Loot.Items[0].ItemID)
}
```

### Step 2: Add `Loot *LootTable` to Template struct

In `internal/game/npc/template.go`, add to Template:

```go
Loot *LootTable `yaml:"loot"`
```

In `Validate()`, add after the RespawnDelay check:

```go
if t.Loot != nil {
    if err := t.Loot.Validate(); err != nil {
        return fmt.Errorf("npc template %q: %w", t.ID, err)
    }
}
```

### Step 3: Implement `internal/game/npc/loot.go`

```go
package npc

import (
	"fmt"
	"math/rand"

	"github.com/google/uuid"
)

// CurrencyDrop defines a currency range dropped by an NPC.
type CurrencyDrop struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
}

// ItemDrop defines a single item drop entry in a loot table.
type ItemDrop struct {
	ItemID string  `yaml:"item"`
	Chance float64 `yaml:"chance"`
	MinQty int     `yaml:"min_qty"`
	MaxQty int     `yaml:"max_qty"`
}

// LootTable defines the drops for an NPC template.
//
// Invariant: Currency.Min <= Currency.Max when Currency is set;
// each ItemDrop has 0 < Chance <= 1 and MinQty <= MaxQty >= 1.
type LootTable struct {
	Currency *CurrencyDrop `yaml:"currency"`
	Items    []ItemDrop    `yaml:"items"`
}

// Validate checks loot table invariants.
//
// Postcondition: returns nil iff all invariants hold.
func (lt *LootTable) Validate() error {
	if lt.Currency != nil {
		if lt.Currency.Min < 0 {
			return fmt.Errorf("loot currency min must be >= 0")
		}
		if lt.Currency.Min > lt.Currency.Max {
			return fmt.Errorf("loot currency min (%d) > max (%d)", lt.Currency.Min, lt.Currency.Max)
		}
	}
	for i, item := range lt.Items {
		if item.ItemID == "" {
			return fmt.Errorf("loot item[%d]: item id is required", i)
		}
		if item.Chance <= 0 || item.Chance > 1.0 {
			return fmt.Errorf("loot item %q: chance must be in (0, 1.0]", item.ItemID)
		}
		if item.MinQty < 1 {
			return fmt.Errorf("loot item %q: min_qty must be >= 1", item.ItemID)
		}
		if item.MinQty > item.MaxQty {
			return fmt.Errorf("loot item %q: min_qty (%d) > max_qty (%d)", item.ItemID, item.MinQty, item.MaxQty)
		}
	}
	return nil
}

// LootResult holds the generated loot from an NPC death.
type LootResult struct {
	Currency int
	Items    []LootItem
}

// LootItem is a single item drop result.
type LootItem struct {
	ItemDefID  string
	InstanceID string
	Quantity   int
}

// GenerateLoot rolls a LootTable and returns the result.
//
// Precondition: lt should be validated.
// Postcondition: Currency in [min, max]; each item quantity in [minQty, maxQty].
func GenerateLoot(lt LootTable) LootResult {
	var result LootResult

	if lt.Currency != nil && lt.Currency.Max > 0 {
		spread := lt.Currency.Max - lt.Currency.Min
		if spread == 0 {
			result.Currency = lt.Currency.Min
		} else {
			result.Currency = lt.Currency.Min + rand.Intn(spread+1)
		}
	}

	for _, item := range lt.Items {
		if rand.Float64() >= item.Chance {
			continue
		}
		qty := item.MinQty
		if item.MaxQty > item.MinQty {
			qty = item.MinQty + rand.Intn(item.MaxQty-item.MinQty+1)
		}
		result.Items = append(result.Items, LootItem{
			ItemDefID:  item.ItemID,
			InstanceID: uuid.NewString(),
			Quantity:   qty,
		})
	}

	return result
}
```

### Step 4: Run tests + commit

```bash
mise exec -- go test ./internal/game/npc/... -race -count=1 -v 2>&1 | tail -30
mise exec -- go build ./... 2>&1
gofmt -w internal/game/npc/loot.go internal/game/npc/loot_test.go internal/game/npc/template.go internal/game/npc/template_test.go
git add internal/game/npc/loot.go internal/game/npc/loot_test.go internal/game/npc/template.go internal/game/npc/template_test.go go.mod go.sum
git commit -m "feat(npc): loot table schema, validation, and loot generation"
```

---

## Task 6: Wire Backpack + Currency onto PlayerSession

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `internal/game/session/manager_test.go` (if exists, else create)

### Step 1: Add fields to `PlayerSession`

In `internal/game/session/manager.go`, add to `PlayerSession` struct:

```go
Backpack *inventory.Backpack
Currency int // total rounds
```

### Step 2: Initialize in `AddPlayer`

In the `AddPlayer` method, after session creation, initialize:

```go
sess.Backpack = inventory.NewBackpack(20, 50.0)
sess.Currency = 0
```

Default: 20 slots, 50kg max weight.

### Step 3: Add tests

Test that a new session has an initialized backpack and zero currency.

### Step 4: Run tests + commit

```bash
mise exec -- go test ./internal/game/session/... -race -count=1 -v 2>&1 | tail -10
mise exec -- go build ./... 2>&1
git add internal/game/session/manager.go
git commit -m "feat(session): add Backpack and Currency to PlayerSession"
```

---

## Task 7: Commands — `inventory`, `get`, `drop`, `balance`

**Files:**
- Modify: `internal/game/command/commands.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/grpc_service.go`

### Step 1: Add command handler constants and BuiltinCommands entries

In `internal/game/command/commands.go`, add:

```go
// Handler constants
HandlerInventory = "inventory"
HandlerGet       = "get"
HandlerDrop      = "drop"
HandlerBalance   = "balance"

// BuiltinCommands entries
{Name: "inventory", Aliases: []string{"inv", "i"}, Help: "Show backpack contents and currency", Category: CategoryWorld, Handler: HandlerInventory},
{Name: "get", Aliases: []string{"take"}, Help: "Pick up item from room floor", Category: CategoryWorld, Handler: HandlerGet},
{Name: "drop", Aliases: nil, Help: "Drop an item from your backpack", Category: CategoryWorld, Handler: HandlerDrop},
{Name: "balance", Aliases: []string{"bal"}, Help: "Show your currency (Rounds/Clips/Crates)", Category: CategoryWorld, Handler: HandlerBalance},
```

### Step 2: Add proto messages

In `api/proto/game/v1/game.proto`, add request messages:

```protobuf
// InventoryRequest asks the server for the player's backpack contents and currency.
message InventoryRequest {}

// GetItemRequest asks the server to pick up an item from the room floor.
message GetItemRequest {
  string target = 1; // item name or "all"
}

// DropItemRequest asks the server to drop an item from the backpack.
message DropItemRequest {
  string target = 1; // item name
}

// BalanceRequest asks the server for the player's currency.
message BalanceRequest {}
```

Add to `ClientMessage` oneof:

```protobuf
InventoryRequest inventory = 21;
GetItemRequest get_item = 22;
DropItemRequest drop_item = 23;
BalanceRequest balance = 24;
```

Add response messages:

```protobuf
// FloorItem describes an item on the room floor.
message FloorItem {
  string instance_id = 1;
  string name = 2;
  int32 quantity = 3;
}

// InventoryItem describes an item in the player's backpack.
message InventoryItem {
  string instance_id = 1;
  string name = 2;
  string kind = 3;
  int32 quantity = 4;
  double weight = 5;
}

// InventoryView shows the player's backpack and currency.
message InventoryView {
  repeated InventoryItem items = 1;
  int32 used_slots = 2;
  int32 max_slots = 3;
  double total_weight = 4;
  double max_weight = 5;
  string currency = 6; // formatted string e.g. "2 Crates, 3 Clips, 17 Rounds"
  int32 total_rounds = 7;
}
```

Add `InventoryView` to `ServerEvent` oneof:

```protobuf
InventoryView inventory_view = 15;
```

Add `FloorItem` to `RoomView`:

```protobuf
repeated FloorItem floor_items = 8;
```

### Step 3: Regenerate proto

```bash
make proto
```

### Step 4: Add handler methods in `grpc_service.go`

Add `floorMgr *inventory.FloorManager` field to `GameServiceServer` and wire through constructor.

Add dispatch cases and handler methods for `handleInventory`, `handleGetItem`, `handleDropItem`, `handleBalance`.

Each handler follows the existing pattern: get player session, perform action, send response.

### Step 5: Run tests + commit

```bash
mise exec -- go build ./... 2>&1
mise exec -- go test ./... -race -count=1 -timeout=300s 2>&1 | grep -E '^(ok|FAIL)'
git add api/proto/ internal/gameserver/gamev1/ internal/game/command/ internal/gameserver/grpc_service.go
git commit -m "feat(commands): inventory, get, drop, balance commands with proto messages"
```

---

## Task 8: Wire Loot Generation into Combat Death

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/combat_handler_loot_test.go`

### Step 1: Add `floorMgr` and `invRegistry` usage to `removeDeadNPCsLocked`

Add `floorMgr *inventory.FloorManager` field to `CombatHandler`. Update `NewCombatHandler` to accept it.

In `removeDeadNPCsLocked`, after removing the dead NPC and scheduling respawn, generate loot:

```go
// Generate loot from NPC's loot table.
if inst.Loot != nil {
    result := npc.GenerateLoot(*inst.Loot)
    // Award currency to the killer (first living player in combat).
    if result.Currency > 0 {
        if killer := h.firstLivingPlayer(cbt); killer != nil {
            killer.Currency += result.Currency
        }
    }
    // Drop items on floor.
    for _, item := range result.Items {
        h.floorMgr.Drop(roomID, inventory.ItemInstance{
            InstanceID: item.InstanceID,
            ItemDefID:  item.ItemDefID,
            Quantity:   item.Quantity,
        })
    }
}
```

Note: `inst.Loot` requires that `npc.Instance` gains a `Loot *LootTable` field copied from the template at spawn time. Update `npc.NewInstance` to copy `Loot` from template.

### Step 2: Add `firstLivingPlayer` helper

```go
func (h *CombatHandler) firstLivingPlayer(cbt *combat.Combat) *session.PlayerSession {
    for _, c := range cbt.Combatants {
        if c.Kind == combat.KindPlayer && !c.IsDead() {
            if sess, ok := h.sessions.GetPlayer(c.ID); ok {
                return sess
            }
        }
    }
    return nil
}
```

### Step 3: Write tests + commit

Test that after NPC death in combat, items appear on the floor and currency is awarded.

```bash
mise exec -- go test ./internal/gameserver/... -race -count=1 -v 2>&1 | tail -20
git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_loot_test.go \
        internal/game/npc/instance.go cmd/gameserver/main.go
git commit -m "feat(combat): generate loot on NPC death, award currency and drop items"
```

---

## Task 9: Wire Floor Items into `look` + Item Content YAML

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (handleLook)
- Create: `content/items/` directory with item YAML files
- Modify: `cmd/gameserver/main.go` (load items, create FloorManager)
- Modify: NPC template YAML files (add loot sections)

### Step 1: Create item YAML files

Create `content/items/medkit.yaml`:
```yaml
id: medkit
name: Medkit
description: A basic first-aid kit.
kind: consumable
weight: 0.5
stackable: true
max_stack: 5
value: 50
```

Create `content/items/scrap_metal.yaml`:
```yaml
id: scrap_metal
name: Scrap Metal
description: Twisted metal scraps. Worth something to the right buyer.
kind: junk
weight: 1.0
stackable: true
max_stack: 10
value: 5
```

Create `content/items/ganger_pistol.yaml`:
```yaml
id: ganger_pistol_item
name: Ganger's Pistol
description: A battered 9mm pistol.
kind: weapon
weapon_ref: ganger_pistol
weight: 1.2
stackable: false
max_stack: 1
value: 75
```

### Step 2: Add loot sections to NPC templates

In `content/npcs/ganger.yaml`, add:
```yaml
loot:
  currency:
    min: 10
    max: 50
  items:
    - item: scrap_metal
      chance: 0.5
      min_qty: 1
      max_qty: 3
    - item: medkit
      chance: 0.2
      min_qty: 1
      max_qty: 1
    - item: ganger_pistol_item
      chance: 0.05
      min_qty: 1
      max_qty: 1
```

In `content/npcs/scavenger.yaml`, add:
```yaml
loot:
  currency:
    min: 5
    max: 30
  items:
    - item: scrap_metal
      chance: 0.75
      min_qty: 1
      max_qty: 5
    - item: medkit
      chance: 0.15
      min_qty: 1
      max_qty: 1
```

### Step 3: Wire in main.go

Add `--item-root` flag. Load items at startup. Create `FloorManager`. Pass both to handlers.

### Step 4: Update handleLook to include floor items

In `handleLook`, after building the `RoomView`, append floor items:

```go
floorItems := s.floorMgr.ItemsInRoom(sess.RoomID)
for _, fi := range floorItems {
    name := fi.ItemDefID
    if def, ok := s.invRegistry.Item(fi.ItemDefID); ok {
        name = def.Name
    }
    roomView.FloorItems = append(roomView.FloorItems, &gamev1.FloorItem{
        InstanceId: fi.InstanceID,
        Name:       name,
        Quantity:   int32(fi.Quantity),
    })
}
```

### Step 5: Build + test + commit

```bash
mise exec -- go build ./... 2>&1
mise exec -- go test ./... -race -count=1 -timeout=300s 2>&1 | grep -E '^(ok|FAIL)'
git add content/items/ content/npcs/ cmd/gameserver/main.go internal/gameserver/grpc_service.go
git commit -m "feat(main): load items, wire FloorManager, show floor items in look"
```

---

## Task 10: Final Verification + Tag

### Step 1: Full test suite

```bash
mise exec -- go test -race -count=1 -timeout=300s $(mise exec -- go list ./... | grep -v 'storage/postgres') 2>&1 | tail -30
```

### Step 2: Coverage

```bash
mise exec -- go test ./internal/game/inventory/... -coverprofile=/tmp/cov_inv.out -count=1
mise exec -- go tool cover -func=/tmp/cov_inv.out | grep total
```

Expected: >= 80%.

### Step 3: Build + vet

```bash
mise exec -- go build -o /dev/null ./cmd/gameserver 2>&1
mise exec -- go build -o /dev/null ./cmd/frontend 2>&1
gofmt -l internal/game/inventory/ internal/game/npc/ internal/gameserver/ cmd/gameserver/
mise exec -- go vet ./... 2>&1
```

### Step 4: Tag

```bash
git tag stage11-complete
git log --oneline -12
```

---

## Critical File Locations

| File | Purpose |
|---|---|
| `internal/game/inventory/item.go` | NEW — `ItemDef`, `LoadItems`, kind constants |
| `internal/game/inventory/currency.go` | NEW — `DecomposeRounds`, `FormatRounds`, currency constants |
| `internal/game/inventory/backpack.go` | NEW — `Backpack`, `ItemInstance`, slot+weight management |
| `internal/game/inventory/floor.go` | NEW — `FloorManager` for room floor item tracking |
| `internal/game/inventory/registry.go` | MODIFY — add `items` map, `RegisterItem`, `Item` methods |
| `internal/game/npc/loot.go` | NEW — `LootTable`, `CurrencyDrop`, `ItemDrop`, `GenerateLoot` |
| `internal/game/npc/template.go` | MODIFY — add `Loot *LootTable` field |
| `internal/game/npc/instance.go` | MODIFY — add `Loot *LootTable` field copied from template |
| `internal/game/session/manager.go` | MODIFY — add `Backpack` and `Currency` to PlayerSession |
| `internal/game/command/commands.go` | MODIFY — add inventory/get/drop/balance commands |
| `api/proto/game/v1/game.proto` | MODIFY — add request/response messages, extend RoomView |
| `internal/gameserver/combat_handler.go` | MODIFY — loot generation in `removeDeadNPCsLocked` |
| `internal/gameserver/grpc_service.go` | MODIFY — add handlers, wire FloorManager |
| `cmd/gameserver/main.go` | MODIFY — load items, create FloorManager, wire everything |
| `content/items/*.yaml` | NEW — item definition YAML files |
| `content/npcs/ganger.yaml` | MODIFY — add `loot:` section |
| `content/npcs/scavenger.yaml` | MODIFY — add `loot:` section |

## Key Invariants to Verify

- Backpack slot count never exceeds MaxSlots
- Backpack total weight never exceeds MaxWeight
- Non-stackable items always have Quantity == 1
- Currency is always >= 0
- DecomposeRounds always roundtrips: crates*500 + clips*25 + rounds == total
- Floor items are removed atomically on pickup (no double-pickup via RWMutex)
- Loot table Currency.Min <= Currency.Max
- Loot table ItemDrop.Chance in (0, 1.0]
- Loot table ItemDrop.MinQty <= MaxQty
- Generated loot currency always in [min, max]
- Generated loot item quantity always in [minQty, maxQty]
