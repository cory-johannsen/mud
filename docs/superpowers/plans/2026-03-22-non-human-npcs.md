# Non-Human NPCs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add animal, robot, and machine NPC types with unique attack verbs, organic/salvage loot drop tables, immobile flag, and appropriate damage type constants.

**Architecture:** Extend NPC Template with `AttackVerb`/`Immobile` fields and `IsAnimal()`/`IsRobot()`/`IsMachine()` helpers; add `OrganicDrops`/`SalvageDrop`/`Equipment` fields to `LootTable`; add `AttackVerb` to `combat.Combatant`; update `attackNarrative` callers in `round.go` to use `actor.AttackVerb`; apply bleed/poison resistance defaults for robot/machine at spawn; enforce immobility in `npcPatrolRandom`; filter `say` HTN tasks for animals; emit `"<Name> is destroyed."` for machine death; add `internal/game/combat/damage_types.go` constants.

**Tech Stack:** Go, YAML content, existing NPC/combat packages

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `internal/game/combat/damage_types.go` | **Create** | Package-level damage type string constants (REQ-NHN-28) |
| `internal/game/combat/damage_types_test.go` | **Create** | Tests for constant values |
| `internal/game/npc/loot.go` | **Modify** | Add `OrganicDrop`, `SalvageDrop`, `Equipment` fields + validation (REQ-NHN-9,11,17) |
| `internal/game/npc/loot_test.go` | **Modify** | Tests for new loot structs + validation |
| `internal/game/npc/template.go` | **Modify** | Add `AttackVerb`, `Immobile` fields; add `IsAnimal()`, `IsRobot()`, `IsMachine()`; add animal loot validation (REQ-NHN-2–5,11,20) |
| `internal/game/npc/template_test.go` | **Modify** | Tests for new fields and helpers |
| `internal/game/npc/instance.go` | **Modify** | Add `AttackVerb`, `Immobile` fields; propagate at spawn; apply robot/machine resistance defaults (REQ-NHN-5,14,15,21,26) |
| `internal/game/npc/instance_test.go` | **Modify** | Tests for spawn propagation and resistance defaults |
| `internal/game/combat/combat.go` | **Modify** | Add `AttackVerb string` field to `Combatant` (REQ-NHN-6) |
| `internal/game/combat/combat_test.go` | **Modify** | Test that AttackVerb zero value is `""` |
| `internal/game/combat/round.go` | **Modify** | Pass `actor.AttackVerb` (defaulting to `"attacks"`/`"strikes"`) to all three `attackNarrative` calls (REQ-NHN-7) |
| `internal/game/combat/round_test.go` | **Modify** | Tests verifying custom attack verb appears in narrative |
| `internal/gameserver/combat_handler.go` | **Modify** | Set `AttackVerb` when building NPC `Combatant`; player `Combatant` uses default `"attacks"`; update `removeDeadNPCsLocked` for organic/salvage loot and machine death message (REQ-NHN-6,7,10,18,25) |
| `internal/gameserver/grpc_service.go` | **Modify** | Skip immobile instances in `npcPatrolRandom`; filter `say` tasks from animal plan in `tickNPCIdle` (REQ-NHN-13,22) |
| `content/npcs/feral_dog.yaml` | **Create** | Example animal NPC (REQ-NHN-8 content convention) |
| `content/npcs/security_drone.yaml` | **Create** | Example robot NPC (REQ-NHN-19 content convention) |
| `content/npcs/auto_turret.yaml` | **Create** | Example machine NPC (REQ-NHN-23,24 content conventions) |

---

## Task 1: Damage Type Constants

**Files:**
- Create: `internal/game/combat/damage_types.go`
- Create: `internal/game/combat/damage_types_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/game/combat/damage_types_test.go
package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func TestDamageTypeConstants(t *testing.T) {
	if combat.DamageTypeBleed != "bleed" {
		t.Errorf("DamageTypeBleed = %q, want %q", combat.DamageTypeBleed, "bleed")
	}
	if combat.DamageTypePoison != "poison" {
		t.Errorf("DamageTypePoison = %q, want %q", combat.DamageTypePoison, "poison")
	}
	if combat.DamageTypeElectric != "electric" {
		t.Errorf("DamageTypeElectric = %q, want %q", combat.DamageTypeElectric, "electric")
	}
	if combat.DamageTypePhysical != "physical" {
		t.Errorf("DamageTypePhysical = %q, want %q", combat.DamageTypePhysical, "physical")
	}
	if combat.DamageTypeFire != "fire" {
		t.Errorf("DamageTypeFire = %q, want %q", combat.DamageTypeFire, "fire")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestDamageTypeConstants -v
```

Expected: compile error — `combat.DamageTypeBleed` undefined.

- [ ] **Step 3: Write the implementation**

```go
// internal/game/combat/damage_types.go
package combat

// Canonical damage type strings used throughout the game.
// All internal code MUST reference these constants instead of string literals (REQ-NHN-29).
const (
	DamageTypeBleed    = "bleed"
	DamageTypePoison   = "poison"
	DamageTypeElectric = "electric"
	DamageTypePhysical = "physical"
	DamageTypeFire     = "fire"
)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestDamageTypeConstants -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/damage_types.go internal/game/combat/damage_types_test.go
git commit -m "feat(combat): add damage type constants (REQ-NHN-28,29)"
```

---

## Task 2: Extend LootTable — OrganicDrops, SalvageDrop, Equipment

**Files:**
- Modify: `internal/game/npc/loot.go`
- Modify: `internal/game/npc/loot_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/npc/loot_test.go`:

