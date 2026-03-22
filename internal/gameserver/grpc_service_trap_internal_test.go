package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/trap"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
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
	svc := newTestGameServiceServer(
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

// TestCheckEntryTraps_HonkeypotRegionGating verifies that a TriggerRegion (honkeypot) trap
// fires only for a player whose home region is in TargetRegions, and does NOT fire for a
// player whose region is absent from that list.
//
// Precondition: trap is armed; TriggerAction == "entry"; TargetRegions == ["lake_oswego"].
// Postcondition: non-targeted player leaves Armed=true; targeted player leaves Armed=false.
func TestCheckEntryTraps_HonkeypotRegionGating(t *testing.T) {
	svc, mgr := makeTrapSvc(t)

	// Add a TriggerRegion template to the service's trapTemplates map.
	svc.trapTemplates["honkeypot_test"] = &trap.TrapTemplate{
		ID:            "honkeypot_test",
		Name:          "Test Honkeypot",
		Trigger:       trap.TriggerRegion,
		TriggerAction: "entry",
		TargetRegions: []string{"lake_oswego"},
		ResetMode:     trap.ResetOneShot,
		Payload:       &trap.TrapPayload{Type: "honkeypot"},
	}

	room := &world.Room{
		ID:     "room_a",
		ZoneID: "test",
	}
	instanceID := trap.TrapInstanceID("test", "room_a", "room", "honkeypot_test")
	mgr.AddTrap(instanceID, "honkeypot_test", true)

	// Non-targeted player (region "beaverton"): trap must NOT fire.
	sess1 := &session.PlayerSession{UID: "uid1", RoomID: "room_a", Region: "beaverton"}
	svc.checkEntryTraps("uid1", sess1, room)

	state, ok := mgr.GetTrap(instanceID)
	if !ok {
		t.Fatal("trap should still exist after non-targeted player entry")
	}
	if !state.Armed {
		t.Error("non-targeted player must not trigger honkeypot: trap should remain armed")
	}

	// Targeted player (region "lake_oswego"): trap MUST fire and be disarmed.
	sess2 := &session.PlayerSession{UID: "uid2", RoomID: "room_a", Region: "lake_oswego"}
	svc.checkEntryTraps("uid2", sess2, room)

	state2, ok2 := mgr.GetTrap(instanceID)
	if !ok2 {
		t.Fatal("trap should still exist after targeted player entry")
	}
	if state2.Armed {
		t.Error("targeted player must trigger honkeypot: trap should be disarmed (Armed=false)")
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
	svc := newTestGameServiceServer(
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

// TestCheckConsumableTraps_SkipsNonConsumable verifies that checkConsumableTraps ignores
// world traps (IsConsumable=false) and leaves them armed.
func TestCheckConsumableTraps_SkipsNonConsumable(t *testing.T) {
	svc, trapMgr := makeTrapSvc(t)

	mineTmpl := &trap.TrapTemplate{
		ID: "mine_ent", Name: "Mine",
		Trigger: trap.TriggerEntry, TriggerRangeFt: 5, ResetMode: trap.ResetOneShot,
		Payload: &trap.TrapPayload{Type: "mine"},
	}
	svc.trapTemplates["mine_ent"] = mineTmpl

	// Add as a WORLD trap (not consumable) — checkConsumableTraps must skip it.
	instanceID := trap.TrapInstanceID("test", "room_a", "room", "world-mine")
	trapMgr.AddTrap(instanceID, "mine_ent", true)

	svc.checkConsumableTraps("room_a", "any-combatant")
	inst, ok := trapMgr.GetTrap(instanceID)
	require.True(t, ok)
	assert.True(t, inst.Armed, "world trap must not be fired by checkConsumableTraps")
}

// TestCheckConsumableTraps_NoActiveCombat_NoPanic verifies that checkConsumableTraps
// returns safely when no active combat is present for the room.
func TestCheckConsumableTraps_NoActiveCombat_NoPanic(t *testing.T) {
	svc, trapMgr := makeTrapSvc(t)
	mineTmpl := &trap.TrapTemplate{
		ID: "mine_nc", Name: "Mine", TriggerRangeFt: 5, ResetMode: trap.ResetOneShot,
		Payload: &trap.TrapPayload{Type: "mine"},
	}
	svc.trapTemplates["mine_nc"] = mineTmpl
	instanceID := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "nc-1")
	require.NoError(t, trapMgr.AddConsumableTrap(instanceID, mineTmpl, 0))

	// No active combat — must return without panic.
	assert.NotPanics(t, func() {
		svc.checkConsumableTraps("room_a", "combatant-x")
	})
	// Trap remains armed (no combat to resolve position).
	inst, ok := trapMgr.GetTrap(instanceID)
	require.True(t, ok)
	assert.True(t, inst.Armed)
}

// REQ-CTR-12: Deployed consumable trap must be disarmable via the existing disarm command path.
// Verifies that findDetectedTrap locates consumable traps so handleDisarmTrap can disarm them.
func TestConsumableTrap_DisarmableViaExistingPath(t *testing.T) {
	svc, trapMgr := makeTrapSvc(t)
	// Wire fixed-high dice so Thievery check always succeeds (roll 20 >= DisableDC 15).
	svc.dice = dice.NewLoggedRoller(&fixedDiceSource{val: 19}, zaptest.NewLogger(t))

	mineTmpl := &trap.TrapTemplate{
		ID: "mine_dis", Name: "Mine", TriggerRangeFt: 5, DisableDC: 15,
		ResetMode: trap.ResetOneShot,
		Payload:   &trap.TrapPayload{Type: "mine"},
	}
	svc.trapTemplates["mine_dis"] = mineTmpl

	instanceID := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "dis-1")
	require.NoError(t, trapMgr.AddConsumableTrap(instanceID, mineTmpl, 5))

	// Add player session in room_a so handleDisarmTrap can look up the session and room.
	_, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "disarmer", Username: "disarmer", CharName: "disarmer", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)

	// Mark the trap detected so findDetectedTrap can return it.
	trapMgr.MarkDetected("disarmer", instanceID)
	require.True(t, trapMgr.IsDetected("disarmer", instanceID))

	// Invoke the actual disarm path (REQ-CTR-12: consumable trap must be found + disarmed).
	ev, err := svc.handleDisarmTrap("disarmer", &gamev1.DisarmTrapRequest{TrapName: "mine"})
	require.NoError(t, err)
	_ = ev

	inst, ok := trapMgr.GetTrap(instanceID)
	require.True(t, ok)
	assert.False(t, inst.Armed, "consumable trap must be disarmable via handleDisarmTrap")
}

// REQ-CTR-11: Out-of-combat consumable trap fires on next room entry.
func TestConsumableTrap_OutOfCombat_FiresOnRoomEntry(t *testing.T) {
	svc, trapMgr := makeTrapSvc(t)

	mineTmpl := &trap.TrapTemplate{
		ID: "mine_ooc", Name: "Mine",
		Trigger: trap.TriggerEntry, TriggerRangeFt: 5,
		ResetMode: trap.ResetOneShot,
		Payload:   &trap.TrapPayload{Type: "mine", Damage: "1d4"},
	}
	svc.trapTemplates["mine_ooc"] = mineTmpl

	instanceID := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "ooc-1")
	require.NoError(t, trapMgr.AddConsumableTrap(instanceID, mineTmpl, 0))

	sess, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
		UID: "ooc_victim", Username: "ooc_victim", CharName: "ooc_victim", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()

	room, ok := svc.world.GetRoom("room_a")
	require.True(t, ok, "room_a must exist in test world")

	svc.checkEntryTraps("ooc_victim", sess, room)

	inst, ok := trapMgr.GetTrap(instanceID)
	require.True(t, ok)
	assert.False(t, inst.Armed, "one-shot consumable trap must disarm after firing on room entry")
}

