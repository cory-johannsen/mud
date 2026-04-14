package combat_test

// REQ-67-1: Attack-roll narrative MUST show the raw d20 result as "1d20 (N)".
// REQ-67-2: Attack-roll narrative MUST show the target's effective AC as "vs AC N".
// REQ-67-3: Breakdown MUST appear for attack (ActionAttack) and strike (ActionStrike).

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
)

// newNarrativeTestCombat creates a minimal two-combatant combat ready for a
// single round: player (AC 14) vs NPC (AC 12).
func newNarrativeTestCombat(t *testing.T) *combat.Combat {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 50, CurrentHP: 50, AC: 14, Level: 1, StrMod: 2, WeaponProficiencyRank: "trained"},
		{ID: "n1", Kind: combat.KindNPC, Name: "Grunt", MaxHP: 30, CurrentHP: 30, AC: 12, Level: 1, StrMod: 1, WeaponProficiencyRank: "trained"},
	}
	cbt, err := eng.StartCombat("room_narrative", combatants, reg, nil, "")
	require.NoError(t, err)
	_ = cbt.StartRound(3)
	return cbt
}

// resolveAttackNarrative sets up a single Attack action by player p1 vs NPC n1
// and returns the narrative string from the attack event.
//
// src controls the dice rolls.
func resolveAttackNarrative(t *testing.T, cbt *combat.Combat, src combat.Source) string {
	t.Helper()
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Grunt"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))
	events := combat.ResolveRound(cbt, src, func(string, int) {}, nil)
	for _, ev := range events {
		if ev.ActionType == combat.ActionAttack && ev.ActorID == "p1" && ev.Narrative != "" {
			return ev.Narrative
		}
	}
	t.Fatal("no attack narrative found in events")
	return ""
}

// TestAttackNarrative_ShowsRawD20Roll verifies that the attack narrative contains
// the raw d20 result in the form "1d20 (N)".
//
// REQ-67-1: attack narrative MUST show raw d20 result.
//
// Precondition: fixedSrc{val:13} → d20 = 14.
// Postcondition: narrative contains "1d20 (14)".
func TestAttackNarrative_ShowsRawD20Roll(t *testing.T) {
	cbt := newNarrativeTestCombat(t)
	// val=13 → Intn(20)=13 → d20 = 14; high enough to guarantee a hit vs AC 12 with mods.
	src := fixedSrc{val: 13}
	narrative := resolveAttackNarrative(t, cbt, src)
	assert.Contains(t, narrative, "1d20 (14)", "narrative must show raw d20 roll (REQ-67-1)")
}

// TestAttackNarrative_ShowsTargetAC verifies that the attack narrative contains
// the target's effective AC in the form "vs AC N".
//
// REQ-67-2: attack narrative MUST show target's effective AC.
//
// Precondition: NPC target has AC=12, no initiative bonus → effectiveAC=12.
// Postcondition: narrative contains "vs AC 12".
func TestAttackNarrative_ShowsTargetAC(t *testing.T) {
	cbt := newNarrativeTestCombat(t)
	src := fixedSrc{val: 13}
	narrative := resolveAttackNarrative(t, cbt, src)
	assert.Contains(t, narrative, "vs AC 12", "narrative must show target effective AC (REQ-67-2)")
}

// TestAttackNarrative_MissShowsBreakdown verifies that even a missed attack shows
// the full breakdown so the player understands why it failed.
//
// REQ-67-1, REQ-67-2: breakdown required on miss too.
//
// Precondition: fixedSrc{val:0} → d20=1; even with mods, 1+mods < AC 12 is not guaranteed
// but val=0 means d20=1; with level-1 trained mods (~5) → total≈6, which misses AC 12.
// Postcondition: narrative contains "1d20 (1)" and "vs AC 12".
func TestAttackNarrative_MissShowsBreakdown(t *testing.T) {
	cbt := newNarrativeTestCombat(t)
	// val=0 → d20=1; total = 1 + 2 (StrMod) + 3 (trained L1 prof) = 6 < AC 12 → miss.
	src := fixedSrc{val: 0}
	narrative := resolveAttackNarrative(t, cbt, src)
	assert.Contains(t, narrative, "1d20 (1)", "narrative must show raw d20 on miss (REQ-67-1)")
	assert.Contains(t, narrative, "vs AC 12", "narrative must show target AC on miss (REQ-67-2)")
}

// TestAttackNarrative_StrikeShowsBreakdown verifies that ActionStrike also
// includes the roll breakdown in both strike narratives.
//
// REQ-67-3: ActionStrike narratives MUST also include the breakdown.
//
// Precondition: val=18 → d20=19; high enough to hit vs AC 12.
// Postcondition: at least one strike narrative contains "1d20 (19)" and "vs AC 12".
func TestAttackNarrative_StrikeShowsBreakdown(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 50, CurrentHP: 50, AC: 14, Level: 1, StrMod: 2, WeaponProficiencyRank: "trained"},
		{ID: "n1", Kind: combat.KindNPC, Name: "Grunt", MaxHP: 30, CurrentHP: 30, AC: 12, Level: 1, StrMod: 1, WeaponProficiencyRank: "trained"},
	}
	cbt, err := eng.StartCombat("room_strike_narrative", combatants, reg, nil, "")
	require.NoError(t, err)
	_ = cbt.StartRound(3)

	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStrike, Target: "Grunt"}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	src := fixedSrc{val: 18} // d20=19
	events := combat.ResolveRound(cbt, src, func(string, int) {}, nil)

	found := false
	for _, ev := range events {
		if ev.ActionType == combat.ActionStrike && ev.ActorID == "p1" && ev.Narrative != "" {
			if strings.Contains(ev.Narrative, "1d20") && strings.Contains(ev.Narrative, "vs AC") {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "at least one Strike narrative must contain roll breakdown (REQ-67-3)")
}

// TestProperty_AttackNarrative_AlwaysShowsBreakdown is a property-based test
// verifying that for any d20 result, the narrative always contains "1d20" and "vs AC".
//
// REQ-67-1, REQ-67-2 (property): breakdown invariant over all d20 values.
func TestProperty_AttackNarrative_AlwaysShowsBreakdown(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Intn(20) returns [0,19]; d20 = val+1 ∈ [1,20]
		val := rapid.IntRange(0, 19).Draw(rt, "diceVal")

		cbt := newNarrativeTestCombat(t)
		require.NoError(rt, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Grunt"}))
		require.NoError(rt, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

		src := fixedSrc{val: val}
		events := combat.ResolveRound(cbt, src, func(string, int) {}, nil)

		for _, ev := range events {
			if ev.ActionType == combat.ActionAttack && ev.ActorID == "p1" && ev.Narrative != "" {
				d20 := val + 1
				expectedD20 := "1d20 (" + itoa(d20) + ")"
				assert.Contains(rt, ev.Narrative, expectedD20,
					"narrative must show 1d20 (%d) for dice val %d (REQ-67-1)", d20, val)
				assert.Contains(rt, ev.Narrative, "vs AC 12",
					"narrative must show target AC for dice val %d (REQ-67-2)", val)
				return
			}
		}
		rt.Fatalf("no attack narrative found for dice val %d; events: %+v", val, events)
	})
}

// itoa converts an int to its decimal string representation without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
