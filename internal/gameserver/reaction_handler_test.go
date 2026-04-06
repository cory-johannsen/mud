package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/session"
)

// REQ-RXN21: empty requirement always returns true.
func TestCheckReactionRequirement_EmptyString_ReturnsTrue(t *testing.T) {
	sess := &session.PlayerSession{}
	assert.True(t, CheckReactionRequirement(sess, ""))
}

// REQ-RXN21: "none" requirement always returns true.
func TestCheckReactionRequirement_NoneString_ReturnsTrue(t *testing.T) {
	sess := &session.PlayerSession{}
	assert.True(t, CheckReactionRequirement(sess, "none"))
}

// REQ-RXN24: wielding_melee_weapon returns false when no loadout is set.
func TestCheckReactionRequirement_WieldingMeleeWeapon_FalseWhenNoLoadout(t *testing.T) {
	sess := &session.PlayerSession{} // LoadoutSet field is nil
	assert.False(t, CheckReactionRequirement(sess, "wielding_melee_weapon"))
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
		Rarity:              "salvage",
	}
	ls := inventory.NewLoadoutSet()
	if err := ls.ActivePreset().EquipMainHand(def); err != nil {
		t.Fatalf("EquipMainHand failed: %v", err)
	}
	sess := &session.PlayerSession{LoadoutSet: ls}
	assert.True(t, CheckReactionRequirement(sess, "wielding_melee_weapon"))
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
		Rarity:              "salvage",
	}
	ls := inventory.NewLoadoutSet()
	if err := ls.ActivePreset().EquipMainHand(def); err != nil {
		t.Fatalf("EquipMainHand failed: %v", err)
	}
	sess := &session.PlayerSession{LoadoutSet: ls}
	assert.False(t, CheckReactionRequirement(sess, "wielding_melee_weapon"))
}

// REQ-RXN22: reroll_save never worsens outcome.
// Outcome int values: CritSuccess=0, Success=1, Failure=2, CritFailure=3.
func TestApplyReactionEffect_RerollSave_NeverWorsensOutcome(t *testing.T) {
	for i := 0; i < 50; i++ {
		original := 3 // CritFailure
		ctx := reaction.ReactionContext{SaveOutcome: &original}
		effect := reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"}
		sess := &session.PlayerSession{}
		ApplyReactionEffect(sess, effect, &ctx)
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
		ApplyReactionEffect(sess, effect, &ctx)
	})
}

// TestApplyReactionEffect_ReduceDamage_ClampsAtZero verifies nil-safety and the zero-damage floor.
func TestApplyReactionEffect_ReduceDamage_ClampsAtZero(t *testing.T) {
	ctx := reaction.ReactionContext{DamagePending: new(2)}
	effect := reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage}
	sess := &session.PlayerSession{}
	ApplyReactionEffect(sess, effect, &ctx)
	assert.GreaterOrEqual(t, *ctx.DamagePending, 0, "pending damage must not go negative")
}

// REQ-RXN22: reduce_damage with nil DamagePending is a no-op (no panic).
func TestApplyReactionEffect_ReduceDamage_NilDamagePending_Noop(t *testing.T) {
	ctx := reaction.ReactionContext{}
	effect := reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage}
	sess := &session.PlayerSession{}
	assert.NotPanics(t, func() {
		ApplyReactionEffect(sess, effect, &ctx)
	})
}

// REQ-RXN28: wielding_shield returns false when LoadoutSet is nil.
func TestCheckReactionRequirement_WieldingShield_FalseWhenNoLoadout(t *testing.T) {
	sess := &session.PlayerSession{} // LoadoutSet field is nil
	assert.False(t, CheckReactionRequirement(sess, "wielding_shield"))
}

// REQ-RXN28: wielding_shield returns true when a shield is equipped in the off-hand.
func TestCheckReactionRequirement_WieldingShield_TrueWhenShieldEquipped(t *testing.T) {
	def := &inventory.WeaponDef{
		ID:                  "scrap_shield",
		Name:                "Scrap Shield",
		DamageDice:          "1d4",
		DamageType:          "bludgeoning",
		Kind:                inventory.WeaponKindShield,
		ProficiencyCategory: "simple_weapons",
		Rarity:              "salvage",
	}
	ls := inventory.NewLoadoutSet()
	if err := ls.ActivePreset().EquipOffHand(def); err != nil {
		t.Fatalf("EquipOffHand failed: %v", err)
	}
	sess := &session.PlayerSession{LoadoutSet: ls}
	assert.True(t, CheckReactionRequirement(sess, "wielding_shield"))
}

