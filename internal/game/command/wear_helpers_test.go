package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// makeWearSession creates a PlayerSession with an empty backpack and equipment,
// suitable for wear/remove command tests.
//
// Precondition: t and reg must be non-nil.
// Postcondition: Returns a non-nil PlayerSession with initialized Backpack, Equipment, and LoadoutSet.
func makeWearSession(t *testing.T, _ *inventory.Registry) *session.PlayerSession {
	t.Helper()
	return &session.PlayerSession{
		UID:        "test-uid",
		CharName:   "Tester",
		LoadoutSet: inventory.NewLoadoutSet(),
		Equipment:  inventory.NewEquipment(),
		Backpack:   inventory.NewBackpack(20, 100.0),
	}
}
