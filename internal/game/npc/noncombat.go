// Package npc — non-combat NPC type-specific config structs and runtime state.
package npc

import (
	"fmt"
	"time"
)

// ---- Merchant ----

// MerchantConfig holds the static configuration for a merchant NPC.
type MerchantConfig struct {
	MerchantType  string          `yaml:"merchant_type"` // weapons|armor|rings_neck|consumables|maps|technology|drugs
	Inventory     []MerchantItem  `yaml:"inventory"`
	SellMargin    float64         `yaml:"sell_margin"`
	BuyMargin     float64         `yaml:"buy_margin"`
	Budget        int             `yaml:"budget"`
	ReplenishRate ReplenishConfig `yaml:"replenish_rate"`
}

// MerchantItem is one entry in a merchant's static inventory.
type MerchantItem struct {
	ItemID    string `yaml:"item_id"`
	BasePrice int    `yaml:"base_price"`
	InitStock int    `yaml:"init_stock"`
	MaxStock  int    `yaml:"max_stock"`
}

// ReplenishConfig controls how often a merchant's stock and budget reset.
// REQ-NPC-13: 0 < MinHours <= MaxHours <= 24.
type ReplenishConfig struct {
	MinHours     int `yaml:"min_hours"`
	MaxHours     int `yaml:"max_hours"`
	StockRefill  int `yaml:"stock_refill"`
	BudgetRefill int `yaml:"budget_refill"`
}

// Validate checks REQ-NPC-13: 0 < MinHours <= MaxHours <= 24.
func (r ReplenishConfig) Validate() error {
	if r.MinHours <= 0 {
		return fmt.Errorf("replenish_rate: min_hours must be > 0, got %d", r.MinHours)
	}
	if r.MaxHours < r.MinHours {
		return fmt.Errorf("replenish_rate: max_hours (%d) must be >= min_hours (%d)", r.MaxHours, r.MinHours)
	}
	if r.MaxHours > 24 {
		return fmt.Errorf("replenish_rate: max_hours must be <= 24, got %d", r.MaxHours)
	}
	return nil
}

// MerchantRuntimeState holds the mutable runtime state of a merchant, persisted to DB.
type MerchantRuntimeState struct {
	Stock           map[string]int
	CurrentBudget   int
	NextReplenishAt time.Time
}

// ---- Guard ----

// GuardConfig holds the static configuration for a guard NPC.
type GuardConfig struct {
	WantedThreshold int    `yaml:"wanted_threshold"`
	PatrolRoom      string `yaml:"patrol_room,omitempty"`
}

// ---- Healer ----

// HealerConfig holds the static configuration for a healer NPC.
type HealerConfig struct {
	PricePerHP    int `yaml:"price_per_hp"`
	DailyCapacity int `yaml:"daily_capacity"`
}

// HealerRuntimeState holds the mutable runtime state of a healer, persisted to DB.
type HealerRuntimeState struct {
	CapacityUsed int
}

// ComputeHealCost returns the credit cost to restore a player from currentHP to maxHP.
//
// Precondition: cfg must not be nil; currentHP <= maxHP; both >= 0.
// Postcondition: Returns cfg.PricePerHP × (maxHP - currentHP).
func ComputeHealCost(cfg *HealerConfig, currentHP, maxHP int) int {
	return cfg.PricePerHP * (maxHP - currentHP)
}

// ComputeHealAmountCost returns the credit cost to restore exactly amount HP.
//
// Precondition: cfg must not be nil; amount >= 0.
// Postcondition: Returns cfg.PricePerHP × amount.
func ComputeHealAmountCost(cfg *HealerConfig, amount int) int {
	return cfg.PricePerHP * amount
}

// CheckHealPrerequisites validates whether a full-heal is allowed.
// Returns a descriptive error if the player is already at full health,
// capacity is exhausted, or the player cannot afford the cost.
//
// Precondition: cfg and state must not be nil; currentHP <= maxHP; credits >= 0.
// Postcondition: Returns nil iff heal is allowed.
func CheckHealPrerequisites(cfg *HealerConfig, state *HealerRuntimeState, currentHP, maxHP, credits int) error {
	if currentHP >= maxHP {
		return fmt.Errorf("you are already at full health")
	}
	remaining := cfg.DailyCapacity - state.CapacityUsed
	if remaining <= 0 {
		return fmt.Errorf("%s has exhausted their daily healing capacity", "the healer")
	}
	healAmount := maxHP - currentHP
	if healAmount > remaining {
		healAmount = remaining
	}
	cost := cfg.PricePerHP * healAmount
	if credits < cost {
		return fmt.Errorf("you need %d credits but only have %d", cost, credits)
	}
	return nil
}

// ApplyHeal computes the result of healing a player, capped at availableCapacity.
// Returns (newHP, creditCost, newCapacityUsed).
//
// Precondition: cfg and state must not be nil; currentHP <= maxHP; availableCapacity >= 0.
// Postcondition: newHP <= maxHP; creditCost = cfg.PricePerHP × healAmount;
// newCapacityUsed = state.CapacityUsed + healAmount.
func ApplyHeal(cfg *HealerConfig, state *HealerRuntimeState, currentHP, maxHP, availableCapacity int) (newHP, creditCost, newCapacityUsed int) {
	missing := maxHP - currentHP
	healAmount := missing
	if healAmount > availableCapacity {
		healAmount = availableCapacity
	}
	cost := cfg.PricePerHP * healAmount
	return currentHP + healAmount, cost, state.CapacityUsed + healAmount
}

// ---- Quest Giver ----

