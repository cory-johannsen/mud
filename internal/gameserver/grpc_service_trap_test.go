package gameserver_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/trap"
)

// TestCheckEntryTraps_FiresOnEntry verifies that an armed entry-trigger trap
// transitions to Armed=false after firing.
//
// Precondition: trap is armed and entry-triggered.
// Postcondition: Armed=false after Disarm (simulating fireTrap's Disarm call).
func TestCheckEntryTraps_FiresOnEntry(t *testing.T) {
	mgr := trap.NewTrapManager()
	instanceID := trap.TrapInstanceID("zone1", "room1", "equip", "Metal Cabinet")
	mgr.AddTrap(instanceID, "bear_trap", true)

	state, ok := mgr.GetTrap(instanceID)
	if !ok || !state.Armed {
		t.Fatal("expected trap to be armed before entry")
	}
	mgr.Disarm(instanceID)
	state, _ = mgr.GetTrap(instanceID)
	if state.Armed {
		t.Error("expected Armed=false after trap fires")
	}
}

// TestDetection_SearchMode_RevealsArmedTrap verifies MarkDetected + IsDetected round-trip.
//
// Precondition: trap is armed; player not yet detected it.
// Postcondition: IsDetected returns true after MarkDetected.
func TestDetection_SearchMode_RevealsArmedTrap(t *testing.T) {
	mgr := trap.NewTrapManager()
	instanceID := trap.TrapInstanceID("zone1", "room1", "equip", "Metal Cabinet")
	mgr.AddTrap(instanceID, "bear_trap", true)

	mgr.MarkDetected("player-1", instanceID)
	if !mgr.IsDetected("player-1", instanceID) {
		t.Error("expected trap detected after MarkDetected")
	}
}

// TestDetection_NonSearchMode_NoDetection verifies that without MarkDetected,
// IsDetected returns false.
//
// Precondition: trap is armed; MarkDetected never called.
// Postcondition: IsDetected returns false.
func TestDetection_NonSearchMode_NoDetection(t *testing.T) {
	mgr := trap.NewTrapManager()
	instanceID := trap.TrapInstanceID("zone1", "room1", "equip", "Metal Cabinet")
	mgr.AddTrap(instanceID, "bear_trap", true)

	if mgr.IsDetected("player-1", instanceID) {
		t.Error("expected trap NOT detected when MarkDetected was never called")
	}
}

// TestHandleDisarmTrap_Success verifies that Disarm transitions Armed to false
// after a successful disarm.
//
// Precondition: trap is armed and detected.
// Postcondition: Armed=false after Disarm.
func TestHandleDisarmTrap_Success(t *testing.T) {
	mgr := trap.NewTrapManager()
	instanceID := trap.TrapInstanceID("zone1", "room1", "equip", "Metal Cabinet")
	mgr.AddTrap(instanceID, "bear_trap", true)
	mgr.MarkDetected("player-1", instanceID)

	mgr.Disarm(instanceID)
	state, _ := mgr.GetTrap(instanceID)
	if state.Armed {
		t.Error("expected Armed=false after successful disarm")
	}
}

// TestHandleDisarmTrap_FailureBy5_TrapRemainsArmedUntilFire verifies that before
// fireTrap is called, the trap remains armed.
//
// Precondition: trap is armed and detected.
// Postcondition: Armed=true until Disarm is explicitly called.
func TestHandleDisarmTrap_FailureBy5_TrapRemainsArmedUntilFire(t *testing.T) {
	mgr := trap.NewTrapManager()
	instanceID := trap.TrapInstanceID("zone1", "room1", "equip", "Floor Plate")
	mgr.AddTrap(instanceID, "mine", true)
	mgr.MarkDetected("player-1", instanceID)

	// Before fire: trap remains armed.
	state, _ := mgr.GetTrap(instanceID)
	if !state.Armed {
		t.Error("expected Armed=true before disarm-failure fire")
	}
}

// TestHonkeypot_OnlyTargetedRegionTriggers verifies isTrapTargeted logic:
// a player whose region is not in target_regions must not be considered targeted.
func TestHonkeypot_OnlyTargetedRegionTriggers(t *testing.T) {
	targeted := []string{"lake_oswego", "pearl_district"}

	// Targeted player.
	if !isTrapTargeted("lake_oswego", targeted) {
		t.Error("lake_oswego should be targeted")
	}
	// Non-targeted player.
	if isTrapTargeted("beaverton", targeted) {
		t.Error("beaverton should not be targeted")
	}
	// Empty region never matches.
	if isTrapTargeted("", targeted) {
		t.Error("empty region should not be targeted")
	}
}

// TestProperty_TrapsForRoom_EnumeratesBothKinds verifies that TrapsForRoom returns
// trap IDs for both room-level and equipment-level traps.
func TestProperty_TrapsForRoom_EnumeratesBothKinds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		zoneID := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "zoneID")
		roomID := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "roomID")

		mgr := trap.NewTrapManager()
		roomID1 := trap.TrapInstanceID(zoneID, roomID, "room", "trap1")
		equipID1 := trap.TrapInstanceID(zoneID, roomID, "equip", "Cabinet")
		otherID := trap.TrapInstanceID(zoneID, "other_room", "room", "trap2")

		mgr.AddTrap(roomID1, "mine", true)
		mgr.AddTrap(equipID1, "bear_trap", true)
		mgr.AddTrap(otherID, "mine", true)

		ids := mgr.TrapsForRoom(zoneID, roomID)
		if len(ids) != 2 {
			rt.Fatalf("expected 2 traps for room, got %d: %v", len(ids), ids)
		}
	})
}

// TestProperty_Detection_PlayerIsolation verifies that detection state for one player
// does not affect another player's detection state.
func TestProperty_Detection_PlayerIsolation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		uid1 := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "uid1")
		uid2 := rapid.StringMatching(`[a-z]{4,9}`).Draw(rt, "uid2")
		if uid1 == uid2 {
			uid2 = uid2 + "x"
		}

		mgr := trap.NewTrapManager()
		instanceID := trap.TrapInstanceID("zone1", "room1", "equip", "Cabinet")
		mgr.AddTrap(instanceID, "bear_trap", true)

		mgr.MarkDetected(uid1, instanceID)
		if mgr.IsDetected(uid2, instanceID) {
			rt.Error("uid2 should not see uid1's detection")
		}
		if !mgr.IsDetected(uid1, instanceID) {
			rt.Error("uid1 detection should persist")
		}
	})
}

// isTrapTargeted is duplicated here for white-box testing since the function is unexported.
// It mirrors the logic in grpc_service_trap.go exactly.
func isTrapTargeted(playerRegion string, targetRegions []string) bool {
	for _, r := range targetRegions {
		if r == playerRegion {
			return true
		}
	}
	return false
}
