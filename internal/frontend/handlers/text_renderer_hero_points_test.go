package handlers_test

import (
	"strings"
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

// TestRenderCharacterSheet_PendingBoosts verifies pending boosts appear below Hero Points when > 0.
func TestRenderCharacterSheet_PendingBoosts(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:          "TestChar",
		HeroPoints:    1,
		PendingBoosts: 2,
	}
	rendered := handlers.RenderCharacterSheet(csv, 80)
	assert.Contains(t, rendered, "Pending Boosts: 2")
}

// TestRenderCharacterSheet_PendingBoosts_AboveAbilities verifies that the pending boosts line
// appears between Hero Points and the Abilities section (i.e., before "--- Abilities ---").
func TestRenderCharacterSheet_PendingBoosts_AboveAbilities(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:          "TestChar",
		HeroPoints:    1,
		PendingBoosts: 2,
	}
	rendered := handlers.RenderCharacterSheet(csv, 80)
	heroIdx := strings.Index(rendered, "Hero Points:")
	boostIdx := strings.Index(rendered, "Pending Boosts: 2")
	abilIdx := strings.Index(rendered, "--- Abilities ---")
	assert.Greater(t, boostIdx, heroIdx, "Pending Boosts should appear after Hero Points")
	assert.Less(t, boostIdx, abilIdx, "Pending Boosts should appear before Abilities section")
}

// TestRenderCharacterSheet_PendingSkillIncreases_AboveAbilities verifies that the pending skill
// increases line appears between Hero Points and the Abilities section.
func TestRenderCharacterSheet_PendingSkillIncreases_AboveAbilities(t *testing.T) {
	csv := &gamev1.CharacterSheetView{
		Name:                  "TestChar",
		PendingSkillIncreases: 3,
	}
	rendered := handlers.RenderCharacterSheet(csv, 80)
	heroIdx := strings.Index(rendered, "Hero Points:")
	skillIdx := strings.Index(rendered, "Pending Skill Increases: 3")
	abilIdx := strings.Index(rendered, "--- Abilities ---")
	assert.Greater(t, skillIdx, heroIdx, "Pending Skill Increases should appear after Hero Points")
	assert.Less(t, skillIdx, abilIdx, "Pending Skill Increases should appear before Abilities section")
}
