// Package reaction defines the player reaction system: trigger types, effect types,
// reaction definitions, and the callback interface for interactive prompting.
package reaction

import "context"

// ReactionTriggerType identifies the combat event that can fire a reaction.
type ReactionTriggerType string

const (
	// TriggerOnSaveFail fires after a player's saving throw is determined to be a Failure.
	TriggerOnSaveFail ReactionTriggerType = "on_save_fail"
	// TriggerOnSaveCritFail fires after a player's saving throw is determined to be a Critical Failure.
	TriggerOnSaveCritFail ReactionTriggerType = "on_save_crit_fail"
	// TriggerOnDamageTaken fires after damage is calculated but before it is applied to a player.
	TriggerOnDamageTaken ReactionTriggerType = "on_damage_taken"
	// TriggerOnEnemyMoveAdjacent fires when an enemy moves into melee range (<=5ft) of the player.
	TriggerOnEnemyMoveAdjacent ReactionTriggerType = "on_enemy_move_adjacent"
	// TriggerOnConditionApplied fires when a condition is about to be applied to the player.
	// Fire point deferred to sub-project 2.
	TriggerOnConditionApplied ReactionTriggerType = "on_condition_applied"
	// TriggerOnAllyDamaged fires when a player ally takes damage in the same combat.
	// Informational only: damage has already been applied; DamagePending is always nil.
	TriggerOnAllyDamaged ReactionTriggerType = "on_ally_damaged"
	// TriggerOnEnemyEntersRoom fires when an NPC combatant moves in the player's current room.
	// Fire point: after consumable trap evaluation in the onCombatantMoved callback (REQ-READY-15).
	TriggerOnEnemyEntersRoom ReactionTriggerType = "on_enemy_enters_room"
	// TriggerOnFall fires when the player would fall. Fire point deferred to a future feature.
	TriggerOnFall ReactionTriggerType = "on_fall"
	// TriggerOnEnemyDefeated fires when the player's attack reduces an enemy to 0 HP.
	// Fire point deferred to a future feature.
	TriggerOnEnemyDefeated ReactionTriggerType = "on_enemy_defeated"
)

// ReactionEffectType identifies what a reaction does when it fires.
type ReactionEffectType string

const (
	// ReactionEffectRerollSave rerolls a failed saving throw, keeping the better result.
	ReactionEffectRerollSave ReactionEffectType = "reroll_save"
	// ReactionEffectStrike executes an immediate attack against the trigger source.
	ReactionEffectStrike ReactionEffectType = "strike"
	// ReactionEffectReduceDamage subtracts shield hardness from pending damage.
	ReactionEffectReduceDamage ReactionEffectType = "reduce_damage"
)

// ReactionEffect describes what happens when a reaction fires.
type ReactionEffect struct {
	// Type is the effect to apply.
	Type ReactionEffectType `yaml:"type"`
	// Target names the target of the effect (e.g. "trigger_source"). Optional.
	Target string `yaml:"target,omitempty"`
	// Keep specifies the reroll strategy (e.g. "better"). Optional.
	Keep string `yaml:"keep,omitempty"`
}

// ReactionDef is the reaction declaration embedded in a Feat or TechnologyDef YAML.
type ReactionDef struct {
	// Triggers lists the combat events that can fire this reaction.
	// Must contain at least one entry. An empty slice is a no-op at registration time.
	Triggers []ReactionTriggerType `yaml:"triggers"`
	// Requirement is an optional predicate the player must satisfy (e.g. "wielding_melee_weapon").
	// Empty string means no requirement.
	Requirement string `yaml:"requirement,omitempty"`
	// Effect is the action taken when the reaction fires.
	Effect ReactionEffect `yaml:"effect"`
	// BonusReactions is the flat number of additional reactions this feat grants per round.
	// Summed across all active feats at StartRound to compute Budget.Max.
	// Default 0 (no bonus). Per REACTION-14, NPCs do not read this field.
	BonusReactions int `yaml:"bonus_reactions,omitempty"`
}

// ReactionContext carries the mutable state the effect can read and modify.
type ReactionContext struct {
	// TriggerUID is the UID of the player whose reaction may fire.
	TriggerUID string
	// SourceUID is the UID or NPC ID of the entity that caused the trigger.
	SourceUID string
	// DamagePending is a pointer to the pending damage amount (for reduce_damage).
	// Nil when the trigger is not damage-related.
	// The callback may modify *DamagePending before ApplyDamage is called.
	DamagePending *int
	// SaveOutcome is a pointer to the save outcome (for reroll_save).
	// Uses combat.Outcome int values: 0=CritSuccess, 1=Success, 2=Failure, 3=CritFailure.
	// Declared as *int (not *combat.Outcome) to avoid an import cycle since combat imports reaction.
	// Nil when the trigger is not save-related.
	SaveOutcome *int
	// ConditionID is the condition being applied (for on_condition_applied). May be empty.
	ConditionID string
}

// ReactionCallback is invoked at trigger fire points during round resolution.
// ctx carries the deadline for the interactive prompt (context.WithTimeout applied by the resolver).
// uid is the combatant who may spend their reaction.
// candidates is the slice of eligible reactions the player may choose from (never nil, may be empty).
// Returns (true, chosen, nil) when the reaction is spent; (false, nil, nil) when declined or
// unavailable; (false, nil, err) on non-deadline error (budget is refunded by caller).
// A nil ReactionCallback MUST be treated as a no-op returning (false, nil, nil).
type ReactionCallback func(
	ctx context.Context,
	uid string,
	trigger ReactionTriggerType,
	rctx ReactionContext,
	candidates []PlayerReaction,
) (spent bool, chosen *PlayerReaction, err error)
