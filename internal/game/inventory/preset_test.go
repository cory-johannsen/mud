package inventory_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

func oneHandedDef(id string) *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID: id, Name: id, DamageDice: "1d6", DamageType: "slashing",
		Kind: inventory.WeaponKindOneHanded,
	}
}

func twoHandedDef(id string) *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID: id, Name: id, DamageDice: "2d8", DamageType: "slashing",
		Kind: inventory.WeaponKindTwoHanded,
	}
}

func shieldDef(id string) *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID: id, Name: id, DamageDice: "1d4", DamageType: "bludgeoning",
		Kind: inventory.WeaponKindShield,
	}
}

func TestWeaponPreset_EquipMainHand_OneHanded(t *testing.T) {
	p := inventory.NewWeaponPreset()
	if err := p.EquipMainHand(oneHandedDef("sword")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.MainHand == nil || p.MainHand.Def.ID != "sword" {
		t.Fatal("expected MainHand equipped with sword")
	}
}

func TestWeaponPreset_EquipMainHand_TwoHanded_ClearsOffHand(t *testing.T) {
	p := inventory.NewWeaponPreset()
	_ = p.EquipOffHand(shieldDef("shield"))
	if err := p.EquipMainHand(twoHandedDef("rifle")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.OffHand != nil {
		t.Fatal("two-handed main hand must clear off-hand")
	}
}

func TestWeaponPreset_EquipOffHand_Shield_BlockedByTwoHandedMain(t *testing.T) {
	p := inventory.NewWeaponPreset()
	_ = p.EquipMainHand(twoHandedDef("rifle"))
	err := p.EquipOffHand(shieldDef("shield"))
	if err == nil {
		t.Fatal("expected error: shield in off-hand blocked by two-handed main")
	}
}

func TestWeaponPreset_EquipOffHand_Shield_WithOneHandedMain(t *testing.T) {
	p := inventory.NewWeaponPreset()
	_ = p.EquipMainHand(oneHandedDef("sword"))
	if err := p.EquipOffHand(shieldDef("shield")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWeaponPreset_EquipOffHand_OneHanded_DualWield(t *testing.T) {
	p := inventory.NewWeaponPreset()
	_ = p.EquipMainHand(oneHandedDef("sword"))
	if err := p.EquipOffHand(oneHandedDef("knife")); err != nil {
		t.Fatalf("unexpected error dual wield: %v", err)
	}
}

func TestWeaponPreset_EquipOffHand_TwoHandedWeapon_Rejected(t *testing.T) {
	p := inventory.NewWeaponPreset()
	err := p.EquipOffHand(twoHandedDef("rifle"))
	if err == nil {
		t.Fatal("expected error: two-handed weapon not allowed in off-hand")
	}
}

func TestWeaponPreset_UnequipMainHand(t *testing.T) {
	p := inventory.NewWeaponPreset()
	_ = p.EquipMainHand(oneHandedDef("sword"))
	p.UnequipMainHand()
	if p.MainHand != nil {
		t.Fatal("expected nil MainHand after unequip")
	}
}

func TestWeaponPreset_UnequipOffHand(t *testing.T) {
	p := inventory.NewWeaponPreset()
	_ = p.EquipOffHand(shieldDef("shield"))
	p.UnequipOffHand()
	if p.OffHand != nil {
		t.Fatal("expected nil OffHand after unequip")
	}
}

func TestWeaponPreset_EquipMainHand_NilDefReturnsError(t *testing.T) {
	p := inventory.NewWeaponPreset()
	if err := p.EquipMainHand(nil); err == nil {
		t.Fatal("expected error for nil def")
	}
}

func TestWeaponPreset_EquipOffHand_NilDefReturnsError(t *testing.T) {
	p := inventory.NewWeaponPreset()
	if err := p.EquipOffHand(nil); err == nil {
		t.Fatal("expected error for nil def")
	}
}

func TestLoadoutSet_NewHasTwoPresets(t *testing.T) {
	ls := inventory.NewLoadoutSet()
	if len(ls.Presets) != 2 {
		t.Fatalf("expected 2 presets, got %d", len(ls.Presets))
	}
	if ls.Active != 0 {
		t.Fatalf("expected Active=0, got %d", ls.Active)
	}
	if ls.SwappedThisRound {
		t.Fatal("expected SwappedThisRound=false on new LoadoutSet")
	}
}

func TestLoadoutSet_Swap_SetsActive(t *testing.T) {
	ls := inventory.NewLoadoutSet()
	if err := ls.Swap(1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ls.Active != 1 {
		t.Fatalf("expected Active=1, got %d", ls.Active)
	}
	if !ls.SwappedThisRound {
		t.Fatal("expected SwappedThisRound=true after swap")
	}
}

func TestLoadoutSet_Swap_BlockedIfAlreadySwapped(t *testing.T) {
	ls := inventory.NewLoadoutSet()
	ls.SwappedThisRound = true
	if err := ls.Swap(1); err == nil {
		t.Fatal("expected error: already swapped this round")
	}
}

func TestLoadoutSet_Swap_InvalidIndex(t *testing.T) {
	ls := inventory.NewLoadoutSet()
	if err := ls.Swap(5); err == nil {
		t.Fatal("expected error for out-of-range preset index")
	}
	if err := ls.Swap(-1); err == nil {
		t.Fatal("expected error for negative preset index")
	}
}

func TestLoadoutSet_ResetRound(t *testing.T) {
	ls := inventory.NewLoadoutSet()
	ls.SwappedThisRound = true
	ls.ResetRound()
	if ls.SwappedThisRound {
		t.Fatal("expected SwappedThisRound=false after ResetRound")
	}
}

func TestLoadoutSet_ActivePreset_ReturnsCorrectPreset(t *testing.T) {
	ls := inventory.NewLoadoutSet()
	p := ls.ActivePreset()
	if p == nil {
		t.Fatal("expected non-nil active preset")
	}
	if p != ls.Presets[0] {
		t.Fatal("expected ActivePreset to return Presets[0] when Active=0")
	}
}

func TestLoadoutSet_ActivePreset_ReturnsSecondAfterSwap(t *testing.T) {
	ls := inventory.NewLoadoutSet()
	_ = ls.Swap(1)
	p := ls.ActivePreset()
	if p != ls.Presets[1] {
		t.Fatal("expected ActivePreset to return Presets[1] after swap to index 1")
	}
}

func TestProperty_WeaponPreset_TwoHandedAlwaysClearsOffHand(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		p := inventory.NewWeaponPreset()

		// Draw a random off-hand item (one-handed or shield)
		offHandID := rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "off_hand_id")
		offHandKind := rapid.SampledFrom([]inventory.WeaponKind{
			inventory.WeaponKindOneHanded,
			inventory.WeaponKindShield,
		}).Draw(rt, "off_hand_kind")
		offDef := &inventory.WeaponDef{
			ID:         offHandID,
			Name:       offHandID,
			DamageDice: "1d6",
			DamageType: "slashing",
			Kind:       offHandKind,
		}
		_ = p.EquipOffHand(offDef)

		// Draw a random two-handed weapon
		mainID := rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "main_id")
		mainDef := &inventory.WeaponDef{
			ID:         mainID,
			Name:       mainID,
			DamageDice: "2d8",
			DamageType: "slashing",
			Kind:       inventory.WeaponKindTwoHanded,
		}
		_ = p.EquipMainHand(mainDef)

		if p.OffHand != nil {
			rt.Fatalf("two-handed main must clear off-hand (off_hand_kind=%q, main_id=%q)", offHandKind, mainID)
		}
	})
}

func TestProperty_LoadoutSet_SwapAlwaysSetsActive(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Draw number of presets (2-8, simulating class-feature expansion)
		numPresets := rapid.IntRange(2, 8).Draw(rt, "num_presets")
		presets := make([]*inventory.WeaponPreset, numPresets)
		for i := range presets {
			presets[i] = inventory.NewWeaponPreset()
		}
		ls := &inventory.LoadoutSet{Presets: presets}

		idx := rapid.IntRange(0, numPresets-1).Draw(rt, "idx")
		// Skip same-index (no-op case)
		if idx == ls.Active {
			return
		}
		err := ls.Swap(idx)
		if err != nil {
			rt.Fatalf("unexpected error on valid swap to index %d: %v", idx, err)
		}
		if ls.Active != idx {
			rt.Fatalf("expected Active=%d, got %d", idx, ls.Active)
		}
		if !ls.SwappedThisRound {
			rt.Fatal("SwappedThisRound must be true after swap")
		}
	})
}

func TestLoadoutSet_ActivePreset_OutOfRange_ReturnsNil(t *testing.T) {
	ls := inventory.NewLoadoutSet()
	ls.Active = 999
	if p := ls.ActivePreset(); p != nil {
		t.Fatal("expected nil ActivePreset for out-of-range Active index")
	}
}

func TestLoadoutSet_Swap_SameIndex_IsNoop(t *testing.T) {
	ls := inventory.NewLoadoutSet()
	if err := ls.Swap(0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// SwappedThisRound must still be false since it's a no-op
	if ls.SwappedThisRound {
		t.Fatal("same-index swap must not consume the round swap allowance")
	}
}
