package gameserver

import (
	"fmt"
	"math"
	"strings"
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

	msgs := ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src, nil)

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

	ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src, nil)

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

	ResolveTechEffects(sess, tech, nil, nil, nil, src, nil)

	assert.LessOrEqual(t, sess.CurrentHP, sess.MaxHP, "HP never above MaxHP")
	assert.Greater(t, sess.CurrentHP, 10, "HP should have increased")
}

// REQ-TER10: Movement effect — target.GridX increases by 1 when direction is "e" (east).
func TestResolveTechEffects_REQ_TER10_MovementPushesTarget(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1"}
	target := makeTarget("npc1", 30, 30, 1) // easy hit
	target.GridX = 5                         // target at column 5
	tech := makeAttackTech(
		[]technology.TechEffect{
			{Type: technology.EffectMovement, Distance: 5, Direction: "e"},
		},
		nil,
	)
	src := &deterministicSrc{val: 10} // hit

	ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src, nil)

	assert.Equal(t, 6, target.GridX, "target pushed 1 cell east: GridX 5 → 6")
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
	ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src, nil)

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
		ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src, nil)
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

		ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src, nil)

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

		ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src, nil)
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

	msgs := ResolveTechEffects(sess, tech, targets, nil, nil, src, nil)

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
		msgs := ResolveTechEffects(sess, tech, targets, nil, nil, src, nil)
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

// mockRoomQuerier is a test double for RoomQuerier. Defined here (Tasks 4 and 6 reuse it — same package).
type mockRoomQuerier struct{ creatures []CreatureInfo }

func (m *mockRoomQuerier) CreaturesInRoom(_, _ string) []CreatureInfo { return m.creatures }

