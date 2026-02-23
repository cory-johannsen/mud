# Combat Stage 5 — Conditions System Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a YAML-defined conditions system with duration tracking, roll modifiers, action restrictions, and the full PF2E dying/wounded chain to the combat engine.

**Architecture:** `internal/game/condition` is a pure data package — knows nothing about `combat`. It owns YAML loading, a registry of `ConditionDef` structs, and `ActiveSet` per combatant. `Combat` holds `Conditions map[string]*condition.ActiveSet`. The combat engine calls `ActiveSet` for modifiers and lifecycle. No circular imports.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`, `pgregory.net/rapid` (property tests), protobuf/grpc, `go.uber.org/zap`.

---

## Task 1: ConditionDef + Registry + YAML Loader

**Files:**
- Create: `internal/game/condition/definition.go`
- Create: `internal/game/condition/definition_test.go`

**Context:** This is a brand new package. `ConditionDef` is the static YAML-parsed definition. `Registry` maps condition IDs to their definitions. The YAML loader reads all `*.yaml` files in a directory.

### Step 1: Write failing tests

```go
// internal/game/condition/definition_test.go
package condition_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/condition"
)

func TestRegistry_Get_Found(t *testing.T) {
	reg := condition.NewRegistry()
	def := &condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent"}
	reg.Register(def)
	got, ok := reg.Get("prone")
	require.True(t, ok)
	assert.Equal(t, def, got)
}

func TestRegistry_Get_NotFound(t *testing.T) {
	reg := condition.NewRegistry()
	_, ok := reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_All_ReturnsCopy(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "a", Name: "A", DurationType: "permanent"})
	reg.Register(&condition.ConditionDef{ID: "b", Name: "B", DurationType: "rounds"})
	all := reg.All()
	assert.Len(t, all, 2)
}

func TestLoadDirectory_ParsesYAML(t *testing.T) {
	dir := t.TempDir()
	yaml := `
id: stunned
name: Stunned
description: "You are stunned."
duration_type: rounds
max_stacks: 3
attack_penalty: 0
ac_penalty: 0
speed_penalty: 0
restrict_actions:
  - attack
  - strike
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stunned.yaml"), []byte(yaml), 0644))

	reg, err := condition.LoadDirectory(dir)
	require.NoError(t, err)
	got, ok := reg.Get("stunned")
	require.True(t, ok)
	assert.Equal(t, "Stunned", got.Name)
	assert.Equal(t, "rounds", got.DurationType)
	assert.Equal(t, 3, got.MaxStacks)
	assert.Equal(t, []string{"attack", "strike"}, got.RestrictActions)
}

func TestLoadDirectory_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	reg, err := condition.LoadDirectory(dir)
	require.NoError(t, err)
	assert.Empty(t, reg.All())
}

func TestLoadDirectory_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(":::bad:::"), 0644))
	_, err := condition.LoadDirectory(dir)
	assert.Error(t, err)
}

func TestPropertyRegistry_RegisterThenGet(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		id := rapid.StringMatching(`[a-z_]{3,12}`).Draw(t, "id")
		reg := condition.NewRegistry()
		def := &condition.ConditionDef{ID: id, Name: id, DurationType: "permanent"}
		reg.Register(def)
		got, ok := reg.Get(id)
		assert.True(t, ok, "registered condition must be retrievable")
		assert.Equal(t, def, got)
	})
}
```

Run: `go test ./internal/game/condition/... -run TestRegistry -v`
Expected: FAIL with "package not found"

### Step 2: Implement `definition.go`

```go
// internal/game/condition/definition.go
package condition

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConditionDef is the static definition of a condition, loaded from YAML.
type ConditionDef struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description"`
	DurationType    string   `yaml:"duration_type"` // "rounds" | "until_save" | "permanent"
	MaxStacks       int      `yaml:"max_stacks"`    // 0 = unstackable
	AttackPenalty   int      `yaml:"attack_penalty"`
	ACPenalty       int      `yaml:"ac_penalty"`
	SpeedPenalty    int      `yaml:"speed_penalty"`
	RestrictActions []string `yaml:"restrict_actions"`
	LuaOnApply      string   `yaml:"lua_on_apply"`  // stored; ignored until Stage 6
	LuaOnRemove     string   `yaml:"lua_on_remove"` // stored; ignored until Stage 6
	LuaOnTick       string   `yaml:"lua_on_tick"`   // stored; ignored until Stage 6
}

// Registry holds all known ConditionDefs keyed by ID.
type Registry struct {
	defs map[string]*ConditionDef
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{defs: make(map[string]*ConditionDef)}
}

// Register adds def to the registry, overwriting any existing entry with the same ID.
//
// Precondition: def must not be nil and def.ID must not be empty.
func (r *Registry) Register(def *ConditionDef) {
	r.defs[def.ID] = def
}

// Get returns the ConditionDef for id, or (nil, false) if not found.
func (r *Registry) Get(id string) (*ConditionDef, bool) {
	d, ok := r.defs[id]
	return d, ok
}

// All returns a snapshot slice of all registered ConditionDefs.
func (r *Registry) All() []*ConditionDef {
	out := make([]*ConditionDef, 0, len(r.defs))
	for _, d := range r.defs {
		out = append(out, d)
	}
	return out
}

// LoadDirectory reads every *.yaml file in dir, parses each as a ConditionDef,
// and returns a populated Registry.
//
// Precondition: dir must be a readable directory.
// Postcondition: Returns a non-nil Registry, or an error if any file fails to parse.
func LoadDirectory(dir string) (*Registry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading condition dir %q: %w", dir, err)
	}

	reg := NewRegistry()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %q: %w", path, err)
		}
		var def ConditionDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("parsing %q: %w", path, err)
		}
		reg.Register(&def)
	}
	return reg, nil
}
```

### Step 3: Run tests

Run: `go test ./internal/game/condition/... -v`
Expected: all PASS

### Step 4: Commit

```bash
git add internal/game/condition/definition.go internal/game/condition/definition_test.go
git commit -m "feat(condition): ConditionDef registry and YAML loader (Stage 5 Task 1)"
```

---

## Task 2: ActiveCondition + ActiveSet

**Files:**
- Create: `internal/game/condition/active.go`
- Create: `internal/game/condition/active_test.go`

**Context:** `ActiveCondition` tracks one applied condition instance on an entity. `ActiveSet` manages all conditions for one combatant. `Tick()` is called each round to decrement durations and expire conditions.

### Step 1: Write failing tests

```go
// internal/game/condition/active_test.go
package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/condition"
)

func prone() *condition.ConditionDef {
	return &condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0}
}

func frightened() *condition.ConditionDef {
	return &condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4}
}

func dying() *condition.ConditionDef {
	return &condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4}
}

func TestActiveSet_Apply_Permanent(t *testing.T) {
	s := condition.NewActiveSet()
	err := s.Apply(prone(), 1, -1)
	require.NoError(t, err)
	assert.True(t, s.Has("prone"))
	assert.Equal(t, 1, s.Stacks("prone"))
}

func TestActiveSet_Apply_Rounds(t *testing.T) {
	s := condition.NewActiveSet()
	err := s.Apply(frightened(), 2, 3)
	require.NoError(t, err)
	assert.True(t, s.Has("frightened"))
	assert.Equal(t, 2, s.Stacks("frightened"))
}

func TestActiveSet_Apply_StacksCapped(t *testing.T) {
	s := condition.NewActiveSet()
	// MaxStacks=4 for dying
	err := s.Apply(dying(), 5, -1) // request 5 stacks; should be capped at 4
	require.NoError(t, err)
	assert.Equal(t, 4, s.Stacks("dying"))
}

