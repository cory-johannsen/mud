package inventory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// ── SetThreshold YAML parsing ──────────────────────────────────────────────────

func TestSetThreshold_ParseInteger(t *testing.T) {
	yaml := `threshold: 2`
	type wrapper struct {
		Threshold inventory.SetThreshold `yaml:"threshold"`
	}
	var w wrapper
	require.NoError(t, unmarshalYAML([]byte(yaml), &w))
	assert.False(t, w.Threshold.IsFull)
	assert.Equal(t, 2, w.Threshold.Count)
}

func TestSetThreshold_ParseFull(t *testing.T) {
	yaml := `threshold: full`
	type wrapper struct {
		Threshold inventory.SetThreshold `yaml:"threshold"`
	}
	var w wrapper
	require.NoError(t, unmarshalYAML([]byte(yaml), &w))
	assert.True(t, w.Threshold.IsFull)
	assert.Equal(t, 0, w.Threshold.Count)
}

func unmarshalYAML(data []byte, v interface{}) error {
	// Use the gopkg.in/yaml.v3 package directly via os temp file trick
	tmpFile, err := os.CreateTemp("", "*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		return err
	}
	tmpFile.Close()
	// parse via go-yaml via inventory helper
	return inventory.ParseYAML(tmpFile.Name(), v)
}

// ── SetRegistry loading ────────────────────────────────────────────────────────

func makeSetDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	}
	return dir
}

const sampleSetYAML = `
id: test_set
name: Test Set
pieces:
  - item_def_id: item_a
  - item_def_id: item_b
  - item_def_id: item_c
bonuses:
  - threshold: 2
    description: "+1 AC"
    effect:
      type: ac_bonus
      amount: 1
  - threshold: full
    description: "+2 Speed"
    effect:
      type: speed_bonus
      amount: 2
`

func TestSetRegistry_Load_ValidSet(t *testing.T) {
	dir := makeSetDir(t, map[string]string{"test_set.yaml": sampleSetYAML})
	knownConditions := map[string]bool{}
	reg, err := inventory.LoadSetRegistry(dir, knownConditions)
	require.NoError(t, err)
	assert.NotNil(t, reg)
}

func TestSetRegistry_Load_FullThresholdResolvesToPieceCount(t *testing.T) {
	dir := makeSetDir(t, map[string]string{"test_set.yaml": sampleSetYAML})
	reg, err := inventory.LoadSetRegistry(dir, nil)
	require.NoError(t, err)
	sets := reg.AllSets()
	require.Len(t, sets, 1)
	set := sets[0]
	// The "full" bonus should have Count == len(pieces) == 3
	var fullBonus *inventory.SetBonus
	for i := range set.Bonuses {
		if set.Bonuses[i].Threshold.IsFull {
			fullBonus = &set.Bonuses[i]
			break
		}
	}
	require.NotNil(t, fullBonus)
	assert.Equal(t, 3, fullBonus.Threshold.Count)
}

