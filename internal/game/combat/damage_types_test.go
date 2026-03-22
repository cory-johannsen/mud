package combat

import "testing"

func TestDamageTypeConstants(t *testing.T) {
	if DamageTypeBleed != "bleed" {
		t.Errorf("DamageTypeBleed: got %q, want %q", DamageTypeBleed, "bleed")
	}
	if DamageTypePoison != "poison" {
		t.Errorf("DamageTypePoison: got %q, want %q", DamageTypePoison, "poison")
	}
	if DamageTypeElectric != "electric" {
		t.Errorf("DamageTypeElectric: got %q, want %q", DamageTypeElectric, "electric")
	}
	if DamageTypePhysical != "physical" {
		t.Errorf("DamageTypePhysical: got %q, want %q", DamageTypePhysical, "physical")
	}
	if DamageTypeFire != "fire" {
		t.Errorf("DamageTypeFire: got %q, want %q", DamageTypeFire, "fire")
	}
}
