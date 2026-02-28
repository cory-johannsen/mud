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
  brutality: 14
  quickness: 12
  grit: 14
  reasoning: 8
  savvy: 10
  flair: 8
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
	assert.Equal(t, 14, tmpl.Abilities.Brutality)
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

func TestNewInstance_SetsFieldsFromTemplate(t *testing.T) {
	tmpl := &npc.Template{
		ID:          "ganger",
		Name:        "Ganger",
		Description: "A scarred street tough.",
		Level:       1,
		MaxHP:       18,
		AC:          14,
		Perception:  5,
	}

	inst := npc.NewInstance("inst-1", tmpl, "room-alley")
	assert.Equal(t, "inst-1", inst.ID)
	assert.Equal(t, "ganger", inst.TemplateID)
	assert.Equal(t, "Ganger", inst.Name)
	assert.Equal(t, "A scarred street tough.", inst.Description)
	assert.Equal(t, "room-alley", inst.RoomID)
	assert.Equal(t, 18, inst.CurrentHP)
	assert.Equal(t, 18, inst.MaxHP)
	assert.Equal(t, 14, inst.AC)
	assert.False(t, inst.IsDead())
}

func TestInstance_IsDead(t *testing.T) {
	tmpl := &npc.Template{ID: "t", Name: "T", Level: 1, MaxHP: 10, AC: 10}
	inst := npc.NewInstance("i1", tmpl, "room-1")
	inst.CurrentHP = 0
	assert.True(t, inst.IsDead())
	inst.CurrentHP = -5
	assert.True(t, inst.IsDead())
}

func TestInstance_HealthDescription(t *testing.T) {
	tmpl := &npc.Template{ID: "t", Name: "T", Level: 1, MaxHP: 100, AC: 10}
	tests := []struct {
		hp   int
		want string
	}{
		{100, "unharmed"},
		{90, "barely scratched"},
		{70, "lightly wounded"},
		{50, "moderately wounded"},
		{25, "heavily wounded"},
		{10, "critically wounded"},
		{0, "dead"},
	}
	for _, tc := range tests {
		inst := npc.NewInstance("i", tmpl, "r")
		inst.CurrentHP = tc.hp
		assert.Equal(t, tc.want, inst.HealthDescription(), "hp=%d", tc.hp)
	}
}

func TestInstance_Property_HealthDescriptionNonEmpty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxHP := rapid.IntRange(1, 300).Draw(rt, "max_hp")
		currentHP := rapid.IntRange(-50, maxHP).Draw(rt, "current_hp")
		tmpl := &npc.Template{ID: "t", Name: "T", Level: 1, MaxHP: maxHP, AC: 10}
		inst := npc.NewInstance("i", tmpl, "r")
		inst.CurrentHP = currentHP
		assert.NotEmpty(rt, inst.HealthDescription())
	})
}

func TestManager_SpawnAndList(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 18, AC: 14}
	mgr := npc.NewManager()

	inst, err := mgr.Spawn(tmpl, "room-alley")
	require.NoError(t, err)
	assert.NotEmpty(t, inst.ID)
	assert.Equal(t, "room-alley", inst.RoomID)

	list := mgr.InstancesInRoom("room-alley")
	require.Len(t, list, 1)
	assert.Equal(t, inst.ID, list[0].ID)
}

func TestManager_InstancesInRoom_Empty(t *testing.T) {
	mgr := npc.NewManager()
	assert.Empty(t, mgr.InstancesInRoom("nonexistent-room"))
}

