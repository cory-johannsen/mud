package handlers_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/frontend/handlers"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
)

// TestRenderCharacterSheet_ShowsHeroPoints verifies that RenderCharacterSheet includes
// a "Hero Points: N" line when HeroPoints is set.
func TestRenderCharacterSheet_ShowsHeroPoints(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:       "TestChar",
		Job:        "Ranger",
		Level:      5,
		CurrentHp:  30,
		MaxHp:      40,
		HeroPoints: 3,
	}
	rendered := handlers.RenderCharacterSheet(csv, 80)
	assert.Contains(t, rendered, "Hero Points: 3")
}

// TestRenderCharacterSheet_ZeroHeroPoints verifies "Hero Points: 0" is shown.
func TestRenderCharacterSheet_ZeroHeroPoints(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:       "TestChar",
		HeroPoints: 0,
	}
	rendered := handlers.RenderCharacterSheet(csv, 80)
	assert.Contains(t, rendered, "Hero Points: 0")
}
