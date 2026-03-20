package combat_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/session"
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

	events := combat.ResolveRound(cbt, src, noopUpdater, nil)

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

	events := combat.ResolveRound(cbt, src, noopUpdater, nil)

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

	combat.ResolveRound(cbt, src, updater, nil)

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

	events := combat.ResolveRound(cbt, src, noopUpdater, nil)

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

	events := combat.ResolveRound(cbt, src, noopUpdater, nil)

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

	events := combat.ResolveRound(cbt, src, noopUpdater, nil)

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

	events := combat.ResolveRound(cbt, src, noopUpdater, nil)

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

// TestResolveRound_ConditionDamageBonusApplied_Attack: a condition with DamageBonus=5 on the actor
// increases damage dealt during ActionAttack beyond what the base roll alone would produce.
func TestResolveRound_ConditionDamageBonusApplied_Attack(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "powered_up", Name: "Powered Up", DurationType: "rounds", MaxStacks: 1, DamageBonus: 5})

	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 100, CurrentHP: 100, AC: 12, Level: 1, StrMod: 1, DexMod: 0},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	_ = cbt.StartRound(3)

	// Apply DamageBonus=5 condition to the player actor.
	if err := cbt.ApplyCondition("p1", "powered_up", 1, 2); err != nil {
		t.Fatalf("ApplyCondition: %v", err)
	}

	// val=19 → d20=20 → guaranteed CritSuccess hit; base dmg=(19%6+1)*2=2*2=4; with bonus: 4+5=9
	src := fixedSrc{val: 19}

	// Measure HP before without bonus (reference run using a fresh combat without the condition).
	regNoBonus := condition.NewRegistry()
	regNoBonus.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	regNoBonus.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	regNoBonus.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	regNoBonus.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	regNoBonus.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})

	engRef := combat.NewEngine()
	combatantsRef := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 100, CurrentHP: 100, AC: 12, Level: 1, StrMod: 1, DexMod: 0},
	}
	cbtRef, err := engRef.StartCombat("room1", combatantsRef, regNoBonus, nil, "")
	if err != nil {
		t.Fatalf("StartCombat ref: %v", err)
	}
	_ = cbtRef.StartRound(3)

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction (bonus): %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 (bonus): %v", err)
	}
	if err := cbtRef.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction (ref): %v", err)
	}
	if err := cbtRef.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 (ref): %v", err)
	}

	combat.ResolveRound(cbt, src, noopUpdater, nil)
	combat.ResolveRound(cbtRef, src, noopUpdater, nil)

	gangerWithBonus := cbt.Combatants[1].CurrentHP
	gangerNoBonus := cbtRef.Combatants[1].CurrentHP

	// The actor with powered_up (DamageBonus=5) must deal more damage.
	if gangerWithBonus >= gangerNoBonus {
		t.Errorf("expected condition DamageBonus to increase damage: hpWithBonus=%d hpNoBonus=%d (lower HP = more damage)", gangerWithBonus, gangerNoBonus)
	}
	damageWithBonus := 100 - gangerWithBonus
	damageNoBonus := 100 - gangerNoBonus
	extraDamage := damageWithBonus - damageNoBonus
	if extraDamage != 5 {
		t.Errorf("expected exactly 5 extra damage from DamageBonus=5, got %d (withBonus=%d noBonus=%d)", extraDamage, damageWithBonus, damageNoBonus)
	}
}

