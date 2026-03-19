package pf2e_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/importer/pf2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func loadFixture(t *testing.T, name string) *pf2e.PF2ESpell {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	spell, err := pf2e.ParseSpell(data)
	require.NoError(t, err)
	return spell
}

func TestConvertSpell_TraditionMapping(t *testing.T) {
	cases := []struct {
		pf2eTradition string
		wantTradition technology.Tradition
	}{
		{"occult", technology.TraditionNeural},
		{"primal", technology.TraditionBioSynthetic},
		{"arcane", technology.TraditionTechnical},
		{"divine", technology.TraditionFanaticDoctrine},
	}
	for _, tc := range cases {
		spell := makeDamageSpell([]string{tc.pf2eTradition}, "2", "30 feet", "", "instantaneous", "")
		results, warnings, err := pf2e.ConvertSpell(spell)
		require.NoError(t, err)
		require.Len(t, results, 1, "tradition %s", tc.pf2eTradition)
		assert.Empty(t, warnings)
		assert.Equal(t, string(tc.wantTradition), results[0].Tradition)
		assert.Equal(t, tc.wantTradition, results[0].Def.Tradition)
	}
}

func TestConvertSpell_NoTradition_SkippedWithWarning(t *testing.T) {
	spell := loadFixture(t, "no_tradition.json")
	results, warnings, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	assert.Empty(t, results)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "no matching tradition")
}

func TestConvertSpell_SingleTradition_NoIDSuffix(t *testing.T) {
	spell := loadFixture(t, "divine_single.json")
	results, _, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "heal", results[0].Def.ID)
}

func TestConvertSpell_MultiTradition_IDSuffix(t *testing.T) {
	spell := loadFixture(t, "save_spell.json")
	results, _, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.Len(t, results, 2)
	ids := map[string]bool{}
	for _, r := range results {
		ids[r.Def.ID] = true
	}
	assert.True(t, ids["fear_fanatic_doctrine"])
	assert.True(t, ids["fear_neural"])
}

func TestConvertSpell_ReactionActionCost(t *testing.T) {
	spell := loadFixture(t, "reaction_spell.json")
	results, _, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 0, results[0].Def.ActionCost)
}

func TestConvertSpell_ActionCostNumeric(t *testing.T) {
	for input, want := range map[string]int{"1": 1, "2": 2, "3": 3} {
		spell := makeDamageSpell([]string{"arcane"}, input, "30 feet", "", "instantaneous", "")
		results, _, err := pf2e.ConvertSpell(spell)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, want, results[0].Def.ActionCost, "input=%q", input)
	}
}

func TestConvertSpell_UnknownActionCost_DefaultsWithWarning(t *testing.T) {
	spell := makeDamageSpell([]string{"arcane"}, "swift", "30 feet", "", "instantaneous", "")
	results, warnings, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 2, results[0].Def.ActionCost)
	assert.NotEmpty(t, warnings)
}

func TestConvertSpell_RangeMapping(t *testing.T) {
	cases := []struct {
		input string
		want  technology.Range
	}{
		{"touch", technology.RangeMelee},
		{"melee", technology.RangeMelee},
		{"self", technology.RangeSelf},
		{"", technology.RangeSelf},
		{"30 feet", technology.RangeRanged},
		{"120 feet", technology.RangeRanged},
	}
	for _, tc := range cases {
		spell := makeDamageSpell([]string{"arcane"}, "2", tc.input, "", "instantaneous", "")
		results, _, err := pf2e.ConvertSpell(spell)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, tc.want, results[0].Def.Range, "input=%q", tc.input)
	}
}

func TestConvertSpell_ZoneRange_FromArea(t *testing.T) {
	spell := loadFixture(t, "fireball.json")
	results, _, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	assert.Equal(t, technology.RangeZone, results[0].Def.Range)
	assert.Equal(t, technology.TargetsZone, results[0].Def.Targets)
}

func TestConvertSpell_DurationMapping(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"instant", "instant"},
		{"instantaneous", "instant"},
		{"", "instant"},
		{"1 round", "rounds:1"},
		{"sustained", "rounds:1"},
		{"3 rounds", "rounds:3"},
		{"1 minute", "minutes:1"},
	}
	for _, tc := range cases {
		spell := makeDamageSpell([]string{"arcane"}, "2", "30 feet", "", tc.input, "")
		results, _, err := pf2e.ConvertSpell(spell)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, tc.want, results[0].Def.Duration, "input=%q", tc.input)
	}
}

