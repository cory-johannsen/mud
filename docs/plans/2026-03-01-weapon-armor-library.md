# Weapon and Armor Library Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Expand the item library from 8 weapons to ~75 lore-friendly post-apocalyptic cyberpunk items including weapons, armor, and a `wear`/`remove` command pair for armor equipping, with PF2e-style cumulative AC computation and team affinity side effects.

**Architecture:** New `ArmorDef` struct (parallel to `WeaponDef`) loaded from `content/armor/` YAML files. `Equipment.ComputedDefenses(dexMod int)` aggregates per-slot stats into `DefenseStats`. Team affinity is data-driven: items declare `team_affinity` + `cross_team_effect` in YAML; `handleWear`/`handleEquip` apply conditions on mismatch. `JobRegistry` maps class IDs to team strings. `ComputedDefenses` replaces the hardcoded AC=12 placeholder in `startCombatLocked`.

**Tech Stack:** Go 1.23, `gopkg.in/yaml.v3`, `pgregory.net/rapid` (property tests), `github.com/stretchr/testify`, gRPC/protobuf

---

## Key File Locations

- `internal/game/inventory/weapon.go` — `WeaponDef` struct
- `internal/game/inventory/equipment.go` — `Equipment`, `SlottedItem`, `ArmorSlot` constants
- `internal/game/inventory/registry.go` — `Registry` struct
- `internal/game/inventory/armor.go` — **NEW**
- `internal/game/ruleset/job.go` — `Job` struct with `Team string` field
- `internal/game/ruleset/job_registry.go` — **NEW**
- `internal/gameserver/combat_handler.go` — `startCombatLocked` (AC hardcoded to 12 at line ~594)
- `internal/gameserver/grpc_service.go` — `handleEquip`, `handleWear` (NEW)
- `internal/game/command/commands.go` — `BuiltinCommands()`, constants
- `internal/game/command/wear.go` — **NEW**
- `internal/game/command/remove_armor.go` — **NEW**
- `api/proto/game/v1/game.proto` — add `WearRequest`, `RemoveArmorRequest`
- `internal/frontend/handlers/bridge_handlers.go` — `bridgeWear`, `bridgeRemoveArmor`
- `cmd/gameserver/main.go` — `--armors-dir` flag, load armors, load jobs
- `content/armor/` — **NEW** directory with ~35 YAML files
- `content/weapons/` — add ~32 new weapon YAML files
- `content/items/` — add item YAML references for new weapons + armor

## Test Commands

```bash
# Run all non-postgres tests with race detector
go test -race $(go list ./... | grep -v postgres)

# Run a specific package
go test -race ./internal/game/inventory/...

# Run a specific test
go test -race ./internal/game/inventory/ -run TestArmorDef_Validate
```

---

## Task 1: `CrossTeamEffect` type and `ArmorDef` struct

**Files:**
- Create: `internal/game/inventory/armor.go`
- Create: `internal/game/inventory/armor_test.go`

### Step 1: Write failing tests

```go
// internal/game/inventory/armor_test.go
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

func TestArmorDef_Validate_Valid(t *testing.T) {
	def := &inventory.ArmorDef{
		ID:           "test_armor",
		Name:         "Test Armor",
		Slot:         inventory.SlotTorso,
		ACBonus:      2,
		DexCap:       3,
		CheckPenalty: -1,
		SpeedPenalty: 0,
		StrengthReq:  12,
		Bulk:         2,
		Group:        "composite",
	}
	assert.NoError(t, def.Validate())
}

func TestArmorDef_Validate_MissingID(t *testing.T) {
	def := &inventory.ArmorDef{
		Name:  "Test",
		Slot:  inventory.SlotTorso,
		Group: "composite",
	}
	assert.ErrorContains(t, def.Validate(), "id")
}

func TestArmorDef_Validate_InvalidSlot(t *testing.T) {
	def := &inventory.ArmorDef{
		ID:    "test",
		Name:  "Test",
		Slot:  inventory.ArmorSlot("invalid_slot"),
		Group: "composite",
	}
	assert.ErrorContains(t, def.Validate(), "slot")
}

func TestArmorDef_Validate_NegativeACBonus(t *testing.T) {
	def := &inventory.ArmorDef{
		ID:    "test",
		Name:  "Test",
		Slot:  inventory.SlotTorso,
		Group: "composite",
		ACBonus: -1,
	}
	assert.ErrorContains(t, def.Validate(), "ac_bonus")
}

func TestLoadArmors_LoadsYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: arm_guards
name: Arm Guards
slot: left_arm
ac_bonus: 1
dex_cap: 4
check_penalty: 0
speed_penalty: 0
strength_req: 10
bulk: 1
group: leather
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "arm_guards.yaml"), []byte(yaml), 0644))
	armors, err := inventory.LoadArmors(dir)
	require.NoError(t, err)
	require.Len(t, armors, 1)
	assert.Equal(t, "arm_guards", armors[0].ID)
	assert.Equal(t, inventory.SlotLeftArm, armors[0].Slot)
	assert.Equal(t, 1, armors[0].ACBonus)
	assert.Equal(t, 4, armors[0].DexCap)
}

func TestLoadArmors_TeamAffinityAndCrossEffect(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: tactical_vest
name: Tactical Vest
slot: torso
ac_bonus: 3
dex_cap: 2
check_penalty: -1
speed_penalty: 0
strength_req: 14
bulk: 2
group: composite
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tactical_vest.yaml"), []byte(yaml), 0644))
	armors, err := inventory.LoadArmors(dir)
	require.NoError(t, err)
	require.Len(t, armors, 1)
	assert.Equal(t, "gun", armors[0].TeamAffinity)
	require.NotNil(t, armors[0].CrossTeamEffect)
	assert.Equal(t, "condition", armors[0].CrossTeamEffect.Kind)
	assert.Equal(t, "clumsy-1", armors[0].CrossTeamEffect.Value)
}

func TestLoadArmors_EmptyDirReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	armors, err := inventory.LoadArmors(dir)
	require.NoError(t, err)
	assert.Empty(t, armors)
}

func TestLoadArmors_InvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(":::invalid"), 0644))
	_, err := inventory.LoadArmors(dir)
	assert.Error(t, err)
}

func TestProperty_ArmorSlot_AllConstantsAreValid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		slot := rapid.SampledFrom([]inventory.ArmorSlot{
			inventory.SlotHead, inventory.SlotTorso, inventory.SlotLeftArm, inventory.SlotRightArm,
			inventory.SlotHands, inventory.SlotLeftLeg, inventory.SlotRightLeg, inventory.SlotFeet,
		}).Draw(rt, "slot")
		def := &inventory.ArmorDef{
			ID:    "test",
			Name:  "Test",
			Slot:  slot,
			Group: "leather",
		}
		assert.NoError(t, def.Validate())
	})
}
```

### Step 2: Run test to verify it fails

```bash
go test ./internal/game/inventory/ -run TestArmorDef
```
Expected: FAIL — `inventory.ArmorDef undefined`

### Step 3: Implement `armor.go`

```go
// internal/game/inventory/armor.go
package inventory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CrossTeamEffect describes the mechanical consequence of a player equipping gear
// associated with a rival team.
type CrossTeamEffect struct {
	Kind  string `yaml:"kind"`  // "condition" or "penalty"
	Value string `yaml:"value"` // condition ID or penalty magnitude string
}

// ArmorGroup identifies the material/construction group of armor for crit specialization.
type ArmorGroup string

const (
	ArmorGroupLeather   ArmorGroup = "leather"
	ArmorGroupChain     ArmorGroup = "chain"
	ArmorGroupComposite ArmorGroup = "composite"
	ArmorGroupPlate     ArmorGroup = "plate"
)

// ArmorDef defines the static properties of an armor piece loaded from YAML.
//
// Precondition: All fields are set to valid values before Validate is called.
type ArmorDef struct {
	ID              string           `yaml:"id"`
	Name            string           `yaml:"name"`
	Description     string           `yaml:"description"`
	Slot            ArmorSlot        `yaml:"slot"`
	ACBonus         int              `yaml:"ac_bonus"`
	DexCap          int              `yaml:"dex_cap"`
	CheckPenalty    int              `yaml:"check_penalty"`    // non-positive; 0 means none
	SpeedPenalty    int              `yaml:"speed_penalty"`    // non-negative feet reduction
	StrengthReq     int              `yaml:"strength_req"`     // min STR to avoid speed penalty
	Bulk            int              `yaml:"bulk"`
	Group           string           `yaml:"group"`
	Traits          []string         `yaml:"traits"`
	TeamAffinity    string           `yaml:"team_affinity"`    // "gun", "machete", or ""
	CrossTeamEffect *CrossTeamEffect `yaml:"cross_team_effect"` // nil = no side effect
}

// validArmorSlots is the set of all legal ArmorSlot values.
var validArmorSlots = map[ArmorSlot]struct{}{
	SlotHead:     {},
	SlotTorso:    {},
	SlotLeftArm:  {},
	SlotRightArm: {},
	SlotHands:    {},
	SlotLeftLeg:  {},
	SlotRightLeg: {},
	SlotFeet:     {},
}

// Validate reports an error if the ArmorDef is missing required fields or contains
// illegal values.
//
// Precondition: def is non-nil.
// Postcondition: Returns nil if and only if the def is well-formed.
func (a *ArmorDef) Validate() error {
	if a.ID == "" {
		return errors.New("armor def: id must not be empty")
	}
	if a.Name == "" {
		return errors.New("armor def: name must not be empty")
	}
	if _, ok := validArmorSlots[a.Slot]; !ok {
		return fmt.Errorf("armor def %q: invalid slot %q", a.ID, a.Slot)
	}
	if a.ACBonus < 0 {
		return fmt.Errorf("armor def %q: ac_bonus must be >= 0, got %d", a.ID, a.ACBonus)
	}
	if a.CheckPenalty > 0 {
		return fmt.Errorf("armor def %q: check_penalty must be <= 0, got %d", a.ID, a.CheckPenalty)
	}
	if a.SpeedPenalty < 0 {
		return fmt.Errorf("armor def %q: speed_penalty must be >= 0, got %d", a.ID, a.SpeedPenalty)
	}
	if a.Group == "" {
		return fmt.Errorf("armor def %q: group must not be empty", a.ID)
	}
	if a.CrossTeamEffect != nil {
		if a.CrossTeamEffect.Kind != "condition" && a.CrossTeamEffect.Kind != "penalty" {
			return fmt.Errorf("armor def %q: cross_team_effect.kind must be condition or penalty", a.ID)
		}
		if a.CrossTeamEffect.Value == "" {
			return fmt.Errorf("armor def %q: cross_team_effect.value must not be empty", a.ID)
		}
	}
	return nil
}

// LoadArmors reads all .yaml files in dir and returns the parsed ArmorDef slice.
//
// Precondition: dir must be a readable directory.
// Postcondition: Returns a non-nil slice and nil error on success; all returned defs pass Validate.
func LoadArmors(dir string) ([]*ArmorDef, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("LoadArmors: cannot read directory %q: %w", dir, err)
	}
	var armors []*ArmorDef
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("LoadArmors: cannot read file %q: %w", path, err)
		}
		var a ArmorDef
		if err := yaml.Unmarshal(data, &a); err != nil {
			return nil, fmt.Errorf("LoadArmors: cannot parse file %q: %w", path, err)
		}
		if err := a.Validate(); err != nil {
			return nil, fmt.Errorf("LoadArmors: invalid armor in %q: %w", path, err)
		}
		armors = append(armors, &a)
	}
	return armors, nil
}
```

