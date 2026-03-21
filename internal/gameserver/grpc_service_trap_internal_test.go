package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/trap"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"go.uber.org/zap/zaptest"
)

// makeTrapSvc returns a minimal GameServiceServer with trapMgr and trapTemplates wired.
// The world contains zone "test" with rooms "room_a" and "room_b" (from testWorldAndSession).
func makeTrapSvc(t *testing.T) (*GameServiceServer, *trap.TrapManager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	mgr := trap.NewTrapManager()
	// "mine_entry" is the linked payload template for "pressure_plate_mine".
	// TriggerPressurePlate requires PayloadTemplate referencing an entry-trigger template.
	tmplMap := map[string]*trap.TrapTemplate{
		"mine_entry": {
			ID:      "mine_entry",
			Trigger: trap.TriggerEntry,
			Payload: &trap.TrapPayload{Type: "mine"},
		},
		"pressure_plate_mine": {
			ID:              "pressure_plate_mine",
			Trigger:         trap.TriggerPressurePlate,
			PayloadTemplate: "mine_entry",
		},
		"bear_trap_interact": {
			ID:      "bear_trap_interact",
			Trigger: trap.TriggerInteraction,
			Payload: &trap.TrapPayload{Type: "bear_trap"},
		},
	}
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		nil, // cmdRegistry
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		zaptest.NewLogger(t),
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		mgr, tmplMap,
	)
	return svc, mgr
}

// TestCheckPressurePlateTraps_FiresArmedTrap verifies that an armed pressure_plate trap
// is disarmed (fired) when checkPressurePlateTraps is called during combat.
//
// Precondition: trap is armed; player is in combat; room belongs to zone "test".
// Postcondition: Armed=false after checkPressurePlateTraps fires the trap.
func TestCheckPressurePlateTraps_FiresArmedTrap(t *testing.T) {
	svc, mgr := makeTrapSvc(t)

	// Use zone "test" / room "room_a" which exist in testWorldAndSession's world.
	room := &world.Room{
		ID:     "room_a",
		ZoneID: "test",
		Equipment: []world.RoomEquipmentConfig{
			{ItemID: "plate_01", Description: "Floor Plate", TrapTemplate: "pressure_plate_mine"},
		},
	}
	instanceID := trap.TrapInstanceID("test", "room_a", "equip", "Floor Plate")
	mgr.AddTrap(instanceID, "pressure_plate_mine", true)

	sess := &session.PlayerSession{UID: "p1", RoomID: "room_a", Status: statusInCombat}

	svc.checkPressurePlateTraps("p1", sess, room)

	state, ok := mgr.GetTrap(instanceID)
	if !ok {
		t.Fatal("trap should still exist in manager")
	}
	if state.Armed {
		t.Error("expected trap to be disarmed after checkPressurePlateTraps fires it")
	}
}

// TestCheckPressurePlateTraps_NoFireOutOfCombat verifies that a pressure_plate trap
// does NOT fire when the player is not in combat.
//
// Precondition: trap is armed; player is NOT in combat.
// Postcondition: Armed=true (trap not fired) after checkPressurePlateTraps returns.
func TestCheckPressurePlateTraps_NoFireOutOfCombat(t *testing.T) {
	svc, mgr := makeTrapSvc(t)

	room := &world.Room{
		ID:     "room_a",
		ZoneID: "test",
		Equipment: []world.RoomEquipmentConfig{
			{ItemID: "plate_01", Description: "Floor Plate", TrapTemplate: "pressure_plate_mine"},
		},
	}
	instanceID := trap.TrapInstanceID("test", "room_a", "equip", "Floor Plate")
	mgr.AddTrap(instanceID, "pressure_plate_mine", true)

	// Out of combat: Status is 0 (not statusInCombat).
	sess := &session.PlayerSession{UID: "p1", RoomID: "room_a", Status: 0}

	svc.checkPressurePlateTraps("p1", sess, room)

	state, ok := mgr.GetTrap(instanceID)
	if !ok {
		t.Fatal("trap should still exist")
	}
	if !state.Armed {
		t.Error("trap should remain armed when player is not in combat")
	}
}

// TestCheckInteractionTrap_FiresArmedTrap verifies that an armed interaction-trigger trap
// fires when checkInteractionTrap is called for the matching equipment.
//
// Precondition: trap is armed; equipment matches equipItemID.
// Postcondition: Armed=false after checkInteractionTrap fires the trap.
func TestCheckInteractionTrap_FiresArmedTrap(t *testing.T) {
	svc, mgr := makeTrapSvc(t)

	room := &world.Room{
		ID:     "room_a",
		ZoneID: "test",
		Equipment: []world.RoomEquipmentConfig{
			{ItemID: "cabinet_01", Description: "Metal Cabinet", TrapTemplate: "bear_trap_interact"},
		},
	}
	instanceID := trap.TrapInstanceID("test", "room_a", "equip", "Metal Cabinet")
	mgr.AddTrap(instanceID, "bear_trap_interact", true)

	sess := &session.PlayerSession{UID: "p1", RoomID: "room_a"}

	svc.checkInteractionTrap("p1", sess, room, "cabinet_01")

	state, ok := mgr.GetTrap(instanceID)
	if !ok {
		t.Fatal("trap should still exist")
	}
	if state.Armed {
		t.Error("expected trap to be disarmed after interaction fires it")
	}
}