```go
package npc_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc"
)

// --- OrganicDrop validation ---

func TestOrganicDropValidation_WeightZero(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "dog_meat", Weight: 0, QuantityMin: 1, QuantityMax: 2},
		},
	}
	if err := lt.Validate(); err == nil {
		t.Fatal("expected error for weight=0, got nil")
	}
}

func TestOrganicDropValidation_QuantityMinZero(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "dog_meat", Weight: 10, QuantityMin: 0, QuantityMax: 1},
		},
	}
	if err := lt.Validate(); err == nil {
		t.Fatal("expected error for quantity_min=0, got nil")
	}
}

func TestOrganicDropValidation_MaxLessThanMin(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "dog_meat", Weight: 10, QuantityMin: 3, QuantityMax: 2},
		},
	}
	if err := lt.Validate(); err == nil {
		t.Fatal("expected error for quantity_max < quantity_min, got nil")
	}
}

func TestOrganicDropValidation_Valid(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "dog_meat", Weight: 10, QuantityMin: 1, QuantityMax: 2},
		},
	}
	if err := lt.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// --- SalvageDrop validation ---

func TestSalvageDropValidation_EmptyItemIDs(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs: []string{},
			QuantityMin: 1,
			QuantityMax: 1,
		},
	}
	if err := lt.Validate(); err == nil {
		t.Fatal("expected error for empty item_ids, got nil")
	}
}

func TestSalvageDropValidation_QuantityMinZero(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs: []string{"circuit_board"},
			QuantityMin: 0,
			QuantityMax: 1,
		},
	}
	if err := lt.Validate(); err == nil {
		t.Fatal("expected error for quantity_min=0, got nil")
	}
}

func TestSalvageDropValidation_MaxLessThanMin(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs: []string{"circuit_board"},
			QuantityMin: 3,
			QuantityMax: 2,
		},
	}
	if err := lt.Validate(); err == nil {
		t.Fatal("expected error for quantity_max < quantity_min, got nil")
	}
}

func TestSalvageDropValidation_Valid(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs: []string{"circuit_board", "power_cell"},
			QuantityMin: 1,
			QuantityMax: 2,
		},
	}
	if err := lt.Validate(); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// --- Property-based test: valid OrganicDrop always passes Validate ---

func TestOrganicDropValidation_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		weight := rapid.IntRange(1, 100).Draw(t, "weight")
		qmin := rapid.IntRange(1, 50).Draw(t, "qmin")
		qmax := rapid.IntRange(qmin, qmin+50).Draw(t, "qmax")
		lt := npc.LootTable{
			OrganicDrops: []npc.OrganicDrop{
				{ItemID: "x", Weight: weight, QuantityMin: qmin, QuantityMax: qmax},
			},
		}
		if err := lt.Validate(); err != nil {
			t.Fatalf("valid OrganicDrop rejected: %v", err)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestOrganicDrop|TestSalvageDrop" -v
```

Expected: compile errors — `npc.OrganicDrop`, `npc.SalvageDrop` undefined.

- [ ] **Step 3: Implement new types in loot.go**

In `internal/game/npc/loot.go`, add the following structs and extend `LootTable` and `Validate()`:

After the existing `ItemDrop` struct, add:

```go
// OrganicDrop defines a weighted random organic loot entry for animal NPCs.
type OrganicDrop struct {
	ItemID      string `yaml:"item_id"`
	Weight      int    `yaml:"weight"`
	QuantityMin int    `yaml:"quantity_min"`
	QuantityMax int    `yaml:"quantity_max"`
}

// SalvageDrop defines a uniform random salvage loot entry for robot NPCs.
type SalvageDrop struct {
	ItemIDs     []string `yaml:"item_ids"`
	QuantityMin int      `yaml:"quantity_min"`
	QuantityMax int      `yaml:"quantity_max"`
}
```

Add the new fields to `LootTable`:

```go
// LootTable defines the possible loot drops for an NPC template.
type LootTable struct {
	Currency     *CurrencyDrop  `yaml:"currency"`
	Items        []ItemDrop     `yaml:"items"`
	// OrganicDrops is a weighted random organic loot table for animal NPCs.
	// Validated and rolled differently from Items (weighted, not chance-based).
	OrganicDrops []OrganicDrop  `yaml:"organic_drops"`
	// SalvageDrop is a uniform random salvage loot table for robot NPCs.
	// nil means no salvage drop.
	SalvageDrop  *SalvageDrop   `yaml:"salvage_drop"`
	// Equipment is a weighted random equipment loot table.
	// Used by machine templates for component/item drops.
	Equipment    []EquipmentEntry `yaml:"equipment"`
}
```

Note: `EquipmentEntry` is already defined in `template.go` (fields `ID string`, `Weight int`).

Extend `Validate()` in `loot.go` — after the existing items loop, add:

```go
	for i, od := range lt.OrganicDrops {
		if od.Weight <= 0 {
			return fmt.Errorf("loot table: organic_drops[%d] weight must be > 0, got %d", i, od.Weight)
		}
		if od.QuantityMin < 1 {
			return fmt.Errorf("loot table: organic_drops[%d] quantity_min must be >= 1, got %d", i, od.QuantityMin)
		}
		if od.QuantityMax < od.QuantityMin {
			return fmt.Errorf("loot table: organic_drops[%d] quantity_max (%d) must be >= quantity_min (%d)", i, od.QuantityMax, od.QuantityMin)
		}
	}
	if lt.SalvageDrop != nil {
		sd := lt.SalvageDrop
		if len(sd.ItemIDs) == 0 {
			return fmt.Errorf("loot table: salvage_drop item_ids must not be empty")
		}
		if sd.QuantityMin < 1 {
			return fmt.Errorf("loot table: salvage_drop quantity_min must be >= 1, got %d", sd.QuantityMin)
		}
		if sd.QuantityMax < sd.QuantityMin {
			return fmt.Errorf("loot table: salvage_drop quantity_max (%d) must be >= quantity_min (%d)", sd.QuantityMax, sd.QuantityMin)
		}
	}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestOrganicDrop|TestSalvageDrop" -v
```

Expected: all PASS.

- [ ] **Step 5: Run full NPC test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/npc/loot.go internal/game/npc/loot_test.go
git commit -m "feat(npc): add OrganicDrops, SalvageDrop, Equipment to LootTable (REQ-NHN-9,17)"
```

---

## Task 3: Template Helpers and New Fields

**Files:**
- Modify: `internal/game/npc/template.go`
- Modify: `internal/game/npc/template_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/npc/template_test.go`:

```go
// TestIsAnimalRobotMachineHelpers verifies type helpers.
func TestIsAnimalRobotMachineHelpers(t *testing.T) {
	cases := []struct {
		typ      string
		animal   bool
		robot    bool
		machine  bool
	}{
		{"animal", true, false, false},
		{"robot", false, true, false},
		{"machine", false, false, true},
		{"human", false, false, false},
		{"mutant", false, false, false},
		{"", false, false, false},
		{"alien", false, false, false},
	}
	for _, tc := range cases {
		tmpl := &npc.Template{Type: tc.typ}
		if got := tmpl.IsAnimal(); got != tc.animal {
			t.Errorf("type=%q IsAnimal() = %v, want %v", tc.typ, got, tc.animal)
		}
		if got := tmpl.IsRobot(); got != tc.robot {
			t.Errorf("type=%q IsRobot() = %v, want %v", tc.typ, got, tc.robot)
		}
		if got := tmpl.IsMachine(); got != tc.machine {
			t.Errorf("type=%q IsMachine() = %v, want %v", tc.typ, got, tc.machine)
		}
	}
}

// TestAttackVerbField verifies YAML round-trip for attack_verb.
func TestAttackVerbField(t *testing.T) {
	yaml := `
id: test_npc
name: Test NPC
level: 1
max_hp: 10
ac: 10
attack_verb: bites
`
	tmpl, err := npc.LoadTemplateFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.AttackVerb != "bites" {
		t.Errorf("AttackVerb = %q, want %q", tmpl.AttackVerb, "bites")
	}
}

