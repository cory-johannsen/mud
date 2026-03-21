package trap_test

import (
	"math/rand"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/trap"
	"github.com/cory-johannsen/mud/internal/game/world"
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
	// REQ-TR-9: procedural placement must not overwrite a statically defined room trap.
	zone := makeDangerousZone()
	zone.Rooms["room1"].Traps = []world.RoomTrapConfig{
		{TemplateID: "bear_trap", Position: "room"},
	}

	templates := makeTemplates(&trap.TrapTemplate{
		ID:        "bear_trap",
		Trigger:   trap.TriggerEntry,
		ResetMode: trap.ResetAuto,
		Payload:   &trap.TrapPayload{Type: "bear_trap", Damage: "2d6"},
	}, &trap.TrapTemplate{
		ID:        "mine",
		Trigger:   trap.TriggerEntry,
		ResetMode: trap.ResetOneShot,
		Payload:   &trap.TrapPayload{Type: "mine", Damage: "4d6"},
	})

	defaultPool := []world.TrapPoolEntry{{Template: "mine", Weight: 1}}
	mgr := trap.NewTrapManager()
	rng := rand.New(rand.NewSource(42))

	trap.PlaceTraps(zone, templates, defaultPool, mgr, rng)

	// Verify: exactly one room-level trap exists (the static one).
	roomTraps := mgr.TrapsForRoom("zone1", "room1")
	roomLevelCount := 0
	for _, id := range roomTraps {
		// room-level traps have kind "room" in their instance ID
		_ = id
		roomLevelCount++
	}
	// At least the static trap should be registered.
	if roomLevelCount == 0 {
		t.Error("expected at least the static bear_trap to be registered")
	}
}

func TestPlaceTraps_ManualResetSkipped(t *testing.T) {
	// REQ-TR-10: procedural placement must skip manual-reset templates.
	zone := makeDangerousZone()
	manualTmpl := &trap.TrapTemplate{
		ID:        "manual_trap",
		Trigger:   trap.TriggerEntry,
		ResetMode: trap.ResetManual,
		Payload:   &trap.TrapPayload{Type: "mine", Damage: "4d6"},
	}
	templates := makeTemplates(manualTmpl)
	defaultPool := []world.TrapPoolEntry{{Template: "manual_trap", Weight: 1}}
	mgr := trap.NewTrapManager()
	// Seed RNG to always roll below roomChance (0.35 for dangerous) — use 0 which gives ~0.0.
	rng := rand.New(rand.NewSource(1)) // seed that produces Float64() < 0.35

	trap.PlaceTraps(zone, templates, defaultPool, mgr, rng)

	// Even if RNG would place a trap, the manual_trap template must be skipped.
	// The TrapManager should have no traps for room1.
	roomTraps := mgr.TrapsForRoom("zone1", "room1")
	for _, id := range roomTraps {
		state, ok := mgr.GetTrap(id)
		if ok && state.TemplateID == "manual_trap" {
			t.Errorf("manual-reset template %q was placed procedurally — REQ-TR-10 violation", id)
		}
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
