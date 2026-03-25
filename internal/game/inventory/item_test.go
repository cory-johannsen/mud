package inventory_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestItemDef_Validate_RejectsEmptyID(t *testing.T) {
	d := &inventory.ItemDef{
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

func TestItemDef_Validate_RejectsEmptyKind(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		MaxStack: 1,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for empty Kind, got nil")
	}
}

func TestItemDef_Validate_RejectsInvalidKind(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     "invalid",
		MaxStack: 1,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for invalid Kind, got nil")
	}
}

func TestItemDef_Validate_RejectsZeroMaxStack(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 0,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for MaxStack==0, got nil")
	}
}

func TestItemDef_Validate_RejectsNegativeWeight(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
		Weight:   -1.0,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for negative Weight, got nil")
	}
}

func TestItemDef_Validate_AcceptsMinimalJunk(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("expected no error for minimal junk, got: %v", err)
	}
}

func TestItemDef_Validate_AcceptsWeaponRef(t *testing.T) {
	d := &inventory.ItemDef{
		ID:        "sword_item",
		Name:      "Sword",
		Kind:      inventory.KindWeapon,
		MaxStack:  1,
		WeaponRef: "sword_def",
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("expected no error for weapon with ref, got: %v", err)
	}
}

func TestItemDef_Validate_RejectsWeaponWithoutRef(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "sword_item",
		Name:     "Sword",
		Kind:     inventory.KindWeapon,
		MaxStack: 1,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for weapon without WeaponRef, got nil")
	}
}

func TestItemDef_Validate_RejectsExplosiveWithoutRef(t *testing.T) {
	d := &inventory.ItemDef{
		ID:       "grenade_item",
		Name:     "Grenade",
		Kind:     inventory.KindExplosive,
		MaxStack: 1,
	}
	if err := d.Validate(); err == nil {
		t.Fatal("expected error for explosive without ExplosiveRef, got nil")
	}
}

func TestItemDef_Validate_AcceptsStackable(t *testing.T) {
	d := &inventory.ItemDef{
		ID:        "ammo",
		Name:      "Ammo Box",
		Kind:      inventory.KindConsumable,
		MaxStack:  20,
		Stackable: true,
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("expected no error for stackable item, got: %v", err)
	}
}

