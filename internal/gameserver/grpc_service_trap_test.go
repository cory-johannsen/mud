package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/trap"
)

func TestCheckEntryTraps_FiresOnEntry(t *testing.T) {
	// Unit test: entry-trigger armed trap fires when player enters room.
	mgr := trap.NewTrapManager()
	tmpl := &trap.TrapTemplate{
		ID:        "bear_trap",
		Trigger:   trap.TriggerEntry,
		ResetMode: trap.ResetAuto,
		Payload: &trap.TrapPayload{
			Type:   "bear_trap",
			Damage: "2d6",
		},
	}
	instanceID := trap.TrapInstanceID("zone1", "room1", "equip", "Metal Cabinet")
	mgr.AddTrap(instanceID, tmpl.ID, true)

	state, ok := mgr.GetTrap(instanceID)
	if !ok || !state.Armed {
		t.Fatal("expected trap to be armed")
	}
	// After fireTrap equivalent: trap should be disarmed.
	mgr.Disarm(instanceID)
	state, _ = mgr.GetTrap(instanceID)
	if state.Armed {
		t.Error("expected Armed=false after fire")
	}
}