// REQ-CTR-7: Multiple overlapping consumable traps must all fire independently.
func TestCheckConsumableTraps_MultipleOverlapping_AllFire(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(),
		nil, nil, nil, nil, nil, nil, nil,
	)
	trapMgr := trap.NewTrapManager()
	mineTmpl := &trap.TrapTemplate{
		ID: "mine_multi", Name: "Mine",
		TriggerRangeFt: 5, BlastRadiusFt: 0, ResetMode: trap.ResetOneShot,
		Payload: &trap.TrapPayload{Type: "mine"},
	}
	tmplMap := map[string]*trap.TrapTemplate{"mine_multi": mineTmpl}
	svc := newTestGameServiceServer(
		worldMgr, sessMgr, nil,
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		zaptest.NewLogger(t),
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		trapMgr, tmplMap,
	)

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-multi", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, "room_a")
	require.NoError(t, err)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "multi_mover", Username: "multi_mover", CharName: "multi_mover", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat
	_, err = combatHandler.Attack("multi_mover", "Guard")
	require.NoError(t, err)
	combatHandler.cancelTimer("room_a")

	id1 := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "multi-1")
	id2 := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "multi-2")
	require.NoError(t, trapMgr.AddConsumableTrap(id1, mineTmpl, 0))
	require.NoError(t, trapMgr.AddConsumableTrap(id2, mineTmpl, 0))

	svc.checkConsumableTraps("room_a", "multi_mover")

	inst1, ok1 := trapMgr.GetTrap(id1)
	inst2, ok2 := trapMgr.GetTrap(id2)
	require.True(t, ok1)
	require.True(t, ok2)
	assert.False(t, inst1.Armed, "first overlapping trap must be disarmed (fired)")
	assert.False(t, inst2.Armed, "second overlapping trap must be disarmed independently (fired)")
}