// REQ-RXN28: wielding_shield returns false when the off-hand holds a non-shield weapon.
func TestCheckReactionRequirement_WieldingShield_FalseWhenOffHandNotShield(t *testing.T) {
	def := &inventory.WeaponDef{
		ID:                  "combat_knife",
		Name:                "Combat Knife",
		DamageDice:          "1d4",
		DamageType:          "slashing",
		Kind:                inventory.WeaponKindOneHanded,
		ProficiencyCategory: "simple_weapons",
		Rarity:              "salvage",
	}
	ls := inventory.NewLoadoutSet()
	if err := ls.ActivePreset().EquipOffHand(def); err != nil {
		t.Fatalf("EquipOffHand failed: %v", err)
	}
	sess := &session.PlayerSession{LoadoutSet: ls}
	assert.False(t, CheckReactionRequirement(sess, "wielding_shield"))
}

// REQ-RXN29: shieldHardness returns the Hardness from the equipped off-hand shield.
func TestShieldHardness_ReturnsHardnessFromOffHandShield(t *testing.T) {
	def := &inventory.WeaponDef{
		ID:                  "riot_shield",
		Name:                "Riot Shield",
		DamageDice:          "1d4",
		DamageType:          "bludgeoning",
		Kind:                inventory.WeaponKindShield,
		ProficiencyCategory: "simple_weapons",
		Rarity:              "salvage",
		Hardness:            3,
	}
	ls := inventory.NewLoadoutSet()
	if err := ls.ActivePreset().EquipOffHand(def); err != nil {
		t.Fatalf("EquipOffHand failed: %v", err)
	}
	sess := &session.PlayerSession{LoadoutSet: ls}
	ctx := reaction.ReactionContext{DamagePending: new(10)}
	effect := reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage}
	ApplyReactionEffect(sess, effect, &ctx)
	assert.Equal(t, 7, *ctx.DamagePending, "shieldHardness 3 should reduce 10 damage to 7")
}

// REQ-RXN29: shieldHardness returns 0 when OffHand is nil.
func TestShieldHardness_ReturnsZeroWhenNoOffHand(t *testing.T) {
	ls := inventory.NewLoadoutSet()
	sess := &session.PlayerSession{LoadoutSet: ls}
	ctx := reaction.ReactionContext{DamagePending: new(5)}
	effect := reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage}
	ApplyReactionEffect(sess, effect, &ctx)
	assert.Equal(t, 5, *ctx.DamagePending, "zero hardness must not reduce damage")
}

// REQ-RXN29: shieldHardness returns 0 when the off-hand holds a non-shield weapon.
func TestShieldHardness_ReturnsZeroWhenOffHandNotShield(t *testing.T) {
	def := &inventory.WeaponDef{
		ID:                  "side_knife",
		Name:                "Side Knife",
		DamageDice:          "1d4",
		DamageType:          "slashing",
		Kind:                inventory.WeaponKindOneHanded,
		ProficiencyCategory: "simple_weapons",
		Rarity:              "salvage",
	}
	ls := inventory.NewLoadoutSet()
	if err := ls.ActivePreset().EquipOffHand(def); err != nil {
		t.Fatalf("EquipOffHand failed: %v", err)
	}
	sess := &session.PlayerSession{LoadoutSet: ls}
	ctx := reaction.ReactionContext{DamagePending: new(5)}
	effect := reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage}
	ApplyReactionEffect(sess, effect, &ctx)
	assert.Equal(t, 5, *ctx.DamagePending, "off-hand non-shield must not reduce damage")
}

// REQ-READY-15: "enemy_enters" readied trigger matches TriggerOnEnemyEntersRoom.
func TestMatchesReadyTrigger_EnemyEnters(t *testing.T) {
	assert.True(t, matchesReadyTrigger("enemy_enters", reaction.TriggerOnEnemyEntersRoom))
}

// REQ-READY-15: "enemy_attacks_me" readied trigger matches TriggerOnDamageTaken.
func TestMatchesReadyTrigger_EnemyAttacksMe(t *testing.T) {
	assert.True(t, matchesReadyTrigger("enemy_attacks_me", reaction.TriggerOnDamageTaken))
}

// REQ-READY-15: "ally_attacked" readied trigger matches TriggerOnAllyDamaged.
func TestMatchesReadyTrigger_AllyAttacked(t *testing.T) {
	assert.True(t, matchesReadyTrigger("ally_attacked", reaction.TriggerOnAllyDamaged))
}

// REQ-READY-15: unknown readied trigger returns false.
func TestMatchesReadyTrigger_Unknown(t *testing.T) {
	assert.False(t, matchesReadyTrigger("foo", reaction.TriggerOnEnemyEntersRoom))
}

// REQ-READY-15: "enemy_enters" does NOT match TriggerOnDamageTaken.
func TestMatchesReadyTrigger_WrongTrigger(t *testing.T) {
	assert.False(t, matchesReadyTrigger("enemy_enters", reaction.TriggerOnDamageTaken))
}
