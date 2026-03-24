package crafting_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/crafting"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestMaterialRegistry_LoadAndLookup(t *testing.T) {
	reg, err := crafting.LoadMaterialRegistry("../../../content/materials.yaml")
	assert.NoError(t, err)
	m, ok := reg.Material("scrap_metal")
	assert.True(t, ok)
	assert.Equal(t, "Scrap Metal", m.Name)
	assert.Equal(t, "mechanical", m.Category)
}

func TestMaterialRegistry_UnknownID(t *testing.T) {
	reg, err := crafting.LoadMaterialRegistry("../../../content/materials.yaml")
	assert.NoError(t, err)
	_, ok := reg.Material("not_a_real_material")
	assert.False(t, ok)
}

func TestRecipeRegistry_FatalOnUnknownMaterial(t *testing.T) {
	matReg, _ := crafting.LoadMaterialRegistry("../../../content/materials.yaml")
	_, err := crafting.LoadRecipeRegistry("testdata/bad_material_recipe/", matReg, nil)
	assert.Error(t, err) // REQ-CRAFT-6
}

func TestCraftingEngine_QuickCraft_CritSuccess(t *testing.T) {
	engine := crafting.NewEngine()
	recipe := &crafting.Recipe{OutputCount: 1}
	materials := map[string]int{"scrap_metal": 2}
	result := engine.ExecuteQuickCraft(recipe, materials, crafting.CritSuccess)
	assert.Equal(t, 2, result.OutputQuantity) // output_count + 1
	assert.True(t, result.MaterialsConsumed)
}

func TestCraftingEngine_QuickCraft_Failure(t *testing.T) {
	engine := crafting.NewEngine()
	recipe := &crafting.Recipe{Materials: []crafting.RecipeMaterial{{ID: "scrap_metal", Quantity: 4}}}
	materials := map[string]int{"scrap_metal": 4}
	result := engine.ExecuteQuickCraft(recipe, materials, crafting.Failure)
	assert.Equal(t, 0, result.OutputQuantity)
	assert.Equal(t, map[string]int{"scrap_metal": 2}, result.MaterialsDeducted) // floor(4/2)=2
}

func TestCraftingEngine_QuickCraft_CritFailure(t *testing.T) {
	engine := crafting.NewEngine()
	recipe := &crafting.Recipe{Materials: []crafting.RecipeMaterial{{ID: "scrap_metal", Quantity: 4}}}
	materials := map[string]int{"scrap_metal": 4}
	result := engine.ExecuteQuickCraft(recipe, materials, crafting.CritFailure)
	assert.Equal(t, 0, result.OutputQuantity)
	assert.Equal(t, map[string]int{"scrap_metal": 4}, result.MaterialsDeducted) // all consumed
}

func TestCraftingEngine_Property_CritSuccessAlwaysMoreOutput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		count := rapid.IntRange(1, 10).Draw(t, "count")
		recipe := &crafting.Recipe{OutputCount: count}
		engine := crafting.NewEngine()
		res := engine.ExecuteQuickCraft(recipe, nil, crafting.CritSuccess)
		assert.Equal(t, count+1, res.OutputQuantity)
	})
}
