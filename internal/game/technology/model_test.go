package technology_test

import (
	"os"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
)

// validDef returns a minimal valid TechnologyDef.
func validDef() *technology.TechnologyDef {
	return &technology.TechnologyDef{
		ID:         "test-tech",
		Name:       "Test Technology",
		Tradition:  technology.TraditionTechnical,
		Level:      1,
		UsageType:  technology.UsageHardwired,
		Range:      technology.RangeSelf,
		Targets:    technology.TargetsSingle,
		Duration:   "instant",
		Resolution: "none",
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{minimalEffect(technology.EffectUtility)},
		},
	}
}

// minimalEffect returns a minimal valid TechEffect for the given type.
func minimalEffect(t technology.EffectType) technology.TechEffect {
	switch t {
	case technology.EffectDamage:
		return technology.TechEffect{Type: t, Dice: "1d6", DamageType: "fire"}
	case technology.EffectHeal:
		return technology.TechEffect{Type: t, Dice: "1d8"}
	case technology.EffectCondition:
		return technology.TechEffect{Type: t, ConditionID: "stunned"}
	case technology.EffectSkillCheck:
		return technology.TechEffect{Type: t, Skill: "perception", DC: 15}
	case technology.EffectMovement:
		return technology.TechEffect{Type: t, Distance: 10, Direction: "away"}
	case technology.EffectZone:
		return technology.TechEffect{Type: t, Radius: 10}
	case technology.EffectSummon:
		return technology.TechEffect{Type: t, NPCID: "drone", SummonRounds: 3}
	case technology.EffectUtility:
		return technology.TechEffect{Type: t, UtilityType: "unlock"}
	case technology.EffectDrain:
		return technology.TechEffect{Type: t, Dice: "1d4", Resource: "ap"}
	default:
		return technology.TechEffect{Type: t}
	}
}

// REQ-T1: Validate rejects unknown Tradition string.
func TestValidate_REQ_T1_UnknownTradition(t *testing.T) {
	d := validDef()
	d.Tradition = technology.Tradition("unknown_tradition")
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tradition")
}

// REQ-T2: Validate rejects Level < 1 or > 10.
func TestValidate_REQ_T2_InvalidLevel(t *testing.T) {
	t.Run("level_zero", func(t *testing.T) {
		d := validDef()
		d.Level = 0
		err := d.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "level")
	})
	t.Run("level_eleven", func(t *testing.T) {
		d := validDef()
		d.Level = 11
		err := d.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "level")
	})
	t.Run("level_negative", func(t *testing.T) {
		d := validDef()
		d.Level = -1
		err := d.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "level")
	})
}

// REQ-T3: Validate rejects empty Effects.
func TestValidate_REQ_T3_EmptyEffects(t *testing.T) {
	d := validDef()
	d.Effects = technology.TieredEffects{}
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "effects")
}

// REQ-T4: Validate rejects AmpedEffects non-empty with AmpedLevel == 0.
func TestValidate_REQ_T4_AmpedEffectsWithoutAmpedLevel(t *testing.T) {
	d := validDef()
	d.AmpedEffects = technology.TieredEffects{OnApply: []technology.TechEffect{minimalEffect(technology.EffectDamage)}}
	d.AmpedLevel = 0
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amped_level")
}

// REQ-T5: Validate rejects AmpedLevel > 0 with empty AmpedEffects.
func TestValidate_REQ_T5_AmpedLevelWithoutAmpedEffects(t *testing.T) {
	d := validDef()
	d.AmpedLevel = 3
	d.AmpedEffects = technology.TieredEffects{}
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amped_effects")
}

// REQ-T6: Validate rejects skill_check effect with DC == 0.
func TestValidate_REQ_T6_SkillCheckEffectDCZero(t *testing.T) {
	d := validDef()
	d.Effects = technology.TieredEffects{
		OnApply: []technology.TechEffect{
			{Type: technology.EffectSkillCheck, Skill: "stealth", DC: 0},
		},
	}
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dc")
}

// REQ-T15: Validate rejects non-empty SaveType when resolution is not "save".
func TestValidate_REQ_T15_SaveTypeWithoutSaveDC(t *testing.T) {
	d := validDef()
	// validDef uses resolution:"none"; setting save_type is invalid for that resolution.
	d.SaveType = "reflex"
	d.SaveDC = 0
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save_type")
}

