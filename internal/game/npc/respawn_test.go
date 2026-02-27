package npc_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// helpers

func makeTemplate(id string, respawnDelay string) *npc.Template {
	return &npc.Template{
		ID: id, Name: id, Description: "test",
		Level: 1, MaxHP: 10, AC: 10,
		RespawnDelay: respawnDelay,
	}
}

func makeRespawnManager(roomID, templateID string, count int, roomOverride string, tmpl *npc.Template) *npc.RespawnManager {
	var override time.Duration
	if roomOverride != "" {
		d, err := time.ParseDuration(roomOverride)
		if err != nil {
			panic(err)
		}
		override = d
	}
	spawns := map[string][]npc.RoomSpawn{
		roomID: {{TemplateID: templateID, Max: count, RespawnDelay: override}},
	}
	templates := map[string]*npc.Template{templateID: tmpl}
	return npc.NewRespawnManager(spawns, templates)
}

// --- PopulateRoom ---

func TestRespawnManager_PopulateRoom_SpawnsUpToCap(t *testing.T) {
	tmpl := makeTemplate("ganger", "5m")
	mgr := npc.NewManager()
	rm := makeRespawnManager("r1", "ganger", 2, "", tmpl)

	rm.PopulateRoom("r1", mgr)

	assert.Len(t, mgr.InstancesInRoom("r1"), 2)
}

func TestRespawnManager_PopulateRoom_DoesNotExceedCap(t *testing.T) {
	tmpl := makeTemplate("ganger", "5m")
	mgr := npc.NewManager()
	_, err := mgr.Spawn(tmpl, "r1")
	require.NoError(t, err)

	rm := makeRespawnManager("r1", "ganger", 2, "", tmpl)
	rm.PopulateRoom("r1", mgr)

	assert.Len(t, mgr.InstancesInRoom("r1"), 2)
}

func TestRespawnManager_PopulateRoom_NoSpawnConfig_DoesNothing(t *testing.T) {
	mgr := npc.NewManager()
	rm := npc.NewRespawnManager(nil, nil)
	rm.PopulateRoom("r1", mgr)
	assert.Empty(t, mgr.InstancesInRoom("r1"))
}

func TestRespawnManager_PopulateRoom_RemovesExcessInstances(t *testing.T) {
	tmpl := makeTemplate("ganger", "5m")
	mgr := npc.NewManager()
	// Pre-populate 4 instances against a cap of 2.
	for i := 0; i < 4; i++ {
		_, err := mgr.Spawn(tmpl, "r1")
		require.NoError(t, err)
	}

	rm := makeRespawnManager("r1", "ganger", 2, "", tmpl)
	rm.PopulateRoom("r1", mgr)

	assert.Len(t, mgr.InstancesInRoom("r1"), 2)
}

// --- Schedule + Tick ---

func TestRespawnManager_Tick_BeforeDeadline_DoesNotSpawn(t *testing.T) {
	tmpl := makeTemplate("ganger", "5m")
	mgr := npc.NewManager()
	rm := makeRespawnManager("r1", "ganger", 2, "", tmpl)

	now := time.Now()
	rm.Schedule("ganger", "r1", now, 5*time.Minute)
	rm.Tick(now.Add(4*time.Minute+59*time.Second), mgr)

	assert.Empty(t, mgr.InstancesInRoom("r1"))
}

func TestRespawnManager_Tick_AfterDeadline_Spawns(t *testing.T) {
	tmpl := makeTemplate("ganger", "5m")
	mgr := npc.NewManager()
	rm := makeRespawnManager("r1", "ganger", 2, "", tmpl)

	now := time.Now()
	rm.Schedule("ganger", "r1", now, 5*time.Minute)
	rm.Tick(now.Add(5*time.Minute), mgr)

	assert.Len(t, mgr.InstancesInRoom("r1"), 1)
}

func TestRespawnManager_Tick_RespectsPopulationCap(t *testing.T) {
	tmpl := makeTemplate("ganger", "5m")
	mgr := npc.NewManager()
	rm := makeRespawnManager("r1", "ganger", 1, "", tmpl)

	_, err := mgr.Spawn(tmpl, "r1")
	require.NoError(t, err)

	now := time.Now()
	rm.Schedule("ganger", "r1", now, 5*time.Minute)
	rm.Tick(now.Add(5*time.Minute), mgr)

	assert.Len(t, mgr.InstancesInRoom("r1"), 1)
}

func TestRespawnManager_Tick_ZeroDelay_NeverRespawns(t *testing.T) {
	tmpl := makeTemplate("ganger", "")
	mgr := npc.NewManager()
	rm := makeRespawnManager("r1", "ganger", 2, "", tmpl)

	now := time.Now()
	rm.Schedule("ganger", "r1", now, 0)
	rm.Tick(time.Now().Add(time.Hour), mgr)

	assert.Empty(t, mgr.InstancesInRoom("r1"))
}

