package trap_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/trap"
)

func TestScalingFor_GlobalDefaults(t *testing.T) {
	// A template with no danger_scaling block should use global defaults (REQ-TR-8).
	tmpl := &trap.TrapTemplate{ID: "bear_trap", DangerScaling: nil}

	cases := []struct {
		level          string
		wantDamage     string
		wantSaveDC     int
		wantStealthDC  int
		wantDisableDC  int
	}{
		{"sketchy", "", 0, 0, 0},
		{"dangerous", "1d6", 2, 2, 2},
		{"all_out_war", "2d6", 4, 4, 4},
	}

	for _, tc := range cases {
		t.Run(tc.level, func(t *testing.T) {
			got := trap.ScalingFor(tmpl, tc.level)
			if got.DamageDice != tc.wantDamage {
				t.Errorf("DamageDice: got %q, want %q", got.DamageDice, tc.wantDamage)
			}
			if got.SaveDCBonus != tc.wantSaveDC {
				t.Errorf("SaveDCBonus: got %d, want %d", got.SaveDCBonus, tc.wantSaveDC)
			}
			if got.StealthDCBonus != tc.wantStealthDC {
				t.Errorf("StealthDCBonus: got %d, want %d", got.StealthDCBonus, tc.wantStealthDC)
			}
			if got.DisableDCBonus != tc.wantDisableDC {
				t.Errorf("DisableDCBonus: got %d, want %d", got.DisableDCBonus, tc.wantDisableDC)
			}
		})
	}
}

func TestScalingFor_PerTemplateOverride(t *testing.T) {
	tmpl := &trap.TrapTemplate{
		ID: "mine",
		DangerScaling: &trap.DangerScalingTier{
			Dangerous: &trap.DangerScalingEntry{
				DamageBonus:    "2d6",
				SaveDCBonus:    3,
				StealthDCBonus: 3,
				DisableDCBonus: 3,
			},
			AllOutWar: &trap.DangerScalingEntry{
				DamageBonus:    "4d6",
				SaveDCBonus:    6,
				StealthDCBonus: 5,
				DisableDCBonus: 5,
			},
		},
	}

	got := trap.ScalingFor(tmpl, "dangerous")
	if got.DamageDice != "2d6" {
		t.Errorf("dangerous DamageDice: got %q, want %q", got.DamageDice, "2d6")
	}
	if got.SaveDCBonus != 3 {
		t.Errorf("dangerous SaveDCBonus: got %d, want 3", got.SaveDCBonus)
	}

	got = trap.ScalingFor(tmpl, "all_out_war")
	if got.DamageDice != "4d6" {
		t.Errorf("all_out_war DamageDice: got %q, want %q", got.DamageDice, "4d6")
	}
	if got.SaveDCBonus != 6 {
		t.Errorf("all_out_war SaveDCBonus: got %d, want 6", got.SaveDCBonus)
	}
}

func TestScalingFor_HonkeypotIgnoresDamageBonus(t *testing.T) {
	// REQ-TR-15: damage_bonus is silently ignored for payload types with no damage field.
	// ScalingFor itself does NOT filter — ResolveTrigger handles that.
	// This test verifies that ScalingFor still RETURNS a DamageDice value,
	// confirming that filtering must happen in payload resolution, not here.
	tmpl := &trap.TrapTemplate{
		ID: "honkeypot_charmer",
		Payload: &trap.TrapPayload{
			Type:            "honkeypot",
			TechnologyEffect: "charm",
		},
		DangerScaling: nil,
	}
	got := trap.ScalingFor(tmpl, "dangerous")
	// ScalingFor returns the global default damage bonus regardless of payload type.
	// ResolveTrigger is responsible for ignoring it.
	if got.DamageDice != "1d6" {
		t.Errorf("expected DamageDice=1d6 from global default, got %q", got.DamageDice)
	}
}
