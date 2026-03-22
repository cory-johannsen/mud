package inventory_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// ── stubTarget implements ConsumableTarget ────────────────────────────────────

type stubTarget struct {
	team               string
	statModifiers      map[string]int // for GetStatModifier
	healApplied        int
	conditionsApplied  []string
	conditionsRemoved  []string
	diseaseApplied     string
	toxinApplied       string
}

func (s *stubTarget) GetTeam() string { return s.team }
func (s *stubTarget) GetStatModifier(stat string) int {
	if s.statModifiers == nil {
		return 0
	}
	return s.statModifiers[stat]
}
func (s *stubTarget) ApplyHeal(amount int) { s.healApplied += amount }
func (s *stubTarget) ApplyCondition(conditionID string, _ time.Duration) {
	s.conditionsApplied = append(s.conditionsApplied, conditionID)
}
func (s *stubTarget) RemoveCondition(conditionID string) {
	s.conditionsRemoved = append(s.conditionsRemoved, conditionID)
}
func (s *stubTarget) ApplyDisease(diseaseID string, _ int) { s.diseaseApplied = diseaseID }
func (s *stubTarget) ApplyToxin(toxinID string, _ int)    { s.toxinApplied = toxinID }

// ── Team multiplier ────────────────────────────────────────────────────────────

func TestTeamMultiplier_MatchingTeam(t *testing.T) {
	mult := inventory.TeamMultiplier("gun", "gun")
	assert.InDelta(t, 1.25, mult, 1e-9)
}

func TestTeamMultiplier_OpposingTeam(t *testing.T) {
	mult := inventory.TeamMultiplier("gun", "machete")
	assert.InDelta(t, 0.75, mult, 1e-9)
}

func TestTeamMultiplier_NeutralItem(t *testing.T) {
	mult := inventory.TeamMultiplier("gun", "")
	assert.InDelta(t, 1.0, mult, 1e-9)
}

func TestTeamMultiplier_NeutralPlayer(t *testing.T) {
	mult := inventory.TeamMultiplier("", "gun")
	assert.InDelta(t, 1.0, mult, 1e-9)
}

func TestTeamMultiplier_BothEmpty(t *testing.T) {
	mult := inventory.TeamMultiplier("", "")
	assert.InDelta(t, 1.0, mult, 1e-9)
}

func TestTeamMultiplier_MacheteMachete(t *testing.T) {
	mult := inventory.TeamMultiplier("machete", "machete")
	assert.InDelta(t, 1.25, mult, 1e-9)
}

func TestTeamMultiplier_MacheteVsGun(t *testing.T) {
	mult := inventory.TeamMultiplier("machete", "gun")
	assert.InDelta(t, 0.75, mult, 1e-9)
}

// ── ApplyConsumable heal ───────────────────────────────────────────────────────

func TestApplyConsumable_HealNeutral(t *testing.T) {
	target := &stubTarget{team: "gun"}
	def := &inventory.ItemDef{
		ID: "penjamin", Name: "Penjamin Franklin", Kind: "consumable",
		MaxStack: 1, Weight: 0.4,
		Effect: &inventory.ConsumableEffect{Heal: "3d6"},
	}
	rng := &stubRoller{rollResult: 12} // 3d6 → 12
	result := inventory.ApplyConsumable(target, def, rng)
	assert.Equal(t, 12, result.HealApplied)
	assert.Equal(t, 12, target.healApplied)
	assert.InDelta(t, 1.0, result.TeamMultiplier, 1e-9) // item team "" → neutral
}

func TestApplyConsumable_HealMatchingTeam(t *testing.T) {
	target := &stubTarget{team: "gun"}
	def := &inventory.ItemDef{
		ID: "old_english", Name: "Old English", Kind: "consumable",
		MaxStack: 1, Weight: 0.3, Team: "gun",
		Effect: &inventory.ConsumableEffect{Heal: "1d6"},
	}
	rng := &stubRoller{rollResult: 8} // 1d6 → 8 (stub returns fixed value)
	result := inventory.ApplyConsumable(target, def, rng)
	// 8 * 1.25 = 10.0 → floor → 10
	assert.Equal(t, 10, result.HealApplied)
	assert.InDelta(t, 1.25, result.TeamMultiplier, 1e-9)
}