### Step 4: Run tests to verify they pass

```bash
go test -race ./internal/game/inventory/ -run TestArmorDef
go test -race ./internal/game/inventory/ -run TestLoadArmors
go test -race ./internal/game/inventory/ -run TestProperty_ArmorSlot
```
Expected: PASS

### Step 5: Commit

```bash
git add internal/game/inventory/armor.go internal/game/inventory/armor_test.go
git commit -m "feat: add ArmorDef struct, loader, and CrossTeamEffect type"
```

---

## Task 2: Add team affinity fields to `WeaponDef`

**Files:**
- Modify: `internal/game/inventory/weapon.go`
- Modify: `internal/game/inventory/weapon_test.go`

### Step 1: Write failing test

In `weapon_test.go`, add at the end of the existing test file:

```go
func TestWeaponDef_TeamAffinity_ParsedFromYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: machete_test
name: Test Machete
damage_dice: "1d8"
damage_type: slashing
range_increment: 0
kind: one_handed
group: blade
team_affinity: machete
cross_team_effect:
  kind: condition
  value: clumsy-1
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "machete_test.yaml"), []byte(yaml), 0644))
	weapons, err := inventory.LoadWeapons(dir)
	require.NoError(t, err)
	require.Len(t, weapons, 1)
	assert.Equal(t, "machete", weapons[0].TeamAffinity)
	require.NotNil(t, weapons[0].CrossTeamEffect)
	assert.Equal(t, "condition", weapons[0].CrossTeamEffect.Kind)
	assert.Equal(t, "clumsy-1", weapons[0].CrossTeamEffect.Value)
}

func TestWeaponDef_NoAffinity_NilCrossTeamEffect(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: generic_pistol
name: Generic Pistol
damage_dice: "1d6"
damage_type: piercing
range_increment: 30
reload_actions: 1
magazine_capacity: 15
firing_modes: [single]
kind: one_handed
group: firearm
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "generic_pistol.yaml"), []byte(yaml), 0644))
	weapons, err := inventory.LoadWeapons(dir)
	require.NoError(t, err)
	require.Len(t, weapons, 1)
	assert.Empty(t, weapons[0].TeamAffinity)
	assert.Nil(t, weapons[0].CrossTeamEffect)
}
```

### Step 2: Run test to verify it fails

```bash
go test ./internal/game/inventory/ -run TestWeaponDef_TeamAffinity
```
Expected: FAIL — `WeaponDef` has no `TeamAffinity` field

### Step 3: Extend `WeaponDef` in `weapon.go`

Add two fields to the `WeaponDef` struct (after the existing `Kind` field):

```go
// In WeaponDef struct, after Kind WeaponKind yaml:"kind":
TeamAffinity     string           `yaml:"team_affinity"`
CrossTeamEffect  *CrossTeamEffect `yaml:"cross_team_effect"`
```

Also add `Group` field (needed for crit specialization, consistent with ArmorDef):

```go
Group string `yaml:"group"`
```

### Step 4: Run tests

```bash
go test -race ./internal/game/inventory/ -run TestWeaponDef
```
Expected: PASS

### Step 5: Commit

```bash
git add internal/game/inventory/weapon.go internal/game/inventory/weapon_test.go
git commit -m "feat: add TeamAffinity and CrossTeamEffect fields to WeaponDef"
```

---

## Task 3: Registry armor methods

**Files:**
- Modify: `internal/game/inventory/registry.go`
- Modify: `internal/game/inventory/registry_test.go`

### Step 1: Write failing tests

Add to `registry_test.go`:

```go
func TestRegistry_RegisterArmor_And_Lookup(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{
		ID:    "test_helm",
		Name:  "Test Helm",
		Slot:  inventory.SlotHead,
		Group: "composite",
	}
	require.NoError(t, reg.RegisterArmor(def))
	got, ok := reg.Armor("test_helm")
	require.True(t, ok)
	assert.Equal(t, def, got)
}

func TestRegistry_Armor_Unknown_ReturnsFalse(t *testing.T) {
	reg := inventory.NewRegistry()
	_, ok := reg.Armor("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_RegisterArmor_DuplicateReturnsError(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{ID: "dup", Name: "Dup", Slot: inventory.SlotHead, Group: "leather"}
	require.NoError(t, reg.RegisterArmor(def))
	assert.Error(t, reg.RegisterArmor(def))
}

func TestRegistry_AllArmors_ReturnsAll(t *testing.T) {
	reg := inventory.NewRegistry()
	for _, id := range []string{"helm", "vest", "boots"} {
		slot := inventory.SlotHead
		if id == "vest" { slot = inventory.SlotTorso }
		if id == "boots" { slot = inventory.SlotFeet }
		require.NoError(t, reg.RegisterArmor(&inventory.ArmorDef{
			ID: id, Name: id, Slot: slot, Group: "leather",
		}))
	}
	all := reg.AllArmors()
	assert.Len(t, all, 3)
}

func TestProperty_Registry_ArmorRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := inventory.NewRegistry()
		id := rapid.StringMatching(`[a-z][a-z0-9_]{1,15}`).Draw(rt, "id")
		def := &inventory.ArmorDef{
			ID:    id,
			Name:  "Test",
			Slot:  inventory.SlotTorso,
			Group: "leather",
		}
		require.NoError(t, reg.RegisterArmor(def))
		got, ok := reg.Armor(id)
		assert.True(t, ok)
		assert.Equal(t, def, got)
	})
}
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/game/inventory/ -run TestRegistry_.*Armor
```
Expected: FAIL — `reg.RegisterArmor undefined`

### Step 3: Add armor methods to `registry.go`

Add `armors map[string]*ArmorDef` field to the `Registry` struct, initialize in `NewRegistry()`, and add three methods:

```go
// In Registry struct, add:
armors map[string]*ArmorDef

// In NewRegistry(), add to return:
armors: make(map[string]*ArmorDef),

// New methods:

// RegisterArmor adds an ArmorDef to the registry.
//
// Precondition: def must be non-nil with a non-empty ID.
// Postcondition: Returns error if an armor with the same ID is already registered.
func (r *Registry) RegisterArmor(def *ArmorDef) error {
	if _, exists := r.armors[def.ID]; exists {
		return fmt.Errorf("armor %q already registered", def.ID)
	}
	r.armors[def.ID] = def
	return nil
}

// Armor returns the ArmorDef with the given ID, or false if not found.
//
// Precondition: id must be non-empty.
// Postcondition: Returns (def, true) if found; (nil, false) otherwise.
func (r *Registry) Armor(id string) (*ArmorDef, bool) {
	def, ok := r.armors[id]
	return def, ok
}

// AllArmors returns all registered ArmorDef instances in unspecified order.
//
// Postcondition: Returns a non-nil slice; may be empty if no armors registered.
func (r *Registry) AllArmors() []*ArmorDef {
	out := make([]*ArmorDef, 0, len(r.armors))
	for _, def := range r.armors {
		out = append(out, def)
	}
	return out
}
```

### Step 4: Run tests

```bash
go test -race ./internal/game/inventory/ -run TestRegistry
```
Expected: PASS

### Step 5: Commit

```bash
git add internal/game/inventory/registry.go internal/game/inventory/registry_test.go
git commit -m "feat: add armor methods to inventory Registry"
```

---

## Task 4: `DefenseStats` and `Equipment.ComputedDefenses`

**Files:**
- Modify: `internal/game/inventory/equipment.go`
- Modify: `internal/game/inventory/equipment_test.go`

### Step 1: Write failing tests

Add to `equipment_test.go` (check existing test file pattern first with `go test ./internal/game/inventory/ -list Test`):