func TestActiveSet_Apply_ZeroMaxStacks_AlwaysOne(t *testing.T) {
	// MaxStacks=0 means unstackable; stacks is always 1
	s := condition.NewActiveSet()
	err := s.Apply(prone(), 3, -1)
	require.NoError(t, err)
	assert.Equal(t, 1, s.Stacks("prone"))
}

func TestActiveSet_Remove(t *testing.T) {
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply(prone(), 1, -1))
	s.Remove("prone")
	assert.False(t, s.Has("prone"))
	assert.Equal(t, 0, s.Stacks("prone"))
}

func TestActiveSet_Remove_NotPresent_NoOp(t *testing.T) {
	s := condition.NewActiveSet()
	s.Remove("nonexistent") // must not panic
	assert.False(t, s.Has("nonexistent"))
}

func TestActiveSet_Tick_DecrementsRounds(t *testing.T) {
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply(frightened(), 2, 3))
	expired := s.Tick()
	assert.Empty(t, expired)
	assert.True(t, s.Has("frightened")) // still present
}

func TestActiveSet_Tick_ExpiresAtZero(t *testing.T) {
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply(frightened(), 1, 1))
	expired := s.Tick()
	assert.Equal(t, []string{"frightened"}, expired)
	assert.False(t, s.Has("frightened"))
}

func TestActiveSet_Tick_PermanentNotExpired(t *testing.T) {
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply(prone(), 1, -1))
	expired := s.Tick()
	assert.Empty(t, expired)
	assert.True(t, s.Has("prone"))
}

func TestActiveSet_Tick_UntilSaveNotExpired(t *testing.T) {
	// until_save conditions are not expired by Tick — they require explicit Remove
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply(dying(), 1, -1))
	expired := s.Tick()
	assert.Empty(t, expired)
	assert.True(t, s.Has("dying"))
}

func TestActiveSet_All_ReturnsCopy(t *testing.T) {
	s := condition.NewActiveSet()
	require.NoError(t, s.Apply(prone(), 1, -1))
	require.NoError(t, s.Apply(frightened(), 2, 2))
	all := s.All()
	assert.Len(t, all, 2)
}

func TestActiveSet_IncrementDyingStacks(t *testing.T) {
	s := condition.NewActiveSet()
	d := dying()
	require.NoError(t, s.Apply(d, 1, -1))
	require.NoError(t, s.Apply(d, 1, -1)) // apply again to increment
	assert.Equal(t, 2, s.Stacks("dying"))
}

func TestPropertyActiveSet_TickNeverBelowMinusOne(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		duration := rapid.IntRange(1, 10).Draw(t, "duration")
		ticks := rapid.IntRange(1, 20).Draw(t, "ticks")
		s := condition.NewActiveSet()
		require.NoError(t, s.Apply(frightened(), 1, duration))
		for i := 0; i < ticks; i++ {
			s.Tick()
		}
		for _, ac := range s.All() {
			assert.GreaterOrEqual(t, ac.DurationRemaining, -1,
				"DurationRemaining must never go below -1")
		}
	})
}

func TestPropertyActiveSet_ApplyRemove_HasFalse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := condition.NewActiveSet()
		require.NoError(t, s.Apply(prone(), 1, -1))
		s.Remove("prone")
		assert.False(t, s.Has("prone"),
			"Has must return false after Remove")
	})
}

func TestPropertyActiveSet_StacksNeverExceedMaxStacks(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxStacks := rapid.IntRange(1, 4).Draw(t, "max_stacks")
		stacks := rapid.IntRange(1, 8).Draw(t, "stacks")
		def := &condition.ConditionDef{
			ID: "test", Name: "Test", DurationType: "rounds", MaxStacks: maxStacks,
		}
		s := condition.NewActiveSet()
		require.NoError(t, s.Apply(def, stacks, 5))
		actual := s.Stacks("test")
		assert.LessOrEqual(t, actual, maxStacks,
			"stacks must never exceed MaxStacks")
	})
}
```

Run: `go test ./internal/game/condition/... -run TestActiveSet -v`
Expected: FAIL

### Step 2: Implement `active.go`

```go
// internal/game/condition/active.go
package condition

// ActiveCondition tracks one applied condition on an entity.
type ActiveCondition struct {
	Def               *ConditionDef
	Stacks            int
	DurationRemaining int // -1 = permanent or until_save
}

// ActiveSet tracks all conditions currently applied to one combatant.
// It is not safe for concurrent use; the caller must serialise access.
type ActiveSet struct {
	conditions map[string]*ActiveCondition
}

// NewActiveSet creates an empty ActiveSet.
func NewActiveSet() *ActiveSet {
	return &ActiveSet{conditions: make(map[string]*ActiveCondition)}
}

// Apply adds or updates a condition on this entity.
// If the condition is already present, stacks are incremented (capped at MaxStacks).
// If MaxStacks == 0 (unstackable), stacks is always stored as 1.
// duration is rounds remaining; use -1 for permanent or until_save.
//
// Postcondition: Has(def.ID) is true after a successful Apply.
func (s *ActiveSet) Apply(def *ConditionDef, stacks, duration int) error {
	// Determine effective stacks
	effectiveStacks := stacks
	if def.MaxStacks == 0 {
		effectiveStacks = 1
	}

	if existing, ok := s.conditions[def.ID]; ok {
		// Increment existing stacks
		newStacks := existing.Stacks + effectiveStacks
		if def.MaxStacks > 0 && newStacks > def.MaxStacks {
			newStacks = def.MaxStacks
		}
		if def.MaxStacks == 0 {
			newStacks = 1
		}
		existing.Stacks = newStacks
		if duration > existing.DurationRemaining {
			existing.DurationRemaining = duration
		}
		return nil
	}

	capped := effectiveStacks
	if def.MaxStacks > 0 && capped > def.MaxStacks {
		capped = def.MaxStacks
	}
	s.conditions[def.ID] = &ActiveCondition{
		Def:               def,
		Stacks:            capped,
		DurationRemaining: duration,
	}
	return nil
}

// Remove deletes the condition with the given ID from the set.
// If the condition is not present, Remove is a no-op.
//
// Postcondition: Has(id) is false.
func (s *ActiveSet) Remove(id string) {
	delete(s.conditions, id)
}

// Tick decrements the DurationRemaining of all "rounds"-type conditions by 1.
// Conditions that reach 0 are removed. "permanent" and "until_save" conditions
// (DurationRemaining == -1) are not affected.
//
// Postcondition: Returns the IDs of conditions that expired this tick.
func (s *ActiveSet) Tick() []string {
	var expired []string
	for id, ac := range s.conditions {
		if ac.Def.DurationType != "rounds" || ac.DurationRemaining < 0 {
			continue
		}
		ac.DurationRemaining--
		if ac.DurationRemaining <= 0 {
			expired = append(expired, id)
			delete(s.conditions, id)
		}
	}
	return expired
}

// Has reports whether the condition with id is currently active.
func (s *ActiveSet) Has(id string) bool {
	_, ok := s.conditions[id]
	return ok
}

// Stacks returns the current stack count for condition id, or 0 if not present.
func (s *ActiveSet) Stacks(id string) int {
	if ac, ok := s.conditions[id]; ok {
		return ac.Stacks
	}
	return 0
}

