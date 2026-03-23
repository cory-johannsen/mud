package npc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc"
)

func TestBossAbilityEffect_Validate_ExactlyOneField(t *testing.T) {
	// Zero fields — invalid
	eff := npc.BossAbilityEffect{}
	assert.Error(t, eff.Validate())

	// Exactly one field — valid
	assert.NoError(t, npc.BossAbilityEffect{AoeCondition: "poisoned"}.Validate())
	assert.NoError(t, npc.BossAbilityEffect{AoeDamageExpr: "3d6"}.Validate())
	assert.NoError(t, npc.BossAbilityEffect{HealPct: 25}.Validate())

	// Two fields — invalid
	assert.Error(t, npc.BossAbilityEffect{AoeCondition: "poisoned", HealPct: 25}.Validate())
	assert.Error(t, npc.BossAbilityEffect{AoeDamageExpr: "2d6", HealPct: 10}.Validate())
}

func TestBossAbility_ValidateTrigger(t *testing.T) {
	base := npc.BossAbility{
		ID: "test", Name: "Test",
		Effect: npc.BossAbilityEffect{AoeDamageExpr: "2d6"},
	}

	for _, trigger := range []string{"hp_pct_below", "round_start", "on_damage_taken"} {
		a := base
		a.Trigger = trigger
		assert.NoError(t, a.Validate(), "trigger %q should be valid", trigger)
	}

	base.Trigger = "on_death"
	assert.Error(t, base.Validate())
}

func TestBossAbility_ValidateCooldown(t *testing.T) {
	a := npc.BossAbility{
		ID: "test", Name: "Test", Trigger: "round_start",
		Effect:   npc.BossAbilityEffect{AoeDamageExpr: "1d6"},
		Cooldown: "not_a_duration",
	}
	assert.Error(t, a.Validate())

	a.Cooldown = "30s"
	assert.NoError(t, a.Validate())

	a.Cooldown = ""
	assert.NoError(t, a.Validate())
}

func TestBossAbility_OnDamageTaken_TriggerValueMustBeZero(t *testing.T) {
	a := npc.BossAbility{
		ID: "test", Name: "Test", Trigger: "on_damage_taken",
		TriggerValue: 50,
		Effect:       npc.BossAbilityEffect{AoeDamageExpr: "2d6"},
	}
	assert.Error(t, a.Validate())

	a.TriggerValue = 0
	assert.NoError(t, a.Validate())
}

func TestProperty_BossAbilityEffect_ExactlyOneSet(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cond := rapid.StringN(0, 20, -1).Draw(t, "cond")
		dmg := rapid.StringN(0, 20, -1).Draw(t, "dmg")
		heal := rapid.IntRange(-100, 100).Draw(t, "heal")

		eff := npc.BossAbilityEffect{
			AoeCondition:  cond,
			AoeDamageExpr: dmg,
			HealPct:       heal,
		}
		setCount := 0
		if cond != "" {
			setCount++
		}
		if dmg != "" {
			setCount++
		}
		if heal != 0 {
			setCount++
		}
		err := eff.Validate()
		if setCount == 1 {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
	})
}
