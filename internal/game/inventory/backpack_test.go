package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"pgregory.net/rapid"
)

func junkDef(id string, weight float64) *inventory.ItemDef {
	return &inventory.ItemDef{
		ID:       id,
		Name:     id,
		Kind:     inventory.KindJunk,
		Weight:   weight,
		MaxStack: 1,
	}
}

func stackDef(id string, weight float64, maxStack int) *inventory.ItemDef {
	return &inventory.ItemDef{
		ID:        id,
		Name:      id,
		Kind:      inventory.KindJunk,
		Weight:    weight,
		Stackable: true,
		MaxStack:  maxStack,
	}
}

func makeRegistry(defs ...*inventory.ItemDef) *inventory.Registry {
	reg := inventory.NewRegistry()
	for _, d := range defs {
		_ = reg.RegisterItem(d)
	}
	return reg
}

func TestBackpack_Add_SingleItem(t *testing.T) {
	def := junkDef("rock", 1.0)
	reg := makeRegistry(def)
	bp := inventory.NewBackpack(5, 50.0)

	inst, err := bp.Add("rock", 1, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.ItemDefID != "rock" {
		t.Errorf("got ItemDefID=%q, want %q", inst.ItemDefID, "rock")
	}
	if inst.Quantity != 1 {
		t.Errorf("got Quantity=%d, want 1", inst.Quantity)
	}
	if inst.InstanceID == "" {
		t.Error("InstanceID should not be empty")
	}
	if bp.UsedSlots() != 1 {
		t.Errorf("got UsedSlots=%d, want 1", bp.UsedSlots())
	}
}

func TestBackpack_Add_StackableItem_MergesIntoExisting(t *testing.T) {
	def := stackDef("ammo", 0.1, 50)
	reg := makeRegistry(def)
	bp := inventory.NewBackpack(5, 100.0)

	inst1, err := bp.Add("ammo", 10, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inst2, err := bp.Add("ammo", 5, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst2.InstanceID != inst1.InstanceID {
		t.Error("expected merge into existing stack")
	}
	if inst2.Quantity != 15 {
		t.Errorf("got Quantity=%d, want 15", inst2.Quantity)
	}
	if bp.UsedSlots() != 1 {
		t.Errorf("got UsedSlots=%d, want 1", bp.UsedSlots())
	}
}

func TestBackpack_Add_ExceedsMaxStack_NewSlot(t *testing.T) {
	def := stackDef("ammo", 0.1, 10)
	reg := makeRegistry(def)
	bp := inventory.NewBackpack(5, 100.0)

	_, err := bp.Add("ammo", 10, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inst2, err := bp.Add("ammo", 5, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp.UsedSlots() != 2 {
		t.Errorf("got UsedSlots=%d, want 2", bp.UsedSlots())
	}
	if inst2.Quantity != 5 {
		t.Errorf("got Quantity=%d, want 5", inst2.Quantity)
	}
}

func TestBackpack_Add_RejectsSlotOverflow(t *testing.T) {
	def := junkDef("rock", 0.1)
	reg := makeRegistry(def)
	bp := inventory.NewBackpack(2, 100.0)

	if _, err := bp.Add("rock", 1, reg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := bp.Add("rock", 1, reg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := bp.Add("rock", 1, reg)
	if err == nil {
		t.Error("expected error for slot overflow")
	}
}

func TestBackpack_Add_RejectsWeightOverflow(t *testing.T) {
	def := junkDef("boulder", 10.0)
	reg := makeRegistry(def)
	bp := inventory.NewBackpack(10, 15.0)

	if _, err := bp.Add("boulder", 1, reg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := bp.Add("boulder", 1, reg)
	if err == nil {
		t.Error("expected error for weight overflow")
	}
	// Verify state unchanged after rejection.
	if bp.UsedSlots() != 1 {
		t.Errorf("got UsedSlots=%d, want 1 (should be unchanged)", bp.UsedSlots())
	}
}

func TestBackpack_Remove_ByInstanceID(t *testing.T) {
	def := junkDef("rock", 1.0)
	reg := makeRegistry(def)
	bp := inventory.NewBackpack(5, 50.0)

	inst, _ := bp.Add("rock", 1, reg)
	err := bp.Remove(inst.InstanceID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bp.UsedSlots() != 0 {
		t.Errorf("got UsedSlots=%d, want 0", bp.UsedSlots())
	}
}

func TestBackpack_Remove_PartialStack(t *testing.T) {
	def := stackDef("ammo", 0.1, 50)
	reg := makeRegistry(def)
	bp := inventory.NewBackpack(5, 100.0)

	inst, _ := bp.Add("ammo", 20, reg)
	err := bp.Remove(inst.InstanceID, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items := bp.FindByItemDefID("ammo")
	if len(items) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(items))
	}
	if items[0].Quantity != 15 {
		t.Errorf("got Quantity=%d, want 15", items[0].Quantity)
	}
}

func TestBackpack_TotalWeight(t *testing.T) {
	rock := junkDef("rock", 2.5)
	ammo := stackDef("ammo", 0.1, 50)
	reg := makeRegistry(rock, ammo)
	bp := inventory.NewBackpack(10, 100.0)

	bp.Add("rock", 1, reg)
	bp.Add("ammo", 10, reg)

	got := bp.TotalWeight(reg)
	want := 2.5 + 10*0.1
	if got != want {
		t.Errorf("got TotalWeight=%f, want %f", got, want)
	}
}

func TestBackpack_FindByItemDefID(t *testing.T) {
	rock := junkDef("rock", 1.0)
	stick := junkDef("stick", 0.5)
	reg := makeRegistry(rock, stick)
	bp := inventory.NewBackpack(10, 100.0)

	bp.Add("rock", 1, reg)
	bp.Add("rock", 1, reg)
	bp.Add("stick", 1, reg)

	rocks := bp.FindByItemDefID("rock")
	if len(rocks) != 2 {
		t.Errorf("got %d rocks, want 2", len(rocks))
	}
	sticks := bp.FindByItemDefID("stick")
	if len(sticks) != 1 {
		t.Errorf("got %d sticks, want 1", len(sticks))
	}
	none := bp.FindByItemDefID("missing")
	if len(none) != 0 {
		t.Errorf("got %d for missing, want 0", len(none))
	}
}

func TestProperty_Backpack_NeverExceedsSlots(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxSlots := rapid.IntRange(1, 20).Draw(t, "maxSlots")
		bp := inventory.NewBackpack(maxSlots, 10000.0)
		def := junkDef("thing", 0.01)
		reg := makeRegistry(def)

		adds := rapid.IntRange(1, 50).Draw(t, "adds")
		for i := 0; i < adds; i++ {
			bp.Add("thing", 1, reg)
		}
		if bp.UsedSlots() > maxSlots {
			t.Fatalf("UsedSlots %d > MaxSlots %d", bp.UsedSlots(), maxSlots)
		}
	})
}

func TestProperty_Backpack_NeverExceedsWeight(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		maxWeight := rapid.Float64Range(1.0, 100.0).Draw(t, "maxWeight")
		bp := inventory.NewBackpack(100, maxWeight)
		w := rapid.Float64Range(0.1, 10.0).Draw(t, "weight")
		def := junkDef("thing", w)
		reg := makeRegistry(def)

		adds := rapid.IntRange(1, 50).Draw(t, "adds")
		for i := 0; i < adds; i++ {
			bp.Add("thing", 1, reg)
		}
		if bp.TotalWeight(reg) > maxWeight {
			t.Fatalf("TotalWeight %f > MaxWeight %f", bp.TotalWeight(reg), maxWeight)
		}
	})
}