func TestLoadItems_LoadsYAML(t *testing.T) {
	dir := t.TempDir()
	content := `id: scrap_metal
name: Scrap Metal
description: A piece of scrap metal.
kind: junk
weight: 0.5
stackable: true
max_stack: 10
value: 5
`
	if err := os.WriteFile(filepath.Join(dir, "scrap.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp YAML: %v", err)
	}
	// Also write a .yml file to verify both extensions are loaded.
	content2 := `id: bandage
name: Bandage
description: A simple bandage.
kind: consumable
weight: 0.1
stackable: true
max_stack: 5
value: 10
`
	if err := os.WriteFile(filepath.Join(dir, "bandage.yml"), []byte(content2), 0644); err != nil {
		t.Fatalf("failed to write temp YAML: %v", err)
	}

	items, err := inventory.LoadItems(dir)
	if err != nil {
		t.Fatalf("LoadItems failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	byID := make(map[string]*inventory.ItemDef)
	for _, item := range items {
		byID[item.ID] = item
	}

	scrap, ok := byID["scrap_metal"]
	if !ok {
		t.Fatal("expected scrap_metal item")
	}
	if scrap.Name != "Scrap Metal" {
		t.Errorf("expected Name 'Scrap Metal', got %q", scrap.Name)
	}
	if scrap.Weight != 0.5 {
		t.Errorf("expected Weight 0.5, got %f", scrap.Weight)
	}
	if scrap.MaxStack != 10 {
		t.Errorf("expected MaxStack 10, got %d", scrap.MaxStack)
	}

	bandage, ok := byID["bandage"]
	if !ok {
		t.Fatal("expected bandage item")
	}
	if bandage.Kind != inventory.KindConsumable {
		t.Errorf("expected Kind 'consumable', got %q", bandage.Kind)
	}
}

func TestRegistry_Item_Lookup(t *testing.T) {
	r := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
	}
	if err := r.RegisterItem(def); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := r.Item(def.ID)
	if !ok {
		t.Fatal("expected item to be found")
	}
	if got.ID != def.ID {
		t.Fatalf("expected ID=%q, got %q", def.ID, got.ID)
	}
}

func TestRegistry_RegisterItem_RejectsDuplicate(t *testing.T) {
	r := inventory.NewRegistry()
	def := &inventory.ItemDef{
		ID:       "junk1",
		Name:     "Junk",
		Kind:     inventory.KindJunk,
		MaxStack: 1,
	}
	if err := r.RegisterItem(def); err != nil {
		t.Fatalf("unexpected error on first register: %v", err)
	}
	if err := r.RegisterItem(def); err == nil {
		t.Fatal("expected collision error on second register, got nil")
	}
}

func TestRegistry_Item_NotFound(t *testing.T) {
	r := inventory.NewRegistry()
	_, ok := r.Item("does-not-exist")
	if ok {
		t.Fatal("expected ok==false for missing item")
	}
}

func TestKindTrap_Constant(t *testing.T) {
	if inventory.KindTrap != "trap" {
		t.Fatalf("expected KindTrap == %q, got %q", "trap", inventory.KindTrap)
	}
}

func TestItemDef_TrapTemplateRef_Field(t *testing.T) {
	item := &inventory.ItemDef{
		ID:              "deployable_mine",
		Name:            "Deployable Mine",
		Kind:            inventory.KindTrap,
		TrapTemplateRef: "mine",
		Weight:          2.0,
		Stackable:       true,
		MaxStack:        5,
		Value:           300,
	}
	if item.TrapTemplateRef != "mine" {
		t.Fatalf("expected TrapTemplateRef == %q, got %q", "mine", item.TrapTemplateRef)
	}
}

func TestRegistry_RegisterItem_TrapKind_RequiresTrapTemplateRef(t *testing.T) {
	reg := inventory.NewRegistry()
	err := reg.RegisterItem(&inventory.ItemDef{
		ID:   "bad_trap",
		Name: "Bad Trap",
		Kind: inventory.KindTrap,
		// TrapTemplateRef intentionally missing — must fail
	})
	if err == nil {
		t.Fatal("expected error for empty TrapTemplateRef with kind=trap, got nil")
	}
}

func TestRegistry_RegisterItem_TrapKind_ValidRef(t *testing.T) {
	reg := inventory.NewRegistry()
	err := reg.RegisterItem(&inventory.ItemDef{
		ID:              "good_trap",
		Name:            "Good Trap",
		Kind:            inventory.KindTrap,
		TrapTemplateRef: "mine",
		Weight:          1.0,
		Stackable:       true,
		MaxStack:        5,
		Value:           100,
	})
	if err != nil {
		t.Fatalf("expected no error for valid trap item, got: %v", err)
	}
	item, ok := reg.Item("good_trap")
	if !ok {
		t.Fatal("expected good_trap to be found in registry")
	}
	if item.TrapTemplateRef != "mine" {
		t.Fatalf("expected TrapTemplateRef == %q, got %q", "mine", item.TrapTemplateRef)
	}
}

// ── REQ-EM-40: ValidateRequiredConsumables ────────────────────────────────────

func TestValidateRequiredConsumables_AllPresent(t *testing.T) {
	items := requiredConsumableItems()
	if err := inventory.ValidateRequiredConsumables(items); err != nil {
		t.Fatalf("expected no error when all 6 required consumables present, got: %v", err)
	}
}

func TestValidateRequiredConsumables_MissingOne(t *testing.T) {
	items := requiredConsumableItems()
	// Remove repair_kit
	filtered := make([]*inventory.ItemDef, 0, len(items))
	for _, it := range items {
		if it.ID != "repair_kit" {
			filtered = append(filtered, it)
		}
	}
	err := inventory.ValidateRequiredConsumables(filtered)
	if err == nil {
		t.Fatal("expected error for missing repair_kit, got nil")
	}
	if !strings.Contains(err.Error(), "repair_kit") {
		t.Errorf("error should mention missing ID 'repair_kit', got: %v", err)
	}
}

func TestValidateRequiredConsumables_Empty(t *testing.T) {
	err := inventory.ValidateRequiredConsumables(nil)
	if err == nil {
		t.Fatal("expected error for empty items slice, got nil")
	}
}

// requiredConsumableItems returns a minimal slice containing all 6 required consumable IDs.
func requiredConsumableItems() []*inventory.ItemDef {
	ids := []string{
		"whores_pasta",
		"poontangesca",
		"four_loko",
		"old_english",
		"penjamin_franklin",
		"repair_kit",
	}
	items := make([]*inventory.ItemDef, len(ids))
	for i, id := range ids {
		items[i] = &inventory.ItemDef{
			ID:       id,
			Name:     id,
			Kind:     inventory.KindConsumable,
			MaxStack: 1,
			Weight:   0.1,
		}
	}
	return items
}

// ── REQ-EM-42: ValidateConsumableEffects ─────────────────────────────────────

func TestValidateConsumableEffects_NoConsumableEffects(t *testing.T) {
	items := []*inventory.ItemDef{
		{ID: "junk1", Name: "Junk", Kind: inventory.KindJunk, MaxStack: 1},
	}
	knownConds := map[string]bool{"fatigued": true}
	if err := inventory.ValidateConsumableEffects(items, knownConds); err != nil {
		t.Fatalf("expected no error for items with no consumable effects, got: %v", err)
	}
}

func TestValidateConsumableEffects_KnownDiseaseID(t *testing.T) {
	items := []*inventory.ItemDef{
		{
			ID: "bad_food", Name: "Bad Food", Kind: inventory.KindConsumable, MaxStack: 1,
			Effect: &inventory.ConsumableEffect{
				ConsumeCheck: &inventory.ConsumeCheck{
					Stat: "constitution", DC: 12,
					OnCriticalFailure: &inventory.CritFailureEffect{
						ApplyDisease: &inventory.DiseaseEffect{DiseaseID: "street_rot", Severity: 1},
					},
				},
			},
		},
	}
	knownConds := map[string]bool{"street_rot": true}
	if err := inventory.ValidateConsumableEffects(items, knownConds); err != nil {
		t.Fatalf("expected no error for known disease_id, got: %v", err)
	}
}

func TestValidateConsumableEffects_UnknownDiseaseID(t *testing.T) {
	items := []*inventory.ItemDef{
		{
			ID: "bad_food", Name: "Bad Food", Kind: inventory.KindConsumable, MaxStack: 1,
			Effect: &inventory.ConsumableEffect{
				ConsumeCheck: &inventory.ConsumeCheck{
					Stat: "constitution", DC: 12,
					OnCriticalFailure: &inventory.CritFailureEffect{
						ApplyDisease: &inventory.DiseaseEffect{DiseaseID: "unknown_plague", Severity: 1},
					},
				},
			},
		},
	}
	knownConds := map[string]bool{"street_rot": true}
	err := inventory.ValidateConsumableEffects(items, knownConds)
	if err == nil {
		t.Fatal("expected error for unknown disease_id, got nil")
	}
	if !strings.Contains(err.Error(), "unknown_plague") {
		t.Errorf("error should mention unknown ID 'unknown_plague', got: %v", err)
	}
}

func TestValidateConsumableEffects_UnknownToxinID(t *testing.T) {
	items := []*inventory.ItemDef{
		{
			ID: "poison", Name: "Poison", Kind: inventory.KindConsumable, MaxStack: 1,
			Effect: &inventory.ConsumableEffect{
				ConsumeCheck: &inventory.ConsumeCheck{
					Stat: "constitution", DC: 15,
					OnCriticalFailure: &inventory.CritFailureEffect{
						ApplyToxin: &inventory.ToxinEffect{ToxinID: "mystery_toxin", Severity: 2},
					},
				},
			},
		},
	}
	knownConds := map[string]bool{"gut_rot": true}
	err := inventory.ValidateConsumableEffects(items, knownConds)
	if err == nil {
		t.Fatal("expected error for unknown toxin_id, got nil")
	}
	if !strings.Contains(err.Error(), "mystery_toxin") {
		t.Errorf("error should mention unknown ID 'mystery_toxin', got: %v", err)
	}
}

func TestValidateConsumableEffects_KnownToxinID(t *testing.T) {
	items := []*inventory.ItemDef{
		{
			ID: "poison", Name: "Poison", Kind: inventory.KindConsumable, MaxStack: 1,
			Effect: &inventory.ConsumableEffect{
				ConsumeCheck: &inventory.ConsumeCheck{
					Stat: "constitution", DC: 15,
					OnCriticalFailure: &inventory.CritFailureEffect{
						ApplyToxin: &inventory.ToxinEffect{ToxinID: "gut_rot", Severity: 2},
					},
				},
			},
		},
	}
	knownConds := map[string]bool{"gut_rot": true}
	if err := inventory.ValidateConsumableEffects(items, knownConds); err != nil {
		t.Fatalf("expected no error for known toxin_id, got: %v", err)
	}
}

func TestProperty_ItemDef_ValidKind_AcceptsAll(t *testing.T) {
	kinds := []string{
		inventory.KindWeapon,
		inventory.KindExplosive,
		inventory.KindConsumable,
		inventory.KindJunk,
	}
	rapid.Check(t, func(rt *rapid.T) {
		kind := rapid.SampledFrom(kinds).Draw(rt, "kind")
		d := &inventory.ItemDef{
			ID:       rapid.StringMatching(`[a-z][a-z0-9_]{2,19}`).Draw(rt, "id"),
			Name:     rapid.StringMatching(`[A-Z][a-zA-Z ]{2,29}`).Draw(rt, "name"),
			Kind:     kind,
			MaxStack: rapid.IntRange(1, 100).Draw(rt, "max_stack"),
			Weight:   rapid.Float64Range(0, 100).Draw(rt, "weight"),
		}
		if kind == inventory.KindWeapon {
			d.WeaponRef = "weapon_" + d.ID
		}
		if kind == inventory.KindExplosive {
			d.ExplosiveRef = "explosive_" + d.ID
		}
		if err := d.Validate(); err != nil {
			rt.Fatalf("expected valid ItemDef to pass validation, got: %v", err)
		}
	})
}

func TestItemDef_SubstanceID_Field_Stored(t *testing.T) {
	d := inventory.ItemDef{
		ID: "stimpak_item", Name: "Stimpak", Kind: inventory.KindConsumable,
		MaxStack: 10, SubstanceID: "stimpak",
	}
	err := d.Validate()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if d.SubstanceID != "stimpak" {
		t.Fatalf("SubstanceID = %q, want %q", d.SubstanceID, "stimpak")
	}
}

func TestItemDef_PoisonSubstanceID_Field_Stored(t *testing.T) {
	d := inventory.ItemDef{
		ID: "poison_dagger", Name: "Poison Dagger", Kind: inventory.KindWeapon,
		MaxStack: 1, WeaponRef: "dagger", PoisonSubstanceID: "viper_venom",
	}
	err := d.Validate()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if d.PoisonSubstanceID != "viper_venom" {
		t.Fatalf("PoisonSubstanceID = %q, want %q", d.PoisonSubstanceID, "viper_venom")
	}
}

func TestItemDef_Validate_ActivationFields_Property(t *testing.T) {
	t.Run("valid activation_cost with charges always passes", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			cost := rapid.IntRange(1, 3).Draw(rt, "cost")
			charges := rapid.IntRange(1, 10).Draw(rt, "charges")
			d := &inventory.ItemDef{ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1,
				ActivationCost: cost, Charges: charges}
			assert.NoError(rt, d.Validate())
		})
	})
	t.Run("nonzero activation_cost with zero charges always errors", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			cost := rapid.IntRange(1, 3).Draw(rt, "cost")
			d := &inventory.ItemDef{ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1,
				ActivationCost: cost, Charges: 0}
			assert.Error(rt, d.Validate())
		})
	})
	t.Run("out-of-range activation_cost always errors", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			// Draw from negative range or >3 range
			cost := rapid.OneOf(
				rapid.IntRange(-10, -1),
				rapid.IntRange(4, 10),
			).Draw(rt, "bad_cost")
			d := &inventory.ItemDef{ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1,
				ActivationCost: cost, Charges: 1}
			assert.Error(rt, d.Validate())
		})
	})
	t.Run("valid recharge entries with known triggers always pass", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			trigger := rapid.SampledFrom([]string{"daily", "midnight", "dawn", "rest"}).Draw(rt, "trigger")
			amount := rapid.IntRange(1, 5).Draw(rt, "amount")
			d := &inventory.ItemDef{ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1,
				ActivationCost: 1, Charges: 1,
				Recharge: []inventory.RechargeEntry{{Trigger: trigger, Amount: amount}},
			}
			assert.NoError(rt, d.Validate())
		})
	})
}