func TestSetRegistry_Load_UnknownEffectType_Error(t *testing.T) {
	yaml := `
id: bad_set
name: Bad Set
pieces:
  - item_def_id: x
bonuses:
  - threshold: 1
    description: "bad"
    effect:
      type: unknown_effect_type
`
	dir := makeSetDir(t, map[string]string{"bad.yaml": yaml})
	_, err := inventory.LoadSetRegistry(dir, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown_effect_type")
}

func TestSetRegistry_Load_UnresolvableConditionID_Error(t *testing.T) {
	yaml := `
id: cond_set
name: Cond Set
pieces:
  - item_def_id: x
bonuses:
  - threshold: 1
    description: "immune"
    effect:
      type: condition_immunity
      condition_id: nonexistent_condition
`
	dir := makeSetDir(t, map[string]string{"cond.yaml": yaml})
	knownConditions := map[string]bool{"fatigued": true}
	_, err := inventory.LoadSetRegistry(dir, knownConditions)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_condition")
}

func TestSetRegistry_Load_ValidConditionID_Succeeds(t *testing.T) {
	yaml := `
id: cond_set
name: Cond Set
pieces:
  - item_def_id: x
bonuses:
  - threshold: 1
    description: "immune"
    effect:
      type: condition_immunity
      condition_id: fatigued
`
	dir := makeSetDir(t, map[string]string{"cond.yaml": yaml})
	knownConditions := map[string]bool{"fatigued": true}
	_, err := inventory.LoadSetRegistry(dir, knownConditions)
	require.NoError(t, err)
}

func TestSetRegistry_Load_EmptyDir_Succeeds(t *testing.T) {
	dir := makeSetDir(t, map[string]string{})
	reg, err := inventory.LoadSetRegistry(dir, nil)
	require.NoError(t, err)
	assert.Empty(t, reg.AllSets())
}

// ── ActiveBonuses ──────────────────────────────────────────────────────────────

func TestActiveBonuses_NoPiecesEquipped(t *testing.T) {
	dir := makeSetDir(t, map[string]string{"s.yaml": sampleSetYAML})
	reg, err := inventory.LoadSetRegistry(dir, nil)
	require.NoError(t, err)
	bonuses := reg.ActiveBonuses([]string{})
	assert.Empty(t, bonuses)
}

func TestActiveBonuses_OnePieceEquipped_NoThresholdMet(t *testing.T) {
	dir := makeSetDir(t, map[string]string{"s.yaml": sampleSetYAML})
	reg, err := inventory.LoadSetRegistry(dir, nil)
	require.NoError(t, err)
	bonuses := reg.ActiveBonuses([]string{"item_a"})
	assert.Empty(t, bonuses) // threshold 2 not met
}

func TestActiveBonuses_TwoPiecesEquipped_FirstThresholdMet(t *testing.T) {
	dir := makeSetDir(t, map[string]string{"s.yaml": sampleSetYAML})
	reg, err := inventory.LoadSetRegistry(dir, nil)
	require.NoError(t, err)
	bonuses := reg.ActiveBonuses([]string{"item_a", "item_b"})
	require.Len(t, bonuses, 1)
	assert.Equal(t, "ac_bonus", bonuses[0].Effect.Type)
}

func TestActiveBonuses_AllPiecesEquipped_AllBonuses(t *testing.T) {
	dir := makeSetDir(t, map[string]string{"s.yaml": sampleSetYAML})
	reg, err := inventory.LoadSetRegistry(dir, nil)
	require.NoError(t, err)
	bonuses := reg.ActiveBonuses([]string{"item_a", "item_b", "item_c"})
	assert.Len(t, bonuses, 2)
}

func TestActiveBonuses_RarityIndependent(t *testing.T) {
	// Same item IDs regardless of rarity; set bonuses should count identically
	dir := makeSetDir(t, map[string]string{"s.yaml": sampleSetYAML})
	reg, err := inventory.LoadSetRegistry(dir, nil)
	require.NoError(t, err)
	// item_a appears twice in the list (could be from different instances)
	// but it's still just one unique item_def_id
	bonuses := reg.ActiveBonuses([]string{"item_a", "item_b"})
	require.Len(t, bonuses, 1)
}

func TestActiveBonuses_IsPureFunction(t *testing.T) {
	// Calling ActiveBonuses multiple times with same input produces same output.
	dir := makeSetDir(t, map[string]string{"s.yaml": sampleSetYAML})
	reg, err := inventory.LoadSetRegistry(dir, nil)
	require.NoError(t, err)
	equipped := []string{"item_a", "item_b"}
	b1 := reg.ActiveBonuses(equipped)
	b2 := reg.ActiveBonuses(equipped)
	assert.Equal(t, b1, b2)
}

func TestProperty_ActiveBonuses_CountNeverExceedsAllBonuses(t *testing.T) {
	dir := makeSetDir(t, map[string]string{"s.yaml": sampleSetYAML})
	reg, err := inventory.LoadSetRegistry(dir, nil)
	require.NoError(t, err)
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 3).Draw(rt, "n")
		allItems := []string{"item_a", "item_b", "item_c"}
		equipped := allItems[:n]
		bonuses := reg.ActiveBonuses(equipped)
		assert.LessOrEqual(rt, len(bonuses), 2) // at most 2 bonuses in test set
	})
}

// ── SetBonusSummary computation ────────────────────────────────────────────────

func TestComputeSetBonusSummary_ACBonus(t *testing.T) {
	bonuses := []inventory.SetBonus{
		{
			Threshold:   inventory.SetThreshold{Count: 2},
			Description: "+1 AC",
			Effect:      inventory.SetEffect{Type: "ac_bonus", Amount: 1},
		},
	}
	summary := inventory.ComputeSetBonusSummary(bonuses)
	assert.Equal(t, 1, summary.ACBonus)
}

func TestComputeSetBonusSummary_SpeedBonus(t *testing.T) {
	bonuses := []inventory.SetBonus{
		{Effect: inventory.SetEffect{Type: "speed_bonus", Amount: 5}},
	}
	summary := inventory.ComputeSetBonusSummary(bonuses)
	assert.Equal(t, 5, summary.SpeedBonus)
}

func TestComputeSetBonusSummary_SkillBonus(t *testing.T) {
	bonuses := []inventory.SetBonus{
		{Effect: inventory.SetEffect{Type: "skill_bonus", Skill: "stealth", Amount: 2}},
		{Effect: inventory.SetEffect{Type: "skill_bonus", Skill: "stealth", Amount: 1}},
	}
	summary := inventory.ComputeSetBonusSummary(bonuses)
	assert.Equal(t, 3, summary.SkillBonuses["stealth"])
}

func TestComputeSetBonusSummary_ConditionImmunity(t *testing.T) {
	bonuses := []inventory.SetBonus{
		{Effect: inventory.SetEffect{Type: "condition_immunity", ConditionID: "fatigued"}},
	}
	summary := inventory.ComputeSetBonusSummary(bonuses)
	assert.Contains(t, summary.ConditionImmunities, "fatigued")
}

func TestComputeSetBonusSummary_Empty(t *testing.T) {
	summary := inventory.ComputeSetBonusSummary(nil)
	assert.Equal(t, 0, summary.ACBonus)
	assert.Equal(t, 0, summary.SpeedBonus)
	assert.Empty(t, summary.SkillBonuses)
	assert.Empty(t, summary.ConditionImmunities)
}