func TestManager_Remove(t *testing.T) {
	tmpl := &npc.Template{ID: "g", Name: "G", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()
	inst, _ := mgr.Spawn(tmpl, "room-1")

	require.NoError(t, mgr.Remove(inst.ID))
	assert.Empty(t, mgr.InstancesInRoom("room-1"))
}

func TestManager_Remove_NotFound(t *testing.T) {
	mgr := npc.NewManager()
	assert.Error(t, mgr.Remove("nonexistent"))
}

func TestManager_Get(t *testing.T) {
	tmpl := &npc.Template{ID: "g", Name: "G", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()
	inst, _ := mgr.Spawn(tmpl, "room-1")

	got, ok := mgr.Get(inst.ID)
	assert.True(t, ok)
	assert.Equal(t, inst.ID, got.ID)

	_, ok = mgr.Get("missing")
	assert.False(t, ok)
}

func TestManager_FindInRoom_PrefixMatch(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()
	inst, _ := mgr.Spawn(tmpl, "room-1")

	found := mgr.FindInRoom("room-1", "gan")
	require.NotNil(t, found)
	assert.Equal(t, inst.ID, found.ID)

	notFound := mgr.FindInRoom("room-1", "xyz")
	assert.Nil(t, notFound)
}

func TestManager_Property_SpawnProducesUniqueIDs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 20).Draw(rt, "n")
		tmpl := &npc.Template{ID: "g", Name: "G", Level: 1, MaxHP: 10, AC: 10}
		mgr := npc.NewManager()
		ids := make(map[string]bool)
		for i := 0; i < n; i++ {
			inst, err := mgr.Spawn(tmpl, "room-1")
			require.NoError(rt, err)
			assert.False(rt, ids[inst.ID], "duplicate ID: %s", inst.ID)
			ids[inst.ID] = true
		}
	})
}

func TestManager_Spawn_SingleInstance_NoSuffix(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()

	inst, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)
	assert.Equal(t, "Ganger", inst.Name)
}

func TestManager_Spawn_TwoInstances_LetterSuffix(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()

	first, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)

	second, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)

	// First instance must have been renamed to A
	got, ok := mgr.Get(first.ID)
	require.True(t, ok)
	assert.Equal(t, "Ganger A", got.Name)

	// Second instance gets B
	assert.Equal(t, "Ganger B", second.Name)
}

func TestManager_Spawn_ThreeInstances_LetterSuffixes(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()

	first, _ := mgr.Spawn(tmpl, "room-1")
	second, _ := mgr.Spawn(tmpl, "room-1")
	third, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)

	gotFirst, _ := mgr.Get(first.ID)
	gotSecond, _ := mgr.Get(second.ID)

	assert.Equal(t, "Ganger A", gotFirst.Name)
	assert.Equal(t, "Ganger B", gotSecond.Name)
	assert.Equal(t, "Ganger C", third.Name)
}

func TestManager_Spawn_DifferentTemplates_NoSuffix(t *testing.T) {
	tmplA := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	tmplB := &npc.Template{ID: "scavenger", Name: "Scavenger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()

	g, err := mgr.Spawn(tmplA, "room-1")
	require.NoError(t, err)
	s, err := mgr.Spawn(tmplB, "room-1")
	require.NoError(t, err)

	assert.Equal(t, "Ganger", g.Name)
	assert.Equal(t, "Scavenger", s.Name)
}

func TestManager_Spawn_DifferentRooms_NoSuffix(t *testing.T) {
	tmpl := &npc.Template{ID: "ganger", Name: "Ganger", Level: 1, MaxHP: 10, AC: 10}
	mgr := npc.NewManager()

	inst1, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)
	inst2, err := mgr.Spawn(tmpl, "room-2")
	require.NoError(t, err)

	assert.Equal(t, "Ganger", inst1.Name)
	assert.Equal(t, "Ganger", inst2.Name)
}

func TestManager_Spawn_Property_SuffixesAreUnique(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		tmpl := &npc.Template{ID: "g", Name: "G", Level: 1, MaxHP: 10, AC: 10}
		mgr := npc.NewManager()

		ids := make([]string, 0, n)
		for i := 0; i < n; i++ {
			inst, err := mgr.Spawn(tmpl, "room-1")
			require.NoError(rt, err)
			ids = append(ids, inst.ID)
		}

		names := make(map[string]bool)
		for _, id := range ids {
			inst, ok := mgr.Get(id)
			require.True(rt, ok)
			assert.False(rt, names[inst.Name], "duplicate name: %s", inst.Name)
			names[inst.Name] = true
		}
	})
}