// All returns a snapshot slice of all active conditions.
func (s *ActiveSet) All() []*ActiveCondition {
	out := make([]*ActiveCondition, 0, len(s.conditions))
	for _, ac := range s.conditions {
		out = append(out, ac)
	}
	return out
}
```

### Step 3: Run tests

Run: `go test ./internal/game/condition/... -v -race`
Expected: all PASS

### Step 4: Commit

```bash
git add internal/game/condition/active.go internal/game/condition/active_test.go
git commit -m "feat(condition): ActiveCondition and ActiveSet with tick/expire logic (Stage 5 Task 2)"
```

---

## Task 3: Modifiers

**Files:**
- Create: `internal/game/condition/modifiers.go`
- Create: `internal/game/condition/modifiers_test.go`

**Context:** Pure functions that compute combat modifiers from an `ActiveSet`. These are called by the combat resolver before each attack roll and AC check. All penalties are returned as negative integers (subtracted from rolls).

### Step 1: Write failing tests

```go
// internal/game/condition/modifiers_test.go
package condition_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/condition"
)

func TestAttackBonus_NoConditions_Zero(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.AttackBonus(s))
}

func TestAttackBonus_Frightened2_MinusTwoToAttack(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, AttackPenalty: 1}
	require.NoError(t, s.Apply(def, 2, 3))
	// frightened 2 = penalty of 2 (stacks * AttackPenalty)
	assert.Equal(t, -2, condition.AttackBonus(s))
}

func TestAttackBonus_Prone_MinusTwo(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2}
	require.NoError(t, s.Apply(def, 1, -1))
	assert.Equal(t, -2, condition.AttackBonus(s))
}

func TestACBonus_NoConditions_Zero(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.ACBonus(s))
}

func TestACBonus_FlatFooted_MinusTwo(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2}
	require.NoError(t, s.Apply(def, 1, 1))
	assert.Equal(t, -2, condition.ACBonus(s))
}

func TestACBonus_Frightened2_MinusTwoToAC(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, ACPenalty: 1}
	require.NoError(t, s.Apply(def, 2, 3))
	assert.Equal(t, -2, condition.ACBonus(s))
}

func TestIsActionRestricted_Stunned_BlocksAttack(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3, RestrictActions: []string{"attack", "strike"}}
	require.NoError(t, s.Apply(def, 1, 2))
	assert.True(t, condition.IsActionRestricted(s, "attack"))
	assert.True(t, condition.IsActionRestricted(s, "strike"))
	assert.False(t, condition.IsActionRestricted(s, "pass"))
}

func TestIsActionRestricted_NoConditions_False(t *testing.T) {
	s := condition.NewActiveSet()
	assert.False(t, condition.IsActionRestricted(s, "attack"))
}

func TestStunnedAPReduction_ReturnsStacks(t *testing.T) {
	s := condition.NewActiveSet()
	def := &condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3}
	require.NoError(t, s.Apply(def, 2, 1))
	assert.Equal(t, 2, condition.StunnedAPReduction(s))
}

func TestStunnedAPReduction_NoStunned_Zero(t *testing.T) {
	s := condition.NewActiveSet()
	assert.Equal(t, 0, condition.StunnedAPReduction(s))
}

func TestPropertyAttackBonus_AlwaysNonPositive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		penalty := rapid.IntRange(0, 10).Draw(t, "penalty")
		stacks := rapid.IntRange(1, 4).Draw(t, "stacks")
		s := condition.NewActiveSet()
		def := &condition.ConditionDef{ID: "test", Name: "Test", DurationType: "permanent", MaxStacks: 4, AttackPenalty: penalty}
		require.NoError(t, s.Apply(def, stacks, -1))
		bonus := condition.AttackBonus(s)
		assert.LessOrEqual(t, bonus, 0, "AttackBonus must always be <= 0")
	})
}

func TestPropertyACBonus_AlwaysNonPositive(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		penalty := rapid.IntRange(0, 10).Draw(t, "penalty")
		stacks := rapid.IntRange(1, 4).Draw(t, "stacks")
		s := condition.NewActiveSet()
		def := &condition.ConditionDef{ID: "test", Name: "Test", DurationType: "permanent", MaxStacks: 4, ACPenalty: penalty}
		require.NoError(t, s.Apply(def, stacks, -1))
		bonus := condition.ACBonus(s)
		assert.LessOrEqual(t, bonus, 0, "ACBonus must always be <= 0")
	})
}
```

Run: `go test ./internal/game/condition/... -run TestAttackBonus -v`
Expected: FAIL

### Step 2: Implement `modifiers.go`

```go
// internal/game/condition/modifiers.go
package condition

// AttackBonus returns the net attack roll modifier from all active conditions.
// For conditions where AttackPenalty is per-stack (e.g. frightened), the penalty
// is multiplied by the current stack count.
//
// Postcondition: Returns <= 0.
func AttackBonus(s *ActiveSet) int {
	total := 0
	for _, ac := range s.conditions {
		if ac.Def.AttackPenalty > 0 {
			total -= ac.Def.AttackPenalty * ac.Stacks
		}
	}
	return total
}

// ACBonus returns the net AC modifier from all active conditions.
//
// Postcondition: Returns <= 0.
func ACBonus(s *ActiveSet) int {
	total := 0
	for _, ac := range s.conditions {
		if ac.Def.ACPenalty > 0 {
			total -= ac.Def.ACPenalty * ac.Stacks
		}
	}
	return total
}

// IsActionRestricted reports whether the given action type string is blocked
// by any active condition's RestrictActions list.
func IsActionRestricted(s *ActiveSet, actionType string) bool {
	for _, ac := range s.conditions {
		for _, r := range ac.Def.RestrictActions {
			if r == actionType {
				return true
			}
		}
	}
	return false
}

// StunnedAPReduction returns the number of AP to subtract from the action queue
// this round due to the stunned condition. Equal to the current stunned stack count.
//
// Postcondition: Returns >= 0.
func StunnedAPReduction(s *ActiveSet) int {
	return s.Stacks("stunned")
}
```

### Step 3: Run all condition tests

Run: `go test ./internal/game/condition/... -v -race`
Expected: all PASS

### Step 4: Commit

```bash
git add internal/game/condition/modifiers.go internal/game/condition/modifiers_test.go
git commit -m "feat(condition): combat modifiers — AttackBonus, ACBonus, action restrictions (Stage 5 Task 3)"
```

---

## Task 4: Starter Condition YAML Files

**Files:**
- Create: `content/conditions/dying.yaml`
- Create: `content/conditions/wounded.yaml`
- Create: `content/conditions/unconscious.yaml`
- Create: `content/conditions/stunned.yaml`
- Create: `content/conditions/frightened.yaml`
- Create: `content/conditions/prone.yaml`
- Create: `content/conditions/flat_footed.yaml`

**Context:** These YAML files define all starter conditions. They are parsed by `condition.LoadDirectory`. Values must be consistent with the combat integration in Task 5 (e.g., `prone` `attack_penalty: 2`, `frightened` `attack_penalty: 1` and `ac_penalty: 1` per stack).

### Step 1: Create the YAML files

```yaml
# content/conditions/dying.yaml
id: dying
name: Dying
description: |
  You are bleeding out. You lose all your actions. At the start of each round,
  make a DC 15 Flat Check (d20, no modifiers). Failure advances dying; success
  recovers you to 1 HP as Wounded 1. Critical success removes dying entirely.
  Reaching dying 4 means death.
duration_type: until_save
max_stacks: 4
attack_penalty: 0
ac_penalty: 0
speed_penalty: 0
restrict_actions:
  - attack
  - strike
  - pass
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

