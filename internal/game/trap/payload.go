package trap

import (
	"fmt"
)

// TriggerResult contains the resolved payload for a fired trap.
type TriggerResult struct {
	// Narrative is a human-readable description of what the trap does.
	Narrative string
	// ConditionID is the condition applied on trigger (empty = no condition).
	ConditionID string
	// DamageDice is the total dice expression after scaling (empty = no damage).
	DamageDice string
	// DamageTypes is the list of damage types (e.g. ["piercing", "fire"]).
	DamageTypes []string
	// SaveType is the saving throw type ("reflex", "will", or "" for no save).
	SaveType string
	// SaveDC is the final save DC after scaling.
	SaveDC int
	// AoE is true for traps that affect all room occupants (Mine).
	AoE bool
	// TechnologyEffect is the technology ID applied by a Honkeypot (empty for non-Honkeypot).
	TechnologyEffect string
}

// combineDice concatenates two dice expressions, e.g. "4d6" + "1d6" → "4d6+1d6".
// Returns base unchanged if bonus is empty.
func combineDice(base, bonus string) string {
	if bonus == "" {
		return base
	}
	if base == "" {
		return bonus
	}
	return base + "+" + bonus
}

// CombineDice is the exported form of combineDice, used in tests.
func CombineDice(base, bonus string) string {
	return combineDice(base, bonus)
}

// ResolveTrigger computes the TriggerResult for tmpl at dangerLevel.
// For Pressure Plate templates, resolution delegates to the linked payload_template.
//
// Precondition: tmpl must be non-nil; templates must contain all templates referenced by payload_template.
// Postcondition: Returns error if payload type is unrecognised or payload_template is missing.
// REQ-TR-15: Damage bonus is silently ignored for honkeypot payload type.
// REQ-TR-16: Pressure Plate reset_mode governs lifecycle; linked template's reset_mode is ignored.
func ResolveTrigger(tmpl *TrapTemplate, dangerLevel string, templates map[string]*TrapTemplate) (TriggerResult, error) {
	// Resolve Pressure Plate by delegation.
	if tmpl.Trigger == TriggerPressurePlate {
		linked, ok := templates[tmpl.PayloadTemplate]
		if !ok {
			return TriggerResult{}, fmt.Errorf("pressure_plate %q: payload_template %q not found", tmpl.ID, tmpl.PayloadTemplate)
		}
		// Use linked template's payload but Pressure Plate's own scaling config.
		// REQ-TR-16: ignore linked template's reset_mode (caller uses tmpl.ResetMode).
		result, err := resolvePayload(linked.Payload, tmpl, dangerLevel)
		if err != nil {
			return TriggerResult{}, fmt.Errorf("pressure_plate %q: %w", tmpl.ID, err)
		}
		return result, nil
	}

	if tmpl.Payload == nil {
		return TriggerResult{}, fmt.Errorf("trap %q: no payload defined", tmpl.ID)
	}
	return resolvePayload(tmpl.Payload, tmpl, dangerLevel)
}

// resolvePayload converts a TrapPayload + template scaling into a TriggerResult.
// scalingTmpl is the template whose DangerScaling block is applied (may differ from payload source for Pressure Plate).
func resolvePayload(payload *TrapPayload, scalingTmpl *TrapTemplate, dangerLevel string) (TriggerResult, error) {
	scaling := ScalingFor(scalingTmpl, dangerLevel)

	switch payload.Type {
	case "mine":
		damage := combineDice(payload.Damage, scaling.DamageDice)
		return TriggerResult{
			Narrative:   "An explosive mine detonates!",
			DamageDice:  damage,
			DamageTypes: []string{"piercing", "fire"},
			SaveType:    payload.SaveType,
			SaveDC:      payload.SaveDC + scaling.SaveDCBonus,
			AoE:         true,
		}, nil

	case "pit":
		damage := combineDice(payload.Damage, scaling.DamageDice)
		return TriggerResult{
			Narrative:   "The floor gives way!",
			ConditionID: payload.Condition,
			DamageDice:  damage,
			DamageTypes: []string{"fall"},
			SaveType:    payload.SaveType,
			SaveDC:      payload.SaveDC + scaling.SaveDCBonus,
		}, nil

	case "bear_trap":
		// REQ-TR-14: Bear Trap applies grabbed condition with no save.
		damage := combineDice(payload.Damage, scaling.DamageDice)
		return TriggerResult{
			Narrative:   "Steel jaws snap shut!",
			ConditionID: "grabbed",
			DamageDice:  damage,
			DamageTypes: []string{"piercing"},
			SaveType:    "", // no save — REQ-TR-14
			SaveDC:      0,
		}, nil

	case "trip_wire":
		damage := combineDice(payload.Damage, scaling.DamageDice)
		return TriggerResult{
			Narrative:   "A wire catches your foot!",
			ConditionID: payload.Condition,
			DamageDice:  damage,
			DamageTypes: []string{"slashing"},
			SaveType:    payload.SaveType,
			SaveDC:      payload.SaveDC + scaling.SaveDCBonus,
		}, nil

	case "honkeypot":
		// REQ-TR-15: damage bonus silently ignored — no damage field.
		return TriggerResult{
			Narrative:        "You are drawn in by an irresistible lure!",
			TechnologyEffect: payload.TechnologyEffect,
			SaveType:         payload.SaveType,
			SaveDC:           payload.SaveDC + scaling.SaveDCBonus,
			AoE:              false,
		}, nil

	default:
		return TriggerResult{}, fmt.Errorf("unrecognised payload type %q", payload.Type)
	}
}
