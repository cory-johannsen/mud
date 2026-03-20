package combat_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// makeSneakCombat builds a Combat for sneak attack extension tests.
// The player (p1) has sessionGetter wired with hasFeat in PassiveFeats["sucker_punch"].
// The NPC (n1) has grabbed condition applied when isGrabbed is true.
// actor.Hidden is set to actorHidden.
// grabbed condition must be registered in the registry.
func makeSneakCombat(t *testing.T, hasFeat, isGrabbed, actorHidden bool) (*combat.Combat, *combat.Combatant) {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "grabbed", Name: "Grabbed", DurationType: "rounds", MaxStacks: 0})
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

	_ = cbt.StartRound(3)

	// Apply grabbed to target after StartRound to avoid tick removal.
	if isGrabbed {
		if def, ok := reg.Get("grabbed"); ok {
			_ = cbt.Conditions["n1"].Apply("n1", def, 1, 2)
		}
	}

	// Set actor Hidden after StartRound.
	cbt.Combatants[0].Hidden = actorHidden

	return cbt, cbt.Combatants[0]
}

// TestApplyPassiveFeats_SuckerPunch_OnGrabbed: player actor with sucker_punch, NPC target with grabbed condition → damage bonus applied.
func TestApplyPassiveFeats_SuckerPunch_OnGrabbed(t *testing.T) {
	// val=19: d20=20 (CritSuccess guaranteed); sucker_punch d6 = (19%6)+1 = 2 bonus.
	src := fixedSrc{val: 19}

	cbtWith := func() *combat.Combat {
		c, _ := makeSneakCombat(t, true, true, false)
		return c
	}()
	cbtWithout := func() *combat.Combat {
		c, _ := makeSneakCombat(t, false, true, false)
		return c
	}()

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

	// Lower HP means more damage was dealt. sucker_punch vs grabbed must deal more.
	if hpWith >= hpWithout {
		t.Errorf("expected sucker_punch to add damage vs grabbed target: hpWith=%d hpWithout=%d", hpWith, hpWithout)
	}
}

// TestApplyPassiveFeats_SuckerPunch_OnHidden: player actor with sucker_punch, actor.Hidden=true → bonus applied, actor.Hidden=false after.
func TestApplyPassiveFeats_SuckerPunch_OnHidden(t *testing.T) {
	src := fixedSrc{val: 19}

	cbtWith, actorWith := makeSneakCombat(t, true, false, true)
	cbtWithout, _ := makeSneakCombat(t, false, false, true)

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

	if hpWith >= hpWithout {
		t.Errorf("expected sucker_punch to add damage when actor hidden: hpWith=%d hpWithout=%d", hpWith, hpWithout)
	}

	// Attacking from hidden must clear Hidden regardless of hit/miss.
	if actorWith.Hidden {
		t.Errorf("expected actor.Hidden to be false after attacking from hidden")
	}
}

// TestApplyPassiveFeats_SuckerPunch_StillTriggersOnFlatFooted: existing flat_footed behavior preserved.
func TestApplyPassiveFeats_SuckerPunch_StillTriggersOnFlatFooted(t *testing.T) {
	// Reuse makeSuckerPunchCombat from round_test.go — same package so accessible.
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

	if hpWith >= hpWithout {
		t.Errorf("expected sucker_punch to still trigger on flat_footed: hpWith=%d hpWithout=%d", hpWith, hpWithout)
	}
}

// TestApplyPassiveFeats_SuckerPunch_NotTriggeredIfNoCondition: no flat_footed, not grabbed, not hidden → no bonus.
func TestApplyPassiveFeats_SuckerPunch_NotTriggeredIfNoCondition(t *testing.T) {
	src := fixedSrc{val: 19}

	cbtWith, _ := makeSneakCombat(t, true, false, false)
	cbtWithout, _ := makeSneakCombat(t, false, false, false)

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

	// No triggering condition — damage must be identical.
	if hpWith != hpWithout {
		t.Errorf("expected no sucker_punch bonus without any triggering condition: hpWith=%d hpWithout=%d", hpWith, hpWithout)
	}
}

// TestApplyPassiveFeats_SuckerPunch_HiddenClearedEvenOnMiss: actor.Hidden=true, dmg=0 (guaranteed miss)
// → bonus==0 but actor.Hidden is false after (concealment lost even on miss).
func TestApplyPassiveFeats_SuckerPunch_HiddenClearedEvenOnMiss(t *testing.T) {
	// val=0 → d20=1, atkTotal=1+2=3 vs AC=14 → guaranteed miss (Failure or CritFailure) → dmg=0
	src := fixedSrc{val: 0}

	// Use AC=14 for the target to guarantee a miss with val=0.
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent", MaxStacks: 0, AttackPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	reg.Register(&condition.ConditionDef{ID: "grabbed", Name: "Grabbed", DurationType: "rounds", MaxStacks: 0})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "stunned", Name: "Stunned", DurationType: "rounds", MaxStacks: 3})

	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, StrMod: 0, DexMod: 0},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 200, CurrentHP: 200, AC: 30, Level: 1, StrMod: 0, DexMod: 0},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	ps := &session.PlayerSession{
		PassiveFeats: map[string]bool{"sucker_punch": true},
	}
	cbt.SetSessionGetter(func(uid string) (*session.PlayerSession, bool) {
		if uid == "p1" {
			return ps, true
		}
		return nil, false
	})

	_ = cbt.StartRound(3)
	actor := cbt.Combatants[0]
	actor.Hidden = true

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	initialHP := cbt.Combatants[1].CurrentHP
	combat.ResolveRound(cbt, src, noopUpdater, nil)

	// No damage on miss — HP unchanged.
	if cbt.Combatants[1].CurrentHP != initialHP {
		t.Errorf("expected no damage on miss: initialHP=%d finalHP=%d", initialHP, cbt.Combatants[1].CurrentHP)
	}

	// Hidden must be cleared even on a miss.
	if actor.Hidden {
		t.Errorf("expected actor.Hidden=false after attacking (even on miss), got true")
	}
}

