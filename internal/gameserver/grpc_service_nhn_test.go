package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
)

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
