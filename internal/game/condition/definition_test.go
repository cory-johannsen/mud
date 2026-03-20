package condition_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml3 "gopkg.in/yaml.v3"
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
	// Mutating the returned slice must not affect the registry
	all[0] = nil
	all2 := reg.All()
	assert.Len(t, all2, 2)
	for _, d := range all2 {
		assert.NotNil(t, d, "registry must not be corrupted by mutating the returned slice")
	}
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

func TestLoadDirectory_NonexistentDir_ReturnsError(t *testing.T) {
	_, err := condition.LoadDirectory("/nonexistent/path/that/does/not/exist")
	assert.Error(t, err)
}

func TestRegistry_Register_OverwritesDuplicate(t *testing.T) {
	reg := condition.NewRegistry()
	first := &condition.ConditionDef{ID: "prone", Name: "First", DurationType: "permanent"}
	second := &condition.ConditionDef{ID: "prone", Name: "Second", DurationType: "permanent"}
	reg.Register(first)
	reg.Register(second)
	got, ok := reg.Get("prone")
	require.True(t, ok)
	assert.Equal(t, "Second", got.Name, "second registration must overwrite the first")
}

func TestLoadDirectory_RealConditions(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)
	for _, id := range []string{"dying", "wounded", "unconscious", "stunned", "frightened", "prone", "flat_footed", "grabbed", "hidden"} {
		_, ok := reg.Get(id)
		assert.True(t, ok, "condition %q must be present", id)
	}
}

func TestSubmergedConditionLoads(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)
	def, ok := reg.Get("submerged")
	require.True(t, ok, "submerged condition must be registered")
	assert.Equal(t, "submerged", def.ID)
	assert.Equal(t, 1, def.MaxStacks)
	assert.Equal(t, "permanent", def.DurationType)
}

func TestCoverConditionYAML(t *testing.T) {
	cases := []struct {
		file        string
		wantID      string
		wantACP     int
		wantReflex  int
		wantStealth int
	}{
		{"lesser_cover", "lesser_cover", 1, 1, 1},
		{"standard_cover", "standard_cover", 2, 2, 2},
		{"greater_cover", "greater_cover", 4, 4, 4},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join("../../../content/conditions", tc.file+".yaml")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			var def condition.ConditionDef
			dec := yaml3.NewDecoder(bytes.NewReader(data))
			dec.KnownFields(true)
			if err := dec.Decode(&def); err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			if def.ID != tc.wantID {
				t.Errorf("ID: got %q, want %q", def.ID, tc.wantID)
			}
			if def.ACPenalty != tc.wantACP {
				t.Errorf("ACPenalty: got %d, want %d", def.ACPenalty, tc.wantACP)
			}
			if def.ReflexBonus != tc.wantReflex {
				t.Errorf("ReflexBonus: got %d, want %d", def.ReflexBonus, tc.wantReflex)
			}
			if def.StealthBonus != tc.wantStealth {
				t.Errorf("StealthBonus: got %d, want %d", def.StealthBonus, tc.wantStealth)
			}
		})
	}
}

// REQ-COND1: New conditions load without error.
func TestNewConditions_LoadFromDirectory(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)

	for _, id := range []string{"slowed", "immobilized", "blinded", "fleeing"} {
		def, ok := reg.Get(id)
		require.True(t, ok, "condition %q not found", id)
		assert.NotEmpty(t, def.Name, "condition %q has empty name", id)
		assert.NotEmpty(t, def.Description, "condition %q has empty description", id)
	}
}

func TestConditionDef_AttackBonus_YAMLRoundTrip(t *testing.T) {
	yml := `
id: test_bonus
name: Test Bonus
description: grants attack bonus
duration_type: rounds
max_stacks: 0
attack_bonus: 3
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
`
	var def condition.ConditionDef
	dec := yaml3.NewDecoder(strings.NewReader(yml))
	dec.KnownFields(true)
	if err := dec.Decode(&def); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if def.AttackBonus != 3 {
		t.Errorf("expected AttackBonus=3, got %d", def.AttackBonus)
	}
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

func TestLoadDirectory_AidedConditionsPresent(t *testing.T) {
	reg, err := condition.LoadDirectory("../../../content/conditions")
	require.NoError(t, err)

	cases := []struct {
		id           string
		wantBonus    int
		wantPenalty  int
		wantDuration string
	}{
		{"aided_strong", 3, 0, "rounds"},
		{"aided", 2, 0, "rounds"},
		{"aided_penalty", 0, 1, "rounds"},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			def, ok := reg.Get(tc.id)
			require.True(t, ok, "condition %q must be present", tc.id)
			assert.Equal(t, tc.id, def.ID)
			assert.Equal(t, tc.wantBonus, def.AttackBonus, "AttackBonus mismatch")
			assert.Equal(t, tc.wantPenalty, def.AttackPenalty, "AttackPenalty mismatch")
			assert.Equal(t, tc.wantDuration, def.DurationType, "DurationType mismatch")
			assert.Equal(t, 0, def.MaxStacks, "MaxStacks must be 0")
		})
	}
}