// REQ-T16: condition effect with no Duration is valid (parent Duration serves as fallback).
func TestValidate_REQ_T16_ConditionEffectNoLocalDuration(t *testing.T) {
	d := validDef()
	d.Effects = technology.TieredEffects{
		OnApply: []technology.TechEffect{
			{Type: technology.EffectCondition, ConditionID: "blinded"},
		},
	}
	err := d.Validate()
	require.NoError(t, err)
}

// REQ-T13 (property): For any EffectType, a TechEffect with that type and valid required fields
// marshals to YAML and unmarshals back with all set fields preserved.
func TestProperty_REQ_T13_EffectYAMLRoundTrip(t *testing.T) {
	allTypes := []technology.EffectType{
		technology.EffectDamage,
		technology.EffectHeal,
		technology.EffectCondition,
		technology.EffectSkillCheck,
		technology.EffectMovement,
		technology.EffectZone,
		technology.EffectSummon,
		technology.EffectUtility,
		technology.EffectDrain,
	}

	rapid.Check(t, func(rt *rapid.T) {
		// pick a random EffectType
		idx := rapid.IntRange(0, len(allTypes)-1).Draw(rt, "type_idx")
		effectType := allTypes[idx]
		effect := minimalEffect(effectType)

		data, err := yaml.Marshal(effect)
		require.NoError(rt, err)

		var got technology.TechEffect
		err = yaml.Unmarshal(data, &got)
		require.NoError(rt, err)

		assert.Equal(rt, effect.Type, got.Type)
		assert.Equal(rt, effect.Dice, got.Dice)
		assert.Equal(rt, effect.DamageType, got.DamageType)
		assert.Equal(rt, effect.Amount, got.Amount)
		assert.Equal(rt, effect.Resource, got.Resource)
		assert.Equal(rt, effect.ConditionID, got.ConditionID)
		assert.Equal(rt, effect.Duration, got.Duration)
		assert.Equal(rt, effect.Skill, got.Skill)
		assert.Equal(rt, effect.DC, got.DC)
		assert.Equal(rt, effect.Distance, got.Distance)
		assert.Equal(rt, effect.Direction, got.Direction)
		assert.Equal(rt, effect.Radius, got.Radius)
		assert.Equal(rt, effect.NPCID, got.NPCID)
		assert.Equal(rt, effect.SummonRounds, got.SummonRounds)
		assert.Equal(rt, effect.UtilityType, got.UtilityType)
	})
}

// REQ-T14 (property): For any combination of valid Tradition, Level [1-10], UsageType, Range,
// Targets, a TechnologyDef with those fields, non-empty Duration, and one valid effect passes
// Validate() and round-trips through YAML without data loss.
func TestProperty_REQ_T14_TechnologyDefValidateAndRoundTrip(t *testing.T) {
	traditions := []technology.Tradition{
		technology.TraditionTechnical,
		technology.TraditionFanaticDoctrine,
		technology.TraditionNeural,
		technology.TraditionBioSynthetic,
	}
	usageTypes := []technology.UsageType{
		technology.UsageHardwired,
		technology.UsagePrepared,
		technology.UsageSpontaneous,
		technology.UsageInnate,
	}
	ranges := []technology.Range{
		technology.RangeSelf,
		technology.RangeMelee,
		technology.RangeRanged,
		technology.RangeZone,
	}
	targetsList := []technology.Targets{
		technology.TargetsSingle,
		technology.TargetsAllEnemies,
		technology.TargetsAllAllies,
		technology.TargetsZone,
	}

	rapid.Check(t, func(rt *rapid.T) {
		tradition := traditions[rapid.IntRange(0, len(traditions)-1).Draw(rt, "tradition")]
		level := rapid.IntRange(1, 10).Draw(rt, "level")
		usageType := usageTypes[rapid.IntRange(0, len(usageTypes)-1).Draw(rt, "usage_type")]
		r := ranges[rapid.IntRange(0, len(ranges)-1).Draw(rt, "range")]
		targets := targetsList[rapid.IntRange(0, len(targetsList)-1).Draw(rt, "targets")]

		d := &technology.TechnologyDef{
			ID:         "prop-test-id",
			Name:       "Property Test Tech",
			Tradition:  tradition,
			Level:      level,
			UsageType:  usageType,
			Range:      r,
			Targets:    targets,
			Duration:   "1 round",
			Resolution: "none",
			Effects: technology.TieredEffects{
				OnApply: []technology.TechEffect{minimalEffect(technology.EffectUtility)},
			},
		}

		err := d.Validate()
		require.NoError(rt, err)

		data, err := yaml.Marshal(d)
		require.NoError(rt, err)

		var got technology.TechnologyDef
		err = yaml.Unmarshal(data, &got)
		require.NoError(rt, err)

		assert.Equal(rt, d.ID, got.ID)
		assert.Equal(rt, d.Name, got.Name)
		assert.Equal(rt, d.Tradition, got.Tradition)
		assert.Equal(rt, d.Level, got.Level)
		assert.Equal(rt, d.UsageType, got.UsageType)
		assert.Equal(rt, d.Range, got.Range)
		assert.Equal(rt, d.Targets, got.Targets)
		assert.Equal(rt, d.Duration, got.Duration)
		assert.Len(rt, got.Effects.AllEffects(), len(d.Effects.AllEffects()))

		err = got.Validate()
		require.NoError(rt, err)
	})
}

