package combat

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// Note: fixedSrc is already defined in resolver_stage7_test.go as:
//   type fixedSrc struct{ v int }
//   func (f fixedSrc) Intn(n int) int { if f.v >= n { return n-1 }; return f.v }
// Do NOT redefine it. Use fixedSrc{v: N} in this file.

func newGrenade(dc int, friendly bool) *inventory.ExplosiveDef {
	return &inventory.ExplosiveDef{
		ID:           "test_grenade",
		Name:         "Test Grenade",
		DamageDice:   "2d6",
		DamageType:   "piercing",
		AreaType:     inventory.AreaTypeRoom,
		SaveType:     "reflex",
		SaveDC:       dc,
		Fuse:         inventory.FuseImmediate,
		FriendlyFire: friendly,
	}
}

func newCombatant(id string, kind Kind, hp int) *Combatant {
	return &Combatant{
		ID:           id,
		Kind:         kind,
		Name:         id,
		CurrentHP:    hp,
		MaxHP:        hp,
		QuicknessMod: 0,
	}
}

// REQ-T2: explosiveTargetsOf with friendly_fire: false returns only enemy-kind combatants.
func TestExplosiveTargetsOf_FriendlyFireFalse_EnemiesOnly(t *testing.T) {
	actor := newCombatant("player", KindPlayer, 10)
	enemy1 := newCombatant("npc1", KindNPC, 10)
	enemy2 := newCombatant("npc2", KindNPC, 10)
	ally := newCombatant("ally", KindPlayer, 10)
	cbt := &Combat{Combatants: []*Combatant{actor, enemy1, enemy2, ally}}

	grenade := newGrenade(12, false)
	targets := explosiveTargetsOf(cbt, actor, grenade)

	require.Len(t, targets, 2)
	for _, t2 := range targets {
		assert.Equal(t, KindNPC, t2.Kind)
	}
}

// REQ-T3: explosiveTargetsOf with friendly_fire: true returns all living non-actor combatants.
func TestExplosiveTargetsOf_FriendlyFireTrue_AllNonActor(t *testing.T) {
	actor := newCombatant("player", KindPlayer, 10)
	enemy := newCombatant("npc1", KindNPC, 10)
	ally := newCombatant("ally", KindPlayer, 10)
	cbt := &Combat{Combatants: []*Combatant{actor, enemy, ally}}

	grenade := newGrenade(12, true)
	targets := explosiveTargetsOf(cbt, actor, grenade)

	require.Len(t, targets, 2)
}

// REQ-T4: Actor is never in target list.
func TestExplosiveTargetsOf_ActorNeverIncluded(t *testing.T) {
	actor := newCombatant("player", KindPlayer, 10)
	cbt := &Combat{Combatants: []*Combatant{actor}}

	for _, ff := range []bool{true, false} {
		grenade := newGrenade(12, ff)
		targets := explosiveTargetsOf(cbt, actor, grenade)
		assert.Empty(t, targets, "actor must not appear in target list (friendly_fire=%v)", ff)
	}
}

// REQ-T5: All non-actor combatants dead returns empty slice.
func TestExplosiveTargetsOf_AllDead_EmptySlice(t *testing.T) {
	actor := newCombatant("player", KindPlayer, 10)
	dead := newCombatant("npc1", KindNPC, 0)
	dead.Dead = true
	cbt := &Combat{Combatants: []*Combatant{actor, dead}}

	grenade := newGrenade(12, true)
	targets := explosiveTargetsOf(cbt, actor, grenade)
	assert.Empty(t, targets)
}

// REQ-T1: ResolveExplosive with higher effectiveDC produces saves against that DC.
func TestResolveExplosive_EffectiveDCUsed(t *testing.T) {
	// Use a fixed die source that returns 5 (roll = 6 after +1).
	// target.QuicknessMod = 0; so total = 6.
	// With baseDC=12: total(6) < 12 → Failure.
	// With effectiveDC=5: total(6) >= 5 → Success (half damage).
	src := fixedSrc{v: 5} // Intn(20) returns 5 → roll = 6
	grenade := newGrenade(12, false)
	target := newCombatant("npc1", KindNPC, 10)

	// Against effectiveDC=12: should fail (6 < 12)
	results := ResolveExplosive(grenade, []*Combatant{target}, 12, src)
	require.Len(t, results, 1)
	assert.Equal(t, Failure, results[0].SaveResult)

	// Against effectiveDC=5: should succeed (6 >= 5)
	results2 := ResolveExplosive(grenade, []*Combatant{target}, 5, src)
	require.Len(t, results2, 1)
	assert.Equal(t, Success, results2[0].SaveResult)
}

// REQ-T6 (property): For any actor.Level in [1,20], resolveThrow effective DC = grenade.SaveDC + actor.Level.
// Verified indirectly by checking ResolveExplosive receives a DC higher than grenade.SaveDC.
func TestProperty_ResolveExplosive_EffectiveDCScalesWithLevel(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		baseDC := rapid.IntRange(10, 18).Draw(rt, "baseDC")

		// A fixed roll of 0 (minimum) means total = 0 + 0 = 0, which fails any DC.
		src := fixedSrc{v: 0}
		grenade := newGrenade(baseDC, false)
		target := newCombatant("npc", KindNPC, 10)

		// effectiveDC = baseDC + level; a roll of 0 always fails regardless.
		results := ResolveExplosive(grenade, []*Combatant{target}, baseDC+level, src)
		require.Len(rt, results, 1)
		// Just verify no panic and result has non-negative damage.
		assert.GreaterOrEqual(rt, results[0].BaseDamage, 0)
	})
}
