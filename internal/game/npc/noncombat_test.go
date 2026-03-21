package npc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestReplenishConfig_Valid(t *testing.T) {
	c := ReplenishConfig{MinHours: 2, MaxHours: 6, StockRefill: 1, BudgetRefill: 100}
	assert.NoError(t, c.Validate(), "valid ReplenishConfig must not error")
}

func TestReplenishConfig_MinZero(t *testing.T) {
	c := ReplenishConfig{MinHours: 0, MaxHours: 6}
	assert.Error(t, c.Validate(), "MinHours == 0 must error")
}

func TestReplenishConfig_MinGtMax(t *testing.T) {
	c := ReplenishConfig{MinHours: 8, MaxHours: 4}
	assert.Error(t, c.Validate(), "MinHours > MaxHours must error")
}

func TestReplenishConfig_MaxGt24(t *testing.T) {
	c := ReplenishConfig{MinHours: 1, MaxHours: 25}
	assert.Error(t, c.Validate(), "MaxHours > 24 must error")
}

func TestQuestGiverConfig_EmptyDialog(t *testing.T) {
	c := QuestGiverConfig{PlaceholderDialog: nil}
	assert.Error(t, c.Validate(), "empty PlaceholderDialog must error")
}

func TestQuestGiverConfig_NonEmptyDialog(t *testing.T) {
	c := QuestGiverConfig{PlaceholderDialog: []string{"Hello, stranger."}}
	assert.NoError(t, c.Validate(), "non-empty PlaceholderDialog must not error")
}

func TestProperty_ReplenishConfig_ValidRangeNeverErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		min := rapid.IntRange(1, 24).Draw(rt, "min")
		max := rapid.IntRange(min, 24).Draw(rt, "max")
		c := ReplenishConfig{MinHours: min, MaxHours: max}
		if err := c.Validate(); err != nil {
			rt.Fatalf("valid ReplenishConfig{min:%d, max:%d} must not error: %v", min, max, err)
		}
	})
}

func TestProperty_ReplenishConfig_InvalidRangeAlwaysErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		min := rapid.IntRange(-10, 0).Draw(rt, "min_le_zero")
		max := rapid.IntRange(1, 24).Draw(rt, "max")
		c := ReplenishConfig{MinHours: min, MaxHours: max}
		if err := c.Validate(); err == nil {
			rt.Fatalf("invalid ReplenishConfig{min:%d, max:%d} must error", min, max)
		}
	})
}

// TestComputeHealCost_FullHeal checks cost = PricePerHP × (MaxHP - CurrentHP).
func TestComputeHealCost_FullHeal(t *testing.T) {
	cfg := &HealerConfig{PricePerHP: 5, DailyCapacity: 100}
	cost := ComputeHealCost(cfg, 60, 100) // missing 40 HP
	assert.Equal(t, 200, cost)
}

// TestComputeHealCost_PartialHeal checks cost = PricePerHP × amount.
func TestComputeHealCost_PartialHeal(t *testing.T) {
	cfg := &HealerConfig{PricePerHP: 3, DailyCapacity: 100}
	cost := ComputeHealCost(cfg, 0, 10) // not used; amount-based
	_ = cost
	partialCost := ComputeHealAmountCost(cfg, 15)
	assert.Equal(t, 45, partialCost)
}

// TestCheckHealPrerequisites_InsufficientCredits verifies error when credits < cost.
func TestCheckHealPrerequisites_InsufficientCredits(t *testing.T) {
	cfg := &HealerConfig{PricePerHP: 10, DailyCapacity: 50}
	state := &HealerRuntimeState{CapacityUsed: 0}
	err := CheckHealPrerequisites(cfg, state, 50 /*current*/, 100 /*max*/, 400 /*credits*/)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credits")
}

// TestCheckHealPrerequisites_CapacityExhausted verifies error when capacity is full.
func TestCheckHealPrerequisites_CapacityExhausted(t *testing.T) {
	cfg := &HealerConfig{PricePerHP: 1, DailyCapacity: 10}
	state := &HealerRuntimeState{CapacityUsed: 10}
	err := CheckHealPrerequisites(cfg, state, 80, 100, 9999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "capacity")
}

// TestCheckHealPrerequisites_AlreadyFullHP verifies error when player is at full HP.
func TestCheckHealPrerequisites_AlreadyFullHP(t *testing.T) {
	cfg := &HealerConfig{PricePerHP: 5, DailyCapacity: 100}
	state := &HealerRuntimeState{CapacityUsed: 0}
	err := CheckHealPrerequisites(cfg, state, 100, 100, 9999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "full health")
}

// TestApplyHeal_FullHeal verifies HP restored to MaxHP.
func TestApplyHeal_FullHeal(t *testing.T) {
	cfg := &HealerConfig{PricePerHP: 5, DailyCapacity: 100}
	state := &HealerRuntimeState{CapacityUsed: 0}
	newHP, cost, newUsed := ApplyHeal(cfg, state, 60, 100, 100 /*available capacity remains*/)
	assert.Equal(t, 100, newHP)
	assert.Equal(t, 200, cost)
	assert.Equal(t, 40, newUsed)
}