```yaml
# content/conditions/wounded.yaml
id: wounded
name: Wounded
description: |
  You have been grievously injured. Wounded N means that if you gain the dying
  condition, you start at dying (N+1) instead of dying 1.
duration_type: permanent
max_stacks: 3
attack_penalty: 0
ac_penalty: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

```yaml
# content/conditions/unconscious.yaml
id: unconscious
name: Unconscious
description: |
  You are knocked out and cannot take any actions. You regain consciousness
  after combat ends.
duration_type: permanent
max_stacks: 0
attack_penalty: 0
ac_penalty: 4
speed_penalty: 0
restrict_actions:
  - attack
  - strike
  - pass
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

```yaml
# content/conditions/stunned.yaml
id: stunned
name: Stunned
description: |
  You are stunned and lose actions. Stunned N means you lose N action points
  at the start of your turn this round. Stunned reduces by 1 each round.
duration_type: rounds
max_stacks: 3
attack_penalty: 0
ac_penalty: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

```yaml
# content/conditions/frightened.yaml
id: frightened
name: Frightened
description: |
  You are frightened. You take a penalty to attack rolls and AC equal to your
  frightened value. Frightened reduces by 1 at the end of each round.
duration_type: rounds
max_stacks: 4
attack_penalty: 1
ac_penalty: 1
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

```yaml
# content/conditions/prone.yaml
id: prone
name: Prone
description: |
  You are lying on the ground. You take a -2 penalty to attack rolls.
  Melee attackers gain a +2 bonus against you; ranged attackers take a -2 penalty.
  Standing up costs 1 action point.
duration_type: permanent
max_stacks: 0
attack_penalty: 2
ac_penalty: 0
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

```yaml
# content/conditions/flat_footed.yaml
id: flat_footed
name: Flat-Footed
description: |
  You are caught off guard. You take a -2 penalty to AC.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 2
speed_penalty: 0
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

### Step 2: Verify YAML loads cleanly

Run: `go test ./internal/game/condition/... -run TestLoadDirectory -v`
Expected: PASS (the existing test uses a temp dir, not content/; this confirms the schema is parseable)

Write a quick integration smoke test to confirm the real files load:

```go
// In definition_test.go, add:
func TestLoadDirectory_RealConditions(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)
	for _, id := range []string{"dying", "wounded", "unconscious", "stunned", "frightened", "prone", "flat_footed"} {
		_, ok := reg.Get(id)
		assert.True(t, ok, "condition %q must be present", id)
	}
}
```

Run: `go test ./internal/game/condition/... -run TestLoadDirectory_RealConditions -v`
Expected: PASS

### Step 3: Commit

```bash
git add content/conditions/ internal/game/condition/definition_test.go
git commit -m "feat(condition): starter condition YAML files + real-file load test (Stage 5 Task 4)"
```

---

## Task 5: Combat Engine — Conditions Integration

**Files:**
- Modify: `internal/game/combat/engine.go`
- Modify: `internal/game/combat/round.go`
- Create: `internal/game/combat/engine_conditions_test.go`
- Create: `internal/game/combat/round_conditions_test.go`

**Context:** Wire `condition.ActiveSet` into `Combat`. Add `ApplyCondition`, `RemoveCondition`, `GetConditions`. Extend `StartRound` with dying recovery checks and stunned AP reduction. Extend `ResolveRound` to apply modifiers and trigger conditions from attack outcomes. The `Combat` struct needs a reference to the `*condition.Registry` for lookups.

### Step 1: Write failing tests

```go
// internal/game/combat/engine_conditions_test.go
package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
)

func makeConditionReg() *condition.Registry {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4, RestrictActions: []string{"attack", "strike", "pass"}})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, AttackPenalty: 1, ACPenalty: 1})
	return reg
}

func makeCombatWithConditions(t *testing.T) (*combat.Engine, *combat.Combat) {
	t.Helper()
	reg := makeConditionReg()
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1, Initiative: 15},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 12, CurrentHP: 12, AC: 12, Level: 1, StrMod: 1, DexMod: 0, Initiative: 10},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg)
	require.NoError(t, err)
	return eng, cbt
}

func TestApplyCondition_Prone(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	err := cbt.ApplyCondition("p1", "prone", 1, -1)
	require.NoError(t, err)
	conds := cbt.GetConditions("p1")
	require.Len(t, conds, 1)
	assert.Equal(t, "prone", conds[0].Def.ID)
}

func TestApplyCondition_UnknownID_ReturnsError(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	err := cbt.ApplyCondition("p1", "nonexistent", 1, -1)
	assert.Error(t, err)
}

func TestApplyCondition_UnknownUID_ReturnsError(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	err := cbt.ApplyCondition("nobody", "prone", 1, -1)
	assert.Error(t, err)
}

func TestRemoveCondition_RemovesIt(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1))
	cbt.RemoveCondition("p1", "prone")
	conds := cbt.GetConditions("p1")
	assert.Empty(t, conds)
}

func TestGetConditions_Empty(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	conds := cbt.GetConditions("p1")
	assert.Empty(t, conds)
}

func TestStartRound_TicksConditions(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "flat_footed", 1, 1))
	// flat_footed duration=1; after tick it should be removed
	events := cbt.StartRound(3)
	assert.False(t, cbt.HasCondition("p1", "flat_footed"))
	_ = events // condition expired events emitted
}

func TestStartRound_StunnedReducesAP(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	require.NoError(t, cbt.ApplyCondition("p1", "stunned", 2, 2))
	_ = cbt.StartRound(3)
	// p1 has 3 AP max - 2 stunned = 1 remaining
	q, ok := cbt.ActionQueues["p1"]
	require.True(t, ok)
	assert.Equal(t, 1, q.RemainingPoints())
}

func TestStartRound_DyingRecovery_Success(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	// set player to 0 HP first
	cbt.Combatants[0].CurrentHP = 0
	require.NoError(t, cbt.ApplyCondition("p1", "dying", 1, -1))
	// inject a controlled src that returns 14 (d20 roll = 15, success >= 15)
	events := cbt.StartRoundWithSrc(3, &fixedSrc{val: 14}) // roll+1 = 15
	_ = events
	// dying removed, wounded applied, HP restored
	assert.False(t, cbt.HasCondition("p1", "dying"))
	assert.True(t, cbt.HasCondition("p1", "wounded"))
	assert.Equal(t, 1, cbt.Combatants[0].CurrentHP)
}

func TestStartRound_DyingRecovery_Failure(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	cbt.Combatants[0].CurrentHP = 0
	require.NoError(t, cbt.ApplyCondition("p1", "dying", 1, -1))
	// inject src that returns 9 (d20=10, failure < 15)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 9})
	assert.True(t, cbt.HasCondition("p1", "dying"))
	assert.Equal(t, 2, cbt.DyingStacks("p1")) // advanced from 1 to 2
}

func TestStartRound_DyingRecovery_DyingFour_Death(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	cbt.Combatants[0].CurrentHP = 0
	require.NoError(t, cbt.ApplyCondition("p1", "dying", 3, -1))
	// failure on dying 3 → dying 4 → dead
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 9})
	assert.True(t, cbt.Combatants[0].IsDead())
}

func TestStartRound_DyingRecovery_CritSuccess(t *testing.T) {
	_, cbt := makeCombatWithConditions(t)
	cbt.Combatants[0].CurrentHP = 0
	require.NoError(t, cbt.ApplyCondition("p1", "dying", 1, -1))
	// crit success: d20 roll=24+1=25
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 24})
	assert.False(t, cbt.HasCondition("p1", "dying"))
	assert.False(t, cbt.HasCondition("p1", "wounded"))
	assert.Equal(t, 1, cbt.Combatants[0].CurrentHP)
}

func TestPropertyDyingStacksNeverExceedFour(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		_, cbt := makeCombatWithConditions(t)
		stacks := rapid.IntRange(1, 4).Draw(t, "stacks")
		cbt.Combatants[0].CurrentHP = 0
		require.NoError(t, cbt.ApplyCondition("p1", "dying", stacks, -1))
		assert.LessOrEqual(t, cbt.DyingStacks("p1"), 4)
	})
}

// fixedSrc returns val for all Intn calls; used to control dice in tests.
type fixedSrc struct{ val int }
func (f *fixedSrc) Intn(_ int) int { return f.val }
```

