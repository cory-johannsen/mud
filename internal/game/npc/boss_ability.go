// Package npc — BossAbility and BossAbilityEffect type definitions.
package npc

import (
	"fmt"
	"time"
)

// BossAbilityEffect holds the mechanical outcome of a boss ability.
// Exactly one field must be set (non-zero / non-empty).
type BossAbilityEffect struct {
	// AoeCondition is the condition ID to apply to all players in the room.
	AoeCondition string `yaml:"aoe_condition"`
	// AoeDamageExpr is a dice expression for AoE damage applied to all players.
	AoeDamageExpr string `yaml:"aoe_damage_expr"`
	// HealPct is the percentage of MaxHP to restore to the boss (non-zero).
	HealPct int `yaml:"heal_pct"`
}

// Validate checks that exactly one field is set.
//
// Precondition: none.
// Postcondition: returns nil iff exactly one field is non-zero/non-empty.
func (e BossAbilityEffect) Validate() error {
	set := 0
	if e.AoeCondition != "" {
		set++
	}
	if e.AoeDamageExpr != "" {
		set++
	}
	if e.HealPct != 0 {
		set++
	}
	if set != 1 {
		return fmt.Errorf("boss_ability_effect: exactly one field must be set, got %d", set)
	}
	return nil
}

// BossAbility defines a special ability that a boss NPC can use during combat.
type BossAbility struct {
	// ID is a unique identifier for this ability within the template.
	ID string `yaml:"id"`
	// Name is the player-visible display name of the ability.
	Name string `yaml:"name"`
	// Trigger determines when this ability fires.
	// Valid values: "hp_pct_below", "round_start", "on_damage_taken".
	Trigger string `yaml:"trigger"`
	// TriggerValue holds the threshold for trigger evaluation.
	// For "hp_pct_below": HP percentage (e.g. 50 = fires below 50% HP).
	// For "round_start": round number (0 = every round).
	// For "on_damage_taken": must be 0 (unused).
	TriggerValue int `yaml:"trigger_value"`
	// Cooldown is a Go duration string (e.g. "30s"). Empty means no cooldown.
	Cooldown string `yaml:"cooldown"`
	// Effect is the mechanical outcome when this ability fires.
	Effect BossAbilityEffect `yaml:"effect"`
}

// Validate checks the ability definition for correctness.
//
// Precondition: none.
// Postcondition: returns nil iff all fields are valid per REQ-AE-33.
func (a BossAbility) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("boss_ability: id must not be empty")
	}
	if a.Name == "" {
		return fmt.Errorf("boss_ability %q: name must not be empty", a.ID)
	}
	validTriggers := map[string]bool{
		"hp_pct_below": true, "round_start": true, "on_damage_taken": true,
	}
	if !validTriggers[a.Trigger] {
		return fmt.Errorf("boss_ability %q: unknown trigger %q", a.ID, a.Trigger)
	}
	if a.Trigger == "on_damage_taken" && a.TriggerValue != 0 {
		return fmt.Errorf("boss_ability %q: trigger_value must be 0 for on_damage_taken", a.ID)
	}
	if a.Cooldown != "" {
		if _, err := time.ParseDuration(a.Cooldown); err != nil {
			return fmt.Errorf("boss_ability %q: cooldown %q is not a valid duration: %w", a.ID, a.Cooldown, err)
		}
	}
	if err := a.Effect.Validate(); err != nil {
		return fmt.Errorf("boss_ability %q: %w", a.ID, err)
	}
	return nil
}