// TestApplyHeal_CapacityLimited verifies heal is capped at remaining capacity.
func TestApplyHeal_CapacityLimited(t *testing.T) {
	cfg := &HealerConfig{PricePerHP: 2, DailyCapacity: 50}
	state := &HealerRuntimeState{CapacityUsed: 45}
	// remaining capacity = 5; player missing = 40 HP
	newHP, cost, newUsed := ApplyHeal(cfg, state, 60, 100, 5)
	assert.Equal(t, 65, newHP)  // 60 + 5
	assert.Equal(t, 10, cost)   // 2 × 5
	assert.Equal(t, 50, newUsed)
}

// TestProperty_ComputeHealCost_NeverNegative property test.
func TestProperty_ComputeHealCost_NeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		pricePerHP := rapid.IntRange(1, 100).Draw(rt, "price")
		current := rapid.IntRange(0, 1000).Draw(rt, "current")
		max := rapid.IntRange(current, 1000).Draw(rt, "max")
		cfg := &HealerConfig{PricePerHP: pricePerHP, DailyCapacity: 9999}
		cost := ComputeHealCost(cfg, current, max)
		if cost < 0 {
			rt.Fatalf("ComputeHealCost must be >= 0, got %d", cost)
		}
	})
}

// TestJobTrainerConfig_Validate_UnknownSkill verifies unknown skill ID is a fatal error.
func TestJobTrainerConfig_Validate_UnknownSkill(t *testing.T) {
	cfg := &JobTrainerConfig{
		OfferedJobs: []TrainableJob{
			{
				JobID: "scavenger", TrainingCost: 100,
				Prerequisites: JobPrerequisites{
					MinSkillRanks: map[string]string{"ghost_skill_xyz": "trained"},
				},
			},
		},
	}
	knownSkills := map[string]bool{"smooth_talk": true, "hustle": true}
	err := cfg.Validate(knownSkills)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ghost_skill_xyz")
}

// TestJobTrainerConfig_Validate_ValidSkill verifies known skill passes.
func TestJobTrainerConfig_Validate_ValidSkill(t *testing.T) {
	cfg := &JobTrainerConfig{
		OfferedJobs: []TrainableJob{
			{
				JobID: "scavenger", TrainingCost: 100,
				Prerequisites: JobPrerequisites{
					MinSkillRanks: map[string]string{"smooth_talk": "trained"},
				},
			},
		},
	}
	knownSkills := map[string]bool{"smooth_talk": true}
	err := cfg.Validate(knownSkills)
	assert.NoError(t, err)
}

// TestJobTrainerConfig_Validate_EmptyOfferedJobs allows empty job list.
func TestJobTrainerConfig_Validate_EmptyOfferedJobs(t *testing.T) {
	cfg := &JobTrainerConfig{OfferedJobs: nil}
	err := cfg.Validate(map[string]bool{})
	assert.NoError(t, err)
}

// TestCheckJobPrerequisites_MinLevel verifies level gate.
func TestCheckJobPrerequisites_MinLevel(t *testing.T) {
	job := TrainableJob{
		JobID: "infiltrator", TrainingCost: 200,
		Prerequisites: JobPrerequisites{MinLevel: 5},
	}
	playerLevel := 3
	playerJobs := map[string]int{}
	playerAttrs := map[string]int{}
	playerSkills := map[string]string{}
	err := CheckJobPrerequisites(job, playerLevel, playerJobs, playerAttrs, playerSkills)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "level 5")
}

// TestCheckJobPrerequisites_AlreadyHasJob verifies duplicate job error.
func TestCheckJobPrerequisites_AlreadyHasJob(t *testing.T) {
	job := TrainableJob{JobID: "scavenger", TrainingCost: 100}
	playerJobs := map[string]int{"scavenger": 2}
	err := CheckJobPrerequisites(job, 1, playerJobs, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already trained")
}

// TestCheckJobPrerequisites_RequiredJobMissing verifies required job gate.
func TestCheckJobPrerequisites_RequiredJobMissing(t *testing.T) {
	job := TrainableJob{
		JobID: "veteran", TrainingCost: 300,
		Prerequisites: JobPrerequisites{RequiredJobs: []string{"soldier"}},
	}
	err := CheckJobPrerequisites(job, 10, map[string]int{}, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "soldier")
}

// TestCheckJobPrerequisites_MinSkillRank verifies skill rank gate.
func TestCheckJobPrerequisites_MinSkillRank(t *testing.T) {
	job := TrainableJob{
		JobID: "infiltrator", TrainingCost: 150,
		Prerequisites: JobPrerequisites{
			MinSkillRanks: map[string]string{"sneak": "expert"},
		},
	}
	err := CheckJobPrerequisites(job, 5, map[string]int{}, nil, map[string]string{"sneak": "trained"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sneak")
}

// TestCheckJobPrerequisites_AllMet verifies no error when all prerequisites are met.
func TestCheckJobPrerequisites_AllMet(t *testing.T) {
	job := TrainableJob{
		JobID: "infiltrator", TrainingCost: 150,
		Prerequisites: JobPrerequisites{
			MinLevel:      3,
			RequiredJobs:  []string{"scavenger"},
			MinSkillRanks: map[string]string{"sneak": "trained"},
		},
	}
	err := CheckJobPrerequisites(job, 5,
		map[string]int{"scavenger": 2},
		nil,
		map[string]string{"sneak": "expert"},
	)
	assert.NoError(t, err)
}
