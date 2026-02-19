package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

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