func TestItemDef_Validate_ActivationFields(t *testing.T) {
	t.Run("activation_cost out of range", func(t *testing.T) {
		d := &inventory.ItemDef{ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1, ActivationCost: 4, Charges: 1}
		assert.Error(t, d.Validate())
	})
	t.Run("activation_cost valid max", func(t *testing.T) {
		d := &inventory.ItemDef{ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1, ActivationCost: 3, Charges: 1}
		assert.NoError(t, d.Validate())
	})
	t.Run("charges zero when cost nonzero", func(t *testing.T) {
		d := &inventory.ItemDef{ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1, ActivationCost: 1, Charges: 0}
		assert.Error(t, d.Validate())
	})
	t.Run("on_deplete invalid value", func(t *testing.T) {
		d := &inventory.ItemDef{ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1, ActivationCost: 1, Charges: 1, OnDeplete: "vanish"}
		assert.Error(t, d.Validate())
	})
	t.Run("on_deplete empty is valid (defaults to expend)", func(t *testing.T) {
		d := &inventory.ItemDef{ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1, ActivationCost: 1, Charges: 1}
		assert.NoError(t, d.Validate())
	})
	t.Run("script and effect both set", func(t *testing.T) {
		d := &inventory.ItemDef{
			ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1,
			ActivationCost: 1, Charges: 1,
			ActivationScript: "my_hook",
			ActivationEffect: &inventory.ConsumableEffect{},
		}
		assert.Error(t, d.Validate())
	})
	t.Run("recharge unknown trigger", func(t *testing.T) {
		d := &inventory.ItemDef{
			ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1,
			ActivationCost: 1, Charges: 1,
			Recharge: []inventory.RechargeEntry{{Trigger: "sunrise", Amount: 1}},
		}
		assert.Error(t, d.Validate())
	})
	t.Run("recharge amount zero", func(t *testing.T) {
		d := &inventory.ItemDef{
			ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1,
			ActivationCost: 1, Charges: 1,
			Recharge: []inventory.RechargeEntry{{Trigger: "dawn", Amount: 0}},
		}
		assert.Error(t, d.Validate())
	})
	t.Run("recharge valid multi-trigger", func(t *testing.T) {
		d := &inventory.ItemDef{
			ID: "x", Name: "x", Kind: inventory.KindConsumable, MaxStack: 1,
			ActivationCost: 2, Charges: 3,
			Recharge: []inventory.RechargeEntry{
				{Trigger: "dawn", Amount: 1},
				{Trigger: "rest", Amount: 2},
			},
		}
		assert.NoError(t, d.Validate())
	})
}

