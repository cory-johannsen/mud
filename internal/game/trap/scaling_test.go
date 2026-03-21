package trap_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/trap"
	"pgregory.net/rapid"
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

func TestScalingFor_ReturnsGlobalDefaultRegardlessOfPayloadType(t *testing.T) {
	// ScalingFor returns global defaults regardless of payload type — filtering is ResolveTrigger's job.
	// This test verifies that ScalingFor still RETURNS a DamageDice value even for payload types
	// that do not use damage, confirming that filtering must happen in payload resolution, not here.
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

func TestScalingFor_Property_OverrideTakesPriority(t *testing.T) {
	// Property: when a per-template override exists for a level, it always takes priority over global defaults.
	rapid.Check(t, func(rt *rapid.T) {
		bonusDamage := rapid.StringMatching(`[1-4]d[46]`).Draw(rt, "bonusDamage")
		saveDCBonus := rapid.IntRange(1, 10).Draw(rt, "saveDCBonus")
		stealthDCBonus := rapid.IntRange(1, 10).Draw(rt, "stealthDCBonus")
		disableDCBonus := rapid.IntRange(1, 10).Draw(rt, "disableDCBonus")

		tmpl := &trap.TrapTemplate{
			ID: "test_trap",
			DangerScaling: &trap.DangerScalingTier{
				Dangerous: &trap.DangerScalingEntry{
					DamageBonus:    bonusDamage,
					SaveDCBonus:    saveDCBonus,
					StealthDCBonus: stealthDCBonus,
					DisableDCBonus: disableDCBonus,
				},
			},
		}

		got := trap.ScalingFor(tmpl, "dangerous")
		if got.DamageDice != bonusDamage {
			rt.Fatalf("override DamageDice: got %q, want %q", got.DamageDice, bonusDamage)
		}
		if got.SaveDCBonus != saveDCBonus {
			rt.Fatalf("override SaveDCBonus: got %d, want %d", got.SaveDCBonus, saveDCBonus)
		}
	})
}

func TestScalingFor_Property_UnknownLevelReturnsZero(t *testing.T) {
	// Property: any unrecognised danger level always returns zero ScalingBonuses.
	rapid.Check(t, func(rt *rapid.T) {
		unknownLevel := rapid.StringMatching(`x[a-z]{4,8}`).Draw(rt, "unknownLevel")
		tmpl := &trap.TrapTemplate{ID: "any_trap", DangerScaling: nil}
		got := trap.ScalingFor(tmpl, unknownLevel)
		if got.DamageDice != "" || got.SaveDCBonus != 0 || got.StealthDCBonus != 0 || got.DisableDCBonus != 0 {
			rt.Fatalf("unknown level %q: expected zero bonuses, got %+v", unknownLevel, got)
		}
	})
}
