package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"pgregory.net/rapid"
)

func makeTestRegistry() *condition.Registry {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4, RestrictActions: []string{"attack", "strike", "pass"}})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "frightened", Name: "Frightened", DurationType: "rounds", MaxStacks: 4, AttackPenalty: 1, ACPenalty: 1})
	return reg
}

// makeTwoCombatantCombat returns a Combat with two living combatants for unit testing.
func makeTwoCombatantCombat(t *testing.T) *combat.Combat {
	t.Helper()
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 12, Level: 1},
	}
	cbt, err := eng.StartCombat("room1", combatants, makeTestRegistry(), nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	return cbt
}

// TestCombat_StartRound_IncrementsRound verifies Round increments from 0→1→2.
func TestCombat_StartRound_IncrementsRound(t *testing.T) {
	c := makeTwoCombatantCombat(t)
	if c.Round != 0 {
		t.Fatalf("initial Round = %d, want 0", c.Round)
	}
	_ = c.StartRound(3)
	if c.Round != 1 {
		t.Errorf("after first StartRound: Round = %d, want 1", c.Round)
	}
	_ = c.StartRound(3)
	if c.Round != 2 {
		t.Errorf("after second StartRound: Round = %d, want 2", c.Round)
	}
}

// TestCombat_StartRound_ResetsQueues verifies all living combatants get a fresh queue with actionsPerRound AP.
func TestCombat_StartRound_ResetsQueues(t *testing.T) {
	c := makeTwoCombatantCombat(t)
	_ = c.StartRound(3)
	for _, cbt := range c.Combatants {
		q, ok := c.ActionQueues[cbt.ID]
		if !ok {
			t.Errorf("combatant %q has no queue after StartRound", cbt.ID)
			continue
		}
		if q.RemainingPoints() != 3 {
			t.Errorf("combatant %q: RemainingPoints = %d, want 3", cbt.ID, q.RemainingPoints())
		}
	}
}

// TestCombat_StartRound_SkipsDeadCombatants verifies dead combatants get no queue entry.
func TestCombat_StartRound_SkipsDeadCombatants(t *testing.T) {
	c := makeTwoCombatantCombat(t)
	// Kill the NPC.
	c.Combatants[1].CurrentHP = 0
	_ = c.StartRound(3)

	if _, ok := c.ActionQueues["n1"]; ok {
		t.Error("dead combatant n1 should have no queue entry after StartRound")
	}
	if _, ok := c.ActionQueues["p1"]; !ok {
		t.Error("living combatant p1 should have a queue entry after StartRound")
	}
}

// TestCombat_QueueAction_Success verifies a valid action is enqueued and AP decremented.
func TestCombat_QueueAction_Success(t *testing.T) {
	c := makeTwoCombatantCombat(t)
	_ = c.StartRound(3)

	err := c.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"})
	if err != nil {
		t.Fatalf("QueueAction returned unexpected error: %v", err)
	}
	q := c.ActionQueues["p1"]
	if q.RemainingPoints() != 2 {
		t.Errorf("RemainingPoints = %d, want 2 after enqueuing 1-AP attack", q.RemainingPoints())
	}
	actions := q.QueuedActions()
	if len(actions) != 1 {
		t.Errorf("QueuedActions length = %d, want 1", len(actions))
	}
}

// TestCombat_QueueAction_UnknownUID verifies an error is returned for an unknown UID.
func TestCombat_QueueAction_UnknownUID(t *testing.T) {
	c := makeTwoCombatantCombat(t)
	_ = c.StartRound(3)

	err := c.QueueAction("ghost", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"})
	if err == nil {
		t.Error("expected error for unknown UID, got nil")
	}
}

// TestCombat_AllActionsSubmitted_False verifies AllActionsSubmitted returns false right after StartRound.
func TestCombat_AllActionsSubmitted_False(t *testing.T) {
	c := makeTwoCombatantCombat(t)
	_ = c.StartRound(3)

	if c.AllActionsSubmitted() {
		t.Error("AllActionsSubmitted should be false immediately after StartRound(3)")
	}
}

// TestCombat_AllActionsSubmitted_True verifies AllActionsSubmitted returns true after all living combatants pass.
func TestCombat_AllActionsSubmitted_True(t *testing.T) {
	c := makeTwoCombatantCombat(t)
	_ = c.StartRound(3)

	pass := combat.QueuedAction{Type: combat.ActionPass}
	if err := c.QueueAction("p1", pass); err != nil {
		t.Fatalf("QueueAction p1 pass: %v", err)
	}
	if err := c.QueueAction("n1", pass); err != nil {
		t.Fatalf("QueueAction n1 pass: %v", err)
	}
	if !c.AllActionsSubmitted() {
		t.Error("AllActionsSubmitted should be true after all combatants passed")
	}
}

// TestPropertyCombat_RoundMonotonicallyIncreases verifies Round == N after N calls to StartRound.
func TestPropertyCombat_RoundMonotonicallyIncreases(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 20).Draw(rt, "rounds")
		cbt := makeTwoCombatantCombat(t)
		for i := 1; i <= n; i++ {
			_ = cbt.StartRound(3)
		}
		if cbt.Round != n {
			rt.Errorf("after %d StartRound calls: Round = %d, want %d", n, cbt.Round, n)
		}
	})
}