func TestItemDef_Validate_PreciousMaterial_InvalidAppliesTo(t *testing.T) {
	d := &inventory.ItemDef{
		ID: "x_street_grade", Name: "X", Kind: inventory.KindPreciousMaterial,
		MaterialID: "x", GradeID: "street_grade",
		MaterialName: "X", MaterialTier: "common",
		AppliesTo: []string{"consumable"}, // invalid
		MaxStack: 1,
	}
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "applies_to")
}

func TestItemDef_Validate_PreciousMaterial_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		gradeID := rapid.SampledFrom([]string{"street_grade", "mil_spec_grade", "ghost_grade"}).Draw(rt, "grade")
		tierID := rapid.SampledFrom([]string{"common", "uncommon", "rare"}).Draw(rt, "tier")
		appliesTo := rapid.SampledFrom([][]string{{"weapon"}, {"armor"}, {"weapon", "armor"}}).Draw(rt, "appliesTo")
		d := &inventory.ItemDef{
			ID: "x_" + gradeID, Name: "X", Kind: inventory.KindPreciousMaterial,
			MaterialID: "x", GradeID: gradeID,
			MaterialName: "X", MaterialTier: tierID,
			AppliesTo: appliesTo,
			MaxStack: 1,
		}
		err := d.Validate()
		if err != nil {
			rt.Fatalf("valid precious_material ItemDef failed Validate(): %v", err)
		}
	})
}

