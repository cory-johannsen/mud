package npc_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestLoadTemplates_ValidDir(t *testing.T) {
	dir := t.TempDir()
	yaml := `id: ganger
name: Ganger
description: A street tough with a scar across his cheek.
level: 1
max_hp: 18
ac: 14
perception: 5
abilities:
  strength: 14
  dexterity: 12
  constitution: 14
  intelligence: 8
  wisdom: 10
  charisma: 8
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ganger.yaml"), []byte(yaml), 0644))

	templates, err := npc.LoadTemplates(dir)
	require.NoError(t, err)
	require.Len(t, templates, 1)

	tmpl := templates[0]
	assert.Equal(t, "ganger", tmpl.ID)
	assert.Equal(t, "Ganger", tmpl.Name)
	assert.Equal(t, 1, tmpl.Level)
	assert.Equal(t, 18, tmpl.MaxHP)
	assert.Equal(t, 14, tmpl.AC)
	assert.Equal(t, 5, tmpl.Perception)
	assert.Equal(t, 14, tmpl.Abilities.Strength)
}

func TestLoadTemplates_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	templates, err := npc.LoadTemplates(dir)
	require.NoError(t, err)
	assert.Empty(t, templates)
}

func TestLoadTemplates_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(":::invalid"), 0644))
	_, err := npc.LoadTemplates(dir)
	assert.Error(t, err)
}

func TestTemplate_Property_IDAndNameNonEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := rapid.StringMatching(`[a-z][a-z0-9_]{0,15}`).Draw(rt, "id")
		name := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz "))).
			Filter(func(s string) bool { return len(s) > 0 }).
			Draw(rt, "name")
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		maxHP := rapid.IntRange(1, 300).Draw(rt, "max_hp")
		ac := rapid.IntRange(10, 30).Draw(rt, "ac")

		tmpl := &npc.Template{
			ID:    id,
			Name:  name,
			Level: level,
			MaxHP: maxHP,
			AC:    ac,
		}
		assert.NotEmpty(rt, tmpl.ID)
		assert.NotEmpty(rt, tmpl.Name)
		assert.Greater(rt, tmpl.Level, 0)
		assert.Greater(rt, tmpl.MaxHP, 0)
	})
}
