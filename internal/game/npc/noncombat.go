// Package npc — non-combat NPC type-specific config structs and runtime state.
package npc

import (
	"fmt"
	"time"
)

// ---- Merchant ----

// MerchantConfig holds the static configuration for a merchant NPC.
type MerchantConfig struct {
	MerchantType  string                `yaml:"merchant_type"` // weapons|armor|rings_neck|consumables|maps|technology|drugs|black_market
	Inventory     []MerchantItem        `yaml:"inventory"`
	SellMargin    float64               `yaml:"sell_margin"`
	BuyMargin     float64               `yaml:"buy_margin"`
	Budget        int                   `yaml:"budget"`
	ReplenishRate ReplenishConfig       `yaml:"replenish_rate"`
	MaterialStock []MaterialStockItem   `yaml:"material_stock,omitempty"`
}

// MerchantItem is one entry in a merchant's static inventory.
type MerchantItem struct {
	ItemID    string `yaml:"item_id"`
	BasePrice int    `yaml:"base_price"`
	InitStock int    `yaml:"init_stock"`
	MaxStock  int    `yaml:"max_stock"`
	// Modifier is the optional pre-set item modifier: "" | "tuned" | "defective".
	// "cursed" is explicitly disallowed (REQ-EM-27).
	Modifier string `yaml:"modifier,omitempty"`
}

// MaterialStockItem is one entry in a merchant's static material stock.
type MaterialStockItem struct {
	ID              string `yaml:"id"`
	Price           int    `yaml:"price"`
	RestockQuantity int    `yaml:"restock_quantity"`
}

// Validate checks REQ-EM-27: merchants MUST NOT stock cursed items.
//
// Precondition: cfg must not be nil.
// Postcondition: Returns an error naming the first cursed item found.
func (cfg *MerchantConfig) Validate() error {
	for _, item := range cfg.Inventory {
		if item.Modifier == "cursed" {
			return fmt.Errorf("merchant config: item %q has modifier 'cursed'; merchants may not stock cursed items (REQ-EM-27)", item.ItemID)
		}
	}
	return nil
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
	// Bribeable indicates whether this guard can be bribed to ignore a Wanted level.
	// REQ-WC-2b: when Bribeable is true, MaxBribeWantedLevel MUST be in range 1-4.
	Bribeable bool `yaml:"bribeable"`
	// MaxBribeWantedLevel is the highest WantedLevel this guard will accept a bribe for.
	// Required when Bribeable is true.
	MaxBribeWantedLevel int `yaml:"max_bribe_wanted_level"`
	// BaseCosts maps WantedLevel (1-4) to the base bribe cost in credits.
	// Required when Bribeable is true. All keys 1-4 must be present with positive values.
	BaseCosts map[int]int `yaml:"base_costs,omitempty"`
}

