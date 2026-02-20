package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/frontend/handlers"
)

// mockCharStore implements handlers.CharacterStore for testing.
type mockCharStore struct {
	chars     []*character.Character
	created   *character.Character
	createErr error
	listErr   error
}

func (m *mockCharStore) ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.chars, nil
}

func (m *mockCharStore) Create(ctx context.Context, c *character.Character) (*character.Character, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	c.ID = 42
	m.created = c
	return c, nil
}

func (m *mockCharStore) GetByID(ctx context.Context, id int64) (*character.Character, error) {
	for _, c := range m.chars {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

func TestFormatCharacterSummary(t *testing.T) {
	c := &character.Character{
		ID:     1,
		Name:   "Zara",
		Class:  "ganger",
		Level:  1,
		Region: "old_town",
	}
	summary := handlers.FormatCharacterSummary(c)
	assert.Contains(t, summary, "Zara")
	assert.Contains(t, summary, "ganger")
	assert.Contains(t, summary, "1")
}

func TestFormatCharacterStats(t *testing.T) {
	c := &character.Character{
		Name:      "Zara",
		Class:     "ganger",
		Level:     1,
		Region:    "old_town",
		MaxHP:     10,
		CurrentHP: 10,
		Abilities: character.AbilityScores{
			Strength: 14, Dexterity: 10, Constitution: 10,
			Intelligence: 10, Wisdom: 8, Charisma: 10,
		},
	}
	stats := handlers.FormatCharacterStats(c)
	assert.Contains(t, stats, "STR")
	assert.Contains(t, stats, "14")
	assert.Contains(t, stats, "HP")
	assert.Contains(t, stats, "10")
}

// TestProperty_FormatCharacterSummary verifies that for any character, the summary
// is non-empty and always contains the character's name, class, and level.
func TestProperty_FormatCharacterSummary(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z][A-Za-z0-9]{1,15}`).Draw(rt, "name")
		class := rapid.StringMatching(`[a-z]{3,10}`).Draw(rt, "class")
		level := rapid.IntRange(1, 20).Draw(rt, "level")

		c := &character.Character{
			Name:   name,
			Class:  class,
			Level:  level,
			Region: "old_town",
		}
		summary := handlers.FormatCharacterSummary(c)
		assert.NotEmpty(rt, summary)
		assert.Contains(rt, summary, name)
		assert.Contains(rt, summary, class)
	})
}

// TestProperty_FormatCharacterStats verifies that for any character, the stats block
// is non-empty and contains all six ability score labels plus HP.
func TestProperty_FormatCharacterStats(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		str := rapid.IntRange(1, 20).Draw(rt, "str")
		dex := rapid.IntRange(1, 20).Draw(rt, "dex")
		con := rapid.IntRange(1, 20).Draw(rt, "con")
		intel := rapid.IntRange(1, 20).Draw(rt, "int")
		wis := rapid.IntRange(1, 20).Draw(rt, "wis")
		cha := rapid.IntRange(1, 20).Draw(rt, "cha")
		hp := rapid.IntRange(1, 100).Draw(rt, "hp")

		c := &character.Character{
			Name:      "Test",
			Class:     "ganger",
			Level:     1,
			Region:    "old_town",
			MaxHP:     hp,
			CurrentHP: hp,
			Abilities: character.AbilityScores{
				Strength: str, Dexterity: dex, Constitution: con,
				Intelligence: intel, Wisdom: wis, Charisma: cha,
			},
		}
		stats := handlers.FormatCharacterStats(c)
		assert.NotEmpty(rt, stats)
		for _, label := range []string{"STR", "DEX", "CON", "INT", "WIS", "CHA", "HP"} {
			assert.Contains(rt, stats, label)
		}
	})
}