// REQ-CTR-9: Blast-radius trap fires on all combatants within radius.
// REQ-CTR-10: Blast-radius trap is one-shot and must be disarmed after firing.
func TestCheckConsumableTraps_BlastRadius_DisarmsAfterFiring(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(),
		nil, nil, nil, nil, nil, nil, nil,
	)
	trapMgr := trap.NewTrapManager()
	mineTmpl := &trap.TrapTemplate{
		ID: "mine_blast", Name: "Mine",
		TriggerRangeFt: 5, BlastRadiusFt: 10, ResetMode: trap.ResetOneShot,
		Payload: &trap.TrapPayload{Type: "mine", Damage: "1d6"},
	}
	tmplMap := map[string]*trap.TrapTemplate{"mine_blast": mineTmpl}
	svc := newTestGameServiceServer(
		worldMgr, sessMgr, nil,
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		zaptest.NewLogger(t),
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		trapMgr, tmplMap,
	)

	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-blast", Name: "Guard", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, "room_a")
	require.NoError(t, err)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "blast_mover", Username: "blast_mover", CharName: "blast_mover", Role: "player",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat
	_, err = combatHandler.Attack("blast_mover", "Guard")
	require.NoError(t, err)
	combatHandler.cancelTimer("room_a")

	instanceID := trap.TrapInstanceID("test", "room_a", trap.TrapKindConsumable, "blast-1")
	require.NoError(t, trapMgr.AddConsumableTrap(instanceID, mineTmpl, 0))

	svc.checkConsumableTraps("room_a", "blast_mover")

	inst, ok := trapMgr.GetTrap(instanceID)
	require.True(t, ok)
	assert.False(t, inst.Armed, "blast-radius trap must be disarmed after firing (REQ-CTR-10)")

	// REQ-CTR-9: Verify player combatant actually took damage from the blast.
	combatants := combatHandler.CombatantsInRoom("room_a")
	var moverCbt *combat.Combatant
	for _, c := range combatants {
		if c.ID == "blast_mover" {
			moverCbt = c
			break
		}
	}
	require.NotNil(t, moverCbt, "blast_mover combatant must exist in room_a")
	assert.Less(t, moverCbt.CurrentHP, 10, "blast should reduce player combatant HP within blast radius (REQ-CTR-9)")
}

// TestProperty_CheckConsumableTraps_NoPanicWithArbitraryInputs verifies that
// checkConsumableTraps never panics regardless of room or combatant IDs.
func TestProperty_CheckConsumableTraps_NoPanicWithArbitraryInputs(t *testing.T) {
	// Build the service once outside the rapid loop (rapid.T is not *testing.T).
	svc, _ := makeTrapSvc(t)
	rapid.Check(t, func(rt *rapid.T) {
		roomID := rapid.SampledFrom([]string{"room_a", "room_b", "nonexistent"}).Draw(rt, "room")
		uid := rapid.StringN(1, 20, -1).Draw(rt, "uid")
		assert.NotPanics(t, func() {
			svc.checkConsumableTraps(roomID, uid)
		})
	})
}
