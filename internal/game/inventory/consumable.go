package inventory

import (
	"math"
	"time"
)

// ConsumableEffect describes all mechanical effects that can be applied when a
// consumable item is used.
type ConsumableEffect struct {
	// Heal is a dice string (e.g. "2d6+4") for HP restoration. Empty = no heal.
	Heal string `yaml:"heal,omitempty"`
	// Conditions are condition effects applied after RemoveConditions.
	Conditions []ConditionEffect `yaml:"conditions,omitempty"`
	// RemoveConditions lists condition IDs to remove before applying new conditions.
	RemoveConditions []string `yaml:"remove_conditions,omitempty"`
	// ConsumeCheck is an optional post-use d20 roll vs DC.
	ConsumeCheck *ConsumeCheck `yaml:"consume_check,omitempty"`
	// RepairField indicates this item is consumed by the repair command, not direct use.
	RepairField bool `yaml:"repair_field,omitempty"`
}

// ConditionEffect pairs a condition ID with an application duration.
type ConditionEffect struct {
	ConditionID string `yaml:"condition_id"`
	Duration    string `yaml:"duration"` // Go time.Duration string e.g. "1h", "30m"
}

// ConsumeCheck represents a post-use d20 roll vs DC using a named stat modifier.
type ConsumeCheck struct {
	Stat              string             `yaml:"stat"`
	DC                int                `yaml:"dc"`
	OnCriticalFailure *CritFailureEffect `yaml:"on_critical_failure,omitempty"`
}

// CritFailureEffect describes effects applied on a critical failure of a ConsumeCheck.
type CritFailureEffect struct {
	Conditions   []ConditionEffect `yaml:"conditions,omitempty"`
	ApplyDisease *DiseaseEffect    `yaml:"apply_disease,omitempty"`
	ApplyToxin   *ToxinEffect      `yaml:"apply_toxin,omitempty"`
}

// DiseaseEffect applies a disease at a given severity.
type DiseaseEffect struct {
	DiseaseID string `yaml:"disease_id"`
	Severity  int    `yaml:"severity"`
}

// ToxinEffect applies a toxin at a given severity.
type ToxinEffect struct {
	ToxinID  string `yaml:"toxin_id"`
	Severity int    `yaml:"severity"`
}

// ConsumableTarget is the minimal interface required to apply consumable effects.
// Implemented by *session.PlayerSession. This interface exists to prevent import
// cycles between the inventory and session packages (REQ-EM-45).
type ConsumableTarget interface {
	GetTeam() string
	// GetStatModifier returns the character's ability modifier for the named stat
	// (e.g. "strength", "dexterity", "constitution"). Used for ConsumeCheck rolls (REQ-EM-41).
	// Returns 0 for unknown stat names.
	GetStatModifier(stat string) int
	ApplyHeal(amount int)
	ApplyCondition(conditionID string, duration time.Duration)
	RemoveCondition(conditionID string)
	ApplyDisease(diseaseID string, severity int)
	ApplyToxin(toxinID string, severity int)
}

// ConsumableResult records the resolved effects for display and auditing.
type ConsumableResult struct {
	HealApplied        int
	ConditionsApplied  []string
	ConditionsRemoved  []string
	DiseaseApplied     string
	ToxinApplied       string
	TeamMultiplier     float64
	ConsumeCheckResult string // "success" | "failure" | "critical_failure" | "not_checked"
}

// TeamMultiplier returns the effectiveness multiplier for a consumable based on
// player team and item team alignment.
//
// | Relationship                          | Multiplier |
// | Player team matches item team         | 1.25×      |
// | Player team opposes item team         | 0.75×      |
// | Item has no team OR player has no team| 1.00×      |
//
// Precondition: playerTeam and itemTeam are "" | "gun" | "machete".
// Postcondition: returns a positive multiplier.
func TeamMultiplier(playerTeam, itemTeam string) float64 {
	if itemTeam == "" || playerTeam == "" {
		return 1.0
	}
	if playerTeam == itemTeam {
		return 1.25
	}
	return 0.75
}