```go
func TestComputedDefenses_NoArmor(t *testing.T) {
	eq := inventory.NewEquipment()
	stats := eq.ComputedDefenses(3)
	assert.Equal(t, 0, stats.ACBonus)
	assert.Equal(t, 3, stats.EffectiveDex) // no cap = full dex
	assert.Equal(t, 0, stats.CheckPenalty)
	assert.Equal(t, 0, stats.SpeedPenalty)
	assert.Equal(t, 0, stats.StrengthReq)
}

func TestComputedDefenses_SingleSlot(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.ArmorDef{
		ID:           "vest",
		Name:         "Vest",
		Slot:         inventory.SlotTorso,
		ACBonus:      3,
		DexCap:       2,
		CheckPenalty: -1,
		SpeedPenalty: 0,
		StrengthReq:  14,
		Group:        "composite",
	}
	require.NoError(t, reg.RegisterArmor(def))
	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{ItemDefID: "vest", Name: "Vest"}

	stats := eq.ComputedDefenses(4) // dex 4, cap is 2
	assert.Equal(t, 3, stats.ACBonus)
	assert.Equal(t, 2, stats.EffectiveDex) // capped at 2
	assert.Equal(t, -1, stats.CheckPenalty)
	assert.Equal(t, 0, stats.SpeedPenalty)
	assert.Equal(t, 14, stats.StrengthReq)
}

func TestComputedDefenses_MultiSlot_SumsPenalties(t *testing.T) {
	reg := inventory.NewRegistry()
	head := &inventory.ArmorDef{ID: "helm", Name: "Helm", Slot: inventory.SlotHead, ACBonus: 1, DexCap: 3, CheckPenalty: -1, Group: "composite"}
	torso := &inventory.ArmorDef{ID: "plate", Name: "Plate", Slot: inventory.SlotTorso, ACBonus: 4, DexCap: 1, CheckPenalty: -2, Group: "plate"}
	require.NoError(t, reg.RegisterArmor(head))
	require.NoError(t, reg.RegisterArmor(torso))
	eq := inventory.NewEquipment()
	eq.Armor[inventory.SlotHead] = &inventory.SlottedItem{ItemDefID: "helm", Name: "Helm"}
	eq.Armor[inventory.SlotTorso] = &inventory.SlottedItem{ItemDefID: "plate", Name: "Plate"}

	stats := eq.ComputedDefenses(5)
	assert.Equal(t, 5, stats.ACBonus)         // 1+4
	assert.Equal(t, 1, stats.EffectiveDex)    // min(5, 3, 1) = 1
	assert.Equal(t, -3, stats.CheckPenalty)   // -1 + -2
}

func TestProperty_ComputedDefenses_ACBonusEqualsSum(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		slots := []inventory.ArmorSlot{
			inventory.SlotHead, inventory.SlotTorso, inventory.SlotHands, inventory.SlotFeet,
		}
		reg := inventory.NewRegistry()
		eq := inventory.NewEquipment()
		totalAC := 0
		for _, slot := range slots {
			ac := rapid.IntRange(0, 6).Draw(rt, "ac")
			totalAC += ac
			id := string(slot) + "_test"
			def := &inventory.ArmorDef{
				ID: id, Name: id, Slot: slot,
				ACBonus: ac, DexCap: 4, Group: "leather",
			}
			require.NoError(t, reg.RegisterArmor(def))
			eq.Armor[slot] = &inventory.SlottedItem{ItemDefID: id, Name: id}
		}
		stats := eq.ComputedDefenses(2)
		assert.Equal(t, totalAC, stats.ACBonus)
	})
}
```

**NOTE:** `ComputedDefenses` needs the `Registry` to look up `ArmorDef` by slot item's `ItemDefID`. Pass it as a parameter: `ComputedDefenses(reg *Registry, dexMod int) DefenseStats`.

Update all test calls accordingly.

### Step 2: Run tests to verify they fail

```bash
go test ./internal/game/inventory/ -run TestComputedDefenses
```
Expected: FAIL — `DefenseStats undefined`

### Step 3: Implement in `equipment.go`

Add after the existing `Equipment` type and methods:

```go
// DefenseStats holds the aggregated defensive statistics computed from all equipped armor.
type DefenseStats struct {
	ACBonus      int // sum of all slot ac_bonus values
	EffectiveDex int // min(dexMod, strictest DexCap) across equipped slots
	CheckPenalty int // sum of all slot check_penalty values (non-positive)
	SpeedPenalty int // sum of speed_penalty values (applied when STR < StrengthReq)
	StrengthReq  int // max strength_req across all equipped slots
}

// ComputedDefenses aggregates PF2e-style defense stats from all currently equipped armor slots.
//
// Precondition: reg must be non-nil; dexMod may be any integer.
// Postcondition: Returns a DefenseStats where ACBonus equals the sum of all equipped slot
// ac_bonus values; EffectiveDex <= dexMod and <= any single slot's DexCap.
func (e *Equipment) ComputedDefenses(reg *Registry, dexMod int) DefenseStats {
	stats := DefenseStats{EffectiveDex: dexMod}
	hasDexCap := false
	for slot, slotted := range e.Armor {
		if slotted == nil {
			continue
		}
		def, ok := reg.Armor(slotted.ItemDefID)
		if !ok {
			// Unknown def — skip silently (logged at call site if desired)
			continue
		}
		_ = slot
		stats.ACBonus += def.ACBonus
		stats.CheckPenalty += def.CheckPenalty
		stats.SpeedPenalty += def.SpeedPenalty
		if def.StrengthReq > stats.StrengthReq {
			stats.StrengthReq = def.StrengthReq
		}
		if !hasDexCap || def.DexCap < stats.EffectiveDex {
			stats.EffectiveDex = def.DexCap
			hasDexCap = true
		}
	}
	if stats.EffectiveDex > dexMod {
		stats.EffectiveDex = dexMod
	}
	return stats
}
```

### Step 4: Run tests

```bash
go test -race ./internal/game/inventory/ -run TestComputedDefenses
go test -race ./internal/game/inventory/ -run TestProperty_ComputedDefenses
```
Expected: PASS

### Step 5: Commit

```bash
git add internal/game/inventory/equipment.go internal/game/inventory/equipment_test.go
git commit -m "feat: add DefenseStats and Equipment.ComputedDefenses"
```

---

## Task 5: `JobRegistry` for team lookup

**Files:**
- Create: `internal/game/ruleset/job_registry.go`
- Create: `internal/game/ruleset/job_registry_test.go`

### Step 1: Write failing tests

```go
// internal/game/ruleset/job_registry_test.go
package ruleset_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestJobRegistry_TeamFor_KnownJob(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	reg.Register(&ruleset.Job{ID: "libertarian", Team: "gun"})
	team := reg.TeamFor("libertarian")
	assert.Equal(t, "gun", team)
}

func TestJobRegistry_TeamFor_UnknownJobReturnsEmpty(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	team := reg.TeamFor("nonexistent")
	assert.Equal(t, "", team)
}

func TestJobRegistry_TeamFor_NoTeamJobReturnsEmpty(t *testing.T) {
	reg := ruleset.NewJobRegistry()
	reg.Register(&ruleset.Job{ID: "drifter", Team: ""})
	team := reg.TeamFor("drifter")
	assert.Equal(t, "", team)
}

func TestJobRegistry_LoadFromDir(t *testing.T) {
	jobs, err := ruleset.LoadJobs("../../../content/jobs")
	require.NoError(t, err)
	reg := ruleset.NewJobRegistry()
	for _, j := range jobs {
		reg.Register(j)
	}
	// All jobs must round-trip
	for _, j := range jobs {
		assert.Equal(t, j.Team, reg.TeamFor(j.ID))
	}
}

func TestProperty_JobRegistry_RoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.StringMatching(`[a-z][a-z_]{1,15}`).Draw(rt, "id")
		team := rapid.SampledFrom([]string{"", "gun", "machete"}).Draw(rt, "team")
		reg := ruleset.NewJobRegistry()
		reg.Register(&ruleset.Job{ID: id, Team: team})
		assert.Equal(t, team, reg.TeamFor(id))
	})
}
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/game/ruleset/ -run TestJobRegistry
```
Expected: FAIL — `ruleset.NewJobRegistry undefined`

### Step 3: Implement `job_registry.go`

```go
// internal/game/ruleset/job_registry.go
package ruleset

// JobRegistry provides fast lookup of team affiliation by job/class ID.
type JobRegistry struct {
	jobs map[string]*Job
}

// NewJobRegistry returns an empty JobRegistry.
//
// Postcondition: Returns a non-nil *JobRegistry.
func NewJobRegistry() *JobRegistry {
	return &JobRegistry{jobs: make(map[string]*Job)}
}

// Register adds a Job to the registry.
//
// Precondition: job must be non-nil with a non-empty ID.
// Postcondition: job is retrievable via TeamFor.
func (r *JobRegistry) Register(job *Job) {
	r.jobs[job.ID] = job
}

// TeamFor returns the team string ("gun", "machete", or "") for the given class ID.
//
// Precondition: classID may be any string.
// Postcondition: Returns "" if classID is unknown or if the job has no team affiliation.
func (r *JobRegistry) TeamFor(classID string) string {
	if j, ok := r.jobs[classID]; ok {
		return j.Team
	}
	return ""
}
```

### Step 4: Run tests

```bash
go test -race ./internal/game/ruleset/ -run TestJobRegistry
```
Expected: PASS

### Step 5: Commit

```bash
git add internal/game/ruleset/job_registry.go internal/game/ruleset/job_registry_test.go
git commit -m "feat: add JobRegistry for team affiliation lookup by class ID"
```

---

## Task 6: `wear` command — armor equip (CMD-1 through CMD-7)

This command allows players to equip armor items: `wear <item_id> <slot>`

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/wear.go`
- Create: `internal/game/command/wear_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Run: `make proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`

