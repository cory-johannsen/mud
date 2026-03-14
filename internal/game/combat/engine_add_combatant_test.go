package combat_test

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// combatWithKnownInitiatives creates a combat where initiatives are pre-set (not rolled),
// so tests are deterministic.
func combatWithKnownInitiatives(t *testing.T, roomID string) (*combat.Engine, *combat.Combat) {
	t.Helper()
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, Initiative: 15},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 5},
	}
	cbt, err := eng.StartCombat(roomID, combatants, makeTestRegistry(), nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	return eng, cbt
}

// REQ-T15: AddCombatant on non-existent roomID returns error.
func TestAddCombatant_NonExistentRoom_ReturnsError(t *testing.T) {
	eng := combat.NewEngine()
	c := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 10, CurrentHP: 10, Initiative: 8}
	err := eng.AddCombatant("no-room", c)
	if err == nil {
		t.Fatal("expected error for non-existent room, got nil")
	}
}

// REQ-T5 (example): AddCombatant inserts in correct initiative-sorted position.
func TestAddCombatant_InsertsInInitiativeOrder(t *testing.T) {
	eng, cbt := combatWithKnownInitiatives(t, "room1")

	// Insert p2 with initiative 10 — should go between p1(15) and n1(5).
	p2 := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 10, CurrentHP: 10, Initiative: 10}
	if err := eng.AddCombatant("room1", p2); err != nil {
		t.Fatal(err)
	}

	// Order: p1(15), p2(10), n1(5)
	if len(cbt.Combatants) != 3 {
		t.Fatalf("want 3 combatants, got %d", len(cbt.Combatants))
	}
	if cbt.Combatants[0].ID != "p1" {
		t.Errorf("pos 0: want p1, got %s", cbt.Combatants[0].ID)
	}
	if cbt.Combatants[1].ID != "p2" {
		t.Errorf("pos 1: want p2, got %s", cbt.Combatants[1].ID)
	}
	if cbt.Combatants[2].ID != "n1" {
		t.Errorf("pos 2: want n1, got %s", cbt.Combatants[2].ID)
	}
}

// REQ-T14 (example): AddCombatant appends to Participants for KindPlayer.
func TestAddCombatant_AppendsToParticipants_Player(t *testing.T) {
	eng, cbt := combatWithKnownInitiatives(t, "room1")
	initialLen := len(cbt.Participants)

	p2 := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 10, CurrentHP: 10, Initiative: 8}
	if err := eng.AddCombatant("room1", p2); err != nil {
		t.Fatal(err)
	}

	if len(cbt.Participants) != initialLen+1 {
		t.Errorf("Participants: want %d, got %d", initialLen+1, len(cbt.Participants))
	}
	found := false
	for _, uid := range cbt.Participants {
		if uid == "p2" {
			found = true
		}
	}
	if !found {
		t.Errorf("p2 not in Participants: %v", cbt.Participants)
	}
}

// REQ-T14 (example): NPCs are NOT added to Participants.
func TestAddCombatant_NPC_NotAddedToParticipants(t *testing.T) {
	eng, cbt := combatWithKnownInitiatives(t, "room1")
	initialLen := len(cbt.Participants)

	npc2 := &combat.Combatant{ID: "n2", Kind: combat.KindNPC, Name: "Ganger2", MaxHP: 10, CurrentHP: 10, Initiative: 3}
	if err := eng.AddCombatant("room1", npc2); err != nil {
		t.Fatal(err)
	}
	if len(cbt.Participants) != initialLen {
		t.Errorf("Participants grew for NPC: want %d, got %d", initialLen, len(cbt.Participants))
	}
}

// REQ-T14 (example): Conditions initialized for new combatant after AddCombatant.
func TestAddCombatant_InitializesConditions(t *testing.T) {
	eng, cbt := combatWithKnownInitiatives(t, "room1")

	p2 := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 10, CurrentHP: 10, Initiative: 8}
	if err := eng.AddCombatant("room1", p2); err != nil {
		t.Fatal(err)
	}

	if cbt.Conditions["p2"] == nil {
		t.Error("Conditions[p2] not initialized after AddCombatant")
	}
}