// TestResolveRound_ConditionDamageBonusApplied_Strike: a condition with DamageBonus=5 on the actor
// increases damage dealt during ActionStrike for both the first and second hit.
func TestResolveRound_ConditionDamageBonusApplied_Strike(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "powered_up", Name: "Powered Up", DurationType: "rounds", MaxStacks: 1, DamageBonus: 5})

	eng := combat.NewEngine()
	// High HP target so it survives both strikes for comparison.
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 200, CurrentHP: 200, AC: 1, Level: 1, StrMod: 1, DexMod: 0},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	_ = cbt.StartRound(3)

	if err := cbt.ApplyCondition("p1", "powered_up", 1, 2); err != nil {
		t.Fatalf("ApplyCondition: %v", err)
	}

	// val=19 → d20=20 → guaranteed CritSuccess for first strike; second strike also hits (AC=1).
	src := fixedSrc{val: 19}

	regRef := condition.NewRegistry()
	regRef.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	regRef.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	regRef.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	regRef.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	regRef.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})

	engRef := combat.NewEngine()
	combatantsRef := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 200, CurrentHP: 200, AC: 1, Level: 1, StrMod: 1, DexMod: 0},
	}
	cbtRef, err := engRef.StartCombat("room1", combatantsRef, regRef, nil, "")
	if err != nil {
		t.Fatalf("StartCombat ref: %v", err)
	}
	_ = cbtRef.StartRound(3)

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction (bonus): %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 (bonus): %v", err)
	}
	if err := cbtRef.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction (ref): %v", err)
	}
	if err := cbtRef.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 (ref): %v", err)
	}

	combat.ResolveRound(cbt, src, noopUpdater, nil)
	combat.ResolveRound(cbtRef, src, noopUpdater, nil)

	gangerWithBonus := cbt.Combatants[1].CurrentHP
	gangerNoBonus := cbtRef.Combatants[1].CurrentHP

	if gangerWithBonus >= gangerNoBonus {
		t.Errorf("expected condition DamageBonus to increase strike damage: hpWithBonus=%d hpNoBonus=%d", gangerWithBonus, gangerNoBonus)
	}
}

// TestProperty_ResolveRound_DamageBonusNeverNegatesHit: with any DamageBonus in [0,20],
// a guaranteed hit still results in target HP <= initialHP (damage is non-negative).
func TestProperty_ResolveRound_DamageBonusNeverNegatesHit(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		bonus := rapid.IntRange(0, 20).Draw(rt, "bonus")

		reg := condition.NewRegistry()
		reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
		reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
		reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
		reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
		reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})
		reg.Register(&condition.ConditionDef{ID: "buffed", Name: "Buffed", DurationType: "rounds", MaxStacks: 1, DamageBonus: bonus})

		eng := combat.NewEngine()
		combatants := []*combat.Combatant{
			{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
			{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 200, CurrentHP: 200, AC: 1, Level: 1, StrMod: 1, DexMod: 0},
		}
		cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
		if err != nil {
			rt.Fatalf("StartCombat: %v", err)
		}
		_ = cbt.StartRound(3)

		if err := cbt.ApplyCondition("p1", "buffed", 1, 2); err != nil {
			rt.Fatalf("ApplyCondition: %v", err)
		}

		// val=19 → guaranteed CritSuccess hit.
		src := fixedSrc{val: 19}

		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})

		initialHP := cbt.Combatants[1].CurrentHP
		combat.ResolveRound(cbt, src, noopUpdater, nil)
		finalHP := cbt.Combatants[1].CurrentHP

		if finalHP > initialHP {
			rt.Errorf("finalHP=%d > initialHP=%d: damage bonus should never heal target", finalHP, initialHP)
		}
	})
}

// makeSuckerPunchCombat returns a Combat configured for sucker_punch tests.
// The player (p1) has sessionGetter wired to return hasFeat in PassiveFeats["sucker_punch"].
// The NPC (n1) has flat_footed applied when isFlatFooted is true.
// src val=19 guarantees a CritSuccess hit; val=5 guarantees a d6 result of 6 for sucker_punch bonus.
func makeSuckerPunchCombat(t *testing.T, hasFeat, isFlatFooted bool) *combat.Combat {
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
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 200, CurrentHP: 200, AC: 1, Level: 1, StrMod: 1, DexMod: 0},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	ps := &session.PlayerSession{
		PassiveFeats: map[string]bool{"sucker_punch": hasFeat},
	}
	cbt.SetSessionGetter(func(uid string) (*session.PlayerSession, bool) {
		if uid == "p1" {
			return ps, true
		}
		return nil, false
	})

	// StartRound before applying flat_footed to avoid it being ticked away by Tick().
	_ = cbt.StartRound(3)

	if isFlatFooted {
		if def, ok := reg.Get("flat_footed"); ok {
			_ = cbt.Conditions["n1"].Apply("n1", def, 1, 1)
		}
	}

	return cbt
}