### Step 1: Add `HandlerWear` constant and `BuiltinCommands` entry (CMD-1, CMD-2)

In `internal/game/command/commands.go`, add:

```go
// After existing HandlerEquip constant:
HandlerWear = "wear"
HandlerRemoveArmor = "remove"
```

In `BuiltinCommands()`, add:

```go
{
    Name:        HandlerWear,
    Description: "Equip a piece of armor from your inventory",
    Usage:       "wear <item_id> <slot>",
    MinArgs:     2,
    MaxArgs:     2,
},
{
    Name:        HandlerRemoveArmor,
    Description: "Remove a piece of armor and return it to inventory",
    Usage:       "remove <slot>",
    MinArgs:     1,
    MaxArgs:     1,
},
```

### Step 2: Write failing tests for `HandleWear` (CMD-3, SWENG-5)

```go
// internal/game/command/wear_test.go
package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func makeWearSession(t *testing.T, reg *inventory.Registry) *session.PlayerSession {
	t.Helper()
	bp := inventory.NewBackpack(20, 100)
	eq := inventory.NewEquipment()
	sess := &session.PlayerSession{
		UID:        "test-uid",
		CharName:   "Tester",
		Backpack:   bp,
		Equipment:  eq,
		Class:      "drifter",
	}
	return sess
}

func TestHandleWear_EquipsArmorFromBackpack(t *testing.T) {
	reg := inventory.NewRegistry()
	armorDef := &inventory.ArmorDef{
		ID: "test_helm", Name: "Test Helm", Slot: inventory.SlotHead, Group: "composite",
	}
	require.NoError(t, reg.RegisterArmor(armorDef))
	itemDef := &inventory.ItemDef{ID: "test_helm_item", Name: "Test Helm", Kind: "armor", ArmorRef: "test_helm", Weight: 1}
	require.NoError(t, reg.RegisterItem(itemDef))

	sess := makeWearSession(t, reg)
	_, err := sess.Backpack.Add("test_helm_item", 1, reg)
	require.NoError(t, err)

	result := command.HandleWear(sess, reg, "test_helm_item head")
	assert.Contains(t, result, "Wore")
	assert.NotNil(t, sess.Equipment.Armor[inventory.SlotHead])
	assert.Equal(t, "test_helm", sess.Equipment.Armor[inventory.SlotHead].ItemDefID)
}

func TestHandleWear_ItemNotInBackpack(t *testing.T) {
	reg := inventory.NewRegistry()
	sess := makeWearSession(t, reg)
	result := command.HandleWear(sess, reg, "nonexistent head")
	assert.Contains(t, result, "not found")
}

func TestHandleWear_ItemNotArmor(t *testing.T) {
	reg := inventory.NewRegistry()
	itemDef := &inventory.ItemDef{ID: "sword", Name: "Sword", Kind: "weapon", Weight: 1}
	require.NoError(t, reg.RegisterItem(itemDef))
	sess := makeWearSession(t, reg)
	_, err := sess.Backpack.Add("sword", 1, reg)
	require.NoError(t, err)
	result := command.HandleWear(sess, reg, "sword head")
	assert.Contains(t, result, "not armor")
}

func TestHandleWear_WrongSlot(t *testing.T) {
	reg := inventory.NewRegistry()
	armorDef := &inventory.ArmorDef{
		ID: "boots", Name: "Boots", Slot: inventory.SlotFeet, Group: "leather",
	}
	require.NoError(t, reg.RegisterArmor(armorDef))
	itemDef := &inventory.ItemDef{ID: "boots_item", Name: "Boots", Kind: "armor", ArmorRef: "boots", Weight: 1}
	require.NoError(t, reg.RegisterItem(itemDef))
	sess := makeWearSession(t, reg)
	_, err := sess.Backpack.Add("boots_item", 1, reg)
	require.NoError(t, err)
	result := command.HandleWear(sess, reg, "boots_item head") // wrong slot
	assert.Contains(t, result, "slot")
}

func TestProperty_HandleWear_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := inventory.NewRegistry()
		sess := makeWearSession(t, reg)
		arg := rapid.String().Draw(rt, "arg")
		assert.NotPanics(t, func() { command.HandleWear(sess, reg, arg) })
	})
}
```

**NOTE:** `ItemDef` needs an `ArmorRef string` field (parallel to `WeaponRef`). Check `internal/game/inventory/item.go` — if it already has this field, use it; if not, add it in this task.

### Step 3: Run tests to verify they fail

```bash
go test ./internal/game/command/ -run TestHandleWear
```
Expected: FAIL

### Step 4: Check/add `ArmorRef` to `ItemDef`

Open `internal/game/inventory/item.go`. If `ArmorRef string` is not present, add it after `WeaponRef`:

```go
ArmorRef    string `yaml:"armor_ref"`    // references an ArmorDef ID; set when Kind == "armor"
```

### Step 5: Implement `HandleWear` (CMD-3)

```go
// internal/game/command/wear.go
package command

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// HandleWear processes the "wear <item_id> <slot>" command.
//
// Precondition: sess, reg must be non-nil; arg is the raw argument string after "wear".
// Postcondition: Returns a non-empty result string. On success, moves item from
// backpack to the specified equipment slot.
func HandleWear(sess *session.PlayerSession, reg *inventory.Registry, arg string) string {
	parts := strings.Fields(arg)
	if len(parts) < 2 {
		return "Usage: wear <item_id> <slot>"
	}
	itemID := parts[0]
	slotStr := parts[1]

	// Resolve item from backpack
	inst := sess.Backpack.FindByItemDefID(itemID)
	if inst == nil {
		return fmt.Sprintf("Item %q not found in inventory.", itemID)
	}

	// Look up item definition
	itemDef, ok := reg.Item(itemID)
	if !ok {
		return fmt.Sprintf("Item definition %q not found.", itemID)
	}
	if itemDef.Kind != "armor" {
		return fmt.Sprintf("%q is not armor.", itemDef.Name)
	}
	if itemDef.ArmorRef == "" {
		return fmt.Sprintf("%q has no armor definition.", itemDef.Name)
	}

	// Look up armor definition
	armorDef, ok := reg.Armor(itemDef.ArmorRef)
	if !ok {
		return fmt.Sprintf("Armor definition %q not found.", itemDef.ArmorRef)
	}

	// Validate slot matches armor's slot
	targetSlot := inventory.ArmorSlot(slotStr)
	if armorDef.Slot != targetSlot {
		return fmt.Sprintf("%q must be worn on %s, not %s.", armorDef.Name, armorDef.Slot, targetSlot)
	}

	// Move from backpack to equipment slot
	if err := sess.Backpack.Remove(inst.InstanceID, 1); err != nil {
		return fmt.Sprintf("Could not remove item from inventory: %v", err)
	}

	// Return previous item to backpack if slot was occupied
	if prev := sess.Equipment.Armor[targetSlot]; prev != nil {
		if _, err := sess.Backpack.Add(prev.ItemDefID, 1, reg); err != nil {
			// Re-equip failed return; put the removed item back
			_, _ = sess.Backpack.Add(itemID, 1, reg)
			return fmt.Sprintf("Inventory full: cannot unequip previous %s.", prev.Name)
		}
	}

	sess.Equipment.Armor[targetSlot] = &inventory.SlottedItem{
		ItemDefID: itemDef.ArmorRef,
		Name:      armorDef.Name,
	}

	return fmt.Sprintf("Wore %s.", armorDef.Name)
}
```

### Step 6: Run tests

```bash
go test -race ./internal/game/command/ -run TestHandleWear
go test -race ./internal/game/command/ -run TestProperty_HandleWear
```
Expected: PASS

### Step 7: Add `WearRequest` proto message (CMD-4)

In `api/proto/game/v1/game.proto`:

1. Add to `ClientMessage` oneof — use the next available field number (check existing oneof for the highest number, then use +1):

```protobuf
WearRequest wear = <next_field>;
```

2. Add message definition:

```protobuf
// WearRequest asks the server to equip an armor item from inventory into a body slot.
message WearRequest {
  string item_id = 1;
  string slot = 2; // "head", "torso", "left_arm", "right_arm", "hands", "left_leg", "right_leg", "feet"
}
```

3. Run `make proto`

### Step 8: Add `bridgeWear` to bridge handlers (CMD-5)

In `internal/frontend/handlers/bridge_handlers.go`, add:

```go
func bridgeWear(args []string) (proto.Message, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("usage: wear <item_id> <slot>")
	}
	return &gamev1.WearRequest{ItemId: args[0], Slot: args[1]}, nil
}
```

Register in `bridgeHandlerMap`:

```go
command.HandlerWear: bridgeWear,
```

Verify `TestAllCommandHandlersAreWired` passes:

```bash
go test -race ./internal/frontend/handlers/ -run TestAllCommandHandlersAreWired
```

### Step 9: Implement `handleWear` in `grpc_service.go` (CMD-6)

```go
// handleWear equips an armor item from the player's backpack into the specified body slot.
//
// Precondition: uid must be a valid connected player; req must be non-nil.
// Postcondition: On success, returns a message event; on error, returns an error event.
func (s *GameServiceServer) handleWear(uid string, req *gamev1.WearRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	arg := req.GetItemId() + " " + req.GetSlot()
	result := command.HandleWear(sess, s.invRegistry, arg)

	// Team affinity check
	itemDef, ok := s.invRegistry.Item(req.GetItemId())
	if ok && itemDef.ArmorRef != "" {
		if armorDef, ok := s.invRegistry.Armor(itemDef.ArmorRef); ok && armorDef.TeamAffinity != "" {
			playerTeam := s.jobRegistry.TeamFor(sess.Class)
			if playerTeam != "" && playerTeam != armorDef.TeamAffinity {
				if armorDef.CrossTeamEffect != nil {
					s.applyEquipEffect(uid, armorDef.CrossTeamEffect)
				}
			}
		}
	}

	return messageEvent(result), nil
}
```

