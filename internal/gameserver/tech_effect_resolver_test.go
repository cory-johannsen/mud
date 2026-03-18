package gameserver

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// deterministicSrc always returns the fixed value for Intn.
type deterministicSrc struct{ val int }

func (d *deterministicSrc) Intn(n int) int {
	if d.val >= n {
		return n - 1
	}
	return d.val
}

// makeSaveTech builds a minimal save-based TechnologyDef for tests.
func makeSaveTech(saveType string, onFailure, onCritFailure []technology.TechEffect) *technology.TechnologyDef {
	return &technology.TechnologyDef{
		ID:         "test-save-tech",
		Name:       "Test Save Tech",
		Resolution: "save",
		SaveType:   saveType,
		SaveDC:     15,
		Effects: technology.TieredEffects{
			OnFailure:     onFailure,
			OnCritFailure: onCritFailure,
		},
	}
}

// makeAttackTech builds a minimal attack-based TechnologyDef for tests.
func makeAttackTech(onHit, onCritHit []technology.TechEffect) *technology.TechnologyDef {
	return &technology.TechnologyDef{
		ID:         "test-attack-tech",
		Name:       "Test Attack Tech",
		Tradition:  technology.TraditionNeural,
		Resolution: "attack",
		Effects: technology.TieredEffects{
			OnHit:     onHit,
			OnCritHit: onCritHit,
		},
	}
}

// makeTarget builds a minimal Combatant for tests.
func makeTarget(name string, currentHP, maxHP, ac int) *combat.Combatant {
	return &combat.Combatant{
		ID:        name,
		Name:      name,
		CurrentHP: currentHP,
		MaxHP:     maxHP,
		AC:        ac,
		Level:     1,
	}
}

// REQ-TER5: OnFailure effects applied when save returns Failure.
// src.Intn(20) returns 0 → roll=1; total=1 vs DC=15 → Failure
func TestResolveTechEffects_REQ_TER5_SaveFailureAppliesOnFailure(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	sess.CurrentHP = 20
	sess.MaxHP = 20
	target := makeTarget("npc1", 30, 30, 12)
	tech := makeSaveTech("cool", []technology.TechEffect{
		{Type: technology.EffectDamage, Dice: "1d6", DamageType: "neural"},
	}, nil)
	src := &deterministicSrc{val: 0} // roll=1, fails DC=15

	msgs := ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

	require.NotEmpty(t, msgs)
	assert.Less(t, target.CurrentHP, 30, "expected damage applied on failure")
}

// REQ-TER7: Damage effect — target.CurrentHP decreases; never below 0.
func TestResolveTechEffects_REQ_TER7_DamageReducesHP(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	target := makeTarget("npc1", 5, 30, 1) // AC=1 → easy hit
	tech := makeAttackTech(
		[]technology.TechEffect{{Type: technology.EffectDamage, Dice: "1d6", DamageType: "acid"}},
		nil,
	)
	src := &deterministicSrc{val: 10} // roll=11 vs AC=1 → hit

	ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

	assert.GreaterOrEqual(t, target.CurrentHP, 0, "HP never below 0")
	assert.Less(t, target.CurrentHP, 5, "HP should be reduced")
}

// REQ-TER8: Heal effect — sess.CurrentHP increases; never above MaxHP.
func TestResolveTechEffects_REQ_TER8_HealIncreasesHP(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	sess.CurrentHP = 10
	sess.MaxHP = 20
	tech := &technology.TechnologyDef{
		ID:         "nanite",
		Resolution: "none",
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{
				{Type: technology.EffectHeal, Dice: "1d8", Amount: 0},
			},
		},
	}
	src := &deterministicSrc{val: 7} // d8 → 8

	ResolveTechEffects(sess, tech, nil, nil, nil, src)

	assert.LessOrEqual(t, sess.CurrentHP, sess.MaxHP, "HP never above MaxHP")
	assert.Greater(t, sess.CurrentHP, 10, "HP should have increased")
}

// REQ-TER10: Movement effect — target.Position increases when direction is "away".
func TestResolveTechEffects_REQ_TER10_MovementPushesTarget(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	target := makeTarget("npc1", 30, 30, 1) // easy hit
	target.Position = 25
	tech := makeAttackTech(
		[]technology.TechEffect{
			{Type: technology.EffectMovement, Distance: 5, Direction: "away"},
		},
		nil,
	)
	src := &deterministicSrc{val: 10} // hit

	ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

	assert.Equal(t, 30, target.Position, "target pushed 5 ft away from 25 → 30")
}

// REQ-TER11: Attack tech — no effects on miss.
func TestResolveTechEffects_REQ_TER11_AttackMissNoEffects(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	target := makeTarget("npc1", 30, 30, 25) // high AC
	tech := makeAttackTech(
		[]technology.TechEffect{{Type: technology.EffectDamage, Dice: "1d6", DamageType: "acid"}},
		nil,
	)
	src := &deterministicSrc{val: 0} // roll=1 vs AC=25 → miss

	before := target.CurrentHP
	ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

	assert.Equal(t, before, target.CurrentHP, "no damage on miss")
}