// Validate checks REQ-WC-2b: when Bribeable is true, MaxBribeWantedLevel must be
// in range 1-4 and BaseCosts must contain all keys 1-4 with positive values.
//
// Precondition: cfg must not be nil.
// Postcondition: Returns nil iff all bribeable constraints are satisfied.
func (cfg *GuardConfig) Validate() error {
	if !cfg.Bribeable {
		return nil
	}
	if cfg.MaxBribeWantedLevel < 1 || cfg.MaxBribeWantedLevel > 4 {
		return fmt.Errorf("guard: max_bribe_wanted_level must be in range 1-4 when bribeable (got %d)", cfg.MaxBribeWantedLevel)
	}
	for _, level := range []int{1, 2, 3, 4} {
		cost, ok := cfg.BaseCosts[level]
		if !ok {
			return fmt.Errorf("guard: base_costs must contain key %d when bribeable", level)
		}
		if cost <= 0 {
			return fmt.Errorf("guard: base_costs[%d] must be positive (got %d)", level, cost)
		}
	}
	return nil
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
	RequiredFeats []string          `yaml:"required_feats,omitempty"` // REQ-JD-3
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
// playerFeats must not be nil (pass empty slice, not nil).
// Postcondition: Returns nil iff all prerequisites are satisfied and the player does not already hold the job.
func CheckJobPrerequisites(job TrainableJob, playerLevel int, playerJobs map[string]int, playerAttrs map[string]int, playerSkills map[string]string, playerFeats []string) error {
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
	for _, featID := range p.RequiredFeats {
		found := false
		for _, heldFeat := range playerFeats {
			if heldFeat == featID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("prerequisite not met: you must have feat %q", featID)
		}
	}
	return nil
}

// ---- Crafter (stub) ----

// CrafterConfig is intentionally empty until the crafting feature spec is written.
// A YAML block `crafter: {}` MUST be present for npc_type: crafter (REQ-NPC-2).
type CrafterConfig struct{}

// ---- ChipDoc ----

// ChipDocConfig holds configuration for chip_doc type NPCs.
type ChipDocConfig struct {
	RemovalCost int `yaml:"removal_cost"`
	CheckDC     int `yaml:"check_dc"`
}

// ---- MotelKeeper ----

// MotelConfig holds configuration for a motel_keeper NPC (REQ-MOT-2).
type MotelConfig struct {
	RestCost int `yaml:"rest_cost"`
}

// ---- BrothelKeeper ----

// BrothelConfig holds configuration for a brothel_keeper NPC (REQ-BR-2).
type BrothelConfig struct {
	RestCost      int      `yaml:"rest_cost"`
	DiseaseChance float64  `yaml:"disease_chance"`
	RobberyChance float64  `yaml:"robbery_chance"`
	DiseasePool   []string `yaml:"disease_pool"`
	FlairBonusDur string   `yaml:"flair_bonus_duration"`
}

// Validate checks REQ-BR-3: rest_cost > 0, disease_chance and robbery_chance in [0,1],
// disease_pool non-empty, flair_bonus_duration parseable as a Go duration.
//
// Precondition: cfg must not be nil.
// Postcondition: Returns nil iff all constraints are satisfied.
func (cfg *BrothelConfig) Validate() error {
	if cfg.RestCost <= 0 {
		return fmt.Errorf("brothel: rest_cost must be > 0, got %d", cfg.RestCost)
	}
	if cfg.DiseaseChance < 0.0 || cfg.DiseaseChance > 1.0 {
		return fmt.Errorf("brothel: disease_chance must be in [0.0, 1.0], got %f", cfg.DiseaseChance)
	}
	if cfg.RobberyChance < 0.0 || cfg.RobberyChance > 1.0 {
		return fmt.Errorf("brothel: robbery_chance must be in [0.0, 1.0], got %f", cfg.RobberyChance)
	}
	if len(cfg.DiseasePool) == 0 {
		return fmt.Errorf("brothel: disease_pool must not be empty")
	}
	if _, err := time.ParseDuration(cfg.FlairBonusDur); err != nil {
		return fmt.Errorf("brothel: flair_bonus_duration %q is not a valid duration: %w", cfg.FlairBonusDur, err)
	}
	return nil
}

// ---- Fixer ----

// FixerConfig holds the static configuration for a fixer NPC.
// Full bribe/fix command behavior is deferred to the wanted-clearing feature.
// REQ-WC-1, REQ-WC-2, REQ-WC-2a: validated in Validate().
type FixerConfig struct {
	// BaseCosts maps WantedLevel (1–4) to base credit cost for clearing that level.
	BaseCosts map[int]int `yaml:"base_costs"`
	// NPCVariance multiplies the base cost to produce the final price. Must be > 0.
	NPCVariance float64 `yaml:"npc_variance"`
	// MaxWantedLevel is the highest WantedLevel this fixer will negotiate. Range 1–4.
	MaxWantedLevel int `yaml:"max_wanted_level"`
	// ClearRecordQuestID is the quest that clears the criminal record. Empty until quests feature.
	ClearRecordQuestID string `yaml:"clear_record_quest_id,omitempty"`
}

// Validate enforces REQ-WC-1 (NPCVariance > 0), REQ-WC-2 (MaxWantedLevel 1–4),
// and REQ-WC-2a (BaseCosts has all keys 1–4 with positive values).
func (f FixerConfig) Validate() error {
	if f.NPCVariance <= 0 {
		return fmt.Errorf("fixer: npc_variance must be > 0, got %f", f.NPCVariance)
	}
	if f.MaxWantedLevel < 1 || f.MaxWantedLevel > 4 {
		return fmt.Errorf("fixer: max_wanted_level must be in range 1–4, got %d", f.MaxWantedLevel)
	}
	for _, key := range []int{1, 2, 3, 4} {
		v, ok := f.BaseCosts[key]
		if !ok {
			return fmt.Errorf("fixer: base_costs missing required key %d", key)
		}
		if v <= 0 {
			return fmt.Errorf("fixer: base_costs[%d] must be positive, got %d", key, v)
		}
	}
	return nil
}

// ---- TechTrainer ----

// TechTrainerConfig holds the static configuration for a tech trainer NPC.
//
// Precondition: Tradition is a valid technology tradition ID; OfferedLevels is non-empty.
type TechTrainerConfig struct {
	Tradition     string            `yaml:"tradition"`
	OfferedLevels []int             `yaml:"offered_levels"`
	BaseCost      int               `yaml:"base_cost"`
	FindQuestID   string            `yaml:"find_quest_id,omitempty"`
	Prerequisites *TechTrainPrereqs `yaml:"prerequisites,omitempty"`
}

// OffersLevel returns true if this trainer can teach the given technology level.
func (c *TechTrainerConfig) OffersLevel(level int) bool {
	for _, l := range c.OfferedLevels {
		if l == level {
			return true
		}
	}
	return false
}

// TrainingCost computes the cost for one technology at the given level.
//
// Precondition: level >= 1.
// Postcondition: Returns BaseCost * level.
func (c *TechTrainerConfig) TrainingCost(level int) int {
	return c.BaseCost * level
}

// TechTrainPrereqs defines the prerequisite gate for accessing a tech trainer.
//
// Precondition: Operator is "and" or "or" (defaults to "and" if empty); Conditions is non-empty.
type TechTrainPrereqs struct {
	Operator   string               `yaml:"operator"`
	Conditions []TechTrainCondition `yaml:"conditions"`
}

// TechTrainCondition is a single prerequisite condition for tech trainer access.
//
// Precondition: Type is "quest_complete" or "faction_rep".
type TechTrainCondition struct {
	Type      string `yaml:"type"`
	QuestID   string `yaml:"quest_id,omitempty"`
	FactionID string `yaml:"faction_id,omitempty"`
	MinTier   string `yaml:"min_tier,omitempty"`
}

// FactionTierChecker is a function that returns true if the player meets the min faction tier.
type FactionTierChecker func(factionID, minTierID string, factionRep map[string]int) bool

// EvalTechTrainPrereqs returns nil if prerequisites are satisfied, or a descriptive error.
//
// Precondition: prereqs non-nil; completedQuestIDs maps quest ID → true for completed quests.
// Postcondition: Returns nil on pass; non-nil error with denial reason on fail.
// If prereqs is nil, returns nil (no prerequisites).
func EvalTechTrainPrereqs(prereqs *TechTrainPrereqs, completedQuestIDs map[string]bool, checkTier FactionTierChecker) error {
	if prereqs == nil || len(prereqs.Conditions) == 0 {
		return nil
	}
	op := prereqs.Operator
	if op == "" {
		op = "and"
	}
	type result struct {
		met bool
		err error
	}
	results := make([]result, len(prereqs.Conditions))
	for i, cond := range prereqs.Conditions {
		switch cond.Type {
		case "quest_complete":
			if completedQuestIDs[cond.QuestID] {
				results[i] = result{met: true}
			} else {
				results[i] = result{met: false, err: fmt.Errorf("you must complete quest %q first", cond.QuestID)}
			}
		case "faction_rep":
			if checkTier != nil && checkTier(cond.FactionID, cond.MinTier, nil) {
				results[i] = result{met: true}
			} else {
				results[i] = result{met: false, err: fmt.Errorf("you need %q reputation with %q", cond.MinTier, cond.FactionID)}
			}
		default:
			results[i] = result{met: false, err: fmt.Errorf("unknown prerequisite type %q", cond.Type)}
		}
	}
	switch op {
	case "or":
		for _, r := range results {
			if r.met {
				return nil
			}
		}
		return results[0].err
	default: // "and"
		for _, r := range results {
			if !r.met {
				return r.err
			}
		}
		return nil
	}
}
