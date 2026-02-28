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
		_ = p.EquipOffHand(shieldDef("s"))
		_ = p.EquipMainHand(twoHandedDef("r"))
		if p.OffHand != nil {
			rt.Fatal("two-handed main must clear off-hand")
		}
	})
}

func TestProperty_LoadoutSet_SwapAlwaysSetsActive(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ls := inventory.NewLoadoutSet()
		idx := rapid.IntRange(0, len(ls.Presets)-1).Draw(rt, "idx")
		err := ls.Swap(idx)
		if err != nil {
			rt.Fatalf("unexpected error on valid swap: %v", err)
		}
		if ls.Active != idx {
			rt.Fatalf("expected Active=%d, got %d", idx, ls.Active)
		}
		if !ls.SwappedThisRound {
			rt.Fatal("SwappedThisRound must be true after swap")
		}
	})
}