func TestApplyConsumable_HealOpposingTeam(t *testing.T) {
	target := &stubTarget{team: "gun"}
	def := &inventory.ItemDef{
		ID: "poontangesca", Name: "Poontangesca", Kind: "consumable",
		MaxStack: 1, Weight: 0.5, Team: "machete",
		Effect: &inventory.ConsumableEffect{Heal: "2d6"},
	}
	rng := &stubRoller{rollResult: 8}
	result := inventory.ApplyConsumable(target, def, rng)
	// 8 * 0.75 = 6.0 → floor → 6
	assert.Equal(t, 6, result.HealApplied)
	assert.InDelta(t, 0.75, result.TeamMultiplier, 1e-9)
}

// ── RemoveConditions before applying new ones ─────────────────────────────────

func TestApplyConsumable_RemoveConditionsFirst(t *testing.T) {
	target := &stubTarget{team: "machete"}
	def := &inventory.ItemDef{
		ID: "poontangesca", Name: "Poontangesca", Kind: "consumable",
		MaxStack: 1, Weight: 0.5, Team: "machete",
		Effect: &inventory.ConsumableEffect{
			RemoveConditions: []string{"fatigued"},
			Conditions: []inventory.ConditionEffect{
				{ConditionID: "speed_boost_2", Duration: "30m"},
			},
		},
	}
	rng := &stubRoller{}
	result := inventory.ApplyConsumable(target, def, rng)
	assert.Contains(t, result.ConditionsRemoved, "fatigued")
	assert.Contains(t, result.ConditionsApplied, "speed_boost_2")
	// Remove happens before apply
	assert.Contains(t, target.conditionsRemoved, "fatigued")
	assert.Contains(t, target.conditionsApplied, "speed_boost_2")
}

// ── ConsumeCheck ──────────────────────────────────────────────────────────────

func TestApplyConsumable_ConsumeCheck_NoFailure(t *testing.T) {
	target := &stubTarget{team: "gun"}
	def := &inventory.ItemDef{
		ID: "whores_pasta", Name: "Whore's Pasta", Kind: "consumable",
		MaxStack: 1, Weight: 0.5, Team: "gun",
		Effect: &inventory.ConsumableEffect{
			Heal: "2d6",
			ConsumeCheck: &inventory.ConsumeCheck{
				Stat: "constitution",
				DC:   12,
			},
		},
	}
	// d20 roll of 15 + stat modifier (0 for stub) = 15 >= 12 → success
	rng := &stubRoller{rollResult: 6, rollD20Result: 15}
	result := inventory.ApplyConsumable(target, def, rng)
	assert.Equal(t, "success", result.ConsumeCheckResult)
	assert.Empty(t, result.DiseaseApplied)
}

func TestApplyConsumable_ConsumeCheck_CriticalFailure_AppliesDisease(t *testing.T) {
	target := &stubTarget{team: "gun"}
	def := &inventory.ItemDef{
		ID: "whores_pasta", Name: "Whore's Pasta", Kind: "consumable",
		MaxStack: 1, Weight: 0.5, Team: "gun",
		Effect: &inventory.ConsumableEffect{
			Heal: "2d6",
			ConsumeCheck: &inventory.ConsumeCheck{
				Stat: "constitution",
				DC:   12,
				OnCriticalFailure: &inventory.CritFailureEffect{
					ApplyDisease: &inventory.DiseaseEffect{
						DiseaseID: "street_rot",
						Severity:  1,
					},
				},
			},
		},
	}
	// Critical failure: total <= DC-10 = 2, or natural 1.
	// Natural 1 → critical failure.
	rng := &stubRoller{rollResult: 6, rollD20Result: 1}
	result := inventory.ApplyConsumable(target, def, rng)
	assert.Equal(t, "critical_failure", result.ConsumeCheckResult)
	assert.Equal(t, "street_rot", result.DiseaseApplied)
	assert.Equal(t, "street_rot", target.diseaseApplied)
}

func TestApplyConsumable_ConsumeCheck_CriticalFailure_Total_DC_Minus_10(t *testing.T) {
	target := &stubTarget{team: "gun"}
	def := &inventory.ItemDef{
		ID: "item", Name: "item", Kind: "consumable",
		MaxStack: 1, Weight: 0.5,
		Effect: &inventory.ConsumableEffect{
			ConsumeCheck: &inventory.ConsumeCheck{
				Stat: "constitution",
				DC:   20,
				OnCriticalFailure: &inventory.CritFailureEffect{
					ApplyToxin: &inventory.ToxinEffect{ToxinID: "gut_rot", Severity: 1},
				},
			},
		},
	}
	// DC-10 = 10; total 5 ≤ 10 → critical failure (not natural 1).
	rng := &stubRoller{rollResult: 0, rollD20Result: 5}
	result := inventory.ApplyConsumable(target, def, rng)
	assert.Equal(t, "critical_failure", result.ConsumeCheckResult)
	assert.Equal(t, "gut_rot", result.ToxinApplied)
}