```go
// internal/game/combat/round_conditions_test.go
package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

func makeCombatForRoundConditions(t *testing.T) *combat.Combat {
	t.Helper()
	reg := makeConditionReg()
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1, Initiative: 15},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 12, CurrentHP: 12, AC: 12, Level: 1, StrMod: 1, DexMod: 0, Initiative: 10},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg)
	require.NoError(t, err)
	return cbt
}

func TestResolveRound_CritFailure_PronApplied(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	// Queue attack from p1 to Ganger
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	// Intn(20)=0 → d20=1, atkTotal=1+2+2=5 vs AC=12 → CritFailure (< 12-10=2 is impossible, 5<12 so Failure)
	// To force CritFailure: need atkTotal < AC-10 = 12-10 = 2.
	// With Intn(20)=0 → d20=1, mods=4, total=5. Not crit. Use AC=20 to force it:
	cbt.Combatants[1].AC = 20
	// Now atkTotal=5, AC=20, AC-10=10, 5<10 → CritFailure

	events := combat.ResolveRound(cbt, &fixedSrc{val: 0}, nil)
	_ = events
	assert.True(t, cbt.HasCondition("p1", "prone"), "attacker should be prone after crit failure")
}

func TestResolveRound_CritSuccess_FlatFootedApplied(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	// To force CritSuccess: atkTotal >= AC+10. Intn(20)=19 → d20=20, mods=4, total=24. AC=12+10=22. 24>=22 → CritSuccess.
	events := combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil)
	_ = events
	assert.True(t, cbt.HasCondition("n1", "flat_footed"), "target should be flat-footed after crit success")
}

func TestResolveRound_ZeroHP_DyingApplied(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	// Set Ganger HP to 1 so a single hit kills
	cbt.Combatants[1].CurrentHP = 1
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	// Intn(20)=19 → crit success, 2x damage. Ganger AC=12, total=24 crit. Damage = (1d6+2)*2. Intn(6)=5 → dmg=(5+1+2)*2=16 > 1.
	_ = combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil)
	// Ganger is now at 0 HP — but it's an NPC, not a player.
	// NPCs just die; dying condition only applies to players.
	assert.True(t, cbt.Combatants[1].IsDead())
	assert.False(t, cbt.HasCondition("n1", "dying"), "NPCs should not get the dying condition — they just die")
}

func TestResolveRound_PlayerZeroHP_DyingApplied(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	// Swap so NPC attacks player; force player to 1 HP
	cbt.Combatants[0].CurrentHP = 1
	cbt.Combatants[0].AC = 1 // guarantee hit
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Alice"}))

	_ = combat.ResolveRound(cbt, &fixedSrc{val: 19}, nil)
	assert.True(t, cbt.HasCondition("p1", "dying"), "player should get dying condition at 0 HP")
	assert.False(t, cbt.Combatants[0].IsDead(), "player must not be marked dead immediately — dying chain handles this")
}

func TestResolveRound_AttackModifiersApplied(t *testing.T) {
	cbt := makeCombatForRoundConditions(t)
	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.ApplyCondition("p1", "prone", 1, -1)) // -2 attack
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	// Intn(20)=5 → d20=6, base_mods=4, prone=-2, total=8
	events := combat.ResolveRound(cbt, &fixedSrc{val: 5}, nil)
	require.NotEmpty(t, events)
	var attackEvent *combat.RoundEvent
	for i := range events {
		if events[i].AttackResult != nil {
			attackEvent = &events[i]
			break
		}
	}
	require.NotNil(t, attackEvent)
	assert.Equal(t, 8, attackEvent.AttackResult.AttackTotal)
}
```

Run: `go test ./internal/game/combat/... -run TestApplyCondition -v`
Expected: FAIL with "StartCombat: too many arguments"

### Step 2: Update `engine.go`

Add `condRegistry *condition.Registry` and `Conditions map[string]*condition.ActiveSet` to `Combat`. Update `StartCombat` signature. Add `ApplyCondition`, `RemoveCondition`, `GetConditions`, `HasCondition`, `DyingStacks`, `StartRoundWithSrc`.

Key changes to `engine.go`:

```go
// Combat holds the live state of a single combat encounter in a room.
type Combat struct {
	RoomID       string
	Combatants   []*Combatant
	turnIndex    int
	Over         bool
	Round        int
	ActionQueues map[string]*ActionQueue
	Conditions   map[string]*condition.ActiveSet  // keyed by combatant UID
	condRegistry *condition.Registry
}

// StartCombat begins a new combat. condRegistry provides condition definitions
// for applying conditions during combat resolution.
//
// Precondition: roomID non-empty; combatants >= 2; condRegistry non-nil.
// Postcondition: Returns the new Combat or error if already active.
func (e *Engine) StartCombat(roomID string, combatants []*Combatant, condRegistry *condition.Registry) (*Combat, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.combats[roomID]; exists {
		return nil, fmt.Errorf("combat already active in room %q", roomID)
	}
	sorted := make([]*Combatant, len(combatants))
	copy(sorted, combatants)
	sortByInitiativeDesc(sorted)
	cbt := &Combat{
		RoomID:       roomID,
		Combatants:   sorted,
		ActionQueues: make(map[string]*ActionQueue),
		Conditions:   make(map[string]*condition.ActiveSet),
		condRegistry: condRegistry,
	}
	for _, c := range sorted {
		cbt.Conditions[c.ID] = condition.NewActiveSet()
	}
	e.combats[roomID] = cbt
	return cbt, nil
}

// ApplyCondition applies condition condID with stacks/duration to combatant uid.
// Returns error if uid or condID not found in the registry.
func (c *Combat) ApplyCondition(uid, condID string, stacks, duration int) error {
	def, ok := c.condRegistry.Get(condID)
	if !ok {
		return fmt.Errorf("unknown condition %q", condID)
	}
	s, ok := c.Conditions[uid]
	if !ok {
		return fmt.Errorf("combatant %q not found", uid)
	}
	return s.Apply(def, stacks, duration)
}

// RemoveCondition removes condID from combatant uid. No-op if not present.
func (c *Combat) RemoveCondition(uid, condID string) {
	if s, ok := c.Conditions[uid]; ok {
		s.Remove(condID)
	}
}

// GetConditions returns a snapshot of active conditions for uid.
func (c *Combat) GetConditions(uid string) []*condition.ActiveCondition {
	if s, ok := c.Conditions[uid]; ok {
		return s.All()
	}
	return nil
}

// HasCondition reports whether uid has condition condID active.
func (c *Combat) HasCondition(uid, condID string) bool {
	if s, ok := c.Conditions[uid]; ok {
		return s.Has(condID)
	}
	return false
}

// DyingStacks returns the current dying stack count for uid, or 0.
func (c *Combat) DyingStacks(uid string) int {
	if s, ok := c.Conditions[uid]; ok {
		return s.Stacks("dying")
	}
	return 0
}
```