// QuestGiverConfig holds the static configuration for a quest giver NPC.
// REQ-NPC-18: PlaceholderDialog must contain at least one entry.
type QuestGiverConfig struct {
	PlaceholderDialog []string `yaml:"placeholder_dialog"`
	QuestIDs          []string `yaml:"quest_ids,omitempty"`
}

// Validate checks REQ-NPC-18: PlaceholderDialog must not be empty.
func (q QuestGiverConfig) Validate() error {
	if len(q.PlaceholderDialog) == 0 {
		return fmt.Errorf("quest_giver: placeholder_dialog must contain at least one entry")
	}
	return nil
}

// ---- Hireling ----

// HirelingConfig holds the static configuration for a hireling NPC.
type HirelingConfig struct {
	DailyCost      int    `yaml:"daily_cost"`
	CombatRole     string `yaml:"combat_role"`
	MaxFollowZones int    `yaml:"max_follow_zones"`
}

// HirelingRuntimeState holds the mutable runtime state of a hireling, persisted to DB.
type HirelingRuntimeState struct {
	HiredByPlayerID string
	ZonesFollowed   int
}

// ---- Banker ----

// BankerConfig holds the static configuration for a banker NPC.
type BankerConfig struct {
	ZoneID       string  `yaml:"zone_id"`
	BaseRate     float64 `yaml:"base_rate"`
	RateVariance float64 `yaml:"rate_variance"`
}

// ---- Job Trainer ----

// JobTrainerConfig holds the static configuration for a job trainer NPC.
type JobTrainerConfig struct {
	OfferedJobs []TrainableJob `yaml:"offered_jobs"`
}

// TrainableJob describes a job that this trainer can teach.
type TrainableJob struct {
	JobID         string           `yaml:"job_id"`
	TrainingCost  int              `yaml:"training_cost"`
	Prerequisites JobPrerequisites `yaml:"prerequisites"`
}

// JobPrerequisites enumerates all gatekeeping conditions for training a job.
type JobPrerequisites struct {
	MinLevel      int               `yaml:"min_level,omitempty"`
	MinJobLevel   map[string]int    `yaml:"min_job_level,omitempty"`
	MinAttributes map[string]int    `yaml:"min_attributes,omitempty"`
	MinSkillRanks map[string]string `yaml:"min_skill_ranks,omitempty"`
	RequiredJobs  []string          `yaml:"required_jobs,omitempty"`
}

// proficiencyRank maps a rank name to a comparable integer.
// Higher integer = higher rank. Unknown → 0.
func proficiencyRank(rank string) int {
	switch rank {
	case "trained":
		return 1
	case "expert":
		return 2
	case "master":
		return 3
	case "legendary":
		return 4
	default:
		return 0
	}
}

// Validate checks that all skill IDs referenced in MinSkillRanks exist in the
// provided skill registry. Returns an error naming the first unknown skill ID.
//
// REQ-NPC-2a: unknown skill IDs MUST be a fatal load error.
//
// Precondition: knownSkills may be nil (treated as empty set, all IDs unknown).
// Postcondition: Returns nil iff all referenced skill IDs are present in knownSkills.
func (c *JobTrainerConfig) Validate(knownSkills map[string]bool) error {
	for _, job := range c.OfferedJobs {
		for skillID := range job.Prerequisites.MinSkillRanks {
			if !knownSkills[skillID] {
				return fmt.Errorf("job_trainer: unknown skill ID %q in job %q prerequisites", skillID, job.JobID)
			}
		}
	}
	return nil
}

// CheckJobPrerequisites validates all prerequisites for training a job.
// Returns the first unmet prerequisite as a descriptive error, or nil.
//
// Precondition: job, playerJobs, playerAttrs, playerSkills must not be nil (pass empty maps, not nil).
// Postcondition: Returns nil iff all prerequisites are satisfied and the player does not already hold the job.
func CheckJobPrerequisites(job TrainableJob, playerLevel int, playerJobs map[string]int, playerAttrs map[string]int, playerSkills map[string]string) error {
	if _, has := playerJobs[job.JobID]; has {
		return fmt.Errorf("you have already trained %q", job.JobID)
	}
	p := job.Prerequisites
	if p.MinLevel > 0 && playerLevel < p.MinLevel {
		return fmt.Errorf("you must be at least level %d (you are level %d)", p.MinLevel, playerLevel)
	}
	for reqJob := range p.MinJobLevel {
		lvl, has := playerJobs[reqJob]
		if !has || lvl < p.MinJobLevel[reqJob] {
			return fmt.Errorf("you must have %q at level %d", reqJob, p.MinJobLevel[reqJob])
		}
	}
	for attr, minVal := range p.MinAttributes {
		if playerAttrs[attr] < minVal {
			return fmt.Errorf("you need %s %d (you have %d)", attr, minVal, playerAttrs[attr])
		}
	}
	for skillID, minRank := range p.MinSkillRanks {
		if proficiencyRank(playerSkills[skillID]) < proficiencyRank(minRank) {
			return fmt.Errorf("you need skill %q at rank %q (you have %q)", skillID, minRank, playerSkills[skillID])
		}
	}
	for _, reqJob := range p.RequiredJobs {
		if _, has := playerJobs[reqJob]; !has {
			return fmt.Errorf("you must have trained %q first", reqJob)
		}
	}
	return nil
}

// ---- Crafter (stub) ----

// CrafterConfig is intentionally empty until the crafting feature spec is written.
// A YAML block `crafter: {}` MUST be present for npc_type: crafter (REQ-NPC-2).
type CrafterConfig struct{}
