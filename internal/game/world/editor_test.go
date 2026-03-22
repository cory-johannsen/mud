package world_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/world"
)

func TestNewWorldEditor_NonWritableDir(t *testing.T) {
	_, err := world.NewWorldEditor("/nonexistent/path/xyz", nil)
	assert.Error(t, err)
}

func TestNewWorldEditor_WritableDir(t *testing.T) {
	dir, mgr := setupEditorFixture(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)
	assert.NotNil(t, editor)
}

func TestWorldEditor_AddRoom_Success(t *testing.T) {
	dir, mgr := setupEditorFixture(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)

	err = editor.AddRoom("test", "r2", "Room Two")
	require.NoError(t, err)

	room, ok := mgr.GetRoom("r2")
	require.True(t, ok)
	assert.Equal(t, "Room Two", room.Title)
}

func TestWorldEditor_AddRoom_DuplicateID(t *testing.T) {
	dir, mgr := setupEditorFixture(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)

	err = editor.AddRoom("test", "r1", "Duplicate")
	assert.ErrorContains(t, err, "already exists")
}

func TestWorldEditor_AddRoom_UnknownZone(t *testing.T) {
	dir, mgr := setupEditorFixture(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)

	err = editor.AddRoom("nonexistent", "r2", "Room Two")
	assert.ErrorContains(t, err, "unknown zone")
}

func TestWorldEditor_AddLink_Success(t *testing.T) {
	dir, mgr := setupEditorFixtureTwoRooms(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)

	err = editor.AddLink("r1", "north", "r2")
	require.NoError(t, err)

	r1, ok := mgr.GetRoom("r1")
	require.True(t, ok)
	found := false
	for _, exit := range r1.Exits {
		if exit.Direction == world.North && exit.TargetRoom == "r2" {
			found = true
		}
	}
	assert.True(t, found, "r1 should have north exit to r2")
}

func TestWorldEditor_AddLink_InvalidDirection(t *testing.T) {
	dir, mgr := setupEditorFixtureTwoRooms(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)

	err = editor.AddLink("r1", "diagonal", "r2")
	assert.ErrorContains(t, err, "invalid direction")
}

func TestWorldEditor_RemoveLink_Success(t *testing.T) {
	dir, mgr := setupEditorFixtureTwoRooms(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)

	require.NoError(t, editor.AddLink("r1", "north", "r2"))
	require.NoError(t, editor.RemoveLink("r1", "north"))

	r1, ok := mgr.GetRoom("r1")
	require.True(t, ok)
	for _, exit := range r1.Exits {
		assert.NotEqual(t, world.North, exit.Direction)
	}
}

func TestWorldEditor_RemoveLink_NotPresent(t *testing.T) {
	dir, mgr := setupEditorFixture(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)

	err = editor.RemoveLink("r1", "north")
	assert.ErrorContains(t, err, "no north exit")
}

func TestWorldEditor_SetRoomField_Title(t *testing.T) {
	dir, mgr := setupEditorFixture(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)

	err = editor.SetRoomField("r1", "title", "New Title")
	require.NoError(t, err)

	room, ok := mgr.GetRoom("r1")
	require.True(t, ok)
	assert.Equal(t, "New Title", room.Title)
}

func TestWorldEditor_SetRoomField_InvalidDangerLevel(t *testing.T) {
	dir, mgr := setupEditorFixture(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)

	err = editor.SetRoomField("r1", "danger_level", "lethal")
	assert.ErrorContains(t, err, "invalid danger level")
}

func TestWorldEditor_SetRoomField_UnknownField(t *testing.T) {
	dir, mgr := setupEditorFixture(t)
	editor, err := world.NewWorldEditor(dir, mgr)
	require.NoError(t, err)

	err = editor.SetRoomField("r1", "color", "blue")
	assert.ErrorContains(t, err, "unknown field")
}

// Property: AddRoom with a new unique ID always succeeds.
func TestWorldEditorAddRoomProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Create a fresh temp dir and manager for each rapid iteration.
		dir := t.TempDir()
		zoneYAML := `zone:
  id: "test"
  name: "Test Zone"
  start_room: "r1"
  rooms:
    - id: "r1"
      title: "Room One"
      description: "A room."
      map_x: 0
      map_y: 0
      exits: []
`
		if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(zoneYAML), 0644); err != nil {
			rt.Skip()
		}
		zones, err := world.LoadZonesFromDir(dir)
		if err != nil {
			rt.Skip()
		}
		mgr, err := world.NewManager(zones)
		if err != nil {
			rt.Skip()
		}
		editor, err := world.NewWorldEditor(dir, mgr)
		if err != nil {
			rt.Skip()
		}
		newID := rapid.StringMatching(`[a-z][a-z0-9_]{1,15}`).Draw(rt, "id")
		if newID == "r1" {
			rt.Skip()
		}
		title := rapid.StringOf(rapid.RuneFrom([]rune("abcdefghijklmnopqrstuvwxyz "))).Filter(func(s string) bool { return len(s) > 0 }).Draw(rt, "title")
		err = editor.AddRoom("test", newID, title)
		assert.NoError(t, err)
	})
}

// setupEditorFixture creates a temp dir with a single-zone/single-room fixture.
func setupEditorFixture(t *testing.T) (string, *world.Manager) {
	t.Helper()
	dir := t.TempDir()
	zoneYAML := `zone:
  id: "test"
  name: "Test Zone"
  start_room: "r1"
  rooms:
    - id: "r1"
      title: "Room One"
      description: "A room."
      map_x: 0
      map_y: 0
      exits: []
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(zoneYAML), 0644))
	zones, err := world.LoadZonesFromDir(dir)
	require.NoError(t, err)
	mgr, err := world.NewManager(zones)
	require.NoError(t, err)
	return dir, mgr
}

// setupEditorFixtureTwoRooms creates a temp dir with two rooms in the same zone.
func setupEditorFixtureTwoRooms(t *testing.T) (string, *world.Manager) {
	t.Helper()
	dir := t.TempDir()
	zoneYAML := `zone:
  id: "test"
  name: "Test Zone"
  start_room: "r1"
  rooms:
    - id: "r1"
      title: "Room One"
      description: "A room."
      map_x: 0
      map_y: 0
      exits: []
    - id: "r2"
      title: "Room Two"
      description: "Another room."
      map_x: 1
      map_y: 0
      exits: []
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(zoneYAML), 0644))
	zones, err := world.LoadZonesFromDir(dir)
	require.NoError(t, err)
	mgr, err := world.NewManager(zones)
	require.NoError(t, err)
	return dir, mgr
}
