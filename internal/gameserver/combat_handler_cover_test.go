package gameserver

import (
	"testing"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"pgregory.net/rapid"
)

// newCoverTestHandler creates a minimal CombatHandler for cover state tests.
//
// Postcondition: Returns a non-nil CombatHandler.
func newCoverTestHandler(t *testing.T) *CombatHandler {
	t.Helper()
	return makeCombatHandlerWithDice(t, newSeqSource(1), func(_ string, _ []*gamev1.CombatEvent) {})
}

func TestRoomCoverStateManagement(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roomID := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "roomID")
		equipID := rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "equipID")
		hp := rapid.IntRange(1, 20).Draw(rt, "hp")

		h := newCoverTestHandler(t)
		h.InitCoverState(roomID, equipID, hp)

		got := h.GetCoverHP(roomID, equipID)
		if got != hp {
			rt.Errorf("GetCoverHP after init: got %d, want %d", got, hp)
		}

		h.DecrementCoverHP(roomID, equipID)
		after := h.GetCoverHP(roomID, equipID)
		if hp > 1 && after != hp-1 {
			rt.Errorf("after decrement: got %d, want %d", after, hp-1)
		}
		if hp == 1 && after != 0 {
			rt.Errorf("at 1HP after decrement: got %d, want 0", after)
		}

		h.ClearCoverForEquipment(roomID, equipID)
		if h.GetCoverHP(roomID, equipID) != -1 {
			rt.Errorf("after clear: expected -1 (unknown), got %d", h.GetCoverHP(roomID, equipID))
		}
	})
}
