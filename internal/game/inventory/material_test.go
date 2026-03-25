package inventory_test

import (
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestDCConstants_CommonStreetGrade(t *testing.T) {
	assert.Equal(t, 16, inventory.DCCommonStreetGrade)
	assert.Equal(t, 21, inventory.DCCommonMilSpecGrade)
	assert.Equal(t, 26, inventory.DCCommonGhostGrade)
}

func TestDCConstants_UncommonTier(t *testing.T) {
	assert.Equal(t, 18, inventory.DCUncommonStreetGrade)
	assert.Equal(t, 23, inventory.DCUncommonMilSpecGrade)
	assert.Equal(t, 28, inventory.DCUncommonGhostGrade)
}

func TestDCConstants_RareTier(t *testing.T) {
	assert.Equal(t, 20, inventory.DCRareStreetGrade)
	assert.Equal(t, 25, inventory.DCRareMilSpecGrade)
	assert.Equal(t, 30, inventory.DCRareGhostGrade)
}

func TestRegistry_RegisterMaterial_AndLookup(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.MaterialDef{
		MaterialID: "scrap_iron", Name: "Scrap Iron",
		GradeID: "street_grade", GradeName: "Street Grade",
		Tier: "common", AppliesTo: []string{"weapon"},
	}
	err := reg.RegisterMaterial(def)
	assert.NoError(t, err)
	got, ok := reg.Material("scrap_iron", "street_grade")
	assert.True(t, ok)
	assert.Equal(t, "Scrap Iron", got.Name)
}

func TestRegistry_RegisterMaterial_InvalidAppliesTo(t *testing.T) {
	reg := inventory.NewRegistry()
	def := &inventory.MaterialDef{
		MaterialID: "x", Name: "X", GradeID: "street_grade",
		GradeName: "Street Grade", Tier: "common",
		AppliesTo: []string{"shield"}, // invalid
	}
	assert.Error(t, reg.RegisterMaterial(def))
}

func TestRegistry_RegisterMaterial_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := inventory.NewRegistry()
		matID := rapid.StringMatching(`[a-z_]{1,15}`).Draw(rt, "matID")
		gradeID := rapid.SampledFrom([]string{"street_grade", "mil_spec_grade", "ghost_grade"}).Draw(rt, "grade")
		def := &inventory.MaterialDef{
			MaterialID: matID, Name: "X",
			GradeID: gradeID, GradeName: "Grade",
			Tier: "common", AppliesTo: []string{"weapon"},
		}
		err := reg.RegisterMaterial(def)
		rt.Log("registerMaterial err:", err)
		// second register of same key must fail
		err2 := reg.RegisterMaterial(def)
		if err == nil {
			assert.Error(rt, err2, "duplicate registration must fail")
		}
	})
}

func TestApplyMaterialEffects_Property_NoEffectOnMiss(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := buildMaterialTestRegistry(rt)
		affixed := rapid.SliceOf(rapid.SampledFrom(allMaterialKeys())).Draw(rt, "affixed")
		ctx := inventory.AttackContext{IsHit: false}
		result := inventory.ApplyMaterialEffects(affixed, ctx, reg)
		// On a miss, no damage-on-hit effects should apply
		assert.Equal(rt, 0, result.PersistentFireDmg)
		assert.Equal(rt, 0, result.PersistentColdDmg)
		assert.Equal(rt, 0, result.PersistentBleedDmg)
		assert.False(rt, result.TargetFlatFooted)
		assert.False(rt, result.TargetSlowed)
	})
}

func TestApplyMaterialEffects_ScrapIron_HitsCyberTarget(t *testing.T) {
	reg := inventory.NewRegistry()
	registerMaterial(t, reg, "scrap_iron", "street_grade", "common", []string{"weapon"})
	registerMaterial(t, reg, "scrap_iron", "mil_spec_grade", "common", []string{"weapon"})
	registerMaterial(t, reg, "scrap_iron", "ghost_grade", "common", []string{"weapon"})

	ctx := inventory.AttackContext{TargetIsCyberAugmented: true, IsHit: true}

	r1 := inventory.ApplyMaterialEffects([]string{"scrap_iron:street_grade"}, ctx, reg)
	assert.Equal(t, 1, r1.DamageBonus)

	r2 := inventory.ApplyMaterialEffects([]string{"scrap_iron:mil_spec_grade"}, ctx, reg)
	assert.Equal(t, 2, r2.DamageBonus)
	assert.Equal(t, 1, r2.TargetLosesAP)

	r3 := inventory.ApplyMaterialEffects([]string{"scrap_iron:ghost_grade"}, ctx, reg)
	assert.Equal(t, 4, r3.DamageBonus)
	assert.True(t, r3.TargetFlatFooted)
}

func TestComputePassiveMaterials_CarbonWeave(t *testing.T) {
	reg := inventory.NewRegistry()
	registerMaterial(t, reg, "carbon_weave", "mil_spec_grade", "uncommon", []string{"armor"})

	si := &inventory.SlottedItem{
		AffixedMaterials: []string{"carbon_weave:mil_spec_grade"},
	}
	armor := map[inventory.ArmorSlot]*inventory.SlottedItem{
		inventory.SlotTorso: si,
	}
	summary := inventory.ComputePassiveMaterials(nil, armor, reg)
	assert.Equal(t, 2, summary.CheckPenaltyReduction)
	assert.Equal(t, 5, summary.SpeedBonus)
}

func TestComputePassiveMaterials_NullWeave_WeaponAndArmor_Accumulate(t *testing.T) {
	reg := inventory.NewRegistry()
	registerMaterial(t, reg, "null_weave", "street_grade", "rare", []string{"weapon", "armor"})

	ew := &inventory.EquippedWeapon{
		AffixedMaterials: []string{"null_weave:street_grade"},
	}
	si := &inventory.SlottedItem{
		AffixedMaterials: []string{"null_weave:street_grade"},
	}
	armor := map[inventory.ArmorSlot]*inventory.SlottedItem{
		inventory.SlotTorso: si,
	}
	summary := inventory.ComputePassiveMaterials([]*inventory.EquippedWeapon{ew}, armor, reg)
	assert.Equal(t, 2, summary.SaveVsTechBonus, "weapon + armor null_weave should accumulate")
}

func registerMaterial(t testing.TB, reg *inventory.Registry, matID, gradeID, tier string, appliesTo []string) {
	t.Helper()
	gradeName := inventory.GradeDisplayNames[gradeID]
	def := &inventory.MaterialDef{
		MaterialID: matID, Name: matID,
		GradeID: gradeID, GradeName: gradeName,
		Tier: tier, AppliesTo: appliesTo,
	}
	require.NoError(t, reg.RegisterMaterial(def))
}

func buildMaterialTestRegistry(rt *rapid.T) *inventory.Registry {
	reg := inventory.NewRegistry()
	for _, key := range allMaterialKeys() {
		parts := strings.SplitN(key, ":", 2)
		matID, gradeID := parts[0], parts[1]
		gradeName := inventory.GradeDisplayNames[gradeID]
		def := &inventory.MaterialDef{
			MaterialID: matID, Name: matID,
			GradeID: gradeID, GradeName: gradeName,
			Tier: "common", AppliesTo: []string{"weapon"},
		}
		_ = reg.RegisterMaterial(def)
	}
	return reg
}

func allMaterialKeys() []string {
	return []string{
		"scrap_iron:street_grade", "hollow_point:street_grade",
		"thermite_lace:mil_spec_grade", "cryo_gel:ghost_grade",
	}
}