// TestImmobileField verifies YAML round-trip for immobile.
func TestImmobileField(t *testing.T) {
	yaml := `
id: test_turret
name: Test Turret
level: 1
max_hp: 10
ac: 10
immobile: true
`
	tmpl, err := npc.LoadTemplateFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tmpl.Immobile {
		t.Errorf("Immobile = false, want true")
	}
}

// TestAnimalLootValidation_RejectsCredits verifies animals cannot have credits.
func TestAnimalLootValidation_RejectsCredits(t *testing.T) {
	raw := `
id: feral_rat
name: Feral Rat
type: animal
level: 1
max_hp: 5
ac: 10
loot:
  currency:
    min: 5
    max: 10
`
	_, err := npc.LoadTemplateFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected validation error for animal with currency loot, got nil")
	}
}

// TestAnimalLootValidation_RejectsItems verifies animals cannot have items.
func TestAnimalLootValidation_RejectsItems(t *testing.T) {
	raw := `
id: feral_rat
name: Feral Rat
type: animal
level: 1
max_hp: 5
ac: 10
loot:
  items:
    - item: pistol
      chance: 0.1
      min_qty: 1
      max_qty: 1
`
	_, err := npc.LoadTemplateFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected validation error for animal with equipment loot, got nil")
	}
}

// TestAnimalLootValidation_RejectsSalvageDrop verifies animals cannot have salvage_drop.
func TestAnimalLootValidation_RejectsSalvageDrop(t *testing.T) {
	raw := `
id: feral_rat
name: Feral Rat
type: animal
level: 1
max_hp: 5
ac: 10
loot:
  salvage_drop:
    item_ids:
      - circuit_board
    quantity_min: 1
    quantity_max: 1
`
	_, err := npc.LoadTemplateFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected validation error for animal with salvage_drop, got nil")
	}
}

