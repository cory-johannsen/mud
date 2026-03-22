package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
)

// TestImmobileNPCFleeSkipped verifies that an immobile NPC's RoomID does not change
// when fleeNPCLocked is called.
func TestImmobileNPCFleeSkipped(t *testing.T) {
	inst := &npc.Instance{
		ID:       "turret1",
		Immobile: true,
		RoomID:   "room1",
	}
	// ExportedFleeNPCImmobile calls fleeNPCLocked and reports whether the room changed.
	changed := gameserver.ExportedFleeNPCImmobile(inst)
	if changed {
		t.Error("expected immobile NPC to be skipped by flee; RoomID must not change")
	}
	if inst.RoomID != "room1" {
		t.Errorf("expected RoomID to remain 'room1', got %q", inst.RoomID)
	}
}

// TestImmobileFlag verifies that the Immobile field is propagated from template to instance.
func TestImmobileFlag(t *testing.T) {
	tmpl := &npc.Template{
		ID:       "turret",
		Name:     "Auto-Turret",
		Type:     "machine",
		Level:    5,
		MaxHP:    50,
		AC:       16,
		Immobile: true,
	}
	inst := npc.NewInstanceWithResolver("t1", tmpl, "room1", nil)
	if !inst.Immobile {
		t.Error("expected Immobile == true for machine template with immobile: true")
	}
}

// TestNonImmobileFlag verifies that non-immobile instances have Immobile == false.
func TestNonImmobileFlag(t *testing.T) {
	tmpl := &npc.Template{
		ID:    "dog",
		Name:  "Dog",
		Type:  "animal",
		Level: 2,
		MaxHP: 18,
		AC:    12,
	}
	inst := npc.NewInstanceWithResolver("d1", tmpl, "room1", nil)
	if inst.Immobile {
		t.Error("expected Immobile == false for animal without immobile flag")
	}
}