// ApplyConsumable applies all effects from the consumable ItemDef to the target.
//
// This is a pure function: it calls only methods on ConsumableTarget and returns
// a ConsumableResult. No direct state mutation occurs here beyond calling target
// methods. It MUST NOT import internal/game/session (REQ-EM-45).
//
// Effect ordering (REQ-EM-43):
//  1. RemoveConditions
//  2. Heal (team-multiplied, floored)
//  3. New Conditions (team-multiplied duration)
//  4. ConsumeCheck (d20 + stat modifier vs DC; natural-1 or total ≤ DC-10 = critical failure)
//
// Preconditions:
//   - target must be non-nil.
//   - def must be non-nil.
//   - rng must be non-nil.
//
// Postcondition: returns a fully populated ConsumableResult.
func ApplyConsumable(target ConsumableTarget, def *ItemDef, rng Roller) ConsumableResult {
	result := ConsumableResult{
		TeamMultiplier:     TeamMultiplier(target.GetTeam(), def.Team),
		ConsumeCheckResult: "not_checked",
	}

	if def.Effect == nil {
		return result
	}
	eff := def.Effect

	// Step 1: remove conditions (REQ-EM-43).
	for _, cid := range eff.RemoveConditions {
		target.RemoveCondition(cid)
		result.ConditionsRemoved = append(result.ConditionsRemoved, cid)
	}

	// Step 2: heal (team-multiplied, floored).
	if eff.Heal != "" {
		rawHeal := rng.Roll(eff.Heal)
		healAmt := int(math.Floor(float64(rawHeal) * result.TeamMultiplier))
		if healAmt > 0 {
			target.ApplyHeal(healAmt)
		}
		result.HealApplied = healAmt
	}

	// Step 3: apply new conditions.
	for _, ce := range eff.Conditions {
		dur, err := time.ParseDuration(ce.Duration)
		if err != nil {
			// Invalid duration — skip (should be caught at load time).
			continue
		}
		// Team-multiplier applied to duration in seconds (floored).
		durSecs := int(math.Floor(float64(dur.Seconds()) * result.TeamMultiplier))
		if durSecs < 0 {
			durSecs = 0
		}
		finalDur := time.Duration(durSecs) * time.Second
		target.ApplyCondition(ce.ConditionID, finalDur)
		result.ConditionsApplied = append(result.ConditionsApplied, ce.ConditionID)
	}

	// Step 4: consume check (REQ-EM-41).
	if eff.ConsumeCheck != nil {
		d20 := rng.RollD20()
		statMod := target.GetStatModifier(eff.ConsumeCheck.Stat)
		total := d20 + statMod
		dc := eff.ConsumeCheck.DC

		// PF2E four-tier critical failure: natural 1 OR total ≤ DC-10.
		isCritFail := d20 == 1 || total <= dc-10
		if isCritFail {
			result.ConsumeCheckResult = "critical_failure"
			applyCritFailure(target, &result, eff.ConsumeCheck.OnCriticalFailure)
		} else if total >= dc {
			result.ConsumeCheckResult = "success"
		} else {
			result.ConsumeCheckResult = "failure"
		}
	}

	return result
}

// applyCritFailure applies the on_critical_failure effects to the target.
func applyCritFailure(target ConsumableTarget, result *ConsumableResult, cfe *CritFailureEffect) {
	if cfe == nil {
		return
	}
	for _, ce := range cfe.Conditions {
		dur, err := time.ParseDuration(ce.Duration)
		if err != nil {
			continue
		}
		target.ApplyCondition(ce.ConditionID, dur)
		result.ConditionsApplied = append(result.ConditionsApplied, ce.ConditionID)
	}
	if cfe.ApplyDisease != nil {
		target.ApplyDisease(cfe.ApplyDisease.DiseaseID, cfe.ApplyDisease.Severity)
		result.DiseaseApplied = cfe.ApplyDisease.DiseaseID
	}
	if cfe.ApplyToxin != nil {
		target.ApplyToxin(cfe.ApplyToxin.ToxinID, cfe.ApplyToxin.Severity)
		result.ToxinApplied = cfe.ApplyToxin.ToxinID
	}
}
