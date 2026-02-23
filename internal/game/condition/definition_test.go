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