// TestResolveRound_SuckerPunch_FlatFooted_AddsDamage: player with sucker_punch feat attacking
// a flat_footed NPC deals more damage than without the feat, on a guaranteed hit.
func TestResolveRound_SuckerPunch_FlatFooted_AddsDamage(t *testing.T) {
	// val=19: d20=20 (CritSuccess guaranteed); d6 = (19%6)+1 = 2; sucker_punch d6 = same src = 2.
	// We use val=19 to guarantee a hit and get deterministic bonus.
	src := fixedSrc{val: 19}

	cbtWith := makeSuckerPunchCombat(t, true, true)
	cbtWithout := makeSuckerPunchCombat(t, false, true)

	if err := cbtWith.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction with: %v", err)
	}
	if err := cbtWith.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 with: %v", err)
	}
	if err := cbtWithout.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction without: %v", err)
	}
	if err := cbtWithout.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 without: %v", err)
	}

	combat.ResolveRound(cbtWith, src, noopUpdater, nil)
	combat.ResolveRound(cbtWithout, src, noopUpdater, nil)

	hpWith := cbtWith.Combatants[1].CurrentHP
	hpWithout := cbtWithout.Combatants[1].CurrentHP

	// Lower HP means more damage was dealt. With sucker_punch must deal more.
	if hpWith >= hpWithout {
		t.Errorf("expected sucker_punch to add damage: hpWith=%d hpWithout=%d (lower HP = more damage)", hpWith, hpWithout)
	}
}

// TestResolveRound_SuckerPunch_NotFlatFooted_NoBonus: player with sucker_punch feat attacking
// a non-flat_footed NPC deals the same damage as without the feat.
func TestResolveRound_SuckerPunch_NotFlatFooted_NoBonus(t *testing.T) {
	src := fixedSrc{val: 19}

	cbtWith := makeSuckerPunchCombat(t, true, false)
	cbtWithout := makeSuckerPunchCombat(t, false, false)

	if err := cbtWith.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction with: %v", err)
	}
	if err := cbtWith.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 with: %v", err)
	}
	if err := cbtWithout.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction without: %v", err)
	}
	if err := cbtWithout.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 without: %v", err)
	}

	combat.ResolveRound(cbtWith, src, noopUpdater, nil)
	combat.ResolveRound(cbtWithout, src, noopUpdater, nil)

	hpWith := cbtWith.Combatants[1].CurrentHP
	hpWithout := cbtWithout.Combatants[1].CurrentHP

	// No flat_footed — sucker_punch must not add bonus damage.
	if hpWith != hpWithout {
		t.Errorf("expected no sucker_punch bonus without flat_footed: hpWith=%d hpWithout=%d", hpWith, hpWithout)
	}
}

// TestProperty_SuckerPunch_DamageNonNegative: regardless of feat/flat_footed combination,
// the NPC's HP must never exceed its initial HP (no healing) and never go below 0.
func TestProperty_SuckerPunch_DamageNonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		hasFeat := rapid.Bool().Draw(rt, "hasFeat")
		isFlatFooted := rapid.Bool().Draw(rt, "isFlatFooted")
		diceVal := rapid.IntRange(0, 19).Draw(rt, "diceVal")
		src := fixedSrc{val: diceVal}

		cbt := makeSuckerPunchCombat(t, hasFeat, isFlatFooted)

		initialHP := cbt.Combatants[1].CurrentHP

		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})

		combat.ResolveRound(cbt, src, noopUpdater, nil)

		finalHP := cbt.Combatants[1].CurrentHP
		if finalHP > initialHP {
			rt.Errorf("finalHP=%d > initialHP=%d: sucker_punch must not heal the target", finalHP, initialHP)
		}
		if finalHP < 0 {
			rt.Errorf("finalHP=%d < 0: HP must not go below zero", finalHP)
		}
	})
}

// makePredatorsEyeCombat builds a combat for predators_eye tests.
// npcType is set on the NPC Combatant. hasFeat and favoredTarget are placed in the player session.
func makePredatorsEyeCombat(t *testing.T, hasFeat bool, favoredTarget, npcType string) *combat.Combat {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})

	eng := combat.NewEngine()
	// AC=1 ensures guaranteed hit; MaxHP=200 keeps target alive through both runs.
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 1},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 200, CurrentHP: 200, AC: 1, Level: 1, StrMod: 1, DexMod: 0, NPCType: npcType},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	ps := &session.PlayerSession{
		PassiveFeats:  map[string]bool{"predators_eye": hasFeat},
		FavoredTarget: favoredTarget,
	}
	cbt.SetSessionGetter(func(uid string) (*session.PlayerSession, bool) {
		if uid == "p1" {
			return ps, true
		}
		return nil, false
	})

	_ = cbt.StartRound(3)
	return cbt
}

