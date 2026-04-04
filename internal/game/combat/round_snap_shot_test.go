package combat_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// snapShotSrc cycles through a fixed sequence of values, one per Intn call.
// Per ActionStrike the sequence is: [d20_s1, dmg_s1, d20_s2, dmg_s2].
type snapShotSrc struct {
	vals []int
	idx  int
}

func (s *snapShotSrc) Intn(_ int) int {
	v := s.vals[s.idx%len(s.vals)]
	s.idx++
	return v
}

// makeSnapShotCombat builds a Combat for snap_shot passive feat tests.
//
// p1 stats: StrMod=2, no proficiency rank → attack modifier = 2; AC=14.
// n1 stats: AC=12, MaxHP=200.
//
// Key arithmetic for the deterministic test (src vals [0, 0, 10, 0]):
//
//	strike1: Intn(20)=0 → d20=1; total=1+2=3 < AC=12 → miss.
//	strike2: Intn(20)=10 → d20=11; total=11+2=13.
//	         with MAP: 13-5=8 < 12 → miss.
//	         without MAP (snap_shot active): 13 >= 12 → hit.
func makeSnapShotCombat(t *testing.T, hasSnapShot bool) *combat.Combat {
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
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 200, CurrentHP: 200, AC: 12, Level: 1, StrMod: 1, DexMod: 0},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	ps := &session.PlayerSession{
		PassiveFeats: map[string]bool{"snap_shot": hasSnapShot},
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

// TestSnapShot_WaivesMAPOnFirstMiss: with snap_shot, a missed first strike does not
// incur the -5 MAP on the second strike; without snap_shot both strikes miss.
func TestSnapShot_WaivesMAPOnFirstMiss(t *testing.T) {
	src := &snapShotSrc{vals: []int{0, 0, 10, 0}}

	cbtWith := makeSnapShotCombat(t, true)
	cbtWithout := makeSnapShotCombat(t, false)

	if err := cbtWith.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction p1 with: %v", err)
	}
	if err := cbtWith.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 with: %v", err)
	}
	if err := cbtWithout.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"}); err != nil {
		t.Fatalf("QueueAction p1 without: %v", err)
	}
	if err := cbtWithout.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1 without: %v", err)
	}

	combat.ResolveRound(cbtWith, src, noopUpdater, nil)
	src.idx = 0 // reset for identical roll sequence
	combat.ResolveRound(cbtWithout, src, noopUpdater, nil)

	hpWith := cbtWith.Combatants[1].CurrentHP
	hpWithout := cbtWithout.Combatants[1].CurrentHP

	if hpWith >= hpWithout {
		t.Errorf("snap_shot must allow second strike to hit after first miss: hpWith=%d hpWithout=%d", hpWith, hpWithout)
	}
	if hpWithout != 200 {
		t.Errorf("both strikes should miss without snap_shot: hpWithout=%d", hpWithout)
	}
}

// TestSnapShot_NoEffectWhenFirstHits: if the first strike hits, snap_shot must NOT
// waive the MAP — both actors must have identical outcomes.
func TestSnapShot_NoEffectWhenFirstHits(t *testing.T) {
	// val=19 → d20=20, total=22 → CritSuccess on both strikes regardless; MAP irrelevant
	// because both strikes always hit.  HP outcomes must be equal.
	src := fixedSrc{val: 19}

	cbtWith := makeSnapShotCombat(t, true)
	cbtWithout := makeSnapShotCombat(t, false)

	_ = cbtWith.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"})
	_ = cbtWith.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
	_ = cbtWithout.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"})
	_ = cbtWithout.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})

	combat.ResolveRound(cbtWith, src, noopUpdater, nil)
	combat.ResolveRound(cbtWithout, src, noopUpdater, nil)

	hpWith := cbtWith.Combatants[1].CurrentHP
	hpWithout := cbtWithout.Combatants[1].CurrentHP

	if hpWith != hpWithout {
		t.Errorf("snap_shot must not affect outcome when first strike hits: hpWith=%d hpWithout=%d", hpWith, hpWithout)
	}
}

// TestProperty_SnapShot_NeverWorseThanNoFeat: for all first-strike miss scenarios,
// snap_shot must never result in more target HP remaining (i.e. never less damage).
func TestProperty_SnapShot_NeverWorseThanNoFeat(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Force first strike to miss: Intn(20) in [0,5] → d20 1-6, total 3-8 < AC=12.
		d20s1 := rapid.IntRange(0, 5).Draw(rt, "d20_s1")
		d20s2 := rapid.IntRange(0, 19).Draw(rt, "d20_s2")
		dmg := rapid.IntRange(0, 5).Draw(rt, "dmg")

		src := &snapShotSrc{vals: []int{d20s1, dmg, d20s2, dmg}}

		cbtWith := makeSnapShotCombat(t, true)
		cbtWithout := makeSnapShotCombat(t, false)

		_ = cbtWith.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"})
		_ = cbtWith.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})
		_ = cbtWithout.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Ganger"})
		_ = cbtWithout.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass})

		combat.ResolveRound(cbtWith, src, noopUpdater, nil)
		src.idx = 0
		combat.ResolveRound(cbtWithout, src, noopUpdater, nil)

		hpWith := cbtWith.Combatants[1].CurrentHP
		hpWithout := cbtWithout.Combatants[1].CurrentHP

		// snap_shot can only help (or be neutral); it must never deal less damage.
		if hpWith > hpWithout {
			rt.Errorf("snap_shot increased target HP vs no feat: hpWith=%d hpWithout=%d (d20s1=%d d20s2=%d dmg=%d)",
				hpWith, hpWithout, d20s1, d20s2, dmg)
		}
	})
}
