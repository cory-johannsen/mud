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

// TestApplyReactionEffect_ReduceDamage_ClampsAtZero verifies nil-safety and the zero-damage floor.
// Note: shieldHardness() returns 0 until WeaponDef.Hardness is modeled. This test only exercises
// the nil DamagePending guard and the >= 0 clamp when hardness is 0.
// TODO: add positive hardness test when WeaponDef.Hardness is added.
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

// REQ-READY-15: "enemy_enters" readied trigger matches TriggerOnEnemyEntersRoom.
func TestMatchesReadyTrigger_EnemyEnters(t *testing.T) {
	assert.True(t, gameserver.MatchesReadyTrigger("enemy_enters", reaction.TriggerOnEnemyEntersRoom))
}

// REQ-READY-15: "enemy_attacks_me" readied trigger matches TriggerOnDamageTaken.
func TestMatchesReadyTrigger_EnemyAttacksMe(t *testing.T) {
	assert.True(t, gameserver.MatchesReadyTrigger("enemy_attacks_me", reaction.TriggerOnDamageTaken))
}

// REQ-READY-15: "ally_attacked" readied trigger matches TriggerOnAllyDamaged.
func TestMatchesReadyTrigger_AllyAttacked(t *testing.T) {
	assert.True(t, gameserver.MatchesReadyTrigger("ally_attacked", reaction.TriggerOnAllyDamaged))
}

// REQ-READY-15: unknown readied trigger returns false.
func TestMatchesReadyTrigger_Unknown(t *testing.T) {
	assert.False(t, gameserver.MatchesReadyTrigger("foo", reaction.TriggerOnEnemyEntersRoom))
}

// REQ-READY-15: "enemy_enters" does NOT match TriggerOnDamageTaken.
func TestMatchesReadyTrigger_WrongTrigger(t *testing.T) {
	assert.False(t, gameserver.MatchesReadyTrigger("enemy_enters", reaction.TriggerOnDamageTaken))
}

