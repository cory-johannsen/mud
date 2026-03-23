package trap_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/trap"
	"pgregory.net/rapid"
)

func makeTemplates(entries ...*trap.TrapTemplate) map[string]*trap.TrapTemplate {
	m := make(map[string]*trap.TrapTemplate)
	for _, e := range entries {
		m[e.ID] = e
	}
	return m
}

func TestResolveTrigger_BearTrap(t *testing.T) {
	tmpl := &trap.TrapTemplate{
		ID:      "bear_trap",
		Trigger: trap.TriggerEntry,
		Payload: &trap.TrapPayload{
			Type:      "bear_trap",
			Damage:    "2d6",
			Condition: "grabbed",
			SaveType:  "",
			SaveDC:    0,
		},
		StealthDC: 16,
		DisableDC: 20,
	}
	result, err := trap.ResolveTrigger(tmpl, "sketchy", makeTemplates(tmpl))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// REQ-TR-14: grabbed condition, no save.
	if result.ConditionID != "grabbed" {
		t.Errorf("ConditionID: got %q, want %q", result.ConditionID, "grabbed")
	}
	if result.SaveType != "" {
		t.Errorf("SaveType: got %q, want empty (no save)", result.SaveType)
	}
	if result.DamageDice != "2d6" {
		t.Errorf("DamageDice: got %q, want %q", result.DamageDice, "2d6")
	}
	if len(result.DamageTypes) == 0 || result.DamageTypes[0] != "piercing" {
		t.Errorf("DamageTypes: got %v, want [piercing]", result.DamageTypes)
	}
}

func TestResolveTrigger_Mine(t *testing.T) {
	tmpl := &trap.TrapTemplate{
		ID:      "mine",
		Trigger: trap.TriggerEntry,
		Payload: &trap.TrapPayload{
			Type:     "mine",
			Damage:   "4d6",
			SaveType: "reflex",
			SaveDC:   18,
		},
	}
	result, err := trap.ResolveTrigger(tmpl, "sketchy", makeTemplates(tmpl))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DamageDice != "4d6" {
		t.Errorf("DamageDice: got %q, want %q", result.DamageDice, "4d6")
	}
	if !result.AoE {
		t.Error("expected AoE=true for mine")
	}
	if result.SaveType != "reflex" {
		t.Errorf("SaveType: got %q, want reflex", result.SaveType)
	}
	found := false
	for _, dt := range result.DamageTypes {
		if dt == "fire" {
			found = true
		}
	}
	if !found {
		t.Errorf("DamageTypes should include fire; got %v", result.DamageTypes)
	}
}

func TestResolveTrigger_PressurePlate_UsesLinkedPayload(t *testing.T) {
	mine := &trap.TrapTemplate{
		ID:        "mine",
		Trigger:   trap.TriggerEntry,
		ResetMode: trap.ResetAuto,
		Payload: &trap.TrapPayload{
			Type:     "mine",
			Damage:   "4d6",
			SaveType: "reflex",
			SaveDC:   18,
		},
	}
	pp := &trap.TrapTemplate{
		ID:              "pressure_plate_mine",
		Trigger:         trap.TriggerPressurePlate,
		PayloadTemplate: "mine",
		ResetMode:       trap.ResetOneShot, // REQ-TR-16: this governs lifecycle
	}

	templates := makeTemplates(mine, pp)
	result, err := trap.ResolveTrigger(pp, "sketchy", templates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Payload comes from mine.
	if !result.AoE {
		t.Error("expected AoE=true (from linked mine payload)")
	}
	if result.SaveType != "reflex" {
		t.Errorf("SaveType: got %q, want reflex", result.SaveType)
	}
}

func TestResolveTrigger_Honkeypot_NoDamage(t *testing.T) {
	tmpl := &trap.TrapTemplate{
		ID:      "honkeypot_charmer",
		Trigger: trap.TriggerRegion,
		Payload: &trap.TrapPayload{
			Type:            "honkeypot",
			TechnologyEffect: "charm",
			SaveType:        "will",
			SaveDC:          20,
		},
		TargetRegions: []string{"lake_oswego"},
	}
	// REQ-TR-15: damage bonus from danger level should be silently ignored.
	result, err := trap.ResolveTrigger(tmpl, "dangerous", makeTemplates(tmpl))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DamageDice != "" {
		t.Errorf("expected no damage for honkeypot, got %q", result.DamageDice)
	}
	if result.TechnologyEffect != "charm" {
		t.Errorf("TechnologyEffect: got %q, want charm", result.TechnologyEffect)
	}
	if result.AoE {
		t.Error("expected AoE=false for honkeypot")
	}
	// SaveDC should be scaled: 20 + 2 (dangerous global) = 22.
	if result.SaveDC != 22 {
		t.Errorf("SaveDC: got %d, want 22 (20+2 dangerous scaling)", result.SaveDC)
	}
}

func TestCombineDice_Property(t *testing.T) {
	// Property: combineDice never returns empty when either input is non-empty.
	// Property: result contains both operands when both are non-empty.
	rapid.Check(t, func(rt *rapid.T) {
		base := rapid.StringMatching(`[1-4]d[46]`).Draw(rt, "base")
		bonus := rapid.StringMatching(`[1-2]d[46]`).Draw(rt, "bonus")

		result := trap.CombineDice(base, bonus)
		if result == "" {
			rt.Fatalf("combineDice(%q, %q) returned empty string", base, bonus)
		}
		// Result must contain both components when both are non-empty.
		if !strings.Contains(result, base) {
			rt.Fatalf("combineDice(%q, %q) = %q: result does not contain base", base, bonus, result)
		}
		if !strings.Contains(result, bonus) {
			rt.Fatalf("combineDice(%q, %q) = %q: result does not contain bonus", base, bonus, result)
		}
	})
}

func TestResolveTrigger_SubstanceID_Propagated(t *testing.T) {
	tmpl := &trap.TrapTemplate{
		ID:      "poison_pit",
		Trigger: trap.TriggerEntry,
		Payload: &trap.TrapPayload{Type: "pit", Damage: "1d6", SubstanceID: "viper_venom"},
	}
	result, err := trap.ResolveTrigger(tmpl, "safe", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SubstanceID != "viper_venom" {
		t.Fatalf("SubstanceID = %q, want %q", result.SubstanceID, "viper_venom")
	}
}