// TestResolveRound_PredatorsEye_MatchingType_AddsDamage: player with predators_eye and FavoredTarget=="robot"
// attacking NPCType=="robot" must deal more damage than without the feat, on a guaranteed hit.
func TestResolveRound_PredatorsEye_MatchingType_AddsDamage(t *testing.T) {
	// val=19: d20=20 (CritSuccess guaranteed); d8 result = (19%8)+1 = 4 for the bonus.
	src := fixedSrc{val: 19}

	cbtWith := makePredatorsEyeCombat(t, true, "robot", "robot")
	cbtWithout := makePredatorsEyeCombat(t, false, "robot", "robot")

	if err := cbtWith.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction with: %v", err)
	}
	if err := cbtWith.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 with: %v", err)
	}
	if err := cbtWithout.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction without: %v", err)
	}
	if err := cbtWithout.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 without: %v", err)
	}

	combat.ResolveRound(cbtWith, src, noopUpdater, nil)
	combat.ResolveRound(cbtWithout, src, noopUpdater, nil)

	hpWith := cbtWith.Combatants[1].CurrentHP
	hpWithout := cbtWithout.Combatants[1].CurrentHP

	// Lower HP means more damage was dealt. With predators_eye vs matching type must deal more.
	if hpWith >= hpWithout {
		t.Errorf("expected predators_eye to add damage vs matching NPC type: hpWith=%d hpWithout=%d (lower HP = more damage)", hpWith, hpWithout)
	}
}

// TestResolveRound_PredatorsEye_NonMatchingType_NoBonus: player with predators_eye and FavoredTarget=="robot"
// attacking NPCType=="animal" must deal no bonus damage.
func TestResolveRound_PredatorsEye_NonMatchingType_NoBonus(t *testing.T) {
	src := fixedSrc{val: 19}

	// FavoredTarget="robot", but NPC is "animal" — no bonus.
	cbtFeat := makePredatorsEyeCombat(t, true, "robot", "animal")
	cbtNoFeat := makePredatorsEyeCombat(t, false, "robot", "animal")

	if err := cbtFeat.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction feat: %v", err)
	}
	if err := cbtFeat.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 feat: %v", err)
	}
	if err := cbtNoFeat.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction no feat: %v", err)
	}
	if err := cbtNoFeat.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 no feat: %v", err)
	}

	combat.ResolveRound(cbtFeat, src, noopUpdater, nil)
	combat.ResolveRound(cbtNoFeat, src, noopUpdater, nil)

	hpFeat := cbtFeat.Combatants[1].CurrentHP
	hpNoFeat := cbtNoFeat.Combatants[1].CurrentHP

	// Type mismatch — damage must be identical.
	if hpFeat != hpNoFeat {
		t.Errorf("expected no predators_eye bonus vs non-matching NPC type: hpFeat=%d hpNoFeat=%d", hpFeat, hpNoFeat)
	}
}

// TestResolveRound_PredatorsEye_EmptyFavoredTarget_NoBonus: player with predators_eye but FavoredTarget==""
// must not receive bonus, even when NPC has a type.
func TestResolveRound_PredatorsEye_EmptyFavoredTarget_NoBonus(t *testing.T) {
	src := fixedSrc{val: 19}

	// hasFeat=true, favoredTarget="" (unset), npcType="robot".
	cbtFeat := makePredatorsEyeCombat(t, true, "", "robot")
	cbtNoFeat := makePredatorsEyeCombat(t, false, "", "robot")

	if err := cbtFeat.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction feat: %v", err)
	}
	if err := cbtFeat.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 feat: %v", err)
	}
	if err := cbtNoFeat.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction no feat: %v", err)
	}
	if err := cbtNoFeat.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 no feat: %v", err)
	}

	combat.ResolveRound(cbtFeat, src, noopUpdater, nil)
	combat.ResolveRound(cbtNoFeat, src, noopUpdater, nil)

	hpFeat := cbtFeat.Combatants[1].CurrentHP
	hpNoFeat := cbtNoFeat.Combatants[1].CurrentHP

	// Empty FavoredTarget means feature not configured — no bonus.
	if hpFeat != hpNoFeat {
		t.Errorf("expected no predators_eye bonus with empty FavoredTarget: hpFeat=%d hpNoFeat=%d", hpFeat, hpNoFeat)
	}
}