// TestUnknownTypeNotRejected verifies Validate does not reject unknown types (REQ-NHN-2).
func TestUnknownTypeNotRejected(t *testing.T) {
	raw := `
id: alien_creature
name: Alien
type: xenomorph
level: 1
max_hp: 10
ac: 10
`
	_, err := npc.LoadTemplateFromBytes([]byte(raw))
	if err != nil {
		t.Fatalf("unknown type should not be rejected; got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestIsAnimalRobot|TestAttackVerb|TestImmobile|TestAnimalLoot|TestUnknownType" -v
```

Expected: compile errors — `IsAnimal`, `IsRobot`, `IsMachine`, `AttackVerb`, `Immobile` undefined.

- [ ] **Step 3: Add fields and helpers to template.go**

In `internal/game/npc/template.go`, add the following fields to the `Template` struct (after the existing `Disposition` field, before the `NPCType` field):

```go
	// AttackVerb is the verb used in attack narratives (e.g. "bites", "shoots").
	// Empty string defaults to "attacks" at use time.
	AttackVerb string `yaml:"attack_verb"`

	// Immobile prevents this NPC from being moved by the wander/patrol system.
	// Machine templates MUST set this to true.
	Immobile bool `yaml:"immobile"`
```

Add three methods after the `Validate` method:

```go
// IsAnimal reports whether this template represents an animal NPC.
// Precondition: none.
// Postcondition: returns true iff t.Type == "animal".
func (t *Template) IsAnimal() bool { return t.Type == "animal" }

// IsRobot reports whether this template represents a robot NPC.
// Precondition: none.
// Postcondition: returns true iff t.Type == "robot".
func (t *Template) IsRobot() bool { return t.Type == "robot" }

// IsMachine reports whether this template represents a machine NPC.
// Precondition: none.
// Postcondition: returns true iff t.Type == "machine".
func (t *Template) IsMachine() bool { return t.Type == "machine" }
```

In `Validate()`, after the existing `t.Loot != nil` block, add animal loot enforcement:

```go
	// REQ-NHN-11: animal templates must not have credits, equipment, or salvage_drop.
	if t.IsAnimal() && t.Loot != nil {
		if t.Loot.Currency != nil {
			return fmt.Errorf("npc template %q: animal type must not have currency loot", t.ID)
		}
		if len(t.Loot.Items) > 0 {
			return fmt.Errorf("npc template %q: animal type must not have equipment (items) loot", t.ID)
		}
		if len(t.Loot.Equipment) > 0 {
			return fmt.Errorf("npc template %q: animal type must not have equipment loot", t.ID)
		}
		if t.Loot.SalvageDrop != nil {
			return fmt.Errorf("npc template %q: animal type must not have salvage_drop loot", t.ID)
		}
	}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestIsAnimalRobot|TestAttackVerb|TestImmobile|TestAnimalLoot|TestUnknownType" -v
```

Expected: all PASS.

- [ ] **Step 5: Run full NPC test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/npc/template.go internal/game/npc/template_test.go
git commit -m "feat(npc): add AttackVerb, Immobile fields and IsAnimal/IsRobot/IsMachine helpers (REQ-NHN-2,3,4,11,20)"
```

---

## Task 4: Instance Spawn Propagation and Robot/Machine Resistance Defaults

**Files:**
- Modify: `internal/game/npc/instance.go`
- Modify: `internal/game/npc/instance_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/npc/instance_test.go`:

```go
import "github.com/cory-johannsen/mud/internal/game/combat"

// TestSpawnPropagatesAttackVerb verifies AttackVerb is copied from template.
func TestSpawnPropagatesAttackVerb(t *testing.T) {
	tmpl := &npc.Template{
		ID: "feral_dog", Name: "Feral Dog", Type: "animal",
		Level: 1, MaxHP: 10, AC: 10,
		AttackVerb: "bites",
	}
	_ = tmpl.Validate()
	inst := npc.NewInstance("i1", tmpl, "room1")
	if inst.AttackVerb != "bites" {
		t.Errorf("AttackVerb = %q, want %q", inst.AttackVerb, "bites")
	}
}

// TestSpawnPropagatesImmobile verifies Immobile is copied from template.
func TestSpawnPropagatesImmobile(t *testing.T) {
	tmpl := &npc.Template{
		ID: "turret", Name: "Turret", Type: "machine",
		Level: 1, MaxHP: 10, AC: 10,
		Immobile: true,
	}
	_ = tmpl.Validate()
	inst := npc.NewInstance("i2", tmpl, "room1")
	if !inst.Immobile {
		t.Error("Immobile = false, want true")
	}
}

// TestRobotSpawnResistanceDefaults verifies bleed/poison defaults of 999.
func TestRobotSpawnResistanceDefaults(t *testing.T) {
	tmpl := &npc.Template{
		ID: "security_drone", Name: "Security Drone", Type: "robot",
		Level: 1, MaxHP: 10, AC: 10,
	}
	_ = tmpl.Validate()
	inst := npc.NewInstance("i3", tmpl, "room1")
	if inst.Resistances[combat.DamageTypeBleed] != 999 {
		t.Errorf("bleed resistance = %d, want 999", inst.Resistances[combat.DamageTypeBleed])
	}
	if inst.Resistances[combat.DamageTypePoison] != 999 {
		t.Errorf("poison resistance = %d, want 999", inst.Resistances[combat.DamageTypePoison])
	}
}

// TestRobotSpawnResistanceTemplateOverrides verifies template values win over defaults.
func TestRobotSpawnResistanceTemplateOverrides(t *testing.T) {
	tmpl := &npc.Template{
		ID: "corroded_bot", Name: "Corroded Bot", Type: "robot",
		Level: 1, MaxHP: 10, AC: 10,
		Resistances: map[string]int{
			"bleed":  5,
			"poison": 10,
		},
	}
	_ = tmpl.Validate()
	inst := npc.NewInstance("i4", tmpl, "room1")
	if inst.Resistances[combat.DamageTypeBleed] != 5 {
		t.Errorf("bleed resistance = %d, want 5", inst.Resistances[combat.DamageTypeBleed])
	}
	if inst.Resistances[combat.DamageTypePoison] != 10 {
		t.Errorf("poison resistance = %d, want 10", inst.Resistances[combat.DamageTypePoison])
	}
}

// TestMachineSpawnResistanceDefaults verifies machines also get bleed/poison defaults.
func TestMachineSpawnResistanceDefaults(t *testing.T) {
	tmpl := &npc.Template{
		ID: "auto_turret", Name: "Auto-Turret", Type: "machine",
		Level: 1, MaxHP: 10, AC: 10,
		Immobile: true,
	}
	_ = tmpl.Validate()
	inst := npc.NewInstance("i5", tmpl, "room1")
	if inst.Resistances[combat.DamageTypeBleed] != 999 {
		t.Errorf("bleed resistance = %d, want 999", inst.Resistances[combat.DamageTypeBleed])
	}
	if inst.Resistances[combat.DamageTypePoison] != 999 {
		t.Errorf("poison resistance = %d, want 999", inst.Resistances[combat.DamageTypePoison])
	}
}

// TestHumanSpawnNoResistanceDefaults verifies human NPCs don't get robot defaults.
func TestHumanSpawnNoResistanceDefaults(t *testing.T) {
	tmpl := &npc.Template{
		ID: "guard", Name: "Guard", Type: "human",
		Level: 1, MaxHP: 10, AC: 10,
	}
	_ = tmpl.Validate()
	inst := npc.NewInstance("i6", tmpl, "room1")
	if inst.Resistances != nil {
		if inst.Resistances[combat.DamageTypeBleed] == 999 {
			t.Error("human NPC should not have bleed resistance default of 999")
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestSpawnPropagates|TestRobotSpawn|TestMachineSpawn|TestHumanSpawn" -v
```

Expected: compile errors — `inst.AttackVerb`, `inst.Immobile` undefined.

- [ ] **Step 3: Add AttackVerb and Immobile to Instance; update NewInstanceWithResolver**

In `internal/game/npc/instance.go`:

Add fields to `Instance` struct (after `Cowering bool`):

```go
	// AttackVerb is the verb used in attack narratives; copied from template.
	// Empty string falls back to "attacks" at the combat layer.
	AttackVerb string
	// Immobile prevents this instance from being moved by the wander/patrol system.
	Immobile bool
```

Update the `NewInstanceWithResolver` function to add resistance defaults and propagate new fields. Add an import for `"github.com/cory-johannsen/mud/internal/game/combat"`.

In `NewInstanceWithResolver`, replace the return statement to include the new fields and insert resistance-default logic before the return:

```go
	// Apply robot/machine bleed+poison resistance defaults (REQ-NHN-14,15,26).
	// Build resistances map by starting with defaults for robot/machine, then
	// overlaying the template's values so template wins.
	var resistances map[string]int
	if tmpl.IsRobot() || tmpl.IsMachine() {
		resistances = map[string]int{
			combat.DamageTypeBleed:  999,
			combat.DamageTypePoison: 999,
		}
		for k, v := range tmpl.Resistances {
			resistances[k] = v
		}
	} else {
		resistances = tmpl.Resistances
	}

	return &Instance{
		// ... all existing fields ...
		Resistances: resistances,
		// ... remaining existing fields ...
		AttackVerb: tmpl.AttackVerb,
		Immobile:   tmpl.Immobile,
	}
```

Note: You must replace the existing `Resistances: tmpl.Resistances,` line in the return statement with `Resistances: resistances,` and add the two new fields. Keep all other fields unchanged.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run "TestSpawnPropagates|TestRobotSpawn|TestMachineSpawn|TestHumanSpawn" -v
```

Expected: all PASS.

- [ ] **Step 5: Run full NPC test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/npc/instance.go internal/game/npc/instance_test.go
git commit -m "feat(npc): propagate AttackVerb/Immobile at spawn; apply robot/machine resistance defaults (REQ-NHN-5,14,15,21,26)"
```

---

## Task 5: AttackVerb on combat.Combatant

**Files:**
- Modify: `internal/game/combat/combat.go`
- Modify: `internal/game/combat/combat_test.go`

- [ ] **Step 1: Write failing test**

Add to `internal/game/combat/combat_test.go`:

```go
// TestCombatantAttackVerbDefault verifies zero value is empty string.
func TestCombatantAttackVerbDefault(t *testing.T) {
	c := combat.Combatant{}
	if c.AttackVerb != "" {
		t.Errorf("AttackVerb default = %q, want empty string", c.AttackVerb)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestCombatantAttackVerbDefault -v
```

Expected: compile error — `c.AttackVerb` undefined.

- [ ] **Step 3: Add AttackVerb to Combatant**

In `internal/game/combat/combat.go`, add after the `CoverTier string` field (last field in the `Combatant` struct):

```go
	// AttackVerb is the verb used in attack narratives for this combatant.
	// NPC combatants use the value from npc.Instance.AttackVerb.
	// Player combatants default to "attacks" at the round layer when this is empty.
	AttackVerb string
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestCombatantAttackVerbDefault -v
```

Expected: PASS.

- [ ] **Step 5: Run full combat test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/combat.go internal/game/combat/combat_test.go
git commit -m "feat(combat): add AttackVerb field to Combatant (REQ-NHN-6)"
```

---

## Task 6: Use AttackVerb in attackNarrative

**Files:**
- Modify: `internal/game/combat/round.go`
- Modify: `internal/game/combat/round_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/combat/round_test.go`:

```go
// TestAttackNarrativeUsesCustomVerb verifies custom verb appears in the narrative.
// This is an integration-style test using RunCombatRound with a custom AttackVerb.
func TestAttackNarrativeUsesCustomVerb(t *testing.T) {
	// We test attackNarrative indirectly by checking that custom verbs appear
	// in CombatRound narrative events when an NPC has AttackVerb set.
	attacker := &combat.Combatant{
		ID: "npc1", Kind: combat.KindNPC, Name: "Feral Dog",
		MaxHP: 30, CurrentHP: 30, AC: 10, Level: 1,
		StrMod: 3, DexMod: 1,
		AttackVerb: "bites",
	}
	defender := &combat.Combatant{
		ID: "p1", Kind: combat.KindPlayer, Name: "Player",
		MaxHP: 30, CurrentHP: 30, AC: 8, Level: 1,
	}
	// Queue attack for attacker.
	cbt := &combat.Combat{
		RoomID:       "room1",
		Combatants:   []*combat.Combatant{attacker, defender},
		ActionQueues: map[string][]combat.QueuedAction{
			attacker.ID: {{Type: combat.ActionAttack, Target: defender.Name}},
			defender.ID: {{Type: combat.ActionPass}},
		},
		Round: 1,
	}
	events := combat.RunCombatRound(cbt, nil, nil, nil)
	// At least one event narrative should contain "bites".
	found := false
	for _, ev := range events {
		if ev.AttackResult != nil && strings.Contains(ev.Narrative, "bites") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected narrative containing %q; events: %v", "bites", events)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestAttackNarrativeUsesCustomVerb -v
```

Expected: FAIL — narrative contains "attacks" not "bites".

- [ ] **Step 3: Update attackNarrative callers in round.go**

In `internal/game/combat/round.go`, locate the three `attackNarrative` calls (at lines ~642, ~761, ~856 based on the grep results).

Replace the three hardcoded verb strings as follows:

**Main attack (line ~642):** Replace `"attacks"` with a helper expression that reads `actor.AttackVerb`, defaulting to `"attacks"` if empty:

```go
				verb := actor.AttackVerb
				if verb == "" {
					verb = "attacks"
				}
				narrative := attackNarrative(actor.Name, verb, target.Name, r.WeaponName, r.Outcome, r.AttackTotal, dmg)
```

**Offhand attack 1 (line ~761):** Replace `"strikes"` similarly:

```go
				verb1 := actor.AttackVerb
				if verb1 == "" {
					verb1 = "strikes"
				}
				narrative1 := attackNarrative(actor.Name, verb1, target.Name, r1.WeaponName, r1.Outcome, r1.AttackTotal, dmg1)
```

**Offhand attack 2 (line ~856):** Same as offhand 1:

```go
				verb2 := actor.AttackVerb
				if verb2 == "" {
					verb2 = "strikes"
				}
				narrative2 := attackNarrative(actor.Name, verb2, target.Name, r2.WeaponName, r2.Outcome, r2.AttackTotal, dmg2)
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run TestAttackNarrativeUsesCustomVerb -v
```

Expected: PASS.

- [ ] **Step 5: Run full combat test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -v
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/round.go internal/game/combat/round_test.go
git commit -m "feat(combat): use actor.AttackVerb in attackNarrative for all three attack slots (REQ-NHN-7)"
```

---

## Task 7: Wire AttackVerb into NPC Combatant Construction

**Files:**
- Modify: `internal/gameserver/combat_handler.go`

Both NPC combatant construction sites (around lines 1207 and 1864) must be updated to populate `AttackVerb` from the NPC instance.

- [ ] **Step 1: Locate both NPC Combatant literal sites**

Search for `npcCbt := &combat.Combatant{` in `internal/gameserver/combat_handler.go` — there are two sites (confirmed in Task 4 research at lines 1207 and 1864).

- [ ] **Step 2: Add AttackVerb to both NPC combatant constructions**

At each `npcCbt := &combat.Combatant{...}` site, add:

```go
		AttackVerb: inst.AttackVerb,
```

Player `Combatant` structs (at lines ~1154 and ~1798) do NOT get `AttackVerb` set — the zero-value `""` correctly falls through to the default `"attacks"` in `round.go`.

- [ ] **Step 3: Run the gameserver tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -v -count=1
```

Expected: all PASS (no behavior change for existing tests — `AttackVerb` defaults to `""` which maps to `"attacks"`).

- [ ] **Step 4: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/combat_handler.go
git commit -m "feat(gameserver): set AttackVerb on NPC Combatant from instance (REQ-NHN-6)"
```

---

## Task 8: Organic Loot Drop on Animal Death and Salvage Drop on Robot Death

**Files:**
- Modify: `internal/gameserver/combat_handler.go`

The `removeDeadNPCsLocked` function (around line 2783) handles loot generation on NPC death. It must be extended to:

1. Generate organic loot for animals (REQ-NHN-10).
2. Generate salvage drops for robots (REQ-NHN-18).
3. Emit `"<Name> is destroyed."` for machines instead of `"<Name> is dead!"` (REQ-NHN-25).

- [ ] **Step 1: Write failing tests**

Add to the combat_handler test file (or a new `combat_handler_nhn_test.go`):

```go
// File: internal/gameserver/combat_handler_nhn_test.go
package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
)

// TestAnimalDeathDropsOrganicLoot verifies organic loot lands on the floor when an animal dies.
func TestAnimalDeathDropsOrganicLoot(t *testing.T) {
	// Build a minimal CombatHandler with a FloorManager and NPC manager.
	floorDrops := map[string][]inventory.ItemInstance{}
	floorMgr := inventory.NewFloorManager()
	npcMgr := npc.NewManager()

	// Create and register an animal template + instance.
	tmpl := &npc.Template{
		ID: "feral_dog", Name: "Feral Dog", Type: "animal",
		Level: 1, MaxHP: 10, AC: 10,
		AttackVerb: "bites",
		Loot: &npc.LootTable{
			OrganicDrops: []npc.OrganicDrop{
				{ItemID: "dog_meat", Weight: 10, QuantityMin: 1, QuantityMax: 1},
			},
		},
	}
	if err := tmpl.Validate(); err != nil {
		t.Fatalf("template invalid: %v", err)
	}
	inst := npc.NewInstance("dog1", tmpl, "room1")
	inst.CurrentHP = 0 // dead
	npcMgr.Add(inst)

	_ = floorDrops
	_ = floorMgr

	// NOTE: Full integration test wiring a CombatHandler is complex; this verifies
	// the helper function GenerateOrganicLoot directly.
	result := npc.GenerateOrganicLoot(*tmpl.Loot)
	if len(result.Items) == 0 {
		t.Fatal("expected at least one item in organic loot result, got none")
	}
	if result.Items[0].ItemDefID != "dog_meat" {
		t.Errorf("item def ID = %q, want %q", result.Items[0].ItemDefID, "dog_meat")
	}
	if result.Items[0].Quantity < 1 || result.Items[0].Quantity > 1 {
		t.Errorf("quantity = %d, want 1", result.Items[0].Quantity)
	}
}

// TestRobotDeathDropsSalvageLoot verifies salvage loot generation.
func TestRobotDeathDropsSalvageLoot(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs:     []string{"circuit_board"},
			QuantityMin: 1,
			QuantityMax: 2,
		},
	}
	result := npc.GenerateSalvageLoot(lt)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 salvage item, got %d", len(result.Items))
	}
	if result.Items[0].ItemDefID != "circuit_board" {
		t.Errorf("ItemDefID = %q, want %q", result.Items[0].ItemDefID, "circuit_board")
	}
	if result.Items[0].Quantity < 1 || result.Items[0].Quantity > 2 {
		t.Errorf("Quantity = %d, want 1-2", result.Items[0].Quantity)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestAnimalDeath|TestRobotDeath" -v
```

Expected: compile error — `npc.GenerateOrganicLoot`, `npc.GenerateSalvageLoot` undefined.

- [ ] **Step 3: Add GenerateOrganicLoot and GenerateSalvageLoot to loot.go**

In `internal/game/npc/loot.go`, add after `GenerateLoot`:

```go
// GenerateOrganicLoot selects one organic drop using weighted random selection
// and returns a LootResult with the selected item and a quantity in [QuantityMin, QuantityMax].
//
// Precondition: lt must have passed Validate(); lt.OrganicDrops must be non-empty.
// Postcondition: Returns a LootResult with exactly one item if OrganicDrops is non-empty;
// returns empty LootResult if OrganicDrops is empty.
func GenerateOrganicLoot(lt LootTable) LootResult {
	if len(lt.OrganicDrops) == 0 {
		return LootResult{}
	}
	// Weighted selection.
	total := 0
	for _, od := range lt.OrganicDrops {
		total += od.Weight
	}
	roll := rand.Intn(total)
	var selected OrganicDrop
	for _, od := range lt.OrganicDrops {
		roll -= od.Weight
		if roll < 0 {
			selected = od
			break
		}
	}
	qty := selected.QuantityMin
	if spread := selected.QuantityMax - selected.QuantityMin; spread > 0 {
		qty += rand.Intn(spread + 1)
	}
	return LootResult{
		Items: []LootItem{{
			ItemDefID:  selected.ItemID,
			InstanceID: uuid.New().String(),
			Quantity:   qty,
		}},
	}
}

// GenerateSalvageLoot selects one item uniformly from SalvageDrop.ItemIDs and returns
// a LootResult with the selected item and a quantity in [QuantityMin, QuantityMax].
//
// Precondition: lt must have passed Validate(); lt.SalvageDrop must be non-nil and non-empty.
// Postcondition: Returns a LootResult with exactly one item.
func GenerateSalvageLoot(lt LootTable) LootResult {
	if lt.SalvageDrop == nil || len(lt.SalvageDrop.ItemIDs) == 0 {
		return LootResult{}
	}
	sd := lt.SalvageDrop
	itemID := sd.ItemIDs[rand.Intn(len(sd.ItemIDs))]
	qty := sd.QuantityMin
	if spread := sd.QuantityMax - sd.QuantityMin; spread > 0 {
		qty += rand.Intn(spread + 1)
	}
	return LootResult{
		Items: []LootItem{{
			ItemDefID:  itemID,
			InstanceID: uuid.New().String(),
			Quantity:   qty,
		}},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestAnimalDeath|TestRobotDeath" -v
```

Expected: all PASS.

- [ ] **Step 5: Update removeDeadNPCsLocked in combat_handler.go**

Locate `removeDeadNPCsLocked` in `internal/gameserver/combat_handler.go` (around line 2783).

The existing loot block is:
```go
		if inst.Loot != nil {
			result := npc.GenerateLoot(*inst.Loot)
			// ...distribute currency...
			// ...drop items on floor...
		}
```

Replace this block with type-aware loot generation:

```go
		if inst.Loot != nil {
			tmpl := h.npcMgr.Template(inst.TemplateID) // see note below
			var result npc.LootResult
			if tmpl != nil && tmpl.IsAnimal() {
				result = npc.GenerateOrganicLoot(*inst.Loot)
			} else if tmpl != nil && tmpl.IsRobot() {
				result = npc.GenerateSalvageLoot(*inst.Loot)
				// Also run standard loot for any items/currency the template may have.
				stdResult := npc.GenerateLoot(*inst.Loot)
				result.Currency += stdResult.Currency
				result.Items = append(result.Items, stdResult.Items...)
			} else {
				result = npc.GenerateLoot(*inst.Loot)
			}
			totalCurrency := result.Currency + inst.Currency
			inst.Currency = 0
			livingParticipants := h.livingParticipantSessions(cbt)
			h.distributeCurrencyLocked(context.Background(), livingParticipants, totalCurrency)
			if h.floorMgr != nil {
				for _, lootItem := range result.Items {
					h.floorMgr.Drop(roomID, inventory.ItemInstance{
						InstanceID: lootItem.InstanceID,
						ItemDefID:  lootItem.ItemDefID,
						Quantity:   lootItem.Quantity,
					})
				}
			}
		} else if inst.Currency > 0 {
			totalCurrency := inst.Currency
			inst.Currency = 0
			livingParticipants := h.livingParticipantSessions(cbt)
			h.distributeCurrencyLocked(context.Background(), livingParticipants, totalCurrency)
		}
```

Note: `h.npcMgr.Template(templateID)` — check whether this method exists on `npc.Manager`. If it does not exist, use the instance's `Type` field directly: replace `tmpl.IsAnimal()` with `inst.IsAnimal()` etc. (which requires adding `IsAnimal()`, `IsRobot()`, `IsMachine()` methods to `Instance` that delegate to the `Type` field — add those to `instance.go`).

**Recommended approach:** Add `IsAnimal()`, `IsRobot()`, `IsMachine()` methods directly to `*Instance` in `instance.go`:

```go
// IsAnimal reports whether this instance is an animal.
func (i *Instance) IsAnimal() bool { return i.Type == "animal" }

// IsRobot reports whether this instance is a robot.
func (i *Instance) IsRobot() bool { return i.Type == "robot" }

// IsMachine reports whether this instance is a machine.
func (i *Instance) IsMachine() bool { return i.Type == "machine" }
```

Then use `inst.IsAnimal()`, `inst.IsRobot()` directly in `removeDeadNPCsLocked` without needing `npcMgr.Template`.

Update the death message (REQ-NHN-25) immediately after the loot block:

```go
		// Announce NPC death in the console.
		deathMsg := fmt.Sprintf("%s is dead!", c.Name)
		if inst.IsMachine() {
			deathMsg = fmt.Sprintf("%s is destroyed.", c.Name)
		}
		h.broadcastFn(inst.RoomID, []*gamev1.CombatEvent{{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH,
			Attacker:  c.Name,
			Narrative: deathMsg,
		}})
```

- [ ] **Step 6: Run full gameserver and NPC test suites**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... ./internal/game/npc/... -v -count=1
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/npc/loot.go internal/game/npc/instance.go internal/gameserver/combat_handler.go internal/gameserver/combat_handler_nhn_test.go
git commit -m "feat(npc/gameserver): organic/salvage loot drops and machine death message (REQ-NHN-10,18,25)"
```

---

## Task 9: Immobility Enforcement in NPC Patrol/Wander

**Files:**
- Modify: `internal/gameserver/grpc_service.go`

- [ ] **Step 1: Locate npcPatrolRandom and tickNPCIdle**

In `internal/gameserver/grpc_service.go`, `npcPatrolRandom` (around line 3720) and `tickNPCIdle` (around line 3677) are the movement and idle-behavior functions.

- [ ] **Step 2: Write failing test**

Add to relevant test file (or new `grpc_service_nhn_test.go`):

```go
// File: internal/gameserver/grpc_service_nhn_test.go
package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
)

// TestImmobileNPCSkipsPatrol verifies immobile NPCs don't move in npcPatrolRandom.
// This test invokes the immobile check path indirectly via tickNPCIdle.
func TestImmobileFlag(t *testing.T) {
	tmpl := &npc.Template{
		ID: "auto_turret", Name: "Auto-Turret", Type: "machine",
		Level: 1, MaxHP: 10, AC: 10, Immobile: true,
	}
	if err := tmpl.Validate(); err != nil {
		t.Fatalf("template invalid: %v", err)
	}
	inst := npc.NewInstance("turret1", tmpl, "room1")
	if !inst.Immobile {
		t.Error("Immobile = false, want true after spawn")
	}
	// The actual skip logic in npcPatrolRandom is covered by checking the flag;
	// integration testing of the skip requires a running GameServiceServer.
	// The unit test here validates the flag propagation precondition.
}
```

- [ ] **Step 3: Run test to verify it passes (it already should given Task 4)**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestImmobileFlag -v
```

Expected: PASS (Task 4 already propagates `Immobile`).

- [ ] **Step 4: Update npcPatrolRandom to skip immobile NPCs**

In `internal/gameserver/grpc_service.go`, in `npcPatrolRandom` (around line 3720), add at the very start of the function:

```go
func (s *GameServiceServer) npcPatrolRandom(inst *npc.Instance) {
	if inst.Immobile {
		return
	}
	// ... existing body ...
```

- [ ] **Step 5: Update tickNPCIdle to skip immobile NPCs**

In `tickNPCIdle` (around line 3678), add after the early-return guard:

```go
func (s *GameServiceServer) tickNPCIdle(inst *npc.Instance, zoneID string, aiReg *ai.Registry) {
	if inst.Immobile {
		return
	}
	// ... existing body ...
```

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -v -count=1
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_nhn_test.go
git commit -m "feat(gameserver): skip immobile NPCs in patrol/wander (REQ-NHN-22)"
```

---

## Task 10: Filter `say` HTN Tasks for Animal Instances

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/grpc_service.go`

The `applyPlanLocked` function in `combat_handler.go` (around line 2408) handles HTN plan execution for combat NPCs. The `tickNPCIdle` in `grpc_service.go` handles idle-phase HTN actions.

- [ ] **Step 1: Write failing test**

Add to `internal/gameserver/combat_handler_nhn_test.go`. First update the import block at the top of that file to:

```go
import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
)
```

Then add the test functions:

```go
// TestAnimalPlanFiltersSayTasks verifies say tasks are stripped from animal plans.
// This test directly exercises the logic by verifying the filtering contract:
// after filtering, no "say" actions remain for animal instances.
func TestAnimalSayFiltering(t *testing.T) {
	actions := []ai.PlannedAction{
		{Action: "attack", Target: "Player"},
		{Action: "say", Target: ""},
		{Action: "attack", Target: "Player"},
	}
	isAnimal := true
	filtered := gameserver.FilterAnimalPlanActions(actions, isAnimal)
	for _, a := range filtered {
		if a.Action == "say" {
			t.Errorf("say action was not filtered from animal plan")
		}
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 actions after filtering, got %d", len(filtered))
	}
}

// TestNonAnimalPlanRetainsSayTasks verifies say tasks are kept for non-animal NPCs.
func TestNonAnimalSayRetained(t *testing.T) {
	actions := []ai.PlannedAction{
		{Action: "attack", Target: "Player"},
		{Action: "say", Target: ""},
	}
	isAnimal := false
	filtered := gameserver.FilterAnimalPlanActions(actions, isAnimal)
	if len(filtered) != 2 {
		t.Errorf("expected 2 actions retained for non-animal, got %d", len(filtered))
	}
}
```

Note: `gameserver.FilterAnimalPlanActions` is a new exported helper to make it testable without constructing a full server.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestAnimalSay|TestNonAnimalSay" -v
```

Expected: compile error — `gameserver.FilterAnimalPlanActions` undefined.

- [ ] **Step 3: Add FilterAnimalPlanActions helper and wire it in**

In `internal/gameserver/combat_handler.go`, add a package-level function (before `applyPlanLocked`):

```go
// FilterAnimalPlanActions removes "say" operator tasks from the plan if isAnimal is true.
// Returns the filtered slice. If the result is empty, the caller MUST fall back to
// the simple attack behavior.
//
// Precondition: actions may be nil or empty.
// Postcondition: returned slice contains no "say" actions when isAnimal is true.
func FilterAnimalPlanActions(actions []ai.PlannedAction, isAnimal bool) []ai.PlannedAction {
	if !isAnimal {
		return actions
	}
	filtered := actions[:0:0] // zero-length, same backing if no alloc needed
	for _, a := range actions {
		if a.Action != "say" {
			filtered = append(filtered, a)
		}
	}
	return filtered
}
```

In `autoQueueNPCsLocked` (around line 2388), after the call to `planner.Plan(ws)` and before calling `h.applyPlanLocked`, add:

```go
					inst, instOK := h.npcMgr.Get(c.ID)
					if instOK {
						actions = FilterAnimalPlanActions(actions, inst.IsAnimal())
					}
					if len(actions) == 0 {
						h.legacyAutoQueueLocked(cbt, c)
						continue
					}
					h.applyPlanLocked(cbt, c, actions)
```

Note: The existing code already calls `h.applyPlanLocked(cbt, c, actions)` — wrap it as above. Be careful not to duplicate the `inst` lookup if it already exists in scope.

For the idle `tickNPCIdle`, add a filter after `actions, err := planner.Plan(ws)`:

```go
	actions, err := planner.Plan(ws)
	if err != nil || len(actions) == 0 {
		return
	}
	actions = FilterAnimalPlanActions(actions, inst.IsAnimal())
	if len(actions) == 0 {
		return // no fallback needed for idle phase; just skip
	}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestAnimalSay|TestNonAnimalSay" -v
```

Expected: all PASS.

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... ./internal/game/... -v -count=1
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/combat_handler.go internal/gameserver/grpc_service.go internal/gameserver/combat_handler_nhn_test.go
git commit -m "feat(gameserver): filter 'say' HTN tasks for animal NPCs with empty-plan fallback (REQ-NHN-13)"
```

---

## Task 11: Example NPC Content Files

**Files:**
- Create: `content/npcs/feral_dog.yaml`
- Create: `content/npcs/security_drone.yaml`
- Create: `content/npcs/auto_turret.yaml`

- [ ] **Step 1: Create feral_dog.yaml**

```yaml
# content/npcs/feral_dog.yaml
id: feral_dog
name: Feral Dog
type: animal
level: 2
max_hp: 18
ac: 12
awareness: 3
attack_verb: bites
abilities:
  brutality: 3
  grit: 2
  quickness: 4
  reasoning: 1
  savvy: 1
  flair: 1
loot:
  organic_drops:
    - item_id: dog_meat
      weight: 10
      quantity_min: 1
      quantity_max: 2
respawn_delay: 10m
disposition: hostile
```

- [ ] **Step 2: Create security_drone.yaml**

```yaml
# content/npcs/security_drone.yaml
id: security_drone
name: Security Drone
type: robot
level: 4
max_hp: 35
ac: 15
awareness: 5
attack_verb: shoots
abilities:
  brutality: 4
  grit: 3
  quickness: 3
  reasoning: 2
  savvy: 1
  flair: 1
weaknesses:
  electric: 5
loot:
  salvage_drop:
    item_ids:
      - circuit_board
      - power_cell
      - scrap_metal
    quantity_min: 1
    quantity_max: 2
respawn_delay: 30m
disposition: hostile
```

- [ ] **Step 3: Create auto_turret.yaml**

```yaml
# content/npcs/auto_turret.yaml
id: auto_turret
name: Auto-Turret
type: machine
level: 5
max_hp: 50
ac: 16
awareness: 6
attack_verb: shoots
immobile: true
abilities:
  brutality: 5
  grit: 4
  quickness: 2
  reasoning: 1
  savvy: 1
  flair: 1
weaknesses:
  electric: 8
loot:
  equipment:
    - id: turret_barrel
      weight: 10
respawn_delay: 60m
disposition: hostile
```

- [ ] **Step 4: Verify YAML loads without error**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/npc/... -run TestLoad -v
```

If no `TestLoad` test exists that covers the content directory, run a quick sanity check:

```bash
cd /home/cjohannsen/src/mud && go run ./cmd/devserver/... --dry-run 2>&1 | head -20
```

If the above command doesn't exist, verify by running all NPC tests (which load from content in some test setups):

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: no compile or load errors.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add content/npcs/feral_dog.yaml content/npcs/security_drone.yaml content/npcs/auto_turret.yaml
git commit -m "content(npcs): add feral_dog, security_drone, auto_turret example templates (REQ-NHN-8,19,23,24)"
```

---

## Task 12: Final Integration — Run Full Test Suite

- [ ] **Step 1: Run all tests**

```bash
cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | tail -30
```

Expected: all packages PASS, zero failures.

- [ ] **Step 2: Build all binaries**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 3: Commit final state if any cleanup was needed**

```bash
cd /home/cjohannsen/src/mud && git status
```

If clean, no commit needed. If there are unstaged fixes, add and commit them:

```bash
cd /home/cjohannsen/src/mud && git add -p && git commit -m "fix(nhn): final cleanup after integration test run"
```

---

## Requirements Coverage Matrix

| Requirement | Task(s) |
|---|---|
| REQ-NHN-1: canonical type values | Task 3 (convention; helpers enforce semantics) |
| REQ-NHN-2: Validate does not reject unknown types | Task 3 |
| REQ-NHN-3: IsAnimal/IsRobot/IsMachine helpers | Task 3 |
| REQ-NHN-4: AttackVerb field on Template | Task 3 |
| REQ-NHN-5: SpawnInstance propagates AttackVerb | Task 4 |
| REQ-NHN-6: combat.Combatant.AttackVerb; NPC uses instance value; player defaults | Tasks 5, 7 |
| REQ-NHN-7: attackNarrative uses actor.AttackVerb | Task 6 |
| REQ-NHN-8: animal verb convention | Task 11 (content) |
| REQ-NHN-9: OrganicDrops on LootTable + validation | Task 2 |
| REQ-NHN-10: animal death rolls organic drop | Task 8 |
| REQ-NHN-11: animal loot restricted in Validate | Task 3 |
| REQ-NHN-12: faction enforcement deferred | N/A (future) |
| REQ-NHN-13: say tasks filtered for animals | Task 10 |
| REQ-NHN-14: robot defaults bleed/poison 999 | Task 4 |
| REQ-NHN-15: template values override defaults | Task 4 |
| REQ-NHN-16: electric weakness convention | Task 11 (content) |
| REQ-NHN-17: SalvageDrop on LootTable + validation | Task 2 |
| REQ-NHN-18: robot death rolls salvage drop | Task 8 |
| REQ-NHN-19: robot verb convention | Task 11 (content) |
| REQ-NHN-20: Immobile field on Template | Task 3 |
| REQ-NHN-21: SpawnInstance propagates Immobile | Task 4 |
| REQ-NHN-22: movement skips immobile instances | Task 9 |
| REQ-NHN-23: machine immobile convention | Task 11 (content) |
| REQ-NHN-24: machine trigger convention | Task 11 (content) |
| REQ-NHN-25: machine death message "destroyed" | Task 8 |
| REQ-NHN-26: machine gets same resistance defaults | Task 4 |
| REQ-NHN-27: machine loot convention | Task 11 (content) |
| REQ-NHN-28: damage_types.go constants | Task 1 |
| REQ-NHN-29: internal code uses constants | Tasks 1, 4 |
