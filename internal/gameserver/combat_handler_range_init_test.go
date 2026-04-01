package gameserver

import (
	"strings"
	"sync"
	"testing"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// TestCombatInit_RangeShownAtCombatStart verifies that when combat is first initiated
// via Attack, the returned events include a range status message showing the player's
// distance to their NPC target.
//
// This is the root cause of BUG-29: range is only shown at round 2+ start, not at
// combat initiation (round 1).
//
// Precondition: Player attacks NPC, starting combat. Player at Position=0, NPC at Position=50.
// Postcondition: The returned init events contain a narrative with "ft" and the NPC name.
func TestCombatInit_RangeShownAtCombatStart(t *testing.T) {
	var mu sync.Mutex
	broadcastFn := func(_ string, _ []*gamev1.CombatEvent) {
		mu.Lock()
		defer mu.Unlock()
	}
	h := makeCombatHandler(t, broadcastFn)
	const roomID = "room-range-init"

	inst := spawnTestNPC(t, h.npcMgr, roomID)
	addTestPlayer(t, h.sessions, "player-range", roomID)

	// Start combat.
	events, err := h.Attack("player-range", inst.Name())
	if err != nil {
		t.Fatalf("Attack: %v", err)
	}
	h.cancelTimer(roomID)

	// Check that init events include range info.
	found := false
	for _, evt := range events {
		lower := strings.ToLower(evt.Narrative)
		if strings.Contains(lower, "ft") && strings.Contains(lower, strings.ToLower(inst.Name())) {
			found = true
			break
		}
	}
	if !found {
		var narratives []string
		for _, evt := range events {
			narratives = append(narratives, evt.Narrative)
		}
		t.Errorf("BUG-29: expected range info in combat init events; got: %v", narratives)
	}
}
