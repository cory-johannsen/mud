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

var materialAppliesTo = map[string][]string{
	"scrap_iron":      {"weapon"},
	"hollow_point":    {"weapon"},
	"carbide_alloy":   {"weapon", "armor"},
	"carbon_weave":    {"armor"},
	"polymer_frame":   {"armor"},
	"thermite_lace":   {"weapon"},
	"cryo_gel":        {"weapon"},
	"quantum_alloy":   {"weapon", "armor"},
	"rad_core":        {"weapon", "armor"},
	"neural_gel":      {"weapon", "armor"},
	"ghost_steel":     {"weapon", "armor"},
	"null_weave":      {"weapon", "armor"},
	"soul_guard_alloy": {"armor"},
	"shadow_plate":    {"weapon"},
	"radiance_plate":  {"weapon"},
}

var materialTier = map[string]string{
	"scrap_iron":      "common",
	"hollow_point":    "common",
	"carbide_alloy":   "common",
	"carbon_weave":    "common",
	"polymer_frame":   "uncommon",
	"thermite_lace":   "uncommon",
	"cryo_gel":        "uncommon",
	"quantum_alloy":   "uncommon",
	"rad_core":        "uncommon",
	"neural_gel":      "uncommon",
	"ghost_steel":     "uncommon",
	"null_weave":      "rare",
	"soul_guard_alloy": "rare",
	"shadow_plate":    "rare",
	"radiance_plate":  "rare",
}

func buildMaterialTestRegistry(rt *rapid.T) *inventory.Registry {
	reg := inventory.NewRegistry()
	for _, key := range allMaterialKeys() {
		parts := strings.SplitN(key, ":", 2)
		matID, gradeID := parts[0], parts[1]
		gradeName := inventory.GradeDisplayNames[gradeID]
		tier := materialTier[matID]
		if tier == "" {
			tier = "common"
		}
		appliesTo := materialAppliesTo[matID]
		if appliesTo == nil {
			appliesTo = []string{"weapon"}
		}
		def := &inventory.MaterialDef{
			MaterialID: matID, Name: matID,
			GradeID: gradeID, GradeName: gradeName,
			Tier: tier, AppliesTo: appliesTo,
		}
		_ = reg.RegisterMaterial(def)
	}
	return reg
}

func allMaterialKeys() []string {
	materials := []string{
		"scrap_iron", "hollow_point", "carbide_alloy", "carbon_weave", "polymer_frame",
		"thermite_lace", "cryo_gel", "quantum_alloy", "rad_core", "neural_gel",
		"ghost_steel", "null_weave", "soul_guard_alloy", "shadow_plate", "radiance_plate",
	}
	grades := []string{"street_grade", "mil_spec_grade", "ghost_grade"}
	var keys []string
	for _, m := range materials {
		for _, g := range grades {
			keys = append(keys, m+":"+g)
		}
	}
	return keys
}

func TestApplyMaterialEffects_ThermiteLace_Ghost_SetsOnFire(t *testing.T) {
	reg := inventory.NewRegistry()
	registerMaterial(t, reg, "thermite_lace", "ghost_grade", "uncommon", []string{"weapon"})
	ctx := inventory.AttackContext{IsHit: true}
	result := inventory.ApplyMaterialEffects([]string{"thermite_lace:ghost_grade"}, ctx, reg)
	assert.Equal(t, 4, result.PersistentFireDmg)
	assert.Equal(t, 2, result.DamageBonus)
	assert.True(t, result.ApplyOnFireCondition, "thermite_lace:ghost_grade on hit must set ApplyOnFireCondition")
}

func TestApplyMaterialEffects_RadCore_Ghost_SetsIrradiated(t *testing.T) {
	reg := inventory.NewRegistry()
	registerMaterial(t, reg, "rad_core", "ghost_grade", "uncommon", []string{"weapon", "armor"})
	ctx := inventory.AttackContext{IsHit: true}
	result := inventory.ApplyMaterialEffects([]string{"rad_core:ghost_grade"}, ctx, reg)
	assert.Equal(t, 4, result.PersistentRadDmg)
	assert.True(t, result.ApplyIrradiatedCondition, "rad_core:ghost_grade on hit must set ApplyIrradiatedCondition")
}

func TestComputePassiveMaterials_CarbonWeave_Ghost_SetsNoCheckPenalty(t *testing.T) {
	reg := inventory.NewRegistry()
	registerMaterial(t, reg, "carbon_weave", "ghost_grade", "uncommon", []string{"armor"})
	si := &inventory.SlottedItem{AffixedMaterials: []string{"carbon_weave:ghost_grade"}}
	armor := map[inventory.ArmorSlot]*inventory.SlottedItem{inventory.SlotTorso: si}
	summary := inventory.ComputePassiveMaterials(nil, armor, reg)
	assert.True(t, summary.NoCheckPenalty, "carbon_weave:ghost_grade must set NoCheckPenalty")
	assert.Equal(t, 10, summary.SpeedBonus)
}

func TestComputePassiveMaterials_SoulGuardAlloy_Ghost_MentalSentinel(t *testing.T) {
	reg := inventory.NewRegistry()
	registerMaterial(t, reg, "soul_guard_alloy", "ghost_grade", "rare", []string{"armor"})
	si := &inventory.SlottedItem{AffixedMaterials: []string{"soul_guard_alloy:ghost_grade"}}
	armor := map[inventory.ArmorSlot]*inventory.SlottedItem{inventory.SlotTorso: si}
	summary := inventory.ComputePassiveMaterials(nil, armor, reg)
	assert.Equal(t, 3, summary.SaveVsMentalBonus)
	assert.Contains(t, summary.ConditionImmunities, "*mental", "soul_guard_alloy:ghost_grade must append *mental sentinel")
}
