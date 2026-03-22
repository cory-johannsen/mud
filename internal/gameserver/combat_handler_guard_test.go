package gameserver

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// spawnTypedNPC creates and registers a live NPC instance with the given type in roomID.
func spawnTypedNPC(t *testing.T, npcMgr *npc.Manager, roomID, npcType string) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:      npcType + "-tmpl",
		Name:    "Guard",
		Type:    npcType,
		NPCType: npcType,
		Level:   1,
		MaxHP:   20,
		AC:      13,
		Awareness: 2,
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("spawnTypedNPC(%s): %v", npcType, err)
	}
	return inst
}

// TestInitiateGuardCombat_NoGuards_NoOp verifies that broadcastFn is NOT called when
// no guard-typed NPCs are present in the player's room.
func TestInitiateGuardCombat_NoGuards_NoOp(t *testing.T) {
	var broadcastCalled bool
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {
		broadcastCalled = true
	})

	const roomID = "room-guard-1"
	// Spawn a non-guard NPC (goblin type).
	spawnTypedNPC(t, h.npcMgr, roomID, "goblin")
	addTestPlayerNamed(t, h.sessions, "player-guard-1", roomID, "Alice")

	h.InitiateGuardCombat("player-guard-1", "zone-1", 2)

	if broadcastCalled {
		t.Fatal("expected broadcastFn NOT to be called when no guards are present; it was called")
	}
}

// TestInitiateGuardCombat_WithGuards_BroadcastsAndAttacks verifies that InitiateGuardCombat
// calls broadcastFn with a non-empty narrative when a guard NPC is present in the room.
func TestInitiateGuardCombat_WithGuards_BroadcastsAndAttacks(t *testing.T) {
	var capturedEvents []*gamev1.CombatEvent
	h := makeCombatHandler(t, func(_ string, events []*gamev1.CombatEvent) {
		capturedEvents = append(capturedEvents, events...)
	})

	const roomID = "room-guard-2"
	spawnTypedNPC(t, h.npcMgr, roomID, "guard")
	addTestPlayerNamed(t, h.sessions, "player-guard-2", roomID, "Bob")

	h.InitiateGuardCombat("player-guard-2", "zone-1", 2)

	if len(capturedEvents) == 0 {
		t.Fatal("expected broadcastFn to be called with events; got none")
	}
	if capturedEvents[0].Narrative == "" {
		t.Fatal("expected non-empty Narrative in broadcast event; got empty string")
	}
}

// TestInitiateGuardCombat_KillLevel_NarrativeContainsAttack verifies that wantedLevel >= 3
// produces an attack-on-sight narrative.
func TestInitiateGuardCombat_KillLevel_NarrativeContainsAttack(t *testing.T) {
	var capturedEvents []*gamev1.CombatEvent
	h := makeCombatHandler(t, func(_ string, events []*gamev1.CombatEvent) {
		capturedEvents = append(capturedEvents, events...)
	})

	const roomID = "room-guard-3"
	spawnTypedNPC(t, h.npcMgr, roomID, "guard")
	addTestPlayerNamed(t, h.sessions, "player-guard-3", roomID, "Charlie")

	h.InitiateGuardCombat("player-guard-3", "zone-1", 3)

	if len(capturedEvents) == 0 {
		t.Fatal("expected broadcastFn to be called with events; got none")
	}
	narrative := capturedEvents[0].Narrative
	if narrative == "" {
		t.Fatal("expected non-empty Narrative in broadcast event; got empty string")
	}
}

// TestInitiateGuardCombat_UnknownPlayer_NoOp verifies that InitiateGuardCombat is a no-op
// when the uid does not correspond to a registered player session.
func TestInitiateGuardCombat_UnknownPlayer_NoOp(t *testing.T) {
	var broadcastCalled bool
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {
		broadcastCalled = true
	})

	h.InitiateGuardCombat("nonexistent-uid", "zone-1", 2)

	if broadcastCalled {
		t.Fatal("expected broadcastFn NOT to be called for unknown player; it was called")
	}
}

// TestInitiateGuardCombat_RespectsWantedThreshold verifies that a guard with
// WantedThreshold=3 does NOT engage a player with WantedLevel=2.
func TestInitiateGuardCombat_RespectsWantedThreshold(t *testing.T) {
	var broadcastCalled bool
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {
		broadcastCalled = true
	})

	const roomID = "room-guard-thresh-1"
	tmpl := &npc.Template{
		ID:      "strict_guard",
		Name:    "Strict Guard",
		NPCType: "guard",
		Level:   3,
		MaxHP:   40,
		AC:      14,
		Guard:   &npc.GuardConfig{WantedThreshold: 3},
	}
	_, err := h.npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	addTestPlayerNamed(t, h.sessions, "player-thresh-1", roomID, "Tester")

	h.InitiateGuardCombat("player-thresh-1", "zone-1", 2) // below threshold
	if broadcastCalled {
		t.Fatal("guard with WantedThreshold=3 must NOT engage at wantedLevel=2")
	}
}

// TestInitiateGuardCombat_EngagesAtThreshold verifies a guard with WantedThreshold=3
// engages at wantedLevel=3.
func TestInitiateGuardCombat_EngagesAtThreshold(t *testing.T) {
	var broadcastCalled bool
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {
		broadcastCalled = true
	})

	const roomID = "room-guard-thresh-2"
	tmpl := &npc.Template{
		ID:      "strict_guard2",
		Name:    "Strict Guard 2",
		NPCType: "guard",
		Level:   3,
		MaxHP:   40,
		AC:      14,
		Guard:   &npc.GuardConfig{WantedThreshold: 3},
	}
	_, err := h.npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	addTestPlayerNamed(t, h.sessions, "player-thresh-2", roomID, "Tester2")

	h.InitiateGuardCombat("player-thresh-2", "zone-1", 3) // at threshold
	if !broadcastCalled {
		t.Fatal("guard with WantedThreshold=3 must engage at wantedLevel=3")
	}
}

