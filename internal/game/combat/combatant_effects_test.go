package combat_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/effect"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

func TestBuildCombatantEffects_ConditionEffectsIncluded(t *testing.T) {
	conds := condition.NewActiveSet()
	def := &condition.ConditionDef{
		ID:           "inspired",
		Name:         "Inspired",
		DurationType: "permanent",
		Bonuses: []effect.Bonus{
			{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus},
		},
	}
	require.NoError(t, def.SynthesiseBonuses())
	require.NoError(t, conds.Apply("uid1", def, 1, -1))

	opts := combat.BuildEffectsOpts{
		BearerUID:  "uid1",
		Conditions: conds,
	}
	es := combat.BuildCombatantEffects(opts)
	require.NotNil(t, es)

	r := effect.Resolve(es, effect.StatAttack)
	assert.Equal(t, 1, r.Total)
}

func TestBuildCombatantEffects_FeatPassiveBonusIncluded(t *testing.T) {
	feats := []*ruleset.ClassFeature{
		{
			ID:     "iron_will",
			Active: false,
			PassiveBonuses: []effect.Bonus{
				{Stat: effect.StatGrit, Value: 2, Type: effect.BonusTypeStatus},
			},
		},
	}
	opts := combat.BuildEffectsOpts{
		BearerUID:    "uid1",
		Conditions:   condition.NewActiveSet(),
		PassiveFeats: feats,
	}
	es := combat.BuildCombatantEffects(opts)
	require.NotNil(t, es)

	r := effect.Resolve(es, effect.StatGrit)
	assert.Equal(t, 2, r.Total)
}

func TestBuildCombatantEffects_TechPassiveBonusIncluded(t *testing.T) {
	techs := []*technology.TechnologyDef{
		{
			ID:      "neural_boost",
			Passive: true,
			PassiveBonuses: []effect.Bonus{
				{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeUntyped},
			},
		},
	}
	opts := combat.BuildEffectsOpts{
		BearerUID:    "uid1",
		Conditions:   condition.NewActiveSet(),
		PassiveTechs: techs,
	}
	es := combat.BuildCombatantEffects(opts)
	require.NotNil(t, es)

	r := effect.Resolve(es, effect.StatAttack)
	assert.Equal(t, 1, r.Total)
}

func TestBuildCombatantEffects_ActiveFeatIgnored(t *testing.T) {
	// A feat with Active=true must not contribute passive bonuses even if
	// PassiveBonuses is populated.
	feats := []*ruleset.ClassFeature{
		{
			ID:     "combat_rage",
			Active: true,
			PassiveBonuses: []effect.Bonus{
				{Stat: effect.StatAttack, Value: 3, Type: effect.BonusTypeStatus},
			},
		},
	}
	opts := combat.BuildEffectsOpts{
		BearerUID:    "uid1",
		Conditions:   condition.NewActiveSet(),
		PassiveFeats: feats,
	}
	es := combat.BuildCombatantEffects(opts)
	r := effect.Resolve(es, effect.StatAttack)
	assert.Equal(t, 0, r.Total)
}

func TestBuildCombatantEffects_NonPassiveTechIgnored(t *testing.T) {
	techs := []*technology.TechnologyDef{
		{
			ID:      "active_only",
			Passive: false,
			PassiveBonuses: []effect.Bonus{
				{Stat: effect.StatAttack, Value: 5, Type: effect.BonusTypeItem},
			},
		},
	}
	opts := combat.BuildEffectsOpts{
		BearerUID:    "uid1",
		Conditions:   condition.NewActiveSet(),
		PassiveTechs: techs,
	}
	es := combat.BuildCombatantEffects(opts)
	r := effect.Resolve(es, effect.StatAttack)
	assert.Equal(t, 0, r.Total)
}

func TestBuildCombatantEffects_WeaponBonusIncluded(t *testing.T) {
	opts := combat.BuildEffectsOpts{
		BearerUID:        "uid1",
		Conditions:       condition.NewActiveSet(),
		WeaponSourceID:   "pistol_of_doom",
		WeaponBonusValue: 2,
	}
	es := combat.BuildCombatantEffects(opts)

	rAttack := effect.Resolve(es, effect.StatAttack)
	rDamage := effect.Resolve(es, effect.StatDamage)
	assert.Equal(t, 2, rAttack.Total)
	assert.Equal(t, 2, rDamage.Total)
}

func TestBuildCombatantEffects_ZeroWeaponBonusSuppressed(t *testing.T) {
	opts := combat.BuildEffectsOpts{
		BearerUID:        "uid1",
		Conditions:       condition.NewActiveSet(),
		WeaponSourceID:   "fists",
		WeaponBonusValue: 0,
	}
	es := combat.BuildCombatantEffects(opts)
	r := effect.Resolve(es, effect.StatAttack)
	assert.Equal(t, 0, r.Total)
}

