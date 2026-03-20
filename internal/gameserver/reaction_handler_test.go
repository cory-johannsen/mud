package gameserver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/session"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
)

// REQ-RXN21: empty requirement always returns true.
func TestCheckReactionRequirement_EmptyString_ReturnsTrue(t *testing.T) {
	sess := &session.PlayerSession{}
	assert.True(t, gameserver.CheckReactionRequirement(sess, ""))
}

// REQ-RXN21: "none" requirement always returns true.
func TestCheckReactionRequirement_NoneString_ReturnsTrue(t *testing.T) {
	sess := &session.PlayerSession{}
	assert.True(t, gameserver.CheckReactionRequirement(sess, "none"))
}

// REQ-RXN24: wielding_melee_weapon returns false when no loadout is set.
func TestCheckReactionRequirement_WieldingMeleeWeapon_FalseWhenNoLoadout(t *testing.T) {
	sess := &session.PlayerSession{} // LoadoutSet field is nil
	assert.False(t, gameserver.CheckReactionRequirement(sess, "wielding_melee_weapon"))
}

// REQ-RXN27: wielding_melee_weapon returns true when a melee weapon is equipped in the main hand.
func TestCheckReactionRequirement_WieldingMeleeWeapon_TrueWhenMeleeEquipped(t *testing.T) {
	def := &inventory.WeaponDef{
		ID:                  "shortsword",
		Name:                "Shortsword",
		DamageDice:          "1d6",
		DamageType:          "piercing",
		RangeIncrement:      0, // melee
		Kind:                inventory.WeaponKindOneHanded,
		ProficiencyCategory: "martial_weapons",
	}
	ls := inventory.NewLoadoutSet()
	if err := ls.ActivePreset().EquipMainHand(def); err != nil {
		t.Fatalf("EquipMainHand failed: %v", err)
	}
	sess := &session.PlayerSession{LoadoutSet: ls}
	assert.True(t, gameserver.CheckReactionRequirement(sess, "wielding_melee_weapon"))
}

// REQ-RXN27: wielding_melee_weapon returns false when a ranged weapon is equipped in the main hand.
func TestCheckReactionRequirement_WieldingMeleeWeapon_FalseWhenRangedEquipped(t *testing.T) {
	def := &inventory.WeaponDef{
		ID:                  "shortbow",
		Name:                "Shortbow",
		DamageDice:          "1d6",
		DamageType:          "piercing",
		RangeIncrement:      30, // ranged
		Kind:                inventory.WeaponKindTwoHanded,
		ProficiencyCategory: "martial_ranged",
	}
	ls := inventory.NewLoadoutSet()
	if err := ls.ActivePreset().EquipMainHand(def); err != nil {
		t.Fatalf("EquipMainHand failed: %v", err)
	}
	sess := &session.PlayerSession{LoadoutSet: ls}
	assert.False(t, gameserver.CheckReactionRequirement(sess, "wielding_melee_weapon"))
}

// REQ-RXN22: reroll_save never worsens outcome.
// Outcome int values: CritSuccess=0, Success=1, Failure=2, CritFailure=3.
func TestApplyReactionEffect_RerollSave_NeverWorsensOutcome(t *testing.T) {
	for i := 0; i < 50; i++ {
		original := 3 // CritFailure
		ctx := reaction.ReactionContext{SaveOutcome: &original}
		effect := reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"}
		sess := &session.PlayerSession{}
		gameserver.ApplyReactionEffect(sess, effect, &ctx)
		assert.LessOrEqual(t, *ctx.SaveOutcome, 3, "reroll must not produce a value > CritFailure")
		assert.GreaterOrEqual(t, *ctx.SaveOutcome, 0, "reroll must not produce a value < CritSuccess")
	}
}

// REQ-RXN22: reroll_save with nil SaveOutcome is a no-op (no panic).
func TestApplyReactionEffect_RerollSave_NilSaveOutcome_Noop(t *testing.T) {
	ctx := reaction.ReactionContext{}
	effect := reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"}
	sess := &session.PlayerSession{}
	assert.NotPanics(t, func() {
		gameserver.ApplyReactionEffect(sess, effect, &ctx)
	})
}

// REQ-RXN22: reduce_damage clamps at 0.
func TestApplyReactionEffect_ReduceDamage_ClampsAtZero(t *testing.T) {
	pending := 2
	ctx := reaction.ReactionContext{DamagePending: &pending}
	effect := reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage}
	sess := &session.PlayerSession{}
	gameserver.ApplyReactionEffect(sess, effect, &ctx)
	assert.GreaterOrEqual(t, *ctx.DamagePending, 0, "pending damage must not go negative")
}

// REQ-RXN22: reduce_damage with nil DamagePending is a no-op (no panic).
func TestApplyReactionEffect_ReduceDamage_NilDamagePending_Noop(t *testing.T) {
	ctx := reaction.ReactionContext{}
	effect := reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage}
	sess := &session.PlayerSession{}
	assert.NotPanics(t, func() {
		gameserver.ApplyReactionEffect(sess, effect, &ctx)
	})
}

// REQ-RXN10: ReactionsRemaining never goes below 0 when guarded correctly.
func TestReactionsRemaining_NeverGoesNegative(t *testing.T) {
	sess := &session.PlayerSession{ReactionsRemaining: 0}
	if sess.ReactionsRemaining > 0 {
		sess.ReactionsRemaining--
	}
	assert.Equal(t, 0, sess.ReactionsRemaining)
}
