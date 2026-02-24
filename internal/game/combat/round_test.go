package combat_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
)

// fixedSrc is a deterministic Source for testing.
// It returns f.val for every Intn call with no bounds clamping,
// enabling test scenarios that need values outside the normal dice range.
type fixedSrc struct{ val int }

func (f fixedSrc) Intn(_ int) int { return f.val }

func makeRoundCombat(t *testing.T) *combat.Combat {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 12, Level: 1, StrMod: 1, DexMod: 0},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	_ = cbt.StartRound(3)
	return cbt
}

func noopUpdater(id string, hp int) {}

// TestResolveRound_AllPass: both combatants pass; 2 events, all ActionPass, nil AttackResult.
func TestResolveRound_AllPass(t *testing.T) {
	cbt := makeRoundCombat(t)
	src := fixedSrc{val: 10}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	events := combat.ResolveRound(cbt, src, noopUpdater)

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	for i, ev := range events {
		if ev.ActionType != combat.ActionPass {
			t.Errorf("event[%d]: expected ActionPass, got %v", i, ev.ActionType)
		}
		if ev.AttackResult != nil {
			t.Errorf("event[%d]: expected nil AttackResult, got non-nil", i)
		}
	}
}

// TestResolveRound_AttackHits: player attacks with high roll; event has non-nil AttackResult.
func TestResolveRound_AttackHits(t *testing.T) {
	cbt := makeRoundCombat(t)
	// val=18 → d20=19 → atkTotal=19+2+2=23 vs AC 12 → CritSuccess
	src := fixedSrc{val: 18}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	events := combat.ResolveRound(cbt, src, noopUpdater)

	var attackEv *combat.RoundEvent
	for i := range events {
		if events[i].ActionType == combat.ActionAttack {
			attackEv = &events[i]
		}
	}
	if attackEv == nil {
		t.Fatal("no ActionAttack event found")
	}
	if attackEv.AttackResult == nil {
		t.Fatal("expected non-nil AttackResult for attack event")
	}
}

// TestResolveRound_AttackKills: target has 1 HP, attacked; target HP→0, targetUpdater called with hp=0.
func TestResolveRound_AttackKills(t *testing.T) {
	cbt := makeRoundCombat(t)
	// Set Ganger to 1 HP
	cbt.Combatants[1].CurrentHP = 1

	// val=18 → d20=19 → atkTotal=23 vs AC12 → CritSuccess; dmg=(val%6+1)=5 * 2=10 > 1 HP
	src := fixedSrc{val: 18}

	updaterCalled := false
	updaterHP := -1
	updater := func(id string, hp int) {
		if id == "n1" {
			updaterCalled = true
			updaterHP = hp
		}
	}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	combat.ResolveRound(cbt, src, updater)

	if cbt.Combatants[1].CurrentHP != 0 {
		t.Errorf("expected Ganger HP=0, got %d", cbt.Combatants[1].CurrentHP)
	}
	if !updaterCalled {
		t.Error("expected targetUpdater to be called for Ganger")
	}
	if updaterHP != 0 {
		t.Errorf("expected targetUpdater called with hp=0, got %d", updaterHP)
	}
}

// TestResolveRound_Strike_TwoAttacks: strike produces 2 events for actor, both ActionStrike.
func TestResolveRound_Strike_TwoAttacks(t *testing.T) {
	cbt := makeRoundCombat(t)
	src := fixedSrc{val: 10}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	events := combat.ResolveRound(cbt, src, noopUpdater)

	strikeCount := 0
	for _, ev := range events {
		if ev.ActorID == "p1" && ev.ActionType == combat.ActionStrike {
			strikeCount++
		}
	}
	if strikeCount != 2 {
		t.Errorf("expected 2 ActionStrike events for p1, got %d", strikeCount)
	}
}

// TestResolveRound_Strike_MAPPenalty: second strike's AttackTotal is exactly 5 less than first.
func TestResolveRound_Strike_MAPPenalty(t *testing.T) {
	cbt := makeRoundCombat(t)
	// Use fixed val so both attacks use same die value → difference must be exactly 5
	src := fixedSrc{val: 10}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	events := combat.ResolveRound(cbt, src, noopUpdater)

	var strikeEvents []combat.RoundEvent
	for _, ev := range events {
		if ev.ActorID == "p1" && ev.ActionType == combat.ActionStrike {
			strikeEvents = append(strikeEvents, ev)
		}
	}
	if len(strikeEvents) != 2 {
		t.Fatalf("expected 2 strike events, got %d", len(strikeEvents))
	}
	first := strikeEvents[0].AttackResult
	second := strikeEvents[1].AttackResult
	if first == nil || second == nil {
		t.Fatal("both strike events must have non-nil AttackResult")
	}
	diff := first.AttackTotal - second.AttackTotal
	if diff != 5 {
		t.Errorf("expected second AttackTotal to be 5 less than first; diff=%d (first=%d, second=%d)",
			diff, first.AttackTotal, second.AttackTotal)
	}
}

// TestResolveRound_DeadCombatantSkipped: dead combatant produces no events.
func TestResolveRound_DeadCombatantSkipped(t *testing.T) {
	cbt := makeRoundCombat(t)
	// Kill Ganger before resolving
	cbt.Combatants[1].CurrentHP = 0

	src := fixedSrc{val: 10}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	// n1 is dead; StartRound already excluded it from ActionQueues, so no queue action needed.

	events := combat.ResolveRound(cbt, src, noopUpdater)

	for _, ev := range events {
		if ev.ActorID == "n1" {
			t.Errorf("dead combatant n1 should produce no events, got event: %+v", ev)
		}
	}
}

// TestResolveRound_Strike_TargetDeadAtStart: target HP is 0 before round resolves;
// both strike events for the actor have nil AttackResult and "nothing" in Narrative.
func TestResolveRound_Strike_TargetDeadAtStart(t *testing.T) {
	cbt := makeRoundCombat(t)
	// Kill Ganger before the round resolves.
	cbt.Combatants[1].CurrentHP = 0

	src := fixedSrc{val: 10}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}

	events := combat.ResolveRound(cbt, src, noopUpdater)

	var strikeEvents []combat.RoundEvent
	for _, ev := range events {
		if ev.ActorID == "p1" && ev.ActionType == combat.ActionStrike {
			strikeEvents = append(strikeEvents, ev)
		}
	}
	if len(strikeEvents) != 2 {
		t.Fatalf("expected 2 ActionStrike events for p1 when target is dead at start, got %d", len(strikeEvents))
	}
	for i, ev := range strikeEvents {
		if ev.AttackResult != nil {
			t.Errorf("strike event[%d]: expected nil AttackResult when target dead at start, got non-nil", i)
		}
		if !containsSubstring(ev.Narrative, "nothing") {
			t.Errorf("strike event[%d]: expected \"nothing\" in Narrative, got %q", i, ev.Narrative)
		}
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}

// TestPropertyResolveRound_DamageNeverExceedsStartingHP: target HP never goes below 0.
func TestPropertyResolveRound_DamageNeverExceedsStartingHP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		diceVal := rapid.IntRange(0, 19).Draw(rt, "diceVal")
		src := fixedSrc{val: diceVal}

		cbt := makeRoundCombat(t)

		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})

		combat.ResolveRound(cbt, src, noopUpdater)

		for _, c := range cbt.Combatants {
			if c.CurrentHP < 0 {
				rt.Errorf("combatant %q HP went below 0: %d", c.ID, c.CurrentHP)
			}
		}
	})
}
