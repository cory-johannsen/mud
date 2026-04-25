package combat_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
)

// coverFixedSrc returns n-1 for every Intn call so that Intn(20)+1 == n.
// This lets tests set an exact d20 attack roll value.
type coverFixedSrc struct{ roll int }

func (s coverFixedSrc) Intn(_ int) int {
	v := s.roll - 1
	if v < 0 {
		v = 0
	}
	return v
}

// buildCoverCombat builds a minimal *Combat where:
//   - Player "p1" has AC=10 (bare minimum; attack bonuses come from level+str)
//   - NPC "n1" has AC=baseAC with a standard_cover condition providing ACPenalty=coverPenalty
//   - n1 has CoverTier="standard_cover" and CoverEquipmentID="equip1"
//
// The attacker p1 queues an ActionAttack against n1.
// The src will always produce attackRoll for d20 calls.
func buildCoverCombat(t *rapid.T, baseAC, coverPenalty, attackRoll int) *combat.Combat {
	t.Helper()
	reg := condition.NewRegistry()
	// Register a synthetic cover condition that penalises the attacker's roll.
	// ACPenalty on the defender's condition set is subtracted from the attacker's roll
	// via condition.ACBonus (returns -ACPenalty).
	reg.Register(&condition.ConditionDef{
		ID:           "test_cover",
		Name:         "Test Cover",
		DurationType: "permanent",
		MaxStacks:    0,
		ACPenalty:    coverPenalty,
	})
	// Also register standard conditions to keep the engine happy.
	reg.Register(&condition.ConditionDef{ID: "prone", Name: "Prone", DurationType: "permanent"})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds"})
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})

	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{
			ID: "p1", Kind: combat.KindPlayer, Name: "Attacker",
			MaxHP: 30, CurrentHP: 30, AC: 10, Level: 0, StrMod: 0, DexMod: 0,
		},
		{
			ID: "n1", Kind: combat.KindNPC, Name: "Defender",
			MaxHP: 30, CurrentHP: 30, AC: baseAC, Level: 0, StrMod: 0, DexMod: 0,
			CoverTier:        "test_cover",
			CoverEquipmentID: "equip1",
		},
	}
	cbt, err := eng.StartCombat("testroom", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	// Apply the cover condition to the defender so ACBonus returns -coverPenalty.
	if err := cbt.ApplyCondition("n1", "test_cover", 1, -1); err != nil {
		t.Fatalf("ApplyCondition: %v", err)
	}
	_ = cbt.StartRound(3)
	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Defender"}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}
	return cbt
}

// TestCoverDegradationOnCoverAbsorbedHit verifies that when an attack roll is at least
// baseAC (would hit without cover) but misses after the cover penalty is applied,
// the coverDegrader callback fires.
func TestCoverDegradationOnCoverAbsorbedHit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		baseAC := rapid.IntRange(5, 15).Draw(t, "baseAC")
		coverPenalty := rapid.SampledFrom([]int{1, 2, 4}).Draw(t, "coverPenalty")
		// The "cover window": roll that hits baseAC but misses baseAC+coverPenalty.
		// With cover: effectiveRoll = attackRoll + acBonus = attackRoll - coverPenalty.
		// So the attack misses when attackRoll - coverPenalty < baseAC
		// i.e., attackRoll < baseAC + coverPenalty.
		// And the attack would hit without cover when attackRoll >= baseAC.
		// Window: baseAC <= attackRoll <= baseAC+coverPenalty-1.
		lo := baseAC
		hi := baseAC + coverPenalty - 1
		if hi < lo {
			return // degenerate
		}
		attackRoll := rapid.IntRange(lo, hi).Draw(t, "attackRoll")

		cbt := buildCoverCombat(t, baseAC, coverPenalty, attackRoll)
		degraded := false
		combat.ResolveRound(cbt, coverFixedSrc{roll: attackRoll}, func(id string, hp int) {}, nil, 0,
			func(roomID, equipID string) bool { degraded = true; return false })
		if !degraded {
			t.Errorf("expected coverDegrader to fire (roll=%d baseAC=%d coverPenalty=%d)",
				attackRoll, baseAC, coverPenalty)
		}
	})
}

// TestNoCoverDegradationOnCleanMiss verifies that when an attack roll is below baseAC
// (misses even without cover), the coverDegrader callback does NOT fire.
func TestNoCoverDegradationOnCleanMiss(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		baseAC := rapid.IntRange(5, 20).Draw(t, "baseAC")
		coverPenalty := rapid.SampledFrom([]int{1, 2, 4}).Draw(t, "coverPenalty")
		if baseAC <= 1 {
			return // no room below baseAC
		}
		attackRoll := rapid.IntRange(1, baseAC-1).Draw(t, "attackRoll")

		cbt := buildCoverCombat(t, baseAC, coverPenalty, attackRoll)
		degraded := false
		combat.ResolveRound(cbt, coverFixedSrc{roll: attackRoll}, func(id string, hp int) {}, nil, 0,
			func(roomID, equipID string) bool { degraded = true; return false })
		if degraded {
			t.Errorf("unexpected coverDegrader call (roll=%d baseAC=%d coverPenalty=%d)",
				attackRoll, baseAC, coverPenalty)
		}
	})
}
