package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"gopkg.in/yaml.v3"
)

func TestItemDef_CombatDomainRoundTrips(t *testing.T) {
	raw := `
id: ai_test
name: Test AI Item
description: test
kind: weapon
weapon_ref: combat_knife
weight: 1.0
stackable: false
max_stack: 1
value: 100
combat_domain: test_domain
combat_script: |
  preconditions.always = function(self) return true end
`
	var d inventory.ItemDef
	if err := yaml.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if d.CombatDomain != "test_domain" {
		t.Fatalf("CombatDomain: want %q, got %q", "test_domain", d.CombatDomain)
	}
	if d.CombatScript == "" {
		t.Fatal("CombatScript should not be empty")
	}
}

func TestItemInstance_CombatScriptState_DefaultNil(t *testing.T) {
	inst := inventory.ItemInstance{InstanceID: "abc", ItemDefID: "ai_test", Quantity: 1}
	if inst.CombatScriptState != nil {
		t.Fatal("CombatScriptState should default to nil")
	}
}

func TestBackpack_MutableItem_ReturnsPointer(t *testing.T) {
	bp := inventory.NewBackpack(10, 100)
	def := &inventory.ItemDef{ID: "x", Name: "X", Kind: "junk", Stackable: false, MaxStack: 1, Weight: 0.1}
	inst := &inventory.ItemInstance{
		InstanceID: "test-instance-1",
		ItemDefID:  def.ID,
		Quantity:   1,
	}
	if err := bp.AddInstance(inst); err != nil {
		t.Fatalf("AddInstance: %v", err)
	}
	items := bp.Items()
	if len(items) == 0 {
		t.Fatal("expected item in backpack")
	}
	mutable := bp.MutableItem(items[0].InstanceID)
	if mutable == nil {
		t.Fatal("MutableItem returned nil")
	}
	mutable.CombatScriptState = map[string]interface{}{"kills": float64(3)}
	// Verify mutation is visible via MutableItem again.
	check := bp.MutableItem(items[0].InstanceID)
	if v, ok := check.CombatScriptState["kills"]; !ok || v.(float64) != 3 {
		t.Fatalf("mutation not persisted: state=%v", check.CombatScriptState)
	}
}