// REQ-T22 (example): turnIndex adjusted when new combatant inserts before current actor.
func TestAddCombatant_AdjustsTurnIndex_WhenInsertingBeforeCurrentActor(t *testing.T) {
	eng, cbt := combatWithKnownInitiatives(t, "room1")

	// Advance so n1 is the current actor (turnIndex goes 0→1).
	cbt.AdvanceTurn()

	// Verify n1 is current before insertion.
	if cur := cbt.CurrentTurn(); cur == nil || cur.ID != "n1" {
		t.Fatalf("before AddCombatant: current actor should be n1, got %v", cur)
	}

	// Insert p2(10) at index 1 (between p1=15 and n1=5).
	p2 := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 10, CurrentHP: 10, Initiative: 10}
	if err := eng.AddCombatant("room1", p2); err != nil {
		t.Fatal(err)
	}

	// After insertion, current actor must still be n1 (now at index 2).
	if cur := cbt.CurrentTurn(); cur == nil || cur.ID != "n1" {
		id := "<nil>"
		if cur != nil {
			id = cur.ID
		}
		t.Errorf("after AddCombatant: current actor should be n1, got %q", id)
	}
}

// REQ-T5 (property): AddCombatant always produces a slice sorted by initiative descending.
func TestProperty_AddCombatant_MaintainsSortOrder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		eng := combat.NewEngine()
		p1 := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, Initiative: 15}
		n1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 10, CurrentHP: 10, Initiative: 5}
		if _, err := eng.StartCombat("room1", []*combat.Combatant{p1, n1}, makeTestRegistry(), nil, ""); err != nil {
			rt.Fatal(err)
		}

		joinerInit := rapid.IntRange(-5, 25).Draw(rt, "joinerInitiative")
		joiner := &combat.Combatant{
			ID:         "p2",
			Kind:       combat.KindPlayer,
			Name:       "Bob",
			MaxHP:      10,
			CurrentHP:  10,
			Initiative: joinerInit,
		}
		if err := eng.AddCombatant("room1", joiner); err != nil {
			rt.Fatal(err)
		}

		cbt, _ := eng.GetCombat("room1")
		for i := 1; i < len(cbt.Combatants); i++ {
			if cbt.Combatants[i].Initiative > cbt.Combatants[i-1].Initiative {
				rt.Fatalf("slice not sorted at index %d: %d > %d",
					i, cbt.Combatants[i].Initiative, cbt.Combatants[i-1].Initiative)
			}
		}
	})
}

// REQ-T14 (property): Participants grows monotonically; length equals player combatants added.
func TestProperty_AddCombatant_ParticipantsGrowsMonotonically(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		eng := combat.NewEngine()
		p1 := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, Initiative: 15}
		n1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 10, CurrentHP: 10, Initiative: 5}
		if _, err := eng.StartCombat("room1", []*combat.Combatant{p1, n1}, makeTestRegistry(), nil, ""); err != nil {
			rt.Fatal(err)
		}

		expectedParticipants := 1 // p1 from StartCombat
		n := rapid.IntRange(0, 5).Draw(rt, "numJoiners")
		for i := 0; i < n; i++ {
			init := rapid.IntRange(1, 20).Draw(rt, fmt.Sprintf("initiative_%d", i))
			joiner := &combat.Combatant{
				ID:         fmt.Sprintf("p%d", i+2),
				Kind:       combat.KindPlayer,
				MaxHP:      10,
				CurrentHP:  10,
				Initiative: init,
			}
			if err := eng.AddCombatant("room1", joiner); err != nil {
				rt.Fatal(err)
			}
			expectedParticipants++
		}

		cbt, _ := eng.GetCombat("room1")
		if len(cbt.Participants) != expectedParticipants {
			rt.Fatalf("Participants: want %d, got %d", expectedParticipants, len(cbt.Participants))
		}
	})
}