func TestFormatTremorsenseOutput_TableDriven(t *testing.T) {
	cases := []struct {
		name     string
		input    []CreatureInfo
		expected string
	}{
		{
			name:     "empty slice returns no-creatures message",
			input:    []CreatureInfo{},
			expected: "[Seismic Sense] No creatures detected.",
		},
		{
			name:     "single visible creature",
			input:    []CreatureInfo{{Name: "Guard", Hidden: false}},
			expected: "[Seismic Sense] Creatures detected in this room: Guard",
		},
		{
			name:     "single hidden creature",
			input:    []CreatureInfo{{Name: "Assassin", Hidden: true}},
			expected: "[Seismic Sense] Creatures detected in this room: Assassin (concealed)",
		},
		{
			name: "mixed visible and hidden",
			input: []CreatureInfo{
				{Name: "Guard", Hidden: false},
				{Name: "Assassin", Hidden: true},
				{Name: "you", Hidden: false},
			},
			expected: "[Seismic Sense] Creatures detected in this room: Guard, Assassin (concealed), you",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatTremorsenseOutput(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestResolveTechEffects_TremorsenseNilQuerier_ReturnsEmpty(t *testing.T) {
	sess := &session.PlayerSession{UID: "u1", RoomID: "room1"}
	tech := &technology.TechnologyDef{
		ID:         "seismic_sense",
		Passive:    true,
		ActionCost: 0,
		Resolution: "",
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{
				{Type: technology.EffectTremorsense},
			},
		},
	}
	msgs := ResolveTechEffects(sess, tech, nil, nil, nil, &deterministicSrc{val: 1}, nil)
	assert.Empty(t, msgs, "nil querier tremorsense should produce no messages")
}

func TestResolveTechEffects_TremorsenseWithQuerier_ReturnsCreatureList(t *testing.T) {
	sess := &session.PlayerSession{UID: "u1", RoomID: "room1"}
	tech := &technology.TechnologyDef{
		ID:         "seismic_sense",
		Passive:    true,
		ActionCost: 0,
		Resolution: "",
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{
				{Type: technology.EffectTremorsense},
			},
		},
	}
	q := &mockRoomQuerier{creatures: []CreatureInfo{
		{Name: "Guard", Hidden: false},
		{Name: "you", Hidden: false},
	}}
	msgs := ResolveTechEffects(sess, tech, nil, nil, nil, &deterministicSrc{val: 1}, q)
	require.Len(t, msgs, 1)
	assert.Equal(t, "[Seismic Sense] Creatures detected in this room: Guard, you", msgs[0])
}

// REQ-TER-MISS: When an attack tech misses and has no on_miss effects, ResolveTechEffects
// must still return a non-empty message (e.g. "Missed <target>.") so the player receives
// feedback. Regression test for issue #108 (Hydro Pressure Organ produces no output).
func TestResolveTechEffects_AttackMiss_EmitsStandaloneLabel(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1", Level: 1}
	sess.Abilities.Savvy = 10
	// Target with AC=30 so any roll misses.
	target := makeTarget("enemy", 20, 20, 30)
	tech := makeAttackTech(
		[]technology.TechEffect{{Type: technology.EffectDamage, Dice: "3d6", DamageType: "bludgeoning"}},
		[]technology.TechEffect{{Type: technology.EffectDamage, Dice: "6d6", DamageType: "bludgeoning"}},
		// No on_miss effects — simulates hydro_pressure_organ.
	)
	// src.Intn(20) returns 0 → roll=1; total=1 vs AC=30 → CritFailure (miss).
	src := &deterministicSrc{val: 0}
	msgs := ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src, nil)
	require.Len(t, msgs, 1, "should emit exactly one message on miss")
	assert.Contains(t, msgs[0], "enemy", "miss message must name the target")
	assert.True(t, strings.HasSuffix(msgs[0], "."), "miss message must end with a period, got: %q", msgs[0])
	// HP must be unchanged on a miss.
	assert.Equal(t, 20, target.CurrentHP, "HP must not change on a miss")
}

// REQ-TER-HIT-NOFX: When an attack tech hits but its hit tier has no effects,
// ResolveTechEffects must emit a standalone label so the player gets feedback.
func TestResolveTechEffects_AttackHit_NoEffects_EmitsStandaloneLabel(t *testing.T) {
	sess := &session.PlayerSession{UID: "p1", Level: 1}
	sess.Abilities.Savvy = 10
	// Target with AC=1 so any roll hits.
	target := makeTarget("enemy", 20, 20, 1)
	// Tech with no on_hit effects.
	tech := &technology.TechnologyDef{
		ID:         "test-no-fx",
		Resolution: "attack",
		Tradition:  technology.TraditionNeural,
		Effects:    technology.TieredEffects{},
	}
	// src.Intn(20) returns 10 → roll=11; total=11 vs AC=1 → Success (hit).
	src := &deterministicSrc{val: 10}
	msgs := ResolveTechEffects(sess, tech, []*combat.Combatant{target}, nil, nil, src, nil)
	require.Len(t, msgs, 1, "should emit exactly one message on hit with no effects")
	assert.Contains(t, msgs[0], "enemy", "hit message must name the target")
}

func genCreatureInfo(t *rapid.T) CreatureInfo {
	return CreatureInfo{
		Name:   rapid.StringN(1, 20, -1).Draw(t, "name"),
		Hidden: rapid.Bool().Draw(t, "hidden"),
	}
}

func TestProperty_FormatTremorsenseOutput_HiddenSuffix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		creatures := rapid.SliceOfN(rapid.Custom(genCreatureInfo), 1, 10).Draw(t, "creatures")
		output := FormatTremorsenseOutput(creatures)

		// Build expected output independently using the spec's mandated logic:
		// hidden creatures are suffixed with " (concealed)", joined by ", ".
		parts := make([]string, len(creatures))
		for i, c := range creatures {
			if c.Hidden {
				parts[i] = c.Name + " (concealed)"
			} else {
				parts[i] = c.Name
			}
		}
		expected := "[Seismic Sense] Creatures detected in this room: " + strings.Join(parts, ", ")
		assert.Equal(t, expected, output,
			"FormatTremorsenseOutput must suffix hidden creatures with (concealed) and leave visible ones unsuffixed")
	})
}