Update `StartRound` to accept a `src Source` parameter for dying recovery checks, and add `StartRoundWithSrc`. The original `StartRound` calls `StartRoundWithSrc` with a live dice source.

Add `RoundConditionEvent` to represent condition changes during a round:

```go
// RoundConditionEvent records a condition applied or removed during a round.
type RoundConditionEvent struct {
	UID         string
	Name        string
	ConditionID string
	CondName    string
	Stacks      int
	Applied     bool // true=applied, false=removed/expired
}
```

Update `StartRound` signature to return `[]RoundConditionEvent` alongside the dead/recovery events:

```go
// StartRound increments Round, ticks conditions, applies dying recovery checks,
// and resets ActionQueues.
//
// Postcondition: Round incremented; conditions ticked; dying recovery resolved;
// returns slice of condition events that occurred during startup.
func (c *Combat) StartRound(actionsPerRound int) []RoundConditionEvent {
	return c.StartRoundWithSrc(actionsPerRound, realSrc{})
}

// StartRoundWithSrc is like StartRound but accepts a Source for dice rolls (testable).
func (c *Combat) StartRoundWithSrc(actionsPerRound int, src Source) []RoundConditionEvent {
	c.Round++
	var events []RoundConditionEvent

	for _, cbt := range c.Combatants {
		if cbt.IsDead() {
			continue
		}
		s := c.Conditions[cbt.ID]

		// Tick durations; collect expired
		expired := s.Tick()
		for _, id := range expired {
			events = append(events, RoundConditionEvent{
				UID: cbt.ID, Name: cbt.Name, ConditionID: id, Applied: false,
			})
		}

		// Dying recovery check
		if s.Has("dying") {
			dyingStacks := s.Stacks("dying")
			roll := src.Intn(20) + 1
			switch {
			case roll >= 25: // crit success
				s.Remove("dying")
				cbt.CurrentHP = 1
				events = append(events, RoundConditionEvent{UID: cbt.ID, Name: cbt.Name, ConditionID: "dying", Applied: false})
			case roll >= 15: // success
				s.Remove("dying")
				def, _ := c.condRegistry.Get("wounded")
				_ = s.Apply(def, 1, -1)
				cbt.CurrentHP = 1
				events = append(events, RoundConditionEvent{UID: cbt.ID, Name: cbt.Name, ConditionID: "dying", Applied: false})
				events = append(events, RoundConditionEvent{UID: cbt.ID, Name: cbt.Name, ConditionID: "wounded", Applied: true})
			default: // failure
				dyingStacks++
				if dyingStacks >= 4 {
					// Dead
					cbt.CurrentHP = 0
					s.Remove("dying")
					events = append(events, RoundConditionEvent{UID: cbt.ID, Name: cbt.Name, ConditionID: "dying", Applied: false})
				} else {
					def, _ := c.condRegistry.Get("dying")
					s.Remove("dying")
					_ = s.Apply(def, dyingStacks, -1)
					events = append(events, RoundConditionEvent{UID: cbt.ID, Name: cbt.Name, ConditionID: "dying", Stacks: dyingStacks, Applied: true})
				}
			}
		}
	}

	// Reset action queues with stunned AP reduction
	c.ActionQueues = make(map[string]*ActionQueue)
	for _, cbt := range c.Combatants {
		if cbt.IsDead() {
			continue
		}
		ap := actionsPerRound
		s := c.Conditions[cbt.ID]
		reduction := condition.StunnedAPReduction(s)
		ap -= reduction
		if ap < 0 {
			ap = 0
		}
		c.ActionQueues[cbt.ID] = NewActionQueue(cbt.ID, ap)
	}

	return events
}

// realSrc wraps the global math/rand for production use.
type realSrc struct{}
func (realSrc) Intn(n int) int { return rand.Intn(n) }
```

Add `math/rand` import to `engine.go`. Add `condition` import.

### Step 3: Update `round.go`

Update `ResolveRound` to apply conditions from attack outcomes. After each attack result in `ActionAttack` and `ActionStrike`:

```go
// After damage application for ActionAttack / ActionStrike first hit:
switch r.Outcome {
case CritFailure:
    _ = cbt.ApplyCondition(actor.ID, "prone", 1, -1)
case CritSuccess:
    _ = cbt.ApplyCondition(target.ID, "flat_footed", 1, 1)
}
if target.CurrentHP <= 0 && target.Kind == KindPlayer {
    woundedStacks := cbt.Conditions[target.ID].Stacks("wounded")
    _ = cbt.ApplyCondition(target.ID, "dying", 1+woundedStacks, -1)
}
```

Apply modifiers before each attack roll in `ActionAttack` and both strikes in `ActionStrike`:

```go
// In ResolveAttack call sites, pass the Combat so modifiers can be read:
// Instead of ResolveAttack(actor, target, src), we need:
atkBonus := condition.AttackBonus(cbt.Conditions[actor.ID])
acBonus := condition.ACBonus(cbt.Conditions[target.ID])
r := ResolveAttack(actor, target, src)
r.AttackTotal += atkBonus
r.AttackTotal += acBonus   // acBonus is negative, so this effectively reduces the roll vs a harder AC
r.Outcome = OutcomeFor(r.AttackTotal, target.AC)
```

Note: `condition` package must be imported in `round.go`.

### Step 4: Fix all callers of `StartCombat` and `StartRound`

The signature changes will break `combat_handler.go` in gameserver. Fix:
- `engine.StartCombat(roomID, combatants)` → `engine.StartCombat(roomID, combatants, condRegistry)`
- `cbt.StartRound(3)` → `cbt.StartRound(3)` (unchanged; returns `[]RoundConditionEvent` which can be ignored with `_`)

Update `CombatHandler` to hold `condRegistry *condition.Registry` and pass it to `StartCombat`.

### Step 5: Run all combat tests

Run: `go test ./internal/game/combat/... -v -race`
Expected: all PASS

### Step 6: Commit

```bash
git add internal/game/combat/
git commit -m "feat(combat): conditions integration — ApplyCondition, dying chain, attack modifiers (Stage 5 Task 5)"
```

---

## Task 6: Proto Additions

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/gamev1/proto_test.go`

**Context:** Add `StatusRequest` to `ClientMessage`, `ConditionEvent` to `ServerEvent`, and `ConditionInfo` message. Extend `RoomView` with `repeated ConditionInfo active_conditions`. Run `make proto` to regenerate.

### Step 1: Edit `game.proto`

In `ClientMessage` oneof, add:
```protobuf
StatusRequest status = 15;
```

In `ServerEvent` oneof, add:
```protobuf
ConditionEvent condition_event = 14;
```

Add new messages after the existing ones:
```protobuf
// StatusRequest asks the server to return the player's active conditions.
message StatusRequest {}

// ConditionEvent reports a condition being applied to or removed from an entity.
message ConditionEvent {
  string target_uid    = 1;
  string target_name   = 2;
  string condition_id  = 3;
  string condition_name = 4;
  int32  stacks        = 5;
  bool   applied       = 6;  // true = applied, false = removed/expired
}

