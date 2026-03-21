package trap_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/trap"
	"pgregory.net/rapid"
)

func TestTrapManager_AddAndArm(t *testing.T) {
	mgr := trap.NewTrapManager()
	mgr.AddTrap("zone1/room1/room/uuid-1", "bear_trap", true)

	state, ok := mgr.GetTrap("zone1/room1/room/uuid-1")
	if !ok {
		t.Fatal("expected trap to be found")
	}
	if !state.Armed {
		t.Error("expected Armed=true")
	}
	if state.TemplateID != "bear_trap" {
		t.Errorf("TemplateID: got %q, want %q", state.TemplateID, "bear_trap")
	}

	// Disarm and verify.
	mgr.Disarm("zone1/room1/room/uuid-1")
	state, ok = mgr.GetTrap("zone1/room1/room/uuid-1")
	if !ok {
		t.Fatal("expected trap to still exist after disarm")
	}
	if state.Armed {
		t.Error("expected Armed=false after Disarm")
	}
}

func TestTrapManager_Detection_SetAndQuery(t *testing.T) {
	mgr := trap.NewTrapManager()
	mgr.AddTrap("zone1/room1/room/uuid-1", "bear_trap", true)

	if mgr.IsDetected("player-1", "zone1/room1/room/uuid-1") {
		t.Error("expected not detected before MarkDetected")
	}
	mgr.MarkDetected("player-1", "zone1/room1/room/uuid-1")
	if !mgr.IsDetected("player-1", "zone1/room1/room/uuid-1") {
		t.Error("expected detected after MarkDetected")
	}
	// Other player unaffected.
	if mgr.IsDetected("player-2", "zone1/room1/room/uuid-1") {
		t.Error("expected player-2 not detected")
	}
}

func TestTrapManager_ClearDetectionForRoom(t *testing.T) {
	mgr := trap.NewTrapManager()
	ids := []string{
		"zone1/room1/room/uuid-1",
		"zone1/room1/equip/equip-abc",
	}
	mgr.AddTrap(ids[0], "bear_trap", true)
	mgr.AddTrap(ids[1], "mine", true)

	mgr.MarkDetected("player-1", ids[0])
	mgr.MarkDetected("player-1", ids[1])
	mgr.MarkDetected("player-2", ids[0])

	// REQ-TR-12: after room reset, all detection state for those IDs is gone.
	mgr.ClearDetectionForRoom(ids)

	if mgr.IsDetected("player-1", ids[0]) {
		t.Error("player-1 should not detect ids[0] after room clear")
	}
	if mgr.IsDetected("player-1", ids[1]) {
		t.Error("player-1 should not detect ids[1] after room clear")
	}
	if mgr.IsDetected("player-2", ids[0]) {
		t.Error("player-2 should not detect ids[0] after room clear")
	}
}

func TestTrapManager_TrapsForRoom(t *testing.T) {
	mgr := trap.NewTrapManager()
	// Add traps for two different rooms.
	mgr.AddTrap("zone1/room1/room/trap-a", "bear_trap", true)
	mgr.AddTrap("zone1/room1/equip/equip-b", "mine", true)
	mgr.AddTrap("zone1/room2/room/trap-c", "pit", false)

	got := mgr.TrapsForRoom("zone1", "room1")
	if len(got) != 2 {
		t.Fatalf("TrapsForRoom(zone1, room1): got %d IDs, want 2; IDs: %v", len(got), got)
	}
	// Verify room2 trap is not included.
	for _, id := range got {
		if id == "zone1/room2/room/trap-c" {
			t.Error("TrapsForRoom(zone1, room1) must not include room2 traps")
		}
	}

	// Empty room returns empty slice.
	empty := mgr.TrapsForRoom("zone1", "room99")
	if len(empty) != 0 {
		t.Errorf("TrapsForRoom for empty room: got %d IDs, want 0", len(empty))
	}
}

func TestTrapManager_Property_DetectionIsolation(t *testing.T) {
	// Property: After MarkDetected(uid, id), IsDetected(uid, id) is true.
	// After ClearDetectionForRoom([]string{id}), IsDetected(uid, id) is false.
	rapid.Check(t, func(rt *rapid.T) {
		mgr := trap.NewTrapManager()
		uid := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "uid")
		zoneID := rapid.StringMatching(`[a-z]{3,6}`).Draw(rt, "zoneID")
		roomID := rapid.StringMatching(`[a-z]{3,6}`).Draw(rt, "roomID")
		trapID := rapid.StringMatching(`[a-z]{3,6}`).Draw(rt, "trapID")
		instanceID := trap.TrapInstanceID(zoneID, roomID, "room", trapID)

		mgr.AddTrap(instanceID, "bear_trap", true)
		mgr.MarkDetected(uid, instanceID)

		if !mgr.IsDetected(uid, instanceID) {
			rt.Fatalf("IsDetected must be true after MarkDetected; uid=%q id=%q", uid, instanceID)
		}

		mgr.ClearDetectionForRoom([]string{instanceID})

		if mgr.IsDetected(uid, instanceID) {
			rt.Fatalf("IsDetected must be false after ClearDetectionForRoom; uid=%q id=%q", uid, instanceID)
		}
	})
}
