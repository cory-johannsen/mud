package trap_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/trap"
)

func TestLoadTrapTemplate_BearTrap(t *testing.T) {
	dir := t.TempDir()
	content := `
id: bear_trap
name: Bear Trap
description: A rusted steel jaw trap hidden under debris.
trigger: entry
payload:
  type: bear_trap
  damage: 2d6
  condition: grabbed
  save_type: ""
  save_dc: 0
stealth_dc: 16
disable_dc: 20
reset_mode: auto
reset_timer: 10m
`
	if err := os.WriteFile(filepath.Join(dir, "bear_trap.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	tmpl, err := trap.LoadTrapTemplate(filepath.Join(dir, "bear_trap.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.ID != "bear_trap" {
		t.Errorf("ID: got %q, want %q", tmpl.ID, "bear_trap")
	}
	if tmpl.Name != "Bear Trap" {
		t.Errorf("Name: got %q, want %q", tmpl.Name, "Bear Trap")
	}
	if tmpl.Trigger != trap.TriggerEntry {
		t.Errorf("Trigger: got %q, want %q", tmpl.Trigger, trap.TriggerEntry)
	}
	if tmpl.Payload == nil {
		t.Fatal("Payload is nil")
	}
	if tmpl.Payload.Type != "bear_trap" {
		t.Errorf("Payload.Type: got %q, want %q", tmpl.Payload.Type, "bear_trap")
	}
	if tmpl.Payload.Damage != "2d6" {
		t.Errorf("Payload.Damage: got %q, want %q", tmpl.Payload.Damage, "2d6")
	}
	if tmpl.Payload.Condition != "grabbed" {
		t.Errorf("Payload.Condition: got %q, want %q", tmpl.Payload.Condition, "grabbed")
	}
	if tmpl.StealthDC != 16 {
		t.Errorf("StealthDC: got %d, want 16", tmpl.StealthDC)
	}
	if tmpl.DisableDC != 20 {
		t.Errorf("DisableDC: got %d, want 20", tmpl.DisableDC)
	}
}

func TestLoadTrapTemplate_PressurePlate_RejectsSelfReference(t *testing.T) {
	dir := t.TempDir()
	// Write the pressure plate pointing at itself via another pressure plate.
	pp1 := `
id: pressure_plate_a
name: Pressure Plate A
trigger: pressure_plate
payload_template: pressure_plate_b
stealth_dc: 14
disable_dc: 18
reset_mode: one_shot
`
	pp2 := `
id: pressure_plate_b
name: Pressure Plate B
trigger: pressure_plate
payload_template: pressure_plate_a
stealth_dc: 14
disable_dc: 18
reset_mode: one_shot
`
	if err := os.WriteFile(filepath.Join(dir, "pressure_plate_a.yaml"), []byte(pp1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pressure_plate_b.yaml"), []byte(pp2), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := trap.LoadTrapTemplates(dir)
	if err == nil {
		t.Fatal("expected error for REQ-TR-11 violation, got nil")
	}
}