func TestConvertSpell_SaveTypeMapping(t *testing.T) {
	cases := []struct {
		saveValue string
		wantSave  string
	}{
		{"Fortitude", "toughness"},
		{"Reflex", "hustle"},
		{"Will", "cool"},
	}
	for _, tc := range cases {
		spell := makeDamageSpell([]string{"arcane"}, "2", "30 feet", "", "instantaneous", tc.saveValue)
		results, _, err := pf2e.ConvertSpell(spell)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "save", results[0].Def.Resolution)
		assert.Equal(t, tc.wantSave, results[0].Def.SaveType)
		assert.Equal(t, 15, results[0].Def.SaveDC)
	}
}

func TestConvertSpell_AttackTrait_ResolutionAttack(t *testing.T) {
	spell := makeSpellWithTraits([]string{"arcane"}, []string{"attack"},
		"2", "30 feet", "", "instantaneous", "",
		map[string]pf2e.SpellDamageEntry{"0": {Value: "2d6", Type: pf2e.SpellDamageType{Value: "fire"}}})
	results, _, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "attack", results[0].Def.Resolution)
	require.NotEmpty(t, results[0].Def.Effects.OnHit)
	require.NotEmpty(t, results[0].Def.Effects.OnCritHit)
}

func TestConvertSpell_SaveBased_HalfStepDiceOnSuccess(t *testing.T) {
	spell := makeDamageSpell([]string{"arcane"}, "2", "30 feet", "", "instantaneous", "Will")
	results, _, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.Len(t, results, 1)
	def := results[0].Def
	require.NotEmpty(t, def.Effects.OnSuccess)
	assert.Equal(t, "1d3", def.Effects.OnSuccess[0].Dice)
}

func TestConvertSpell_SaveBased_DoubleDiceOnCritFailure(t *testing.T) {
	spell := makeDamageSpell([]string{"arcane"}, "2", "30 feet", "", "instantaneous", "Will")
	results, _, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.Len(t, results, 1)
	def := results[0].Def
	require.NotEmpty(t, def.Effects.OnCritFailure)
	assert.Equal(t, "2d6", def.Effects.OnCritFailure[0].Dice)
}

func TestConvertSpell_NoEffects_UtilityFallback(t *testing.T) {
	spell := loadFixture(t, "mindlink.json")
	results, _, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.Len(t, results, 1)
	def := results[0].Def
	require.NotEmpty(t, def.Effects.OnApply)
	assert.Equal(t, technology.EffectUtility, def.Effects.OnApply[0].Type)
	assert.NotEmpty(t, def.Effects.OnApply[0].Description)
}

func TestConvertSpell_ConditionFromTraits(t *testing.T) {
	spell := makeSpellWithTraits([]string{"arcane"}, []string{"mental", "frightened"},
		"2", "30 feet", "", "rounds:1", "", nil)
	results, _, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.Len(t, results, 1)
	var found bool
	for _, e := range results[0].Def.Effects.OnApply {
		if e.Type == technology.EffectCondition && e.ConditionID == "frightened" {
			found = true
		}
	}
	assert.True(t, found, "expected frightened condition effect")
}

func TestConvertSpell_SaveSpell_ConditionEffect(t *testing.T) {
	spell := loadFixture(t, "save_spell.json")
	results, _, err := pf2e.ConvertSpell(spell)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	def := results[0].Def
	var found bool
	for _, e := range def.Effects.OnFailure {
		if e.Type == technology.EffectCondition && e.ConditionID == "frightened" {
			found = true
		}
	}
	assert.True(t, found)
}