func TestItemDef_Validate_PreciousMaterial_Missing_Fields(t *testing.T) {
	// Missing material_id
	d := &inventory.ItemDef{ID: "x", Name: "X", Kind: inventory.KindPreciousMaterial}
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "material_id")
}

func TestItemDef_Validate_PreciousMaterial_InvalidGradeID(t *testing.T) {
	d := &inventory.ItemDef{
		ID: "x_street_grade", Name: "X", Kind: inventory.KindPreciousMaterial,
		MaterialID: "x", GradeID: "not_valid", MaterialName: "X",
		MaterialTier: "common", AppliesTo: []string{"weapon"},
	}
	err := d.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "grade_id")
}

func TestItemDef_Validate_PreciousMaterial_Valid(t *testing.T) {
	d := &inventory.ItemDef{
		ID: "scrap_iron_street_grade", Name: "Scrap Iron (Street Grade)",
		Kind:         inventory.KindPreciousMaterial,
		MaterialID:   "scrap_iron", GradeID: "street_grade",
		MaterialName: "Scrap Iron", MaterialTier: "common",
		AppliesTo: []string{"weapon"},
		MaxStack: 1,
	}
	assert.NoError(t, d.Validate())
}

func TestLoadPreciousMaterials_LoadsAll45(t *testing.T) {
	reg := inventory.NewRegistry()
	err := inventory.LoadPreciousMaterials(reg, "../../../content/items/precious_materials")
	require.NoError(t, err)
	materials := []string{
		"scrap_iron", "hollow_point", "carbide_alloy", "carbon_weave", "polymer_frame",
		"thermite_lace", "cryo_gel", "quantum_alloy", "rad_core", "neural_gel",
		"ghost_steel", "null_weave", "soul_guard_alloy", "shadow_plate", "radiance_plate",
	}
	grades := []string{"street_grade", "mil_spec_grade", "ghost_grade"}
	for _, matID := range materials {
		for _, gradeID := range grades {
			_, ok := reg.Material(matID, gradeID)
			assert.True(t, ok, "material %s:%s should be registered", matID, gradeID)
		}
	}
}

func TestLoadPreciousMaterials_MissingFile_ReturnsError(t *testing.T) {
	reg := inventory.NewRegistry()
	err := inventory.LoadPreciousMaterials(reg, "/nonexistent/dir")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}