func TestBuildCombatantEffects_ConditionStacksScaleBonuses(t *testing.T) {
	conds := condition.NewActiveSet()
	def := &condition.ConditionDef{
		ID:           "frightened",
		Name:         "Frightened",
		DurationType: "permanent",
		MaxStacks:    4,
		Bonuses: []effect.Bonus{
			{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeStatus},
		},
	}
	require.NoError(t, def.SynthesiseBonuses())
	require.NoError(t, conds.Apply("uid1", def, 3, -1))

	opts := combat.BuildEffectsOpts{
		BearerUID:  "uid1",
		Conditions: conds,
	}
	es := combat.BuildCombatantEffects(opts)
	r := effect.Resolve(es, effect.StatAttack)
	assert.Equal(t, -3, r.Total)
}

func TestSyncConditionApplyAndRemove(t *testing.T) {
	cbt := &combat.Combatant{ID: "uid1"}
	def := &condition.ConditionDef{
		ID:           "inspired",
		Name:         "Inspired",
		DurationType: "permanent",
		Bonuses: []effect.Bonus{
			{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus},
		},
	}
	require.NoError(t, def.SynthesiseBonuses())

	combat.SyncConditionApply(cbt, "uid1", def, 1)
	require.NotNil(t, cbt.Effects)
	r := effect.Resolve(cbt.Effects, effect.StatAttack)
	assert.Equal(t, 1, r.Total)

	combat.SyncConditionRemove(cbt, def.ID)
	r = effect.Resolve(cbt.Effects, effect.StatAttack)
	assert.Equal(t, 0, r.Total)
}

func TestSyncConditionsTickRemovesExpired(t *testing.T) {
	cbt := &combat.Combatant{ID: "uid1"}
	def := &condition.ConditionDef{
		ID:           "burning",
		Name:         "Burning",
		DurationType: "rounds",
		Bonuses: []effect.Bonus{
			{Stat: effect.StatAttack, Value: -1, Type: effect.BonusTypeStatus},
		},
	}
	require.NoError(t, def.SynthesiseBonuses())
	combat.SyncConditionApply(cbt, "uid1", def, 1)
	require.Equal(t, -1, effect.Resolve(cbt.Effects, effect.StatAttack).Total)

	combat.SyncConditionsTick(cbt, []string{"burning"})
	assert.Equal(t, 0, effect.Resolve(cbt.Effects, effect.StatAttack).Total)
}

func TestOverrideNarrativeEvents_EmitsWhenPreviouslyContributing(t *testing.T) {
	// Before: only status +1 contributing.
	before := effect.NewEffectSet()
	before.Apply(effect.Effect{
		EffectID:  "weak_buff",
		SourceID:  "condition:weak_buff",
		CasterUID: "uid1",
		Bonuses:   []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind:   effect.DurationUntilRemove,
	})

	// After: a stronger status +3 suppresses weak_buff.
	after := effect.NewEffectSet()
	after.Apply(effect.Effect{
		EffectID:  "weak_buff",
		SourceID:  "condition:weak_buff",
		CasterUID: "uid1",
		Bonuses:   []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind:   effect.DurationUntilRemove,
	})
	after.Apply(effect.Effect{
		EffectID:  "strong_buff",
		SourceID:  "condition:strong_buff",
		CasterUID: "uid1",
		Bonuses:   []effect.Bonus{{Stat: effect.StatAttack, Value: 3, Type: effect.BonusTypeStatus}},
		DurKind:   effect.DurationUntilRemove,
	})

	events := combat.OverrideNarrativeEvents(before, after, []effect.Stat{effect.StatAttack})
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "condition:weak_buff")
	assert.Contains(t, events[0], "condition:strong_buff")
}

func TestOverrideNarrativeEvents_DetectsNewSuppression(t *testing.T) {
	before := effect.NewEffectSet()
	before.Apply(effect.Effect{
		EffectID:  "inspire",
		SourceID:  "condition:inspire_courage",
		CasterUID: "xin",
		Bonuses:   []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind:   effect.DurationUntilRemove,
	})

	after := effect.NewEffectSet()
	after.Apply(effect.Effect{
		EffectID:  "inspire",
		SourceID:  "condition:inspire_courage",
		CasterUID: "xin",
		Bonuses:   []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind:   effect.DurationUntilRemove,
	})
	after.Apply(effect.Effect{
		EffectID:  "heroism",
		SourceID:  "condition:heroism",
		CasterUID: "kira",
		Bonuses:   []effect.Bonus{{Stat: effect.StatAttack, Value: 2, Type: effect.BonusTypeStatus}},
		DurKind:   effect.DurationUntilRemove,
	})

	events := combat.OverrideNarrativeEvents(before, after, []effect.Stat{effect.StatAttack})
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "overridden")
}

func TestOverrideNarrativeEvents_NoChangeNoEvents(t *testing.T) {
	s := effect.NewEffectSet()
	s.Apply(effect.Effect{
		EffectID:  "e1",
		SourceID:  "condition:bless",
		CasterUID: "",
		Bonuses:   []effect.Bonus{{Stat: effect.StatAttack, Value: 1, Type: effect.BonusTypeStatus}},
		DurKind:   effect.DurationUntilRemove,
	})
	events := combat.OverrideNarrativeEvents(s, s, []effect.Stat{effect.StatAttack})
	assert.Empty(t, events)
}
