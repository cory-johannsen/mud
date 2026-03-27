package trap_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/trap"
	"github.com/cory-johannsen/mud/internal/game/world"
	"pgregory.net/rapid"
)

func makeDangerousZone() *world.Zone {
	return &world.Zone{
		ID:          "zone1",
		DangerLevel: "dangerous",
		Rooms: map[string]*world.Room{
			"room1": {
				ID:     "room1",
				ZoneID: "zone1",
			},
		},
	}
}

func TestPlaceTraps_NoOverwriteStaticTrap(t *testing.T) {
	// REQ-TR-9: procedural placement must not add a second room-level trap
	// when a static trap already covers the room. Force the procedural roll
	// to trigger by setting RoomTrapChance=1.0.
	zone := makeDangerousZone()
	zone.TrapProbabilities = &world.TrapProbabilities{
		RoomTrapChance: new(1.0),
	}
	zone.Rooms["room1"].Traps = []world.RoomTrapConfig{
		{TemplateID: "bear_trap", Position: "room"},
	}

	templates := makeTemplates(
		&trap.TrapTemplate{ID: "bear_trap", Trigger: trap.TriggerEntry, ResetMode: trap.ResetAuto, Payload: &trap.TrapPayload{Type: "bear_trap", Damage: "2d6"}},
		&trap.TrapTemplate{ID: "mine", Trigger: trap.TriggerEntry, ResetMode: trap.ResetOneShot, Payload: &trap.TrapPayload{Type: "mine", Damage: "4d6"}},
	)
	defaultPool := []world.TrapPoolEntry{{Template: "mine", Weight: 1}}
	mgr := trap.NewTrapManager()
	rng := rand.New(rand.NewSource(42))

	trap.PlaceTraps(zone, templates, defaultPool, mgr, rng)

	// Count room-level traps (instance IDs contain "/room/").
	allTraps := mgr.TrapsForRoom("zone1", "room1")
	roomLevelCount := 0
	for _, id := range allTraps {
		if strings.Contains(id, "/room/") {
			roomLevelCount++
		}
	}
	// REQ-TR-9: exactly 1 room-level trap (the static one); no procedural second trap.
	if roomLevelCount != 1 {
		t.Errorf("expected exactly 1 room-level trap (static only), got %d", roomLevelCount)
	}
}

func TestPlaceTraps_ManualResetSkipped(t *testing.T) {
	// REQ-TR-10: procedural placement must skip manual-reset templates.
	// Force 100% room trap chance so the procedural path always fires.
	zone := makeDangerousZone()
	zone.TrapProbabilities = &world.TrapProbabilities{
		RoomTrapChance: new(1.0),
	}
	manualTmpl := &trap.TrapTemplate{
		ID:        "manual_trap",
		Trigger:   trap.TriggerEntry,
		ResetMode: trap.ResetManual,
		Payload:   &trap.TrapPayload{Type: "mine", Damage: "4d6"},
	}
	templates := makeTemplates(manualTmpl)
	defaultPool := []world.TrapPoolEntry{{Template: "manual_trap", Weight: 1}}
	mgr := trap.NewTrapManager()
	rng := rand.New(rand.NewSource(42))

	trap.PlaceTraps(zone, templates, defaultPool, mgr, rng)

	// Verify: no trap with TemplateID "manual_trap" was placed (REQ-TR-10).
	allTraps := mgr.TrapsForRoom("zone1", "room1")
	for _, id := range allTraps {
		state, ok := mgr.GetTrap(id)
		if ok && state.TemplateID == "manual_trap" {
			t.Errorf("manual-reset template %q was procedurally placed — REQ-TR-10 violation", id)
		}
	}
	// Also verify 0 traps were placed (only manual template in pool, all skipped).
	if len(allTraps) != 0 {
		t.Errorf("expected 0 traps (all manual-reset, all skipped), got %d", len(allTraps))
	}
}

func TestPlaceTraps_SafeRoomNeverPlaced(t *testing.T) {
	zone := &world.Zone{
		ID:          "zone1",
		DangerLevel: "safe",
		Rooms: map[string]*world.Room{
			"room1": {ID: "room1", ZoneID: "zone1"},
		},
	}
	templates := makeTemplates(&trap.TrapTemplate{
		ID:      "mine",
		Trigger: trap.TriggerEntry,
		Payload: &trap.TrapPayload{Type: "mine"},
	})
	defaultPool := []world.TrapPoolEntry{{Template: "mine", Weight: 1}}
	mgr := trap.NewTrapManager()
	rng := rand.New(rand.NewSource(0))

	trap.PlaceTraps(zone, templates, defaultPool, mgr, rng)

	// No procedural traps should be placed in safe rooms (roomChance=0, coverChance=0).
	roomTraps := mgr.TrapsForRoom("zone1", "room1")
	if len(roomTraps) != 0 {
		t.Errorf("safe room: expected 0 traps, got %d", len(roomTraps))
	}
}

func TestSelectFromPool_Property_AlwaysInPool(t *testing.T) {
	// Property: selectFromPool always returns a template ID that belongs to the pool,
	// or "" when the pool is empty or all weights are zero.
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 5).Draw(rt, "n")
		pool := make([]world.TrapPoolEntry, n)
		ids := make(map[string]bool)
		for i := range pool {
			id := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, fmt.Sprintf("id%d", i))
			w := rapid.IntRange(0, 3).Draw(rt, fmt.Sprintf("w%d", i))
			pool[i] = world.TrapPoolEntry{Template: id, Weight: w}
			if w > 0 {
				ids[id] = true
			}
		}
		rng := rand.New(rand.NewSource(rapid.Int64().Draw(rt, "seed")))
		result := trap.SelectFromPool(pool, rng)
		totalWeight := 0
		for _, e := range pool {
			totalWeight += e.Weight
		}
		if totalWeight == 0 || len(pool) == 0 {
			if result != "" {
				rt.Fatalf("empty/zero-weight pool: expected \"\", got %q", result)
			}
		} else {
			if !ids[result] {
				rt.Fatalf("result %q not in pool %v", result, pool)
			}
		}
	})
}