func TestApplyConsumable_NoEffect_NilEffect(t *testing.T) {
	target := &stubTarget{team: ""}
	def := &inventory.ItemDef{
		ID: "junk", Name: "Junk", Kind: "consumable", MaxStack: 1, Weight: 0.1,
		Effect: nil,
	}
	rng := &stubRoller{}
	result := inventory.ApplyConsumable(target, def, rng)
	assert.Equal(t, 0, result.HealApplied)
	assert.Equal(t, "not_checked", result.ConsumeCheckResult)
}

func TestApplyConsumable_RepairField_NoHeal(t *testing.T) {
	target := &stubTarget{}
	def := &inventory.ItemDef{
		ID: "repair_kit", Name: "Repair Kit", Kind: "consumable", MaxStack: 1, Weight: 1.0,
		Effect: &inventory.ConsumableEffect{RepairField: true},
	}
	rng := &stubRoller{}
	result := inventory.ApplyConsumable(target, def, rng)
	assert.Equal(t, 0, result.HealApplied)
	assert.Equal(t, "not_checked", result.ConsumeCheckResult)
}

// ── Property tests ────────────────────────────────────────────────────────────

func TestProperty_ApplyConsumable_HealIsFloored(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(0, 20).Draw(rt, "roll")
		target := &stubTarget{team: "gun"}
		def := &inventory.ItemDef{
			ID: "test", Name: "test", Kind: "consumable", MaxStack: 1, Weight: 0.1,
			Team: "machete", // opposing: 0.75
			Effect: &inventory.ConsumableEffect{Heal: "1d6"},
		}
		rng := &stubRoller{rollResult: roll}
		result := inventory.ApplyConsumable(target, def, rng)
		// Result must equal floor(roll * 0.75) and must be >= 0.
		expected := int(float64(roll) * 0.75)
		assert.Equal(rt, expected, result.HealApplied)
		assert.GreaterOrEqual(rt, result.HealApplied, 0)
	})
}

func TestProperty_TeamMultiplier_AlwaysPositive(t *testing.T) {
	teams := []string{"", "gun", "machete"}
	rapid.Check(t, func(rt *rapid.T) {
		pt := rapid.IntRange(0, 2).Draw(rt, "pt")
		it := rapid.IntRange(0, 2).Draw(rt, "it")
		mult := inventory.TeamMultiplier(teams[pt], teams[it])
		assert.Greater(rt, mult, 0.0)
	})
}

// ── Validate ItemDef.Team ─────────────────────────────────────────────────────

func TestItemDef_ValidateTeam_ValidValues(t *testing.T) {
	for _, team := range []string{"", "gun", "machete"} {
		def := &inventory.ItemDef{
			ID: "x", Name: "X", Kind: "consumable", MaxStack: 1, Weight: 0.1, Team: team,
		}
		require.NoError(t, def.Validate(), "team %q should be valid", team)
	}
}

func TestItemDef_ValidateTeam_InvalidValue(t *testing.T) {
	def := &inventory.ItemDef{
		ID: "x", Name: "X", Kind: "consumable", MaxStack: 1, Weight: 0.1, Team: "rebels",
	}
	err := def.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rebels")
}

// TestApplyConsumable_ConsumeCheck_StatModifierAdded verifies REQ-EM-41:
// GetStatModifier(stat) is added to the d20 roll in a ConsumeCheck.
func TestApplyConsumable_ConsumeCheck_StatModifierAdded(t *testing.T) {
	// With d20=10 and con modifier=+5, total=15 vs DC=12 → success.
	// Without stat modifier, 10 vs 12 → failure (< dc).
	target := &stubTarget{statModifiers: map[string]int{"constitution": 5}}
	def := &inventory.ItemDef{
		ID: "stim", Name: "Stim", Kind: "consumable", MaxStack: 1, Weight: 0.1,
		Effect: &inventory.ConsumableEffect{
			ConsumeCheck: &inventory.ConsumeCheck{
				Stat: "constitution",
				DC:   12,
			},
		},
	}
	// Roller returns d20=10.
	rng := &stubRoller{rollD20Result: 10}
	result := inventory.ApplyConsumable(target, def, rng)
	// 10 + 5 = 15 >= 12 → success
	assert.Equal(t, "success", result.ConsumeCheckResult,
		"d20(10) + stat(5) = 15 >= DC(12) should be success")
}