// ConditionInfo describes one active condition on a combatant.
message ConditionInfo {
  string id                 = 1;
  string name               = 2;
  int32  stacks             = 3;
  int32  duration_remaining = 4;  // -1 = permanent or until_save
}
```

Extend `RoomView`:
```protobuf
// Add to existing RoomView message:
repeated ConditionInfo active_conditions = 6;
```

### Step 2: Regenerate proto

Run: `make proto`
Expected: regenerated files in `internal/gameserver/gamev1/`

### Step 3: Add roundtrip tests

In `internal/gameserver/gamev1/proto_test.go`, add:

```go
func TestStatusRequest_Roundtrip(t *testing.T) {
	orig := &gamev1.ClientMessage{
		RequestId: "r1",
		Payload:   &gamev1.ClientMessage_Status{Status: &gamev1.StatusRequest{}},
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	var got gamev1.ClientMessage
	require.NoError(t, proto.Unmarshal(data, &got))
	_, ok := got.Payload.(*gamev1.ClientMessage_Status)
	assert.True(t, ok)
}

func TestConditionEvent_Roundtrip(t *testing.T) {
	orig := &gamev1.ConditionEvent{
		TargetUid:     "p1",
		TargetName:    "Alice",
		ConditionId:   "prone",
		ConditionName: "Prone",
		Stacks:        1,
		Applied:       true,
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	var got gamev1.ConditionEvent
	require.NoError(t, proto.Unmarshal(data, &got))
	assert.Equal(t, orig.ConditionId, got.ConditionId)
	assert.Equal(t, orig.Applied, got.Applied)
}

func TestConditionInfo_InRoomView_Roundtrip(t *testing.T) {
	orig := &gamev1.RoomView{
		RoomId: "r1",
		Title:  "Test Room",
		ActiveConditions: []*gamev1.ConditionInfo{
			{Id: "prone", Name: "Prone", Stacks: 1, DurationRemaining: -1},
		},
	}
	data, err := proto.Marshal(orig)
	require.NoError(t, err)
	var got gamev1.RoomView
	require.NoError(t, proto.Unmarshal(data, &got))
	require.Len(t, got.ActiveConditions, 1)
	assert.Equal(t, "prone", got.ActiveConditions[0].Id)
}
```

Run: `go test ./internal/gameserver/gamev1/... -v`
Expected: all PASS

### Step 4: Commit

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat(proto): StatusRequest, ConditionEvent, ConditionInfo, RoomView.active_conditions (Stage 5 Task 6)"
```

---

## Task 7: Gameserver — Condition Wiring + Status Handler

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `cmd/gameserver/main.go`
- Modify: `internal/gameserver/combat_handler_test.go`

**Context:** Wire the `condition.Registry` into `CombatHandler`. Convert `RoundConditionEvent` → `*gamev1.ConditionEvent` and broadcast alongside combat events. Add `Status(uid)` handler. Add `StatusRequest` case in `grpc_service.go`.

### Step 1: Update `combat_handler.go`

Add `condRegistry *condition.Registry` field to `CombatHandler`. Update constructor:

```go
func NewCombatHandler(
    engine *combat.Engine,
    npcMgr *npc.Manager,
    sessions *session.Manager,
    diceRoller *dice.Roller,
    broadcastFn func(roomID string, events []*gamev1.CombatEvent),
    roundDuration time.Duration,
    condRegistry *condition.Registry,
) *CombatHandler {
    return &CombatHandler{
        engine:        engine,
        npcMgr:        npcMgr,
        sessions:      sessions,
        dice:          diceRoller,
        broadcastFn:   broadcastFn,
        roundDuration: roundDuration,
        timers:        make(map[string]*combat.RoundTimer),
        condRegistry:  condRegistry,
    }
}
```

In `startCombatLocked`, pass `condRegistry` to `engine.StartCombat`.

In `resolveAndAdvance`, after calling `combat.ResolveRound`, collect `RoundConditionEvent`s from `cbt.StartRound` (called at round start) and convert them to `ServerEvent`s for broadcast:

```go
func conditionEventToProto(e combat.RoundConditionEvent, reg *condition.Registry) *gamev1.ServerEvent {
    def, _ := reg.Get(e.ConditionID)
    name := e.ConditionID
    if def != nil {
        name = def.Name
    }
    return &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_ConditionEvent{
            ConditionEvent: &gamev1.ConditionEvent{
                TargetUid:     e.UID,
                TargetName:    e.Name,
                ConditionId:   e.ConditionID,
                ConditionName: name,
                Stacks:        int32(e.Stacks),
                Applied:       e.Applied,
            },
        },
    }
}
```

Add `Status(uid string) ([]*condition.ActiveCondition, error)`:

```go
func (h *CombatHandler) Status(uid string) ([]*condition.ActiveCondition, error) {
    sess, ok := h.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }
    h.combatMu.Lock()
    defer h.combatMu.Unlock()
    cbt, ok := h.engine.GetCombat(sess.RoomID)
    if !ok {
        return nil, nil // no combat active; no conditions
    }
    return cbt.GetConditions(uid), nil
}
```

### Step 2: Update `grpc_service.go`

Add `case *gamev1.ClientMessage_Status` to the command dispatch switch:

```go
case *gamev1.ClientMessage_Status:
    if err := s.handleStatus(stream, uid, msg.RequestId); err != nil {
        s.logger.Error("status handler error", zap.Error(err))
    }
```

Add `handleStatus`:

```go
func (s *GameServiceServer) handleStatus(stream gamev1.GameService_SessionServer, uid, reqID string) error {
    conds, err := s.combatH.Status(uid)
    if err != nil {
        return sendError(stream, reqID, err.Error())
    }
    infos := make([]*gamev1.ConditionInfo, 0, len(conds))
    for _, ac := range conds {
        infos = append(infos, &gamev1.ConditionInfo{
            Id:                ac.Def.ID,
            Name:              ac.Def.Name,
            Stacks:            int32(ac.Stacks),
            DurationRemaining: int32(ac.DurationRemaining),
        })
    }
    // Send as a RoomView with only active_conditions populated (reuse existing message)
    // or send individual ConditionEvent per condition. Use ConditionEvents for consistency.
    for _, info := range infos {
        ev := &gamev1.ServerEvent{
            RequestId: reqID,
            Payload: &gamev1.ServerEvent_ConditionEvent{
                ConditionEvent: &gamev1.ConditionEvent{
                    TargetUid:     uid,
                    ConditionId:   info.Id,
                    ConditionName: info.Name,
                    Stacks:        info.Stacks,
                    Applied:       true,
                },
            },
        }
        if err := stream.Send(ev); err != nil {
            return err
        }
    }
    if len(infos) == 0 {
        // Send empty message so frontend knows the response is complete
        _ = stream.Send(&gamev1.ServerEvent{
            RequestId: reqID,
            Payload:   &gamev1.ServerEvent_ConditionEvent{ConditionEvent: &gamev1.ConditionEvent{}},
        })
    }
    return nil
}
```

### Step 3: Update `cmd/gameserver/main.go`

Load condition registry and pass it to `NewCombatHandler`:

```go
condDir := filepath.Join("content", "conditions")
condRegistry, err := condition.LoadDirectory(condDir)
if err != nil {
    logger.Fatal("loading condition definitions", zap.Error(err))
}
// Pass condRegistry to NewCombatHandler
combatHandler := gameserver.NewCombatHandler(
    combatEngine, npcManager, sessionManager, diceRoller,
    broadcastFn, roundDuration, condRegistry,
)
```

Add import: `"github.com/cory-johannsen/mud/internal/game/condition"` and `"path/filepath"`.

### Step 4: Update `combat_handler_test.go`

Add `makeConditionReg()` helper (same as in combat tests) and pass it to `NewCombatHandler` in the test setup. Add test:

```go
func TestCombatHandler_Status_NoActiveCombat(t *testing.T) {
    h := makeTestCombatHandler(t)
    conds, err := h.Status("p1")
    assert.NoError(t, err)
    assert.Nil(t, conds) // no combat active
}
```

### Step 5: Run all gameserver tests

Run: `go test ./internal/gameserver/... -v -race`
Expected: all PASS

### Step 6: Commit

```bash
git add internal/gameserver/ cmd/gameserver/
git commit -m "feat(gameserver): condition registry wiring, Status handler, ConditionEvent broadcast (Stage 5 Task 7)"
```

---

## Task 8: Frontend — status Command + Renderers

**Files:**
- Modify: `internal/game/command/commands.go`
- Modify: `internal/frontend/handlers/game_bridge.go`
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/text_renderer_test.go`

**Context:** Add `status`/`st` command. Add `ConditionEvent` handler in the event loop. Add `RenderConditionEvent` and `RenderStatus` renderers.

### Step 1: Add `status` command to `commands.go`

In the existing command list, add after `HandlerStrike`:

```go
const HandlerStatus = "status"
```

In the registered commands slice, add:
```go
{Name: "status", Aliases: []string{"st"}, Help: "Show your active conditions.", Category: CategoryCombat, Handler: HandlerStatus},
```

### Step 2: Add dispatch in `game_bridge.go`

In the command dispatch switch, add:
```go
case command.HandlerStatus:
    msg = &gamev1.ClientMessage{
        RequestId: newRequestID(),
        Payload:   &gamev1.ClientMessage_Status{Status: &gamev1.StatusRequest{}},
    }
```

In `forwardServerEvents`, add a case for `ConditionEvent`:
```go
case *gamev1.ServerEvent_ConditionEvent:
    ce := p.ConditionEvent
    if ce.ConditionId == "" {
        // empty sentinel from handleStatus when no conditions
        _ = conn.WriteLine(telnet.Colorize(telnet.Cyan, "No active conditions."))
        continue
    }
    _ = conn.WriteLine(RenderConditionEvent(ce))
```

### Step 3: Add renderers in `text_renderer.go`

```go
// RenderConditionEvent formats a ConditionEvent as colored Telnet text.
func RenderConditionEvent(ce *gamev1.ConditionEvent) string {
    if ce.Applied {
        return telnet.Colorf(telnet.BrightRed, "[CONDITION] %s is now %s (stacks: %d).",
            ce.TargetName, ce.ConditionName, ce.Stacks)
    }
    return telnet.Colorf(telnet.Cyan, "[CONDITION] %s fades from %s.",
        ce.ConditionName, ce.TargetName)
}

// RenderStatus formats a slice of ConditionInfo as a bulleted condition list.
func RenderStatus(conds []*gamev1.ConditionInfo) string {
    if len(conds) == 0 {
        return telnet.Colorize(telnet.Cyan, "No active conditions.")
    }
    var b strings.Builder
    b.WriteString(telnet.Colorize(telnet.BrightWhite, "Active conditions:"))
    b.WriteString("\r\n")
    for _, c := range conds {
        dur := "permanent"
        if c.DurationRemaining >= 0 {
            dur = fmt.Sprintf("%d rounds", c.DurationRemaining)
        }
        b.WriteString(telnet.Colorf(telnet.Yellow, "  %-20s stacks: %d  (%s)", c.Name, c.Stacks, dur))
        b.WriteString("\r\n")
    }
    return b.String()
}
```

### Step 4: Write renderer tests

```go
// In text_renderer_test.go, add:
func TestRenderConditionEvent_Applied(t *testing.T) {
    ce := &gamev1.ConditionEvent{
        TargetName:    "Alice",
        ConditionName: "Prone",
        ConditionId:   "prone",
        Stacks:        1,
        Applied:       true,
    }
    result := handlers.RenderConditionEvent(ce)
    assert.Contains(t, result, "Alice")
    assert.Contains(t, result, "Prone")
    assert.Contains(t, result, "CONDITION")
}

func TestRenderConditionEvent_Removed(t *testing.T) {
    ce := &gamev1.ConditionEvent{
        TargetName:    "Alice",
        ConditionName: "Frightened",
        ConditionId:   "frightened",
        Stacks:        0,
        Applied:       false,
    }
    result := handlers.RenderConditionEvent(ce)
    assert.Contains(t, result, "fades")
    assert.Contains(t, result, "Alice")
}

func TestRenderStatus_Empty(t *testing.T) {
    result := handlers.RenderStatus(nil)
    assert.Contains(t, result, "No active conditions")
}

func TestRenderStatus_WithConditions(t *testing.T) {
    conds := []*gamev1.ConditionInfo{
        {Id: "frightened", Name: "Frightened", Stacks: 2, DurationRemaining: 3},
        {Id: "wounded", Name: "Wounded", Stacks: 1, DurationRemaining: -1},
    }
    result := handlers.RenderStatus(conds)
    assert.Contains(t, result, "Frightened")
    assert.Contains(t, result, "Wounded")
    assert.Contains(t, result, "permanent")
    assert.Contains(t, result, "3 rounds")
}
```

Run: `go test ./internal/frontend/handlers/... -v`
Expected: all PASS

### Step 5: Update `showGameHelp` in `game_bridge.go`

`status` is already added to `CategoryCombat` — verify it appears in the combat section of help output. The existing `showGameHelp` loops over categories, so no change needed if `CategoryCombat` is already in the map. Confirm by checking the existing category map includes `{command.CategoryCombat, "Combat"}`.

### Step 6: Commit

```bash
git add internal/game/command/commands.go internal/frontend/handlers/
git commit -m "feat(frontend): status command, RenderConditionEvent, RenderStatus (Stage 5 Task 8)"
```

---

## Task 9: Final Verification

**Goal:** Confirm the full Stage 5 implementation passes all tests and builds cleanly.

### Step 1: Run full test suite

Run: `go test ./... -race -timeout 5m`
Expected: all packages PASS (skip `internal/storage/postgres` if Docker is unavailable)

### Step 2: Build both binaries

Run: `go build ./cmd/frontend/... && go build ./cmd/gameserver/...`
Expected: no errors

### Step 3: Verify coverage on new packages

Run: `go test ./internal/game/condition/... -cover`
Expected: coverage ≥ 80%

Run: `go test ./internal/game/combat/... -cover`
Expected: coverage ≥ 80%

### Step 4: Commit

If all passes without changes needed, record the verification:

```bash
git tag stage5-complete
```

---

## Verification Checklist

- [ ] `internal/game/condition` package compiles and all tests pass
- [ ] `content/conditions/*.yaml` all load without error
- [ ] `go test ./internal/game/combat/... -race` passes
- [ ] `go test ./internal/gameserver/... -race` passes
- [ ] `go test ./internal/frontend/handlers/... -race` passes
- [ ] `go build ./cmd/gameserver/...` succeeds
- [ ] `go build ./cmd/frontend/...` succeeds
- [ ] `status` command is visible in the combat help section
- [ ] Crit failure in combat applies `prone` to attacker
- [ ] Crit success applies `flat_footed` to target
- [ ] Player at 0 HP gets `dying 1` (not immediately dead)
- [ ] Dying recovery check: success → wounded + 1 HP; failure → dying advances; dying 4 → dead
- [ ] Frightened/prone modifiers reduce attack totals correctly