// TestProperty_PredatorsEye_DamageNonNegative: regardless of feat/type combination,
// NPC HP must never exceed initial HP (no healing) and must never go below 0.
func TestProperty_PredatorsEye_DamageNonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		hasFeat := rapid.Bool().Draw(rt, "hasFeat")
		favored := rapid.SampledFrom([]string{"human", "robot", "animal", "mutant", ""}).Draw(rt, "favored")
		npcType := rapid.SampledFrom([]string{"human", "robot", "animal", "mutant"}).Draw(rt, "npcType")
		diceVal := rapid.IntRange(0, 19).Draw(rt, "diceVal")
		src := fixedSrc{val: diceVal}

		cbt := makePredatorsEyeCombat(t, hasFeat, favored, npcType)

		initialHP := cbt.Combatants[1].CurrentHP

		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})

		combat.ResolveRound(cbt, src, noopUpdater, nil)

		finalHP := cbt.Combatants[1].CurrentHP
		if finalHP > initialHP {
			rt.Errorf("finalHP=%d > initialHP=%d: predators_eye must not heal the target", finalHP, initialHP)
		}
		if finalHP < 0 {
			rt.Errorf("finalHP=%d < 0: HP must not go below zero", finalHP)
		}
	})
}

// TestResolveRound_InitiativeBonus: player with InitiativeBonus=2 and StrMod=2 attacking with
// a fixed dice source that always rolls 0 (d20 = 1) must have AttackTotal >= 5 (1 + 2 + 2).
func TestResolveRound_InitiativeBonus(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})

	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 2, DexMod: 0, InitiativeBonus: 2},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 18, CurrentHP: 18, AC: 1, Level: 1, StrMod: 0, DexMod: 0},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	_ = cbt.StartRound(3)

	// val=0 → d20 = 0+1 = 1 (see ResolveAttack: roll = src.Intn(20)+1)
	src := fixedSrc{val: 0}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	events := combat.ResolveRound(cbt, src, noopUpdater, nil)

	var attackEv *combat.RoundEvent
	for i := range events {
		if events[i].ActionType == combat.ActionAttack && events[i].ActorID == "p1" {
			attackEv = &events[i]
		}
	}
	if attackEv == nil {
		t.Fatal("no ActionAttack event found for p1")
	}
	if attackEv.AttackResult == nil {
		t.Fatal("expected non-nil AttackResult")
	}
	// roll=1 + StrMod=2 + InitiativeBonus=2 = 5
	if attackEv.AttackResult.AttackTotal < 5 {
		t.Errorf("expected AttackTotal >= 5 (roll=1 + StrMod=2 + InitiativeBonus=2), got %d", attackEv.AttackResult.AttackTotal)
	}
}

// TestPropertyResolveRound_DamageNeverExceedsStartingHP: target HP never goes below 0.
func TestPropertyResolveRound_DamageNeverExceedsStartingHP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		diceVal := rapid.IntRange(0, 19).Draw(rt, "diceVal")
		src := fixedSrc{val: diceVal}

		cbt := makeRoundCombat(t)

		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})

		combat.ResolveRound(cbt, src, noopUpdater, nil)

		for _, c := range cbt.Combatants {
			if c.CurrentHP < 0 {
				rt.Errorf("combatant %q HP went below 0: %d", c.ID, c.CurrentHP)
			}
		}
	})
}

// TestResolveRound_NilReactionFn_NoPanic: passing nil for reactionFn must not panic.
// REQ-RXN18: nil reactionFn must not panic.
func TestResolveRound_NilReactionFn_NoPanic(t *testing.T) {
	cbt := makeRoundCombat(t)
	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}
	src := fixedSrc{val: 0}
	// Must not panic when reactionFn is nil.
	_ = combat.ResolveRound(cbt, src, noopUpdater, nil)
}

// TestResolveRound_ReactionFn_CalledOnDamageTaken: reactionFn must be called with
// TriggerOnDamageTaken when an NPC attacks a player and deals damage.
func TestResolveRound_ReactionFn_CalledOnDamageTaken(t *testing.T) {
	cbt := makeRoundCombat(t)
	// NPC attacks player; val=19 guarantees CritSuccess hit.
	src := fixedSrc{val: 19}

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Alice"}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	called := false
	fn := reaction.ReactionCallback(func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
		if uid == "p1" && trigger == reaction.TriggerOnDamageTaken {
			called = true
		}
		return false, nil
	})

	_ = combat.ResolveRound(cbt, src, noopUpdater, fn)

	if !called {
		t.Error("expected reactionFn to be called with TriggerOnDamageTaken for player target")
	}
}
