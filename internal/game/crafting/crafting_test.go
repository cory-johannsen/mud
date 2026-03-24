package crafting_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/crafting"
	"github.com/stretchr/testify/assert"
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