func TestPropertyConvertSpell_NoPanic(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		spell := &pf2e.PF2ESpell{
			Name: rapid.StringMatching(`[a-zA-Z ]{1,30}`).Draw(rt, "name"),
			System: pf2e.SpellSystem{
				Description: pf2e.SpellDescription{Value: rapid.String().Draw(rt, "desc")},
				Level:       pf2e.SpellLevel{Value: rapid.IntRange(1, 10).Draw(rt, "level")},
				Traits: pf2e.SpellTraits{
					Value:      rapid.SliceOfN(rapid.SampledFrom([]string{"fire", "attack", "mental", "frightened", "slowed"}), 0, 5).Draw(rt, "traits"),
					Traditions: rapid.SliceOfN(rapid.SampledFrom([]string{"arcane", "divine", "occult", "primal", "psychic", ""}), 0, 4).Draw(rt, "traditions"),
				},
				Time:     pf2e.SpellTime{Value: rapid.SampledFrom([]string{"1", "2", "3", "reaction", "free", "weird"}).Draw(rt, "time")},
				Range:    pf2e.SpellRange{Value: rapid.SampledFrom([]string{"touch", "self", "30 feet", "60 feet", "20-foot burst", ""}).Draw(rt, "range")},
				Target:   pf2e.SpellTarget{Value: rapid.SampledFrom([]string{"1 creature", "all enemies", "all allies", ""}).Draw(rt, "target")},
				Duration: pf2e.SpellDuration{Value: rapid.SampledFrom([]string{"instantaneous", "instant", "1 round", "3 rounds", "1 minute", "sustained", "", "weird"}).Draw(rt, "duration")},
				Save:     pf2e.SpellSave{Value: rapid.SampledFrom([]string{"Fortitude", "Reflex", "Will", ""}).Draw(rt, "save")},
			},
		}
		_, _, _ = pf2e.ConvertSpell(spell)
	})
}

func TestPropertyConvertSpell_AllResultsValidate(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		trad := rapid.SampledFrom([]string{"arcane", "divine", "occult", "primal"}).Draw(rt, "tradition")
		save := rapid.SampledFrom([]string{"Fortitude", "Reflex", "Will", ""}).Draw(rt, "save")
		var dmg map[string]pf2e.SpellDamageEntry
		if save != "" {
			dmg = map[string]pf2e.SpellDamageEntry{
				"0": {Value: "1d6", Type: pf2e.SpellDamageType{Value: "fire"}},
			}
		}
		spell := &pf2e.PF2ESpell{
			Name: rapid.StringMatching(`[a-zA-Z]{1,20}`).Draw(rt, "name"),
			System: pf2e.SpellSystem{
				Description: pf2e.SpellDescription{Value: "test"},
				Level:       pf2e.SpellLevel{Value: rapid.IntRange(1, 10).Draw(rt, "level")},
				Traits:      pf2e.SpellTraits{Traditions: []string{trad}},
				Time:        pf2e.SpellTime{Value: "2"},
				Range:       pf2e.SpellRange{Value: "30 feet"},
				Target:      pf2e.SpellTarget{Value: "1 creature"},
				Duration:    pf2e.SpellDuration{Value: "instantaneous"},
				Save:        pf2e.SpellSave{Value: save},
				Damage:      dmg,
			},
		}
		results, _, err := pf2e.ConvertSpell(spell)
		require.NoError(rt, err)
		for _, r := range results {
			assert.NoError(rt, r.Def.Validate(), "def %q must pass Validate()", r.Def.ID)
		}
	})
}

func makeDamageSpell(traditions []string, timeVal, rangeVal, targetVal, durationVal, saveVal string) *pf2e.PF2ESpell {
	return makeSpellWithTraits(traditions, nil, timeVal, rangeVal, targetVal, durationVal, saveVal,
		map[string]pf2e.SpellDamageEntry{"0": {Value: "1d6", Type: pf2e.SpellDamageType{Value: "fire"}}})
}

func makeSpellWithTraits(traditions []string, traits []string, timeVal, rangeVal, targetVal, durationVal, saveVal string, damage map[string]pf2e.SpellDamageEntry) *pf2e.PF2ESpell {
	if damage == nil && saveVal == "" {
		damage = map[string]pf2e.SpellDamageEntry{
			"0": {Value: "1d6", Type: pf2e.SpellDamageType{Value: "fire"}},
		}
	}
	return &pf2e.PF2ESpell{
		Name: "Test Spell",
		System: pf2e.SpellSystem{
			Description: pf2e.SpellDescription{Value: "A test spell."},
			Level:       pf2e.SpellLevel{Value: 1},
			Traits:      pf2e.SpellTraits{Value: traits, Traditions: traditions},
			Time:        pf2e.SpellTime{Value: timeVal},
			Range:       pf2e.SpellRange{Value: rangeVal},
			Target:      pf2e.SpellTarget{Value: targetVal},
			Duration:    pf2e.SpellDuration{Value: durationVal},
			Save:        pf2e.SpellSave{Value: saveVal},
			Damage:      damage,
		},
	}
}