// REQ-TER12 (property): For save-based tech, CritSuccess tier never applies on Failure.
func TestProperty_REQ_TER12_CritSuccessTierNotAppliedOnFailure(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sess := &session.PlayerSession{UID: "p1"}
		hp := rapid.IntRange(10, 50).Draw(rt, "hp")
		target := makeTarget("npc1", hp, hp, 12)
		// Always-fail src: roll=1 vs DC=30 → always Failure
		src := &deterministicSrc{val: 0}
		tech := &technology.TechnologyDef{
			ID:         "test",
			Resolution: "save",
			SaveType:   "cool",
			SaveDC:     30,
			Effects: technology.TieredEffects{
				OnCritSuccess: []technology.TechEffect{
					{Type: technology.EffectDamage, Dice: "10d10", DamageType: "neural"},
				},
				OnFailure: []technology.TechEffect{
					{Type: technology.EffectDamage, Dice: "1d4", DamageType: "neural"},
				},
			},
		}
		before := target.CurrentHP
		ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)
		dmg := before - target.CurrentHP
		assert.LessOrEqual(rt, dmg, 4, "only OnFailure 1d4 should apply, not OnCritSuccess 10d10")
	})
}

// REQ-TER13 (property): Damage output always within dice bounds.
func TestProperty_REQ_TER13_DamageWithinDiceBounds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dieSize := rapid.IntRange(1, 12).Draw(rt, "die_size")
		numDice := rapid.IntRange(1, 4).Draw(rt, "num_dice")
		flat := rapid.IntRange(0, 10).Draw(rt, "flat")
		expr := fmt.Sprintf("%dd%d", numDice, dieSize)

		sess := &session.PlayerSession{UID: "p1"}
		target := makeTarget("npc1", 1000, 1000, 1) // easy hit, high HP
		tech := makeAttackTech(
			[]technology.TechEffect{{Type: technology.EffectDamage, Dice: expr, Amount: flat, DamageType: "acid"}},
			nil,
		)
		src := &deterministicSrc{val: 10} // always hit
		before := target.CurrentHP

		ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)

		dmg := before - target.CurrentHP
		minDmg := numDice + flat
		maxDmg := numDice*dieSize + flat
		assert.GreaterOrEqual(rt, dmg, minDmg, "damage at least minimum")
		assert.LessOrEqual(rt, dmg, maxDmg, "damage at most maximum")
	})
}

// REQ-TER14 (property): target.CurrentHP never goes negative.
func TestProperty_REQ_TER14_HPNeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		initialHP := rapid.IntRange(1, 20).Draw(rt, "hp")
		target := makeTarget("npc1", initialHP, initialHP, 1)
		sess := &session.PlayerSession{UID: "p1"}
		tech := makeAttackTech(
			[]technology.TechEffect{{Type: technology.EffectDamage, Dice: "4d6", DamageType: "neural"}},
			nil,
		)
		src := &deterministicSrc{val: 15}

		ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src)
		assert.GreaterOrEqual(rt, target.CurrentHP, 0, "HP must not go negative")
	})
}

// REQ-TER21: Area-targeting tech applies effects to every target in the slice.
func TestResolveTechEffects_REQ_TER21_AreaTargetingAppliesToAll(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	targets := []*combat.Combatant{
		makeTarget("npc1", 30, 30, 1),
		makeTarget("npc2", 30, 30, 1),
		makeTarget("npc3", 30, 30, 1),
	}
	tech := &technology.TechnologyDef{
		ID:         "terror_broadcast",
		Resolution: "save",
		SaveType:   "cool",
		SaveDC:     30, // always fail with 0 mods
		Effects: technology.TieredEffects{
			OnFailure: []technology.TechEffect{
				{Type: technology.EffectDamage, Dice: "1d4", DamageType: "neural"},
			},
		},
	}
	src := &deterministicSrc{val: 0} // all fail

	msgs := ResolveTechEffects(sess, tech, targets, nil, nil, src)

	for _, tgt := range targets {
		assert.Less(t, tgt.CurrentHP, 30, "all targets should take damage")
	}
	assert.GreaterOrEqual(t, len(msgs), 3, "one message per target")
}

// REQ-TER22 (property): Area-targeting with N enemies produces N messages.
func TestProperty_REQ_TER22_AreaMessagesEqualTargetCount(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 5).Draw(rt, "n_targets")
		targets := make([]*combat.Combatant, n)
		for i := range targets {
			targets[i] = makeTarget(fmt.Sprintf("npc%d", i), 100, 100, 1)
		}
		sess := &session.PlayerSession{UID: "p1"}
		tech := &technology.TechnologyDef{
			ID:         "area_tech",
			Resolution: "save",
			SaveType:   "cool",
			SaveDC:     30,
			Effects: technology.TieredEffects{
				OnFailure: []technology.TechEffect{
					{Type: technology.EffectDamage, Dice: "1d4", DamageType: "neural"},
				},
			},
		}
		src := &deterministicSrc{val: 0}
		msgs := ResolveTechEffects(sess, tech, targets, nil, nil, src)
		assert.Equal(rt, n, len(msgs))
	})
}

// TestProperty_AbilityModifier_MatchesFloorDiv verifies that abilityModifier
// produces floor((score-10)/2) for all scores 1–20.
func TestProperty_AbilityModifier_MatchesFloorDiv(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		score := rapid.IntRange(1, 20).Draw(rt, "score")
		got := abilityModifier(score)
		diff := score - 10
		expected := int(math.Floor(float64(diff) / 2.0))
		assert.Equal(rt, expected, got, "abilityModifier(%d)", score)
	})
}

// Compile-time check that deterministicSrc satisfies condition.Registry usage (unused var suppresses lint).
var _ *condition.Registry = nil
