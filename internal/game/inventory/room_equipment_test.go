package inventory_test

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestRoomEquipmentManager_SpawnInitializesInstances(t *testing.T) {
	cfg := []world.RoomEquipmentConfig{
		{ItemID: "medkit", MaxCount: 2, RespawnAfter: 5 * time.Minute, Immovable: false},
	}
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room1", cfg)

	items := mgr.EquipmentInRoom("room1")
	assert.Len(t, items, 2)
	for _, it := range items {
		assert.Equal(t, "medkit", it.ItemDefID)
		assert.False(t, it.Immovable)
		assert.NotEmpty(t, it.InstanceID)
	}
}

func TestRoomEquipmentManager_ImmovableCannotBePickedUp(t *testing.T) {
	cfg := []world.RoomEquipmentConfig{
		{ItemID: "water_fountain", MaxCount: 1, RespawnAfter: 0, Immovable: true},
	}
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room1", cfg)

	items := mgr.EquipmentInRoom("room1")
	require.Len(t, items, 1)

	ok := mgr.Pickup("room1", items[0].InstanceID)
	assert.False(t, ok, "immovable item should not be pickable")

	after := mgr.EquipmentInRoom("room1")
	assert.Len(t, after, 1, "item should still be present")
}

func TestRoomEquipmentManager_PickupRemovesMovableItem(t *testing.T) {
	cfg := []world.RoomEquipmentConfig{
		{ItemID: "medkit", MaxCount: 1, RespawnAfter: 5 * time.Minute, Immovable: false},
	}
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room1", cfg)

	items := mgr.EquipmentInRoom("room1")
	require.Len(t, items, 1)

	ok := mgr.Pickup("room1", items[0].InstanceID)
	assert.True(t, ok)
	assert.Empty(t, mgr.EquipmentInRoom("room1"))
}

func TestRoomEquipmentManager_EmptyRoomReturnsEmpty(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	assert.Empty(t, mgr.EquipmentInRoom("nonexistent"))
}

func TestRoomEquipmentManager_AddConfig(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("r1", nil)
	mgr.AddConfig("r1", world.RoomEquipmentConfig{ItemID: "bandage", MaxCount: 3, Immovable: false})
	items := mgr.EquipmentInRoom("r1")
	assert.Len(t, items, 3)
	for _, it := range items {
		assert.Equal(t, "bandage", it.ItemDefID)
	}
}

func TestRoomEquipmentManager_RemoveConfig(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("r1", []world.RoomEquipmentConfig{
		{ItemID: "bandage", MaxCount: 2, Immovable: false},
	})
	ok := mgr.RemoveConfig("r1", "bandage")
	assert.True(t, ok)
	assert.Empty(t, mgr.EquipmentInRoom("r1"))
}

func TestRoomEquipmentManager_RemoveConfigNotFound(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("r1", nil)
	ok := mgr.RemoveConfig("r1", "nonexistent")
	assert.False(t, ok)
}

func TestRoomEquipmentManager_ListConfigs(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	cfg := []world.RoomEquipmentConfig{
		{ItemID: "medkit", MaxCount: 1, Immovable: false},
		{ItemID: "water_fountain", MaxCount: 1, Immovable: true},
	}
	mgr.InitRoom("r1", cfg)
	cfgs := mgr.ListConfigs("r1")
	assert.Len(t, cfgs, 2)
}

func TestRoomEquipmentManager_ProcessRespawns(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("r1", []world.RoomEquipmentConfig{
		{ItemID: "medkit", MaxCount: 1, RespawnAfter: 1 * time.Millisecond, Immovable: false},
	})
	items := mgr.EquipmentInRoom("r1")
	require.Len(t, items, 1)
	mgr.Pickup("r1", items[0].InstanceID)
	assert.Empty(t, mgr.EquipmentInRoom("r1"))

	time.Sleep(5 * time.Millisecond)
	mgr.ProcessRespawns()
	assert.Len(t, mgr.EquipmentInRoom("r1"), 1)
}

func TestProperty_RoomEquipmentManager_SpawnCountNeverExceedsMax(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxCount := rapid.IntRange(0, 10).Draw(rt, "maxCount")
		cfg := []world.RoomEquipmentConfig{
			{ItemID: "item", MaxCount: maxCount, RespawnAfter: 0, Immovable: false},
		}
		mgr := inventory.NewRoomEquipmentManager()
		mgr.InitRoom("r1", cfg)
		items := mgr.EquipmentInRoom("r1")
		assert.LessOrEqual(t, len(items), maxCount)
	})
}

func TestRoomEquipmentManager_GetInstance_Found(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("r1", []world.RoomEquipmentConfig{
		{ItemID: "medkit", MaxCount: 1, Immovable: false},
	})
	items := mgr.EquipmentInRoom("r1")
	require.Len(t, items, 1)

	got := mgr.GetInstance("r1", items[0].InstanceID)
	require.NotNil(t, got)
	assert.Equal(t, "medkit", got.ItemDefID)
	assert.Equal(t, items[0].InstanceID, got.InstanceID)
}

func TestRoomEquipmentManager_GetInstance_NotFound(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("r1", nil)
	got := mgr.GetInstance("r1", "nonexistent")
	assert.Nil(t, got)
}

func TestRoomEquipmentManager_GetInstance_ByDescription(t *testing.T) {
	cfg := []world.RoomEquipmentConfig{
		{ItemID: "zone_map", MaxCount: 1, Immovable: true, Script: "zone_map_use", Description: "Zone Map"},
	}
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("room1", cfg)

	// Match by description (exact case)
	inst := mgr.GetInstance("room1", "Zone Map")
	require.NotNil(t, inst, "should match by description")
	assert.Equal(t, "zone_map", inst.ItemDefID)

	// Match by description (different case)
	inst2 := mgr.GetInstance("room1", "zone map")
	require.NotNil(t, inst2, "should match case-insensitively")

	// No match for unrelated query
	inst3 := mgr.GetInstance("room1", "Medkit")
	assert.Nil(t, inst3)
}

func TestRoomEquipmentManager_GetInstance_ByDescription_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		desc := rapid.StringMatching(`[A-Za-z ]{1,20}`).Draw(t, "desc")
		cfg := []world.RoomEquipmentConfig{
			{ItemID: "item1", MaxCount: 1, Immovable: true, Description: desc},
		}
		mgr := inventory.NewRoomEquipmentManager()
		mgr.InitRoom("r1", cfg)
		inst := mgr.GetInstance("r1", desc)
		require.NotNil(t, inst)
		assert.Equal(t, "item1", inst.ItemDefID)
	})
}

func TestRoomEquipmentManager_RemoveConfig_CancelsPendingRespawns(t *testing.T) {
	mgr := inventory.NewRoomEquipmentManager()
	mgr.InitRoom("r1", []world.RoomEquipmentConfig{
		{ItemID: "medkit", MaxCount: 1, RespawnAfter: 10 * time.Second, Immovable: false},
	})
	items := mgr.EquipmentInRoom("r1")
	require.Len(t, items, 1)
	mgr.Pickup("r1", items[0].InstanceID) // schedules respawn
	mgr.RemoveConfig("r1", "medkit")      // should cancel respawn
	// Even after waiting, ProcessRespawns should not re-add the item
	time.Sleep(1 * time.Millisecond)
	mgr.ProcessRespawns()
	assert.Empty(t, mgr.EquipmentInRoom("r1"))
}