Add `applyEquipEffect` helper (add near other helpers in `grpc_service.go`):

```go
// applyEquipEffect applies a CrossTeamEffect to the player's session when they equip
// rival-team gear.
//
// Precondition: uid must be a valid connected player; effect must be non-nil.
// Postcondition: Condition or penalty applied; warn-logged if condition ID is unknown.
func (s *GameServiceServer) applyEquipEffect(uid string, effect *inventory.CrossTeamEffect) {
	if effect.Kind == "condition" {
		cond := s.condRegistry.Get(effect.Value)
		if cond == nil {
			s.logger.Warn("unknown cross-team condition", zap.String("condition", effect.Value))
			return
		}
		s.condH.Apply(uid, cond)
	}
	// "penalty" kind is logged and deferred to future check-penalty aggregation
}
```

Wire into the `dispatch` type switch:

```go
case *gamev1.WearRequest:
    return s.handleWear(uid, req)
```

### Step 10: Verify all tests pass (CMD-7)

```bash
go test -race $(go list ./... | grep -v postgres)
```
Expected: PASS

### Step 11: Commit

```bash
git add internal/game/command/commands.go internal/game/command/wear.go internal/game/command/wear_test.go \
        api/proto/game/v1/game.proto internal/frontend/handlers/bridge_handlers.go \
        internal/gameserver/grpc_service.go
git commit -m "feat: add wear command for armor equipping with team affinity check"
```

---

## Task 7: `remove` command — armor unequip (CMD-1 through CMD-7)

**Files:**
- Modify: `internal/game/command/commands.go` (HandlerRemoveArmor already added in Task 6)
- Create: `internal/game/command/remove_armor.go`
- Create: `internal/game/command/remove_armor_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Run: `make proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`

### Step 1: Write failing tests

```go
// internal/game/command/remove_armor_test.go
package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestHandleRemoveArmor_RemovesEquippedSlot(t *testing.T) {
	reg := inventory.NewRegistry()
	armorDef := &inventory.ArmorDef{ID: "boots", Name: "Boots", Slot: inventory.SlotFeet, Group: "leather"}
	require.NoError(t, reg.RegisterArmor(armorDef))
	itemDef := &inventory.ItemDef{ID: "boots_item", Name: "Boots", Kind: "armor", ArmorRef: "boots", Weight: 1}
	require.NoError(t, reg.RegisterItem(itemDef))

	sess := makeWearSession(t, reg)
	sess.Equipment.Armor[inventory.SlotFeet] = &inventory.SlottedItem{ItemDefID: "boots", Name: "Boots"}

	result := command.HandleRemoveArmor(sess, reg, "feet")
	assert.Contains(t, result, "Removed")
	assert.Nil(t, sess.Equipment.Armor[inventory.SlotFeet])
}

func TestHandleRemoveArmor_EmptySlotReturnsError(t *testing.T) {
	reg := inventory.NewRegistry()
	sess := makeWearSession(t, reg)
	result := command.HandleRemoveArmor(sess, reg, "head")
	assert.Contains(t, result, "nothing")
}

func TestHandleRemoveArmor_InvalidSlotReturnsError(t *testing.T) {
	reg := inventory.NewRegistry()
	sess := makeWearSession(t, reg)
	result := command.HandleRemoveArmor(sess, reg, "invalid_slot")
	assert.Contains(t, result, "slot")
}

func TestProperty_HandleRemoveArmor_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := inventory.NewRegistry()
		sess := makeWearSession(t, reg)
		slot := rapid.String().Draw(rt, "slot")
		assert.NotPanics(t, func() { command.HandleRemoveArmor(sess, reg, slot) })
	})
}
```

### Step 2: Run tests to verify they fail

```bash
go test ./internal/game/command/ -run TestHandleRemoveArmor
```

### Step 3: Implement `HandleRemoveArmor`

```go
// internal/game/command/remove_armor.go
package command

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// HandleRemoveArmor processes the "remove <slot>" command.
//
// Precondition: sess, reg must be non-nil; arg is the slot name string.
// Postcondition: On success, moves armor from slot to backpack; slot is set to nil.
func HandleRemoveArmor(sess *session.PlayerSession, reg *inventory.Registry, arg string) string {
	slot := inventory.ArmorSlot(arg)
	if _, ok := inventory.ValidArmorSlots()[slot]; !ok {
		return fmt.Sprintf("Unknown slot %q. Valid slots: head, torso, left_arm, right_arm, hands, left_leg, right_leg, feet.", arg)
	}

	slotted := sess.Equipment.Armor[slot]
	if slotted == nil {
		return fmt.Sprintf("You're wearing nothing on your %s.", slot)
	}

	// Find the item def ID in items registry that references this armor def
	// We need to put the item back in the backpack as an item (by item def ID)
	// The slotted.ItemDefID is the ArmorDef ID. We need the ItemDef ID.
	// Search items by ArmorRef:
	itemDefID := ""
	for _, item := range reg.AllItems() {
		if item.ArmorRef == slotted.ItemDefID {
			itemDefID = item.ID
			break
		}
	}
	if itemDefID == "" {
		// Fallback: use the armor def ID directly
		itemDefID = slotted.ItemDefID
	}

	if _, err := sess.Backpack.Add(itemDefID, 1, reg); err != nil {
		return fmt.Sprintf("Cannot remove %s: inventory full.", slotted.Name)
	}

	sess.Equipment.Armor[slot] = nil
	return fmt.Sprintf("Removed %s.", slotted.Name)
}
```

**NOTE:** `Registry.AllItems()` may not exist. Check `registry.go`. If missing, add it:

```go
func (r *Registry) AllItems() []*ItemDef {
	out := make([]*ItemDef, 0, len(r.items))
	for _, it := range r.items {
		out = append(out, it)
	}
	return out
}
```

Also add `ValidArmorSlots() map[ArmorSlot]struct{}` to `equipment.go` (exposes the private `validArmorSlots` map):

```go
// ValidArmorSlots returns the set of all legal ArmorSlot values.
//
// Postcondition: Returns a non-nil map containing all 8 valid slot constants.
func ValidArmorSlots() map[ArmorSlot]struct{} {
	return validArmorSlots
}
```

Wait — `validArmorSlots` is defined in `armor.go`. Move it to `equipment.go` since that's where `ArmorSlot` lives, or keep it in `armor.go` and export from there. Keep it in `armor.go` and export via `ValidArmorSlots()` in `armor.go`.

### Step 4: Run tests

```bash
go test -race ./internal/game/command/ -run TestHandleRemoveArmor
```
Expected: PASS

### Step 5: Add `RemoveArmorRequest` proto + wire (CMD-4, CMD-5, CMD-6)

Follow the same pattern as Task 6 Steps 7-9:

Proto:
```protobuf
RemoveArmorRequest remove_armor = <next_field>;

// RemoveArmorRequest asks the server to remove armor from a body slot.
message RemoveArmorRequest {
  string slot = 1;
}
```

Bridge handler:
```go
func bridgeRemoveArmor(args []string) (proto.Message, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("usage: remove <slot>")
	}
	return &gamev1.RemoveArmorRequest{Slot: args[0]}, nil
}
```
Register: `command.HandlerRemoveArmor: bridgeRemoveArmor`

grpc_service handler:
```go
func (s *GameServiceServer) handleRemoveArmor(uid string, req *gamev1.RemoveArmorRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("player not found"), nil
	}
	result := command.HandleRemoveArmor(sess, s.invRegistry, req.GetSlot())
	return messageEvent(result), nil
}
```

Wire: `case *gamev1.RemoveArmorRequest: return s.handleRemoveArmor(uid, req)`

Run `make proto` after proto changes.

### Step 6: Run all tests (CMD-7)

```bash
go test -race $(go list ./... | grep -v postgres)
```

### Step 7: Commit

```bash
git add internal/game/command/remove_armor.go internal/game/command/remove_armor_test.go \
        api/proto/game/v1/game.proto internal/frontend/handlers/bridge_handlers.go \
        internal/gameserver/grpc_service.go internal/game/inventory/equipment.go \
        internal/game/inventory/armor.go
git commit -m "feat: add remove command for armor unequipping"
```

---

## Task 8: Wire `ComputedDefenses` into combat AC and load jobs + armors in `main.go`

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (add `jobRegistry` field)
- Modify: `internal/gameserver/combat_handler.go` (`startCombatLocked`)
- Modify: `cmd/gameserver/main.go`

### Step 1: Add `--armors-dir` flag and load armors in `main.go`

In `cmd/gameserver/main.go`, after the existing `weaponsDir` flag:

```go
armorsDir  := flag.String("armors-dir",  "content/armor",  "path to armor YAML definitions directory")
jobsDir    := flag.String("jobs-dir",    "content/jobs",   "path to job YAML definitions directory")
```

After loading weapons and items, add:

```go
// Load armor definitions
armors, err := inventory.LoadArmors(*armorsDir)
if err != nil {
    zap.L().Fatal("failed to load armors", zap.Error(err))
}
for _, a := range armors {
    if err := invRegistry.RegisterArmor(a); err != nil {
        zap.L().Fatal("failed to register armor", zap.String("id", a.ID), zap.Error(err))
    }
}
zap.L().Info("loaded armor definitions", zap.Int("count", len(armors)))

