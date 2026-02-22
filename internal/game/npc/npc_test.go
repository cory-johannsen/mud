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

func TestLoadTemplates_NonYAMLFilesSkipped(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore me"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0755))
	// Write a valid .yml file (not .yaml — should be skipped)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "template.yml"), []byte("id: x\nname: X\nlevel: 1\nmax_hp: 1\nac: 10\n"), 0644))

	templates, err := npc.LoadTemplates(dir)
	require.NoError(t, err)
	assert.Empty(t, templates)
}

func TestLoadTemplates_ValidationError(t *testing.T) {
	dir := t.TempDir()
	// level: 0 is invalid per Validate
	yaml := `id: broken
name: Broken
level: 0
max_hp: 10
ac: 10
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte(yaml), 0644))
	_, err := npc.LoadTemplates(dir)
	assert.Error(t, err)
}

func TestLoadTemplates_NonExistentDir(t *testing.T) {
	_, err := npc.LoadTemplates("/tmp/does-not-exist-mud-npc-test")
	assert.Error(t, err)
}

func TestTemplate_Property_Validate_ValidInputsPass(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmpl := &npc.Template{
			ID:    rapid.StringMatching(`[a-z][a-z0-9_]{0,15}`).Draw(rt, "id"),
			Name:  rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz "))).Filter(func(s string) bool { return len(s) > 0 }).Draw(rt, "name"),
			Level: rapid.IntRange(1, 20).Draw(rt, "level"),
			MaxHP: rapid.IntRange(1, 300).Draw(rt, "max_hp"),
			AC:    rapid.IntRange(10, 30).Draw(rt, "ac"),
		}
		assert.NoError(rt, tmpl.Validate(), "valid template should pass Validate")
	})
}

func TestTemplate_Property_Validate_InvalidInputsFail(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Pick one field to invalidate
		field := rapid.IntRange(0, 4).Draw(rt, "field")
		tmpl := &npc.Template{
			ID:    "valid",
			Name:  "Valid",
			Level: 1,
			MaxHP: 10,
			AC:    10,
		}
		switch field {
		case 0:
			tmpl.ID = ""
		case 1:
			tmpl.Name = ""
		case 2:
			tmpl.Level = rapid.IntRange(-10, 0).Draw(rt, "bad_level")
		case 3:
			tmpl.MaxHP = rapid.IntRange(-10, 0).Draw(rt, "bad_max_hp")
		case 4:
			tmpl.AC = rapid.IntRange(0, 9).Draw(rt, "bad_ac")
		}
		assert.Error(rt, tmpl.Validate(), "invalid template should fail Validate")
	})
}
