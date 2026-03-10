package combat_test

import (
	"fmt"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
)

// makeHiddenCombat builds a Combat for hidden flat-check tests.
// The NPC (n1) attacks the player (p1) who has Hidden set to targetHidden.
// NPC has high StrMod to ensure attacks that pass flat check will hit.
func makeHiddenCombat(t *testing.T, targetHidden bool) (*combat.Combat, *combat.Combatant) {
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
		// Player is listed first so NPC acts second (after player passes).
		// AC=1 ensures NPC hits when flat check passes.
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 100, CurrentHP: 100, AC: 1, Level: 1, StrMod: 0, DexMod: 0},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 200, CurrentHP: 200, AC: 14, Level: 1, StrMod: 5, DexMod: 0},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}

	_ = cbt.StartRound(3)

	player := cbt.Combatants[0]
	player.Hidden = targetHidden

	return cbt, player
}

// TestResolveRound_HiddenFlatCheckFail_MissesAttack verifies that when the flat check roll is ≤ 10,
// the NPC attack event contains a flat-check-fail narrative and deals no damage to the player.
//
// fixedSrc{val:4} means:
//   - Flat check: Intn(20)+1 = 4+1 = 5 → ≤ 10 → flat check FAILS → NPC misses.
func TestResolveRound_HiddenFlatCheckFail_MissesAttack(t *testing.T) {
	// val=4: flat check roll = Intn(20)+1 = 5 ≤ 10 → miss.
	src := fixedSrc{val: 4}

	cbt, player := makeHiddenCombat(t, true)
	initialHP := player.CurrentHP

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Alice"}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	events := combat.ResolveRound(cbt, src, noopUpdater)

	// Find the NPC attack event.
	var npcEvent *combat.RoundEvent
	for i := range events {
		if events[i].ActorID == "n1" && events[i].ActionType == combat.ActionAttack {
			npcEvent = &events[i]
			break
		}
	}
	if npcEvent == nil {
		t.Fatalf("expected NPC attack event, got none; events=%v", events)
	}

	// Narrative must mention flat check failure.
	if !strings.Contains(npcEvent.Narrative, "flat check") {
		t.Errorf("expected flat check narrative, got: %q", npcEvent.Narrative)
	}

	// Player must not have taken damage.
	if player.CurrentHP != initialHP {
		t.Errorf("expected no damage on flat check fail: initialHP=%d finalHP=%d", initialHP, player.CurrentHP)
	}

	// AttackResult must be nil (no attack was resolved).
	if npcEvent.AttackResult != nil {
		t.Errorf("expected nil AttackResult on flat check fail, got non-nil")
	}
}

// TestResolveRound_HiddenFlatCheckPass_HitsNormally verifies that when the flat check roll is > 10,
// the NPC attack proceeds normally and can deal damage.
//
// fixedSrc{val:14} means:
//   - Flat check: Intn(20)+1 = 14+1 = 15 → > 10 → flat check passes.
//   - Attack d20: Intn(20)+1 = 15, StrMod=5 → total=20 vs AC=1 → CritSuccess → damage.
//   - Damage: Intn(N)+1 = 15 → damage dealt.
func TestResolveRound_HiddenFlatCheckPass_HitsNormally(t *testing.T) {
	// val=14: flat check = 15 > 10 (passes); then attack proceeds normally.
	src := fixedSrc{val: 14}

	cbt, player := makeHiddenCombat(t, true)
	initialHP := player.CurrentHP

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Alice"}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	events := combat.ResolveRound(cbt, src, noopUpdater)

	// Find the NPC attack event.
	var npcEvent *combat.RoundEvent
	for i := range events {
		if events[i].ActorID == "n1" && events[i].ActionType == combat.ActionAttack {
			npcEvent = &events[i]
			break
		}
	}
	if npcEvent == nil {
		t.Fatalf("expected NPC attack event, got none; events=%v", events)
	}

	// Narrative must NOT mention flat check failure.
	if strings.Contains(npcEvent.Narrative, "flat check") {
		t.Errorf("expected no flat check failure narrative when check passes, got: %q", npcEvent.Narrative)
	}

	// AttackResult must be non-nil (normal attack was resolved).
	if npcEvent.AttackResult == nil {
		t.Errorf("expected non-nil AttackResult when flat check passes")
	}

	// Damage must have been dealt (high StrMod guarantees hit vs AC=1).
	if player.CurrentHP >= initialHP {
		t.Errorf("expected damage when flat check passes: initialHP=%d finalHP=%d", initialHP, player.CurrentHP)
	}
}

