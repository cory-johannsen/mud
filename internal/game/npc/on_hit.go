// Package npc provides NPC template definitions and live instance management.
package npc

// AttributeModifierDef describes a temporary attribute modifier applied by an on-hit condition.
type AttributeModifierDef struct {
	// Attribute is the attribute affected (e.g. "quickness").
	Attribute string `yaml:"attribute"`
	// Modifier is the signed integer delta applied to the attribute.
	Modifier int `yaml:"modifier"`
	// DurationRounds is the number of combat rounds the modifier lasts.
	DurationRounds int `yaml:"duration_rounds"`
}

// SaveDef describes a saving throw that resists an on-hit condition.
type SaveDef struct {
	// Attribute is the save attribute (e.g. "grit").
	Attribute string `yaml:"attribute"`
	// DC is the difficulty class of the saving throw.
	DC int `yaml:"dc"`
}

// OnHitConditionDef defines a condition applied to a target on a successful hit.
type OnHitConditionDef struct {
	// ConditionID is the condition identifier applied on hit.
	ConditionID string `yaml:"condition_id"`
	// AttributeModifier is an optional temporary attribute modifier granted or penalised by the condition.
	AttributeModifier *AttributeModifierDef `yaml:"attribute_modifier"`
	// Save is an optional saving throw the target may make to resist the condition.
	Save *SaveDef `yaml:"save"`
}