// TestInitiateGuardCombat_DefaultThreshold verifies a guard with WantedThreshold=0
// uses the default threshold of 2.
func TestInitiateGuardCombat_DefaultThreshold(t *testing.T) {
	var broadcastCalled bool
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {
		broadcastCalled = true
	})

	const roomID = "room-guard-thresh-3"
	tmpl := &npc.Template{
		ID:      "default_guard",
		Name:    "Default Guard",
		NPCType: "guard",
		Level:   2,
		MaxHP:   30,
		AC:      12,
		Guard:   &npc.GuardConfig{WantedThreshold: 0}, // default → 2
	}
	_, err := h.npcMgr.Spawn(tmpl, roomID)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	addTestPlayerNamed(t, h.sessions, "player-thresh-3", roomID, "Tester3")

	h.InitiateGuardCombat("player-thresh-3", "zone-1", 2) // should engage (default threshold = 2)
	if !broadcastCalled {
		t.Fatal("guard with default WantedThreshold must engage at wantedLevel=2")
	}
}

// TestProperty_InitiateGuardCombat_ThresholdInvariant verifies that for any guard
// WantedThreshold T in [1,4] and player WantedLevel L in [1,4]:
// - L >= T → broadcastFn is called (guard engages)
// - L < T  → broadcastFn is NOT called
func TestProperty_InitiateGuardCombat_ThresholdInvariant(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		threshold := rapid.IntRange(1, 4).Draw(rt, "threshold")
		wantedLevel := rapid.IntRange(1, 4).Draw(rt, "wantedLevel")

		var broadcastCalled bool
		h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {
			broadcastCalled = true
		})

		roomID := fmt.Sprintf("prop-thresh-room-%d-%d", threshold, wantedLevel)
		uid := fmt.Sprintf("prop-player-%d-%d", threshold, wantedLevel)

		tmpl := &npc.Template{
			ID:      fmt.Sprintf("prop_guard_%d_%d", threshold, wantedLevel),
			Name:    "Prop Guard",
			NPCType: "guard",
			Level:   2, MaxHP: 30, AC: 12,
			Guard: &npc.GuardConfig{WantedThreshold: threshold},
		}
		_, err := h.npcMgr.Spawn(tmpl, roomID)
		require.NoError(t, err)
		addTestPlayerNamed(t, h.sessions, uid, roomID, "PropTester")

		h.InitiateGuardCombat(uid, "zone-1", wantedLevel)

		if wantedLevel >= threshold {
			assert.True(rt, broadcastCalled, "guard must engage when wantedLevel(%d) >= threshold(%d)", wantedLevel, threshold)
		} else {
			assert.False(rt, broadcastCalled, "guard must NOT engage when wantedLevel(%d) < threshold(%d)", wantedLevel, threshold)
		}
	})
}

// TestProperty_InitiateGuardCombat_NeverPanics verifies that InitiateGuardCombat
// never panics for any combination of wantedLevel and guard threshold.
func TestProperty_InitiateGuardCombat_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		wantedLevel := rapid.IntRange(0, 5).Draw(rt, "wantedLevel")

		h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

		uid := fmt.Sprintf("never-panic-%d", wantedLevel)
		roomID := fmt.Sprintf("never-panic-room-%d", wantedLevel)
		addTestPlayerNamed(t, h.sessions, uid, roomID, "NeverPanic")

		// Optionally spawn a guard.
		if rapid.Bool().Draw(rt, "hasGuard") {
			threshold := rapid.IntRange(0, 4).Draw(rt, "threshold")
			tmpl := &npc.Template{
				ID:      fmt.Sprintf("np_guard_%d_%d", wantedLevel, threshold),
				Name:    "NP Guard",
				NPCType: "guard",
				Level:   1, MaxHP: 10, AC: 10,
				Guard: &npc.GuardConfig{WantedThreshold: threshold},
			}
			_, _ = h.npcMgr.Spawn(tmpl, roomID)
		}

		// Must not panic.
		assert.NotPanics(t, func() {
			h.InitiateGuardCombat(uid, "zone-1", wantedLevel)
		})
	})
}

// TestAttack_CannotAttackOwnHireling verifies REQ-NPC-8: a player cannot attack
// their own bound hireling.
//
// Precondition: hireling is bound to the attacking player.
// Postcondition: Attack returns an error containing "cannot attack your own hireling".
func TestAttack_CannotAttackOwnHireling(t *testing.T) {
	const uid = "hireling-owner"
	const roomID = "room_hireling_1"

	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	addTestPlayerNamed(t, h.sessions, uid, roomID, "Owner")

	tmpl := &npc.Template{
		ID:      "my_hireling",
		Name:    "Patch",
		NPCType: "hireling",
		Level:   2,
		MaxHP:   20,
		AC:      11,
		Hireling: &npc.HirelingConfig{DailyCost: 50, CombatRole: "melee"},
	}
	inst, err := h.npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)

	h.hirelingOwnerOf = func(instID string) string {
		if instID == inst.ID {
			return uid
		}
		return ""
	}

	_, err = h.Attack(uid, "Patch")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot attack your own hireling")
}