// TestCheckInteractionTrap_NoFireWrongTrigger verifies that a non-interaction trap
// does NOT fire via checkInteractionTrap.
//
// Precondition: trap is armed but has TriggerPressurePlate, not TriggerInteraction.
// Postcondition: Armed=true (trap not fired) — wrong trigger type is rejected.
func TestCheckInteractionTrap_NoFireWrongTrigger(t *testing.T) {
	svc, mgr := makeTrapSvc(t)

	room := &world.Room{
		ID:     "room_a",
		ZoneID: "test",
		Equipment: []world.RoomEquipmentConfig{
			{ItemID: "plate_01", Description: "Floor Plate", TrapTemplate: "pressure_plate_mine"},
		},
	}
	instanceID := trap.TrapInstanceID("test", "room_a", "equip", "Floor Plate")
	mgr.AddTrap(instanceID, "pressure_plate_mine", true)

	sess := &session.PlayerSession{UID: "p1", RoomID: "room_a"}

	svc.checkInteractionTrap("p1", sess, room, "plate_01")

	state, ok := mgr.GetTrap(instanceID)
	if !ok {
		t.Fatal("trap should still exist")
	}
	if !state.Armed {
		t.Error("pressure plate trap should NOT fire via checkInteractionTrap (wrong trigger type)")
	}
}

// makeTrapSvcWithDisarmTemplates returns a GameServiceServer with templates suitable
// for disarm testing: a trap with DisableDC=15 and a mine for blast testing.
// The world has zone "test" / room "room_a" with Equipment pre-populated for the bear trap.
func makeTrapSvcWithDisarmTemplates(t *testing.T) (*GameServiceServer, *trap.TrapManager) {
	t.Helper()
	zone := &world.Zone{
		ID:          "test",
		Name:        "Test",
		Description: "Test zone",
		StartRoom:   "room_a",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "test",
				Title:       "Room A",
				Description: "The first room.",
				Equipment: []world.RoomEquipmentConfig{
					{ItemID: "cabinet_01", Description: "Metal Cabinet", TrapTemplate: "bear_trap_disarm"},
				},
				Properties: map[string]string{},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	if err != nil {
		t.Fatalf("failed to create world manager: %v", err)
	}
	sessMgr := session.NewManager()
	mgr := trap.NewTrapManager()
	tmplMap := map[string]*trap.TrapTemplate{
		"bear_trap_disarm": {
			ID:        "bear_trap_disarm",
			Name:      "Bear Trap",
			Trigger:   trap.TriggerInteraction,
			DisableDC: 15,
			Payload:   &trap.TrapPayload{Type: "bear_trap"},
		},
		"mine_disarm": {
			ID:        "mine_disarm",
			Name:      "Mine",
			Trigger:   trap.TriggerEntry,
			DisableDC: 10,
			Payload:   &trap.TrapPayload{Type: "mine"},
		},
	}
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		nil,
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		zaptest.NewLogger(t),
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		mgr, tmplMap,
	)
	return svc, mgr
}

// TestHandleDisarmTrap_SuccessPath exercises the success branch of handleDisarmTrap.
// Uses a fixed dice source (always rolls 20) so total >> DisableDC.
//
// Precondition: trap is armed and detected; dice always roll 20.
// Postcondition: trap Armed=false after successful disarm.
func TestHandleDisarmTrap_SuccessPath(t *testing.T) {
	svc, mgr := makeTrapSvcWithDisarmTemplates(t)

	// Wire a fixed-high dice so the roll always succeeds (1d20 → 20 >= DC 15).
	svc.dice = dice.NewLoggedRoller(&fixedDiceSource{val: 19}, zaptest.NewLogger(t))

	instanceID := trap.TrapInstanceID("test", "room_a", "equip", "Metal Cabinet")
	mgr.AddTrap(instanceID, "bear_trap_disarm", true)
	mgr.MarkDetected("p1", instanceID)

	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "p1", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	ev, err := svc.handleDisarmTrap("p1", &gamev1.DisarmTrapRequest{TrapName: "bear trap"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify trap is now disarmed.
	state, ok := mgr.GetTrap(instanceID)
	if !ok {
		t.Fatal("trap should still exist after successful disarm")
	}
	if state.Armed {
		t.Error("expected trap Armed=false after successful disarm")
	}
	_ = ev
}

// TestHandleDisarmTrap_NotDetected verifies that a trap the player hasn't detected
// cannot be disarmed.
//
// Precondition: trap is armed but NOT detected by the player.
// Postcondition: trap Armed=true (unchanged); response indicates trap not found.
func TestHandleDisarmTrap_NotDetected(t *testing.T) {
	svc, mgr := makeTrapSvcWithDisarmTemplates(t)
	svc.dice = dice.NewLoggedRoller(&fixedDiceSource{val: 19}, zaptest.NewLogger(t))

	instanceID := trap.TrapInstanceID("test", "room_a", "equip", "Metal Cabinet")
	mgr.AddTrap(instanceID, "bear_trap_disarm", true)
	// NOTE: MarkDetected is NOT called — trap is undetected.

	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "p1", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	ev, err := svc.handleDisarmTrap("p1", &gamev1.DisarmTrapRequest{TrapName: "bear trap"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Trap should still be armed — findDetectedTrap returns "" for undetected traps.
	state, _ := mgr.GetTrap(instanceID)
	if !state.Armed {
		t.Error("undetected trap should remain armed")
	}
	_ = ev
}
