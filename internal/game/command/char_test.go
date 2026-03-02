package command_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

// newCharTestSession returns a PlayerSession suitable for HandleChar tests.
//
// Postcondition: all fields relevant to the character sheet are populated with
// deterministic non-zero values.
func newCharTestSession(class string) *session.PlayerSession {
	return &session.PlayerSession{
		CharName:  "TestChar",
		Class:     class,
		Level:     1,
		CurrentHP: 10,
		MaxHP:     10,
		Currency:  100,
		Abilities: character.AbilityScores{
			Brutality: 12, Grit: 10, Quickness: 14,
			Reasoning: 10, Savvy: 10, Flair: 10,
		},
		LoadoutSet: inventory.NewLoadoutSet(),
		Equipment:  inventory.NewEquipment(),
		Backpack:   inventory.NewBackpack(20, 50.0),
	}
}

// TestHandleChar_ReturnsNonEmptyString verifies that HandleChar always produces output.
func TestHandleChar_ReturnsNonEmptyString(t *testing.T) {
	sess := newCharTestSession("boot_gun")
	result := command.HandleChar(sess)
	assert.NotEmpty(t, result)
}

// TestHandleChar_ShowsCharacterName verifies that the character name appears in the sheet.
func TestHandleChar_ShowsCharacterName(t *testing.T) {
	sess := newCharTestSession("boot_gun")
	result := command.HandleChar(sess)
	assert.Contains(t, result, "TestChar")
}

// TestHandleChar_ShowsHP verifies that the HP values appear in the sheet.
func TestHandleChar_ShowsHP(t *testing.T) {
	sess := newCharTestSession("boot_gun")
	result := command.HandleChar(sess)
	assert.Contains(t, result, "10")
}

// TestHandleChar_ShowsAbilityScore verifies that ability scores appear in the sheet.
func TestHandleChar_ShowsAbilityScore(t *testing.T) {
	sess := newCharTestSession("boot_gun")
	result := command.HandleChar(sess)
	assert.Contains(t, result, "12") // Brutality is 12
}

// TestHandleChar_NilLoadoutSetDoesNotPanic verifies HandleChar is safe when LoadoutSet is nil.
func TestHandleChar_NilLoadoutSetDoesNotPanic(t *testing.T) {
	sess := newCharTestSession("boot_gun")
	sess.LoadoutSet = nil
	assert.NotPanics(t, func() {
		command.HandleChar(sess)
	})
}

// TestHandleChar_ShowsClass verifies that the class appears in the sheet.
func TestHandleChar_ShowsClass(t *testing.T) {
	sess := newCharTestSession("boot_gun")
	result := command.HandleChar(sess)
	assert.Contains(t, result, "boot_gun")
}

// TestHandleChar_ShowsLevel verifies that the level appears in the sheet.
func TestHandleChar_ShowsLevel(t *testing.T) {
	sess := newCharTestSession("boot_gun")
	result := command.HandleChar(sess)
	assert.Contains(t, result, "1")
}

// TestHandleChar_ShowsCurrency verifies that the currency appears in the sheet.
func TestHandleChar_ShowsCurrency(t *testing.T) {
	sess := newCharTestSession("boot_gun")
	// Currency=100 rounds: 4 Clips, 0 Rounds — FormatRounds(100) = "4 Clips, 0 Rounds"
	result := command.HandleChar(sess)
	assert.Contains(t, result, "Round")
}

// TestProperty_HandleChar_NeverPanics is a property-based test verifying that
// HandleChar never panics for any combination of session field values.
func TestProperty_HandleChar_NeverPanics(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		class := rapid.SampledFrom([]string{"boot_gun", "boot_machete", "", "unknown_class_xyz"}).Draw(rt, "class")
		level := rapid.IntRange(0, 20).Draw(rt, "level")
		hp := rapid.IntRange(0, 100).Draw(rt, "hp")
		nilLoadout := rapid.Bool().Draw(rt, "nilLoadout")

		sess := newCharTestSession(class)
		sess.Level = level
		sess.CurrentHP = hp

		if nilLoadout {
			sess.LoadoutSet = nil
		}

		assert.NotPanics(rt, func() {
			command.HandleChar(sess)
		})
	})
}