// Additional positive test: a fully valid def passes Validate.
func TestValidate_ValidDef_Passes(t *testing.T) {
	d := validDef()
	err := d.Validate()
	require.NoError(t, err)
}

// Additional test: skill_check effect with missing Skill is rejected.
func TestValidate_SkillCheckEffectMissingSkill(t *testing.T) {
	d := validDef()
	d.Effects = technology.TieredEffects{
		OnApply: []technology.TechEffect{
			{Type: technology.EffectSkillCheck, Skill: "", DC: 15},
		},
	}
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill")
}

// Additional test: valid amped technology def passes Validate.
func TestValidate_ValidAmpedDef_Passes(t *testing.T) {
	d := validDef()
	d.AmpedLevel = 5
	d.AmpedEffects = technology.TieredEffects{OnApply: []technology.TechEffect{minimalEffect(technology.EffectDamage)}}
	err := d.Validate()
	require.NoError(t, err)
}

// Additional test: amped effect with invalid type is rejected.
func TestValidate_InvalidAmpedEffectType(t *testing.T) {
	d := validDef()
	d.AmpedLevel = 3
	d.AmpedEffects = technology.TieredEffects{OnApply: []technology.TechEffect{{Type: technology.EffectType("bad_type")}}}
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amped_effects")
}

// Additional test: amped skill_check effect with missing Skill is rejected.
func TestValidate_InvalidAmpedEffectSkillCheckMissingSkill(t *testing.T) {
	d := validDef()
	d.AmpedLevel = 3
	d.AmpedEffects = technology.TieredEffects{
		OnApply: []technology.TechEffect{
			{Type: technology.EffectSkillCheck, Skill: "", DC: 15},
		},
	}
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amped_effects")
	assert.Contains(t, err.Error(), "skill")
}

// Additional test: amped skill_check effect with DC == 0 is rejected.
func TestValidate_InvalidAmpedEffectSkillCheckDCZero(t *testing.T) {
	d := validDef()
	d.AmpedLevel = 3
	d.AmpedEffects = technology.TieredEffects{
		OnApply: []technology.TechEffect{
			{Type: technology.EffectSkillCheck, Skill: "stealth", DC: 0},
		},
	}
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amped_effects")
	assert.Contains(t, err.Error(), "dc")
}

// REQ-TG1: UsageHardwired constant has value "hardwired"; "cantrip" is no longer valid
func TestUsageHardwired_ConstantValue(t *testing.T) {
	assert.Equal(t, technology.UsageType("hardwired"), technology.UsageHardwired)
}

func TestValidUsageTypes_NoCantripKey(t *testing.T) {
	def := validDef()
	def.UsageType = technology.UsageType("cantrip")
	err := def.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "usage_type")
}

// Additional test: SaveType with SaveDC > 0 and resolution:"save" is valid.
func TestValidate_SaveTypeWithSaveDC_Valid(t *testing.T) {
	d := validDef()
	d.Resolution = "save"
	d.SaveType = "fortitude"
	d.SaveDC = 18
	err := d.Validate()
	require.NoError(t, err)
}