// Load job registry for team affiliation
jobList, err := ruleset.LoadJobs(*jobsDir)
if err != nil {
    zap.L().Fatal("failed to load jobs", zap.Error(err))
}
jobReg := ruleset.NewJobRegistry()
for _, j := range jobList {
    jobReg.Register(j)
}
```

Pass `jobReg` to `NewGameServiceServer` (update signature accordingly).

### Step 2: Add `jobRegistry` to `GameServiceServer` and `CombatHandler`

In `grpc_service.go`, add to `GameServiceServer` struct:

```go
jobRegistry *ruleset.JobRegistry
```

In `NewGameServiceServer`, accept `jobReg *ruleset.JobRegistry` and store it.

In `combat_handler.go`, add to `CombatHandler` struct:

```go
jobRegistry *ruleset.JobRegistry
```

Update `NewCombatHandler` to accept and store it. Pass from `grpc_service.go`.

### Step 3: Replace hardcoded AC=12 in `startCombatLocked`

Find line ~594 in `combat_handler.go`:

```go
// Placeholder defaults: AC/Level/StrMod/DexMod will come from character sheet once Stage 7 (inventory) is complete.
playerCbt := &combat.Combatant{
    ...
    AC:        12,
    DexMod:    1,
```

Replace with:

```go
dexMod := 1 // TODO: derive from character sheet stats when available
defStats := sess.Equipment.ComputedDefenses(h.invRegistry, dexMod)
playerAC := 10 + defStats.ACBonus + defStats.EffectiveDex

playerCbt := &combat.Combatant{
    ID:        sess.UID,
    Kind:      combat.KindPlayer,
    Name:      sess.CharName,
    MaxHP:     sess.CurrentHP,
    CurrentHP: sess.CurrentHP,
    AC:        playerAC,
    Level:     1,
    StrMod:    2,
    DexMod:    dexMod,
}
```

### Step 4: Run all tests

```bash
go test -race $(go list ./... | grep -v postgres)
```
Expected: PASS (the hardcoded AC=12 test in `combat_handler_test.go` may need updating — change expected AC to 10+0+1=11 for no armor + dexMod=1, or update any test that asserts AC==12).

### Step 5: Commit

```bash
git add cmd/gameserver/main.go internal/gameserver/grpc_service.go \
        internal/gameserver/combat_handler.go
git commit -m "feat: wire ComputedDefenses into combat AC, load armors and jobs at startup"
```

---

## Task 9: Weapon content YAML expansion

Add ~32 new weapon YAML files to `content/weapons/` and corresponding item YAMLs to `content/items/`.

**Weapon YAML schema reminder:**
```yaml
id: assault_rifle
name: Assault Rifle
description: Military-surplus carbine chambered in 5.56mm.
damage_dice: "1d8"
damage_type: piercing
range_increment: 60
reload_actions: 1
magazine_capacity: 30
firing_modes: [single, burst, automatic]
traits: [gun-team, military-surplus]
kind: two_handed
group: firearm
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

**Item YAML schema:**
```yaml
id: assault_rifle
name: Assault Rifle
description: Military-surplus carbine.
kind: weapon
weapon_ref: assault_rifle
weight: 3
stackable: false
value: 500
```

### Melee — Machete-team (8 weapons)

Create `content/weapons/vibroblade.yaml`:
```yaml
id: vibroblade
name: Vibroblade
description: A high-frequency mono-molecular blade that hums with barely contained energy.
damage_dice: "1d10"
damage_type: slashing
range_increment: 0
traits: [finesse, machete-team]
kind: one_handed
group: blade
team_affinity: machete
```

Create `content/weapons/mono_wire_whip.yaml`:
```yaml
id: mono_wire_whip
name: Mono-Wire Whip
description: A reel of monomolecular wire that can strip limbs with a flick of the wrist.
damage_dice: "1d12"
damage_type: slashing
range_increment: 0
traits: [reach, machete-team, two-hand]
kind: two_handed
group: blade
team_affinity: machete
cross_team_effect:
  kind: condition
  value: clumsy-1
```

Create `content/weapons/chainsaw.yaml`:
```yaml
id: chainsaw
name: Chainsaw
description: A salvaged power tool repurposed as a devastating close-combat weapon.
damage_dice: "2d6"
damage_type: slashing
range_increment: 0
traits: [machete-team, loud, two-hand]
kind: two_handed
group: blade
team_affinity: machete
```

Create `content/weapons/rebar_club.yaml`:
```yaml
id: rebar_club
name: Rebar Club
description: A length of bent construction rebar, heavy enough to cave in a helmet.
damage_dice: "1d8"
damage_type: bludgeoning
range_increment: 0
traits: [machete-team]
kind: one_handed
group: club
```

Create `content/weapons/tactical_hatchet.yaml`:
```yaml
id: tactical_hatchet
name: Tactical Hatchet
description: A military-surplus hatchet with a weighted head for maximum impact.
damage_dice: "1d6"
damage_type: slashing
range_increment: 0
traits: [machete-team, thrown-10]
kind: one_handed
group: blade
team_affinity: machete
```

Create `content/weapons/ceramic_shiv.yaml`:
```yaml
id: ceramic_shiv
name: Ceramic Shiv
description: A blade fashioned from ceramic tile — undetectable by metal scanners.
damage_dice: "1d4"
damage_type: piercing
range_increment: 0
traits: [machete-team, agile, finesse, concealable]
kind: one_handed
group: blade
team_affinity: machete
```

Create `content/weapons/cleaver.yaml`:
```yaml
id: cleaver
name: Butcher's Cleaver
description: A heavy cleaver with a notched edge that hooks through armor gaps.
damage_dice: "1d8"
damage_type: slashing
range_increment: 0
traits: [machete-team]
kind: one_handed
group: blade
team_affinity: machete
```

Note: `machete` and `suburban_machete` already exist. Update their YAML to add `team_affinity: machete`.

### Melee — Gun-team (4 weapons)

Create `content/weapons/combat_knife.yaml`:
```yaml
id: combat_knife
name: Combat Knife
description: A serrated military-issue blade, standard issue for street soldiers.
damage_dice: "1d4"
damage_type: piercing
range_increment: 0
traits: [gun-team, agile, finesse]
kind: one_handed
group: blade
team_affinity: gun
```

Create `content/weapons/spiked_knuckles.yaml`:
```yaml
id: spiked_knuckles
name: Spiked Knuckles
description: Brass knuckles welded with sharpened bolts for added puncture.
damage_dice: "1d6"
damage_type: piercing
range_increment: 0
traits: [gun-team, agile, unarmed]
kind: one_handed
group: brawling
team_affinity: gun
```

Create `content/weapons/stun_baton.yaml`:
```yaml
id: stun_baton
name: Stun Baton
description: A collapsible electrified baton favored by corporate security.
damage_dice: "1d6"
damage_type: electricity
range_increment: 0
traits: [gun-team, nonlethal]
kind: one_handed
group: club
team_affinity: gun
```

Create `content/weapons/bayonet.yaml`:
```yaml
id: bayonet
name: Bayonet
description: A rifle-mount blade that doubles as a fighting knife.
damage_dice: "1d6"
damage_type: piercing
range_increment: 0
traits: [gun-team, agile]
kind: one_handed
group: blade
team_affinity: gun
```

### Pistols (4 new, ganger_pistol already exists)

Create `content/weapons/holdout_derringer.yaml`:
```yaml
id: holdout_derringer
name: Holdout Derringer
description: A tiny two-shot pistol that fits in a palm — the last line of defense.
damage_dice: "1d4"
damage_type: piercing
range_increment: 20
reload_actions: 2
magazine_capacity: 2
firing_modes: [single]
traits: [gun-team, concealable]
kind: one_handed
group: firearm
team_affinity: gun
```

Create `content/weapons/heavy_revolver.yaml`:
```yaml
id: heavy_revolver
name: Heavy Revolver
description: A six-shot magnum revolver with enough kick to knock a man off his feet.
damage_dice: "1d10"
damage_type: piercing
range_increment: 40
reload_actions: 3
magazine_capacity: 6
firing_modes: [single]
traits: [gun-team, powerful]
kind: one_handed
group: firearm
team_affinity: gun
```

Create `content/weapons/smartgun_pistol.yaml`:
```yaml
id: smartgun_pistol
name: Smartgun
description: A cybernetically-linked pistol with targeting assist — requires neural interface.
damage_dice: "1d6"
damage_type: piercing
range_increment: 40
reload_actions: 1
magazine_capacity: 18
firing_modes: [single]
traits: [gun-team, smart-linked, military-surplus]
kind: one_handed
group: firearm
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

Create `content/weapons/flechette_pistol.yaml`:
```yaml
id: flechette_pistol
name: Flechette Pistol
description: Fires a burst of needle-like darts — devastating at close range.
damage_dice: "1d8"
damage_type: piercing
range_increment: 15
reload_actions: 1
magazine_capacity: 20
firing_modes: [single, burst]
traits: [gun-team, scatter]
kind: one_handed
group: firearm
team_affinity: gun
```

Create `content/weapons/emp_pistol.yaml`:
```yaml
id: emp_pistol
name: EMP Pistol
description: Fires electromagnetic pulses that fry electronics and stagger cyborgs.
damage_dice: "1d6"
damage_type: electricity
range_increment: 20
reload_actions: 2
magazine_capacity: 8
firing_modes: [single]
traits: [gun-team, emp, nonlethal]
kind: one_handed
group: firearm
team_affinity: gun
```

### SMGs (4 weapons)

Create `content/weapons/street_sweeper_smg.yaml`:
```yaml
id: street_sweeper_smg
name: Street Sweeper SMG
description: A crude but reliable submachine gun cobbled together in back-alley workshops.
damage_dice: "1d6"
damage_type: piercing
range_increment: 30
reload_actions: 1
magazine_capacity: 32
firing_modes: [single, automatic]
traits: [gun-team]
kind: two_handed
group: firearm
team_affinity: gun
```

Create `content/weapons/corp_security_smg.yaml`:
```yaml
id: corp_security_smg
name: Corp Security SMG
description: Standard-issue corporate security weapon — compact, reliable, and lethal.
damage_dice: "1d6"
damage_type: piercing
range_increment: 35
reload_actions: 1
magazine_capacity: 30
firing_modes: [single, burst, automatic]
traits: [gun-team, military-surplus]
kind: two_handed
group: firearm
team_affinity: gun
```

Create `content/weapons/suppressed_smg.yaml`:
```yaml
id: suppressed_smg
name: Suppressed SMG
description: An SMG fitted with an integral suppressor — nearly silent at close range.
damage_dice: "1d6"
damage_type: piercing
range_increment: 25
reload_actions: 1
magazine_capacity: 30
firing_modes: [single, automatic]
traits: [gun-team, suppressed, concealable]
kind: two_handed
group: firearm
team_affinity: gun
```

Create `content/weapons/cyberlinked_smg.yaml`:
```yaml
id: cyberlinked_smg
name: Cyber-Linked SMG
description: A neural-interfaced SMG that tracks targets through the user's own vision.
damage_dice: "1d8"
damage_type: piercing
range_increment: 40
reload_actions: 1
magazine_capacity: 25
firing_modes: [single, burst, automatic]
traits: [gun-team, smart-linked, military-surplus]
kind: two_handed
group: firearm
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

### Shotguns (2 new, combat_shotgun already exists)

Create `content/weapons/sawn_off.yaml`:
```yaml
id: sawn_off
name: Sawn-Off Shotgun
description: A double-barrel shotgun with the barrels hacked short — brutal at point-blank.
damage_dice: "2d6"
damage_type: piercing
range_increment: 10
reload_actions: 2
magazine_capacity: 2
firing_modes: [single]
traits: [gun-team, scatter, concealable]
kind: one_handed
group: firearm
team_affinity: gun
```

Create `content/weapons/riot_shotgun.yaml`:
```yaml
id: riot_shotgun
name: Riot Shotgun
description: A semi-automatic shotgun with an extended tube magazine used by enforcement units.
damage_dice: "1d10"
damage_type: piercing
range_increment: 20
reload_actions: 1
magazine_capacity: 8
firing_modes: [single]
traits: [gun-team, military-surplus]
kind: two_handed
group: firearm
team_affinity: gun
```

Create `content/weapons/flechette_spreader.yaml`:
```yaml
id: flechette_spreader
name: Flechette Spreader
description: A wide-bore shotgun firing clouds of nano-shards — ignores most light armor.
damage_dice: "1d10"
damage_type: piercing
range_increment: 15
reload_actions: 2
magazine_capacity: 6
firing_modes: [single]
traits: [gun-team, scatter, armor-piercing]
kind: two_handed
group: firearm
team_affinity: gun
```

### Rifles (6 weapons)

Create `content/weapons/assault_rifle.yaml`, `content/weapons/sniper_rifle.yaml`, `content/weapons/battle_rifle.yaml`, `content/weapons/laser_rifle.yaml`, `content/weapons/railgun_carbine.yaml`, `content/weapons/anti_materiel_rifle.yaml`:

```yaml
# assault_rifle.yaml
id: assault_rifle
name: Assault Rifle
description: Military-surplus carbine — the backbone of any well-armed street crew.
damage_dice: "1d8"
damage_type: piercing
range_increment: 60
reload_actions: 1
magazine_capacity: 30
firing_modes: [single, burst, automatic]
traits: [gun-team, military-surplus]
kind: two_handed
group: firearm
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

```yaml
# sniper_rifle.yaml
id: sniper_rifle
name: Sniper Rifle
description: A long-barreled precision rifle for taking shots from rooftops and shadows.
damage_dice: "1d12"
damage_type: piercing
range_increment: 200
reload_actions: 2
magazine_capacity: 5
firing_modes: [single]
traits: [gun-team, military-surplus, two-hand, precise]
kind: two_handed
group: firearm
team_affinity: gun
```

```yaml
# battle_rifle.yaml
id: battle_rifle
name: Battle Rifle
description: A full-power semi-automatic rifle chambered in a hard-hitting caliber.
damage_dice: "1d10"
damage_type: piercing
range_increment: 80
reload_actions: 1
magazine_capacity: 20
firing_modes: [single, burst]
traits: [gun-team, military-surplus]
kind: two_handed
group: firearm
team_affinity: gun
```

```yaml
# laser_rifle.yaml
id: laser_rifle
name: Laser Rifle
description: A corporate-tech directed-energy weapon — silent, precise, and deadly.
damage_dice: "2d6"
damage_type: fire
range_increment: 100
reload_actions: 3
magazine_capacity: 20
firing_modes: [single]
traits: [gun-team, corp-tech, silent]
kind: two_handed
group: energy
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

```yaml
# railgun_carbine.yaml
id: railgun_carbine
name: Railgun Carbine
description: A compact electromagnetic accelerator that punches through vehicle armor.
damage_dice: "2d8"
damage_type: piercing
range_increment: 100
reload_actions: 3
magazine_capacity: 10
firing_modes: [single]
traits: [gun-team, corp-tech, armor-piercing, military-surplus]
kind: two_handed
group: energy
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

```yaml
# anti_materiel_rifle.yaml
id: anti_materiel_rifle
name: Anti-Materiel Rifle
description: A massive bolt-action rifle designed to destroy vehicles — and anything else.
damage_dice: "3d8"
damage_type: piercing
range_increment: 250
reload_actions: 4
magazine_capacity: 5
firing_modes: [single]
traits: [gun-team, military-surplus, heavy, armor-piercing]
kind: two_handed
group: firearm
team_affinity: gun
```

### New Explosives (2 new, frag_grenade + pipe_bomb already exist)

Create `content/explosives/incendiary_grenade.yaml` and `content/explosives/emp_grenade.yaml` following the existing explosive YAML schema.

### Exotic Weapons (4 weapons)

Create `content/weapons/flamethrower.yaml`, `content/weapons/net_launcher.yaml`, `content/weapons/sonic_disruptor.yaml`, `content/weapons/grapple_gun.yaml` following the established pattern.

### Item YAML files for all new weapons

For each new weapon YAML, create a matching `content/items/<id>.yaml`:

```yaml
# Example: content/items/assault_rifle.yaml
id: assault_rifle
name: Assault Rifle
description: Military-surplus carbine.
kind: weapon
weapon_ref: assault_rifle
weight: 3
stackable: false
value: 500
```

Create item files for all ~32 new weapons. Weight and value guidelines:
- Light melee (shiv, knife): weight 1, value 50-150
- Standard melee: weight 2, value 100-300
- Heavy melee (chainsaw, vibroblade): weight 3, value 300-800
- Pistols: weight 1, value 200-600
- SMGs: weight 2, value 400-800
- Shotguns: weight 3, value 300-600
- Rifles: weight 3-4, value 500-2000
- Exotic: weight 4-5, value 800-3000

### Step: Update existing weapon YAMLs with team_affinity

Open these existing files and add team affinity:
- `content/weapons/cheap_blade.yaml` — add `team_affinity: machete`
- `content/weapons/suburban_machete.yaml` — add `team_affinity: machete`
- `content/weapons/harvest_sickle.yaml` — add `team_affinity: machete`
- `content/weapons/dock_hook.yaml` — no affinity (neutral)
- `content/weapons/steel_pipe.yaml` — no affinity (neutral)
- `content/weapons/golf_club.yaml` — no affinity (neutral)
- `content/weapons/ganger_pistol.yaml` — add `team_affinity: gun`
- `content/weapons/combat_shotgun.yaml` — add `team_affinity: gun`

### Step: Run tests to verify all content loads

```bash
go test -race ./internal/game/inventory/ -run TestLoadWeapons
go test -race ./... | grep -v postgres
```

### Step: Commit

```bash
git add content/weapons/ content/items/ content/explosives/
git commit -m "content: add comprehensive weapon library (~40 weapons) with team affinity"
```

---

## Task 10: Armor content YAML files

Create `content/armor/` directory and ~35 armor YAML files. Create corresponding `content/items/` entries.

**Armor YAML schema:**
```yaml
id: tactical_vest
name: Tactical Vest
description: ...
slot: torso
ac_bonus: 3
dex_cap: 2
check_penalty: -1
speed_penalty: 0
strength_req: 14
bulk: 2
group: composite
traits: [gun-team]
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

### Head Slot (5 items)

```yaml
# ballistic_cap.yaml
id: ballistic_cap
name: Ballistic Cap
description: A reinforced baseball cap with kevlar lining — minimal protection, maximum stealth.
slot: head
ac_bonus: 1
dex_cap: 5
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 0
group: leather
```

```yaml
# combat_helmet.yaml
id: combat_helmet
name: Combat Helmet
description: A composite-shell helmet with chin strap and brow guard.
slot: head
ac_bonus: 2
dex_cap: 3
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 1
group: composite
traits: [gun-team]
team_affinity: gun
```

```yaml
# riot_visor.yaml
id: riot_visor
name: Riot Visor
description: Full-face ballistic visor used by corporate enforcement units.
slot: head
ac_bonus: 2
dex_cap: 2
check_penalty: -1
speed_penalty: 0
strength_req: 10
bulk: 1
group: composite
traits: [gun-team, military-surplus]
team_affinity: gun
```

```yaml
# corp_security_helm.yaml
id: corp_security_helm
name: Corp Security Helm
description: A sleek military-grade helmet with integrated HUD mount points.
slot: head
ac_bonus: 3
dex_cap: 2
check_penalty: -1
speed_penalty: 0
strength_req: 12
bulk: 1
group: plate
traits: [gun-team, military-surplus]
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

```yaml
# neural_interface_helm.yaml
id: neural_interface_helm
name: Neural Interface Helm
description: A helmet with direct neural interface ports for smart-linked weapons.
slot: head
ac_bonus: 2
dex_cap: 3
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 1
group: composite
traits: [gun-team, corp-tech, smart-linked]
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

### Torso Slot (6 items)

```yaml
# leather_jacket.yaml
id: leather_jacket
name: Leather Jacket
description: A studded leather jacket — light protection with maximum street cred.
slot: torso
ac_bonus: 1
dex_cap: 5
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 1
group: leather
```

```yaml
# tactical_vest.yaml
id: tactical_vest
name: Tactical Vest
description: Salvaged military-grade composite plating over a kevlar underlayer.
slot: torso
ac_bonus: 3
dex_cap: 2
check_penalty: -1
speed_penalty: 0
strength_req: 14
bulk: 2
group: composite
traits: [gun-team]
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

```yaml
# kevlar_vest.yaml
id: kevlar_vest
name: Kevlar Vest
description: A soft-armor vest providing reliable protection against small arms.
slot: torso
ac_bonus: 2
dex_cap: 3
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 1
group: composite
```

```yaml
# corp_suit_liner.yaml
id: corp_suit_liner
name: Corp Suit Liner
description: Ballistic mesh woven into a business suit — protection without the profile.
slot: torso
ac_bonus: 1
dex_cap: 5
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 1
group: chain
traits: [concealable]
```

```yaml
# military_plate.yaml
id: military_plate
name: Military Plate
description: Full front-and-back ballistic plate carrier — heavy but nearly impenetrable.
slot: torso
ac_bonus: 5
dex_cap: 1
check_penalty: -2
speed_penalty: 5
strength_req: 16
bulk: 3
group: plate
traits: [gun-team, military-surplus]
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

```yaml
# exo_frame_torso.yaml
id: exo_frame_torso
name: Exo-Frame Torso
description: A powered exoskeletal chest piece — corp-tech in the wrong hands.
slot: torso
ac_bonus: 6
dex_cap: 0
check_penalty: -3
speed_penalty: 0
strength_req: 0
bulk: 4
group: plate
traits: [gun-team, corp-tech, powered]
team_affinity: gun
cross_team_effect:
  kind: condition
  value: clumsy-1
```

### Left/Right Arm (4 items each, same definitions different slot)

Create `content/armor/left_arm_guards.yaml`, `content/armor/right_arm_guards.yaml`, etc.:

```yaml
# left_arm_guards.yaml
id: left_arm_guards
name: Left Arm Guards
description: Padded forearm guards salvaged from riot gear.
slot: left_arm
ac_bonus: 1
dex_cap: 4
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 1
group: leather
```

Repeat for `right_arm_guards.yaml` with `slot: right_arm`. Create equivalent pairs for:
- `tactical_vambrace` (left/right) — composite, ac_bonus: 2, gun-team
- `ballistic_sleeve` (left/right) — chain, ac_bonus: 1, concealable
- `exo_frame_arm` (left/right) — plate, ac_bonus: 3, corp-tech, gun-team, cross_team_effect: clumsy-1

### Hands (4 items)

```yaml
# fingerless_gloves.yaml
id: fingerless_gloves
name: Fingerless Gloves
description: Light leather gloves with reinforced knuckles.
slot: hands
ac_bonus: 0
dex_cap: 5
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 0
group: leather
```

```yaml
# tactical_gloves.yaml
id: tactical_gloves
name: Tactical Gloves
description: Full-finger gloves with carbon-fiber knuckle plates.
slot: hands
ac_bonus: 1
dex_cap: 4
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 0
group: leather
```

```yaml
# armored_gauntlets.yaml
id: armored_gauntlets
name: Armored Gauntlets
description: Heavy composite gauntlets with steel-backed knuckles.
slot: hands
ac_bonus: 2
dex_cap: 2
check_penalty: -1
speed_penalty: 0
strength_req: 12
bulk: 1
group: plate
```

```yaml
# shock_gloves.yaml
id: shock_gloves
name: Shock Gloves
description: Insulated gloves with integrated shock coils — melee hits electrify.
slot: hands
ac_bonus: 1
dex_cap: 3
check_penalty: 0
speed_penalty: 0
strength_req: 0
bulk: 1
group: composite
traits: [gun-team, corp-tech]
team_affinity: gun
```

### Left/Right Leg (4 items each)

Create pairs for:
- `leg_guards` (left/right) — leather, ac_bonus: 1
- `tactical_greaves` (left/right) — composite, ac_bonus: 2, gun-team
- `ballistic_leggings` (left/right) — chain, ac_bonus: 1
- `exo_frame_leg` (left/right) — plate, ac_bonus: 3, corp-tech, gun-team

### Feet (4 items)

```yaml
# street_boots.yaml — leather, ac_bonus: 0, just flavor
# tactical_boots.yaml — composite, ac_bonus: 1
# mag_boots.yaml — corp-tech, ac_bonus: 1, special trait
# exo_frame_feet.yaml — plate, ac_bonus: 2, corp-tech
```

### Item YAML files for all armor

For each armor, create `content/items/<id>.yaml`:

```yaml
# content/items/tactical_vest.yaml
id: tactical_vest
name: Tactical Vest
description: Salvaged military-grade composite plating.
kind: armor
armor_ref: tactical_vest
weight: 2
stackable: false
value: 400
```

### Step: Run tests

```bash
go test -race $(go list ./... | grep -v postgres)
```

### Step: Commit

```bash
git add content/armor/ content/items/
git commit -m "content: add comprehensive armor library (~35 pieces) with team affinity"
```

---

## Task 11: Content completeness property test

**Files:**
- Create: `internal/game/inventory/content_test.go`

This test loads all content from disk and verifies structural integrity.

### Step 1: Write the test

```go
// internal/game/inventory/content_test.go
package inventory_test

import (
	"os"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestContent_AllArmorSlotsValid verifies every armor YAML has a valid slot value.
func TestContent_AllArmorSlotsValid(t *testing.T) {
	armors, err := inventory.LoadArmors("../../../content/armor")
	require.NoError(t, err, "content/armor should load without error")
	require.NotEmpty(t, armors, "at least one armor should exist")

	validSlots := inventory.ValidArmorSlots()
	for _, a := range armors {
		_, ok := validSlots[a.Slot]
		assert.True(t, ok, "armor %q has invalid slot %q", a.ID, a.Slot)
	}
}

// TestContent_AllWeaponArmorRefsResolve verifies every item YAML with armor_ref or weapon_ref
// references a def that can be loaded.
func TestContent_AllWeaponArmorRefsResolve(t *testing.T) {
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
	for _, item := range items {
		switch item.Kind {
		case "weapon":
			if item.WeaponRef != "" {
				_, ok := reg.Weapon(item.WeaponRef)
				assert.True(t, ok, "item %q references unknown weapon_ref %q", item.ID, item.WeaponRef)
			}
		case "armor":
			if item.ArmorRef != "" {
				_, ok := reg.Armor(item.ArmorRef)
				assert.True(t, ok, "item %q references unknown armor_ref %q", item.ID, item.ArmorRef)
			}
		}
	}
}

// TestContent_AllArmorCrossTeamEffectsHaveValidKind verifies cross_team_effect.kind is
// always "condition" or "penalty".
func TestContent_AllArmorCrossTeamEffectsHaveValidKind(t *testing.T) {
	armors, err := inventory.LoadArmors("../../../content/armor")
	require.NoError(t, err)
	for _, a := range armors {
		if a.CrossTeamEffect != nil {
			assert.Contains(t, []string{"condition", "penalty"}, a.CrossTeamEffect.Kind,
				"armor %q cross_team_effect.kind must be condition or penalty", a.ID)
			assert.NotEmpty(t, a.CrossTeamEffect.Value,
				"armor %q cross_team_effect.value must not be empty", a.ID)
		}
	}
}

// TestContent_AllWeaponCrossTeamEffectsHaveValidKind mirrors the armor test for weapons.
func TestContent_AllWeaponCrossTeamEffectsHaveValidKind(t *testing.T) {
	weapons, err := inventory.LoadWeapons("../../../content/weapons")
	require.NoError(t, err)
	for _, w := range weapons {
		if w.CrossTeamEffect != nil {
			assert.Contains(t, []string{"condition", "penalty"}, w.CrossTeamEffect.Kind,
				"weapon %q cross_team_effect.kind must be condition or penalty", w.ID)
		}
	}
}

// TestProperty_ArmorACBonus_NonNegative verifies ac_bonus is always >= 0 in loaded content.
func TestProperty_ArmorACBonus_NonNegative(t *testing.T) {
	armors, err := inventory.LoadArmors("../../../content/armor")
	require.NoError(t, err)

	rapid.Check(t, func(rt *rapid.T) {
		if len(armors) == 0 {
			return
		}
		a := rapid.SampledFrom(armors).Draw(rt, "armor")
		assert.GreaterOrEqual(t, a.ACBonus, 0)
	})
}
```

### Step 2: Run test to verify it passes (content must be in place from Task 9+10)

```bash
go test -race ./internal/game/inventory/ -run TestContent_
```
Expected: PASS

### Step 3: Commit

```bash
git add internal/game/inventory/content_test.go
git commit -m "test: add content completeness property tests for armor and weapon library"
```

---

## Final Verification

```bash
# All tests, race detector
go test -race $(go list ./... | grep -v postgres)
```

Expected: all PASS, 0 failures.

Then use `superpowers:finishing-a-development-branch` to merge and deploy.
