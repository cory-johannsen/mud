package npc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_TemplateByID_ReturnsTemplate(t *testing.T) {
	mgr := NewManager()
	tmpl := &Template{
		ID: "test_tmpl", Name: "Test", NPCType: "combat",
		MaxHP: 10, AC: 10, Level: 1,
	}
	_, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)
	got := mgr.TemplateByID("test_tmpl")
	require.NotNil(t, got)
	assert.Equal(t, "test_tmpl", got.ID)
}

func TestManager_TemplateByID_MissingReturnsNil(t *testing.T) {
	mgr := NewManager()
	assert.Nil(t, mgr.TemplateByID("nonexistent"))
}

func TestManager_InstanceByID_ReturnsInstance(t *testing.T) {
	mgr := NewManager()
	tmpl := &Template{ID: "bandit", Name: "Bandit", NPCType: "combat", MaxHP: 10, AC: 10, Level: 1}
	inst, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)
	got := mgr.InstanceByID(inst.ID)
	require.NotNil(t, got)
	assert.Equal(t, inst.ID, got.ID)
}

func TestManager_InstanceByID_MissingReturnsNil(t *testing.T) {
	mgr := NewManager()
	assert.Nil(t, mgr.InstanceByID("ghost"))
}

func TestManager_TemplateLevel_ReturnsLevelForKnownTemplate(t *testing.T) {
	mgr := NewManager()
	tmpl := &Template{
		ID: "guard_alpha", Name: "Guard Alpha", NPCType: "combat",
		MaxHP: 20, AC: 12, Level: 5,
	}
	_, err := mgr.Spawn(tmpl, "room-1")
	require.NoError(t, err)
	level, ok := mgr.TemplateLevel("guard_alpha")
	require.True(t, ok)
	assert.Equal(t, 5, level)
}

func TestManager_TemplateLevel_ReturnsFalseForUnknownTemplate(t *testing.T) {
	mgr := NewManager()
	level, ok := mgr.TemplateLevel("nonexistent_npc_xyz_12345")
	assert.False(t, ok)
	assert.Equal(t, 0, level)
}

// TestInstancesInRoom_StableOrder verifies that InstancesInRoom returns instances
// in a deterministic order regardless of internal map iteration order.
//
// Precondition: Multiple instances are spawned in the same room.
// Postcondition: Repeated calls return the same order (sorted by instance ID).
func TestInstancesInRoom_StableOrder(t *testing.T) {
	mgr := NewManager()
	tmpl := &Template{ID: "bandit", Name: "Bandit", NPCType: "combat", MaxHP: 10, AC: 10, Level: 1}
	for i := 0; i < 5; i++ {
		_, err := mgr.Spawn(tmpl, "room-1")
		require.NoError(t, err)
	}

	first := mgr.InstancesInRoom("room-1")
	require.Len(t, first, 5)

	for range 10 {
		got := mgr.InstancesInRoom("room-1")
		require.Len(t, got, 5)
		for i := range first {
			assert.Equal(t, first[i].ID, got[i].ID, "order must be stable across calls")
		}
	}
}