func TestRespawnManager_Tick_MultipleScheduled_SpawnsAll(t *testing.T) {
	tmpl := makeTemplate("ganger", "1m")
	mgr := npc.NewManager()
	rm := makeRespawnManager("r1", "ganger", 3, "", tmpl)

	now := time.Now()
	rm.Schedule("ganger", "r1", now, time.Minute)
	rm.Schedule("ganger", "r1", now, time.Minute)
	rm.Tick(now.Add(time.Minute), mgr)

	assert.Len(t, mgr.InstancesInRoom("r1"), 2)
}

func TestRespawnManager_Tick_CapReachedMidBatch_ExtraDropped(t *testing.T) {
	tmpl := makeTemplate("ganger", "1m")
	mgr := npc.NewManager()
	rm := makeRespawnManager("r1", "ganger", 1, "", tmpl) // cap = 1

	now := time.Now()
	rm.Schedule("ganger", "r1", now, time.Minute)
	rm.Schedule("ganger", "r1", now, time.Minute)
	rm.Tick(now.Add(time.Minute), mgr)

	// Only 1 should be spawned despite 2 being scheduled — cap is 1.
	assert.Len(t, mgr.InstancesInRoom("r1"), 1)
}

// --- ResolvedDelay ---

func TestRespawnManager_ResolvedDelay_UsesRoomOverride(t *testing.T) {
	tmpl := makeTemplate("ganger", "10m")
	rm := makeRespawnManager("r1", "ganger", 2, "1m", tmpl)
	assert.Equal(t, time.Minute, rm.ResolvedDelay("ganger", "r1"))
}

func TestRespawnManager_ResolvedDelay_FallsBackToTemplate(t *testing.T) {
	tmpl := makeTemplate("ganger", "5m")
	rm := makeRespawnManager("r1", "ganger", 2, "", tmpl)
	assert.Equal(t, 5*time.Minute, rm.ResolvedDelay("ganger", "r1"))
}

func TestRespawnManager_ResolvedDelay_ZeroWhenNoDelay(t *testing.T) {
	tmpl := makeTemplate("ganger", "")
	rm := makeRespawnManager("r1", "ganger", 2, "", tmpl)
	assert.Equal(t, time.Duration(0), rm.ResolvedDelay("ganger", "r1"))
}

// --- Property tests ---

func TestProperty_Tick_SpawnsNeverExceedCap(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cap := rapid.IntRange(1, 5).Draw(rt, "cap")
		existing := rapid.IntRange(0, cap).Draw(rt, "existing")
		scheduled := rapid.IntRange(0, 5).Draw(rt, "scheduled")

		tmpl := makeTemplate("ganger", "1m")
		mgr := npc.NewManager()
		for i := 0; i < existing; i++ {
			_, err := mgr.Spawn(tmpl, "r1")
			require.NoError(rt, err)
		}

		spawns := map[string][]npc.RoomSpawn{
			"r1": {{TemplateID: "ganger", Max: cap, RespawnDelay: time.Minute}},
		}
		rm := npc.NewRespawnManager(spawns, map[string]*npc.Template{"ganger": tmpl})

		now := time.Now()
		for i := 0; i < scheduled; i++ {
			rm.Schedule("ganger", "r1", now, time.Minute)
		}
		rm.Tick(now.Add(time.Minute), mgr)

		got := len(mgr.InstancesInRoom("r1"))
		if got > cap {
			rt.Fatalf("spawned %d > cap %d (existing=%d scheduled=%d)", got, cap, existing, scheduled)
		}
	})
}

func TestProperty_PopulateRoom_NeverExceedsCap(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		cap := rapid.IntRange(1, 5).Draw(rt, "cap")
		preExisting := rapid.IntRange(0, cap+2).Draw(rt, "pre")

		tmpl := makeTemplate(fmt.Sprintf("npc%d", cap), "1m")
		mgr := npc.NewManager()
		for i := 0; i < preExisting; i++ {
			_, _ = mgr.Spawn(tmpl, "r1")
		}

		spawns := map[string][]npc.RoomSpawn{
			"r1": {{TemplateID: tmpl.ID, Max: cap, RespawnDelay: time.Minute}},
		}
		rm := npc.NewRespawnManager(spawns, map[string]*npc.Template{tmpl.ID: tmpl})
		rm.PopulateRoom("r1", mgr)

		got := len(mgr.InstancesInRoom("r1"))
		if got > cap {
			rt.Fatalf("got %d instances > cap %d (pre-existing=%d)", got, cap, preExisting)
		}
	})
}