// TestResolveRound_SuckerPunch_HiddenStrikeFirstOnlyTriggersSneak: ActionStrike with actor.Hidden=true
// must trigger sucker_punch on the first strike (Hidden=true at entry) and NOT on the second strike
// (Hidden is cleared to false after the first applyPassiveFeats call).
//
// Verification strategy: with fixedSrc{val:19}, sucker_punch d6 bonus = (19%6)+1 = 2.
// A hidden=true player must deal exactly 2 more damage total than an identical non-hidden player
// (only one sneak bonus across the two-strike sequence).
func TestResolveRound_SuckerPunch_HiddenStrikeFirstOnlyTriggersSneak(t *testing.T) {
	// val=19 → d20=20 (CritSuccess, guaranteed hit); sucker_punch d6 = (19%6)+1 = 2.
	src := fixedSrc{val: 19}

	// Combat with Hidden=true, sucker_punch active.
	cbtHidden, actorHidden := makeSneakCombat(t, true, false, true)
	// Combat with Hidden=false, sucker_punch active (baseline — no sneak trigger).
	cbtVisible, _ := makeSneakCombat(t, true, false, false)

	if err := cbtHidden.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction hidden p1: %v", err)
	}
	if err := cbtHidden.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction hidden n1: %v", err)
	}
	if err := cbtVisible.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction visible p1: %v", err)
	}
	if err := cbtVisible.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction visible n1: %v", err)
	}

	initialHPHidden := cbtHidden.Combatants[1].CurrentHP
	initialHPVisible := cbtVisible.Combatants[1].CurrentHP

	combat.ResolveRound(cbtHidden, src, noopUpdater, nil)
	combat.ResolveRound(cbtVisible, src, noopUpdater, nil)

	finalHPHidden := cbtHidden.Combatants[1].CurrentHP
	finalHPVisible := cbtVisible.Combatants[1].CurrentHP

	dmgHidden := initialHPHidden - finalHPHidden
	dmgVisible := initialHPVisible - finalHPVisible

	// Hidden player must deal strictly more damage (first strike got the sneak bonus).
	if dmgHidden <= dmgVisible {
		t.Errorf("expected hidden ActionStrike to deal more damage than visible: dmgHidden=%d dmgVisible=%d", dmgHidden, dmgVisible)
	}

	// The extra damage must equal exactly one sucker_punch roll (19+1=20 with fixedSrc val=19), not two.
	// fixedSrc.Intn(n) always returns val unchanged, so src.Intn(6)+1 = 19+1 = 20.
	// Two rolls would indicate both strikes triggered sneak from Hidden; only the first must.
	// (The second strike may still trigger sneak via flat_footed applied by the first CritSuccess,
	// which is identical across both combats and does not contribute to the difference.)
	expectedBonus := 19 + 1 // fixedSrc val=19; Intn(6) returns 19; +1 → 20
	actualBonus := dmgHidden - dmgVisible
	if actualBonus != expectedBonus {
		t.Errorf("expected exactly one sucker_punch bonus (%d) from Hidden on first strike, got bonus=%d (dmgHidden=%d dmgVisible=%d)",
			expectedBonus, actualBonus, dmgHidden, dmgVisible)
	}

	// actor.Hidden must be false after the round (cleared on the first strike).
	if actorHidden.Hidden {
		t.Errorf("expected actor.Hidden=false after ActionStrike, got true")
	}
}

// TestProperty_SuckerPunch_Extended_DamageNonNegative: regardless of grabbed/hidden/feat combination,
// NPC HP must never exceed initial HP (no healing) and must never go below 0.
func TestProperty_SuckerPunch_Extended_DamageNonNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		hasFeat := rapid.Bool().Draw(rt, "hasFeat")
		isGrabbed := rapid.Bool().Draw(rt, "isGrabbed")
		actorHidden := rapid.Bool().Draw(rt, "actorHidden")
		diceVal := rapid.IntRange(0, 19).Draw(rt, "diceVal")
		src := fixedSrc{val: diceVal}

		cbt, _ := makeSneakCombat(t, hasFeat, isGrabbed, actorHidden)
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
