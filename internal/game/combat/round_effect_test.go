package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/effect"
)

// TestResolveRound_EffectPipelineContributesToAttack verifies that an attack
// bonus residing in Combatant.Effects (the unified effect pipeline) is
// factored into the attack roll total computed by ResolveRound. This is the
// end-to-end signal that round.go has migrated off the legacy
// condition.AttackBonus accessor.
func TestResolveRound_EffectPipelineContributesToAttack(t *testing.T) {
	// fixedSrc.val == 0 means d20 == 1 after the +1 inside ResolveAttack.
	// We rely on a high target AC to force a non-crit-success outcome so the
	// attack total is directly observable via the event.
	eng := combat.NewEngine()

	attacker := &combat.Combatant{
		ID: "p1", Kind: combat.KindPlayer, Name: "Alice",
		MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1,
		StrMod: 2, DexMod: 1, Initiative: 15,
		WeaponProficiencyRank: "trained",
	}
	defender := &combat.Combatant{
		ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		MaxHP: 12, CurrentHP: 12, AC: 100, Level: 1,
		StrMod: 0, DexMod: 0, Initiative: 10,
	}

	cbt, err := eng.StartCombat("room1", []*combat.Combatant{attacker, defender}, makeConditionReg(), nil, "")
	require.NoError(t, err)

	// Attach a typed +2 status bonus to StatAttack on the attacker via the
	// unified effect pipeline. This bypasses the legacy condition path
	// entirely; if round.go still resolved attack bonuses via
	// condition.AttackBonus, the bonus would be dropped and the attack total
	// would be 2 points lower.
	attacker.Effects = effect.NewEffectSet()
	attacker.Effects.Apply(effect.Effect{
		EffectID:  "heroism",
		SourceID:  "condition:heroism",
		CasterUID: "p1",
		Bonuses:   []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeStatus}},
		DurKind:   effect.DurationUntilRemove,
	})
	// Defender needs a non-nil EffectSet too; empty is fine.
	defender.Effects = effect.NewEffectSet()

	_ = cbt.StartRoundWithSrc(3, &fixedSrc{val: 0})
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionAttack, Target: "Ganger"}))
	require.NoError(t, cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionPass}))
	require.NoError(t, cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionPass}))

	events := combat.ResolveRound(cbt, &fixedSrc{val: 0}, func(id string, hp int) {}, nil, 0)
	require.NotEmpty(t, events)

	var atkEvent *combat.RoundEvent
	for i := range events {
		if events[i].ActorID == "p1" && events[i].ActionType == combat.ActionAttack {
			atkEvent = &events[i]
			break
		}
	}
	require.NotNil(t, atkEvent, "expected an attack event from p1")
	require.NotNil(t, atkEvent.AttackResult, "attack event must carry an AttackResult")

	// Baseline (no effect bonus): d20(1) + StrMod(2) + profBonus(3, trained L1) = 6.
	// With +2 status bonus from the effect pipeline, the total must be 8.
	// A strict equality check catches both under- and over-counting.
	assert.Equal(t, 8, atkEvent.AttackResult.AttackTotal,
		"attack total must include +2 StatAttack bonus sourced from Combatant.Effects")
}