// TestResolveRound_HiddenClearedAfterNPCAttack verifies that target.Hidden is always false
// after an NPC targets a hidden player, regardless of flat check outcome.
func TestResolveRound_HiddenClearedAfterNPCAttack(t *testing.T) {
	for _, flatRollVal := range []int{0, 4, 9, 10, 11, 14, 19} {
		val := flatRollVal
		t.Run("flatCheckRollVal="+fmt.Sprintf("%d", val), func(t *testing.T) {
			src := fixedSrc{val: val}

			cbt, player := makeHiddenCombat(t, true)
			if !player.Hidden {
				t.Fatal("precondition: player.Hidden must be true before round")
			}

			if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
				t.Fatalf("QueueAction p1: %v", err)
			}
			if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Alice"}); err != nil {
				t.Fatalf("QueueAction n1: %v", err)
			}

			combat.ResolveRound(cbt, src, noopUpdater)

			if player.Hidden {
				t.Errorf("expected player.Hidden=false after NPC targeted player (flatRollVal=%d)", val)
			}
		})
	}
}

// TestResolveRound_HiddenFlatCheck_StrikeSkipsBothOnFail verifies that when the NPC uses
// ActionStrike against a hidden player and the flat check fails, BOTH strikes are skipped
// and only one flat-check-fail event is emitted.
//
// fixedSrc{val:4}: flat check = 5 ≤ 10 → fails.
func TestResolveRound_HiddenFlatCheck_StrikeSkipsBothOnFail(t *testing.T) {
	src := fixedSrc{val: 4}

	cbt, player := makeHiddenCombat(t, true)
	initialHP := player.CurrentHP

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Alice"}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	events := combat.ResolveRound(cbt, src, noopUpdater)

	// Count NPC Strike events.
	var npcStrikeEvents []combat.RoundEvent
	for _, ev := range events {
		if ev.ActorID == "n1" && ev.ActionType == combat.ActionStrike {
			npcStrikeEvents = append(npcStrikeEvents, ev)
		}
	}

	// Exactly one event: the flat-check-fail event.
	if len(npcStrikeEvents) != 1 {
		t.Errorf("expected 1 NPC strike event (flat-check-fail), got %d: %v", len(npcStrikeEvents), npcStrikeEvents)
	}
	if len(npcStrikeEvents) > 0 && !strings.Contains(npcStrikeEvents[0].Narrative, "flat check") {
		t.Errorf("expected flat check narrative in strike event, got: %q", npcStrikeEvents[0].Narrative)
	}

	// No damage dealt.
	if player.CurrentHP != initialHP {
		t.Errorf("expected no damage on flat check fail for Strike: initialHP=%d finalHP=%d", initialHP, player.CurrentHP)
	}
}

// TestPropertyResolveRound_HiddenFlatCheck_NeverDamageOnFail verifies that for any flat roll ≤ 10,
// the player takes no damage from an NPC ActionAttack.
func TestPropertyResolveRound_HiddenFlatCheck_NeverDamageOnFail(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Force flat check to fail: Intn(20)+1 ≤ 10 means Intn(20) ≤ 9, i.e. val ∈ [0,9].
		val := rapid.IntRange(0, 9).Draw(rt, "flatRollVal")
		src := fixedSrc{val: val}

		cbt, player := makeHiddenCombat(t, true)
		initialHP := player.CurrentHP

		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Alice"})

		combat.ResolveRound(cbt, src, noopUpdater)

		if player.CurrentHP != initialHP {
			rt.Errorf("flatRollVal=%d (flat check=%d ≤ 10): expected no damage, got initialHP=%d finalHP=%d",
				val, val+1, initialHP, player.CurrentHP)
		}
		if player.Hidden {
			rt.Errorf("flatRollVal=%d: expected player.Hidden=false after NPC attack", val)
		}
	})
}

// TestPropertyResolveRound_HiddenFlatCheck_StrikeNeverDamageOnFail verifies that for any flat
// roll ≤ 10, the player takes no damage from an NPC ActionStrike and Hidden is cleared.
func TestPropertyResolveRound_HiddenFlatCheck_StrikeNeverDamageOnFail(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Force flat check to fail: Intn(20)+1 ≤ 10 means val ∈ [0,9].
		val := rapid.IntRange(0, 9).Draw(rt, "flatRollVal")
		src := fixedSrc{val: val}

		cbt, player := makeHiddenCombat(t, true)
		initialHP := player.CurrentHP

		_ = cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass})
		_ = cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Alice"})

		combat.ResolveRound(cbt, src, noopUpdater)

		if player.CurrentHP != initialHP {
			rt.Errorf("flatRollVal=%d (flat check=%d ≤ 10): expected no damage from Strike, got initialHP=%d finalHP=%d",
				val, val+1, initialHP, player.CurrentHP)
		}
		if player.Hidden {
			rt.Errorf("flatRollVal=%d: expected player.Hidden=false after NPC Strike", val)
		}
	})
}