// REQ-TER1: resolution:"save" without save_type rejected.
func TestValidate_REQ_TER1_SaveResolutionWithoutSaveType(t *testing.T) {
	d := validDef()
	d.Resolution = "save"
	d.SaveDC = 15
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save_type")
}

// REQ-TER2: resolution:"save" without save_dc rejected.
func TestValidate_REQ_TER2_SaveResolutionWithoutSaveDC(t *testing.T) {
	d := validDef()
	d.Resolution = "save"
	d.SaveType = "cool"
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save_dc")
}

// REQ-TER3: resolution:"none" accepts empty save_type and zero save_dc.
func TestValidate_REQ_TER3_NoneResolutionNoSaveRequired(t *testing.T) {
	d := validDef()
	d.Resolution = "none"
	err := d.Validate()
	require.NoError(t, err)
}

// REQ-TER4: resolution:"attack" with save_type set is rejected.
func TestValidate_REQ_TER4_AttackResolutionWithSaveTypeRejected(t *testing.T) {
	d := validDef()
	d.Resolution = "attack"
	d.SaveType = "cool"
	d.SaveDC = 15
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "save_type")
}

// REQ-PTM2: Passive: true + action_cost > 0 fails validation
func TestValidate_REQ_PTM2_PassiveRequiresZeroActionCost(t *testing.T) {
	d := validDef()
	d.Passive = true
	d.ActionCost = 1
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "passive")
}

// REQ-PTM1: Passive: true + action_cost == 0 passes validation
func TestValidate_REQ_PTM1_PassiveWithZeroActionCostValid(t *testing.T) {
	d := validDef()
	d.Passive = true
	d.ActionCost = 0
	require.NoError(t, d.Validate())
}

// REQ-PTM1: YAML round-trip — seismic_sense has passive: true and action_cost: 0
func TestSeismicSense_IsPassive(t *testing.T) {
	data, err := os.ReadFile("../../../content/technologies/innate/seismic_sense.yaml")
	require.NoError(t, err)
	var def technology.TechnologyDef
	require.NoError(t, yaml.Unmarshal(data, &def))
	assert.True(t, def.Passive)
	assert.Equal(t, 0, def.ActionCost)
	require.NoError(t, def.Validate(), "seismic_sense.yaml must pass full validation")
}

// REQ-PTM2 (property): For any ActionCost > 0, a passive TechnologyDef must fail Validate.
// For ActionCost == 0, it must pass.
func TestPropertyValidate_REQ_PTM2_PassiveActionCostConstraint(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		d := validDef()
		d.Passive = true
		d.ActionCost = rapid.IntRange(1, 10).Draw(rt, "actionCost")
		err := d.Validate()
		if err == nil {
			rt.Fatalf("expected error for passive tech with action_cost=%d, got nil", d.ActionCost)
		}
	})
}

// REQ-PTM1 (property): A passive TechnologyDef with ActionCost == 0 must always pass Validate
// (given an otherwise valid def).
func TestPropertyValidate_REQ_PTM1_PassiveZeroActionCostAlwaysValid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		d := validDef()
		d.Passive = true
		d.ActionCost = 0
		err := d.Validate()
		if err != nil {
			rt.Fatalf("expected no error for passive tech with action_cost=0, got: %v", err)
		}
	})
}

// TieredEffects round-trip YAML test.
func TestTieredEffects_YAMLRoundTrip(t *testing.T) {
	input := `
resolution: save
save_type: cool
save_dc: 15
effects:
  on_failure:
    - type: damage
      dice: 2d4
      damage_type: neural
  on_crit_failure:
    - type: condition
      condition_id: frightened
      value: 2
      duration: rounds:1
`
	var def technology.TechnologyDef
	require.NoError(t, yaml.Unmarshal([]byte(input), &def))
	assert.Equal(t, "save", def.Resolution)
	assert.Equal(t, "cool", def.SaveType)
	assert.Equal(t, 15, def.SaveDC)
	assert.Len(t, def.Effects.OnFailure, 1)
	assert.Equal(t, technology.EffectDamage, def.Effects.OnFailure[0].Type)
	assert.Len(t, def.Effects.OnCritFailure, 1)
	assert.Equal(t, technology.EffectCondition, def.Effects.OnCritFailure[0].Type)
}
