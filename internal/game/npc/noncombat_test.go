package npc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestReplenishConfig_Valid(t *testing.T) {
	c := ReplenishConfig{MinHours: 2, MaxHours: 6, StockRefill: 1, BudgetRefill: 100}
	assert.NoError(t, c.Validate(), "valid ReplenishConfig must not error")
}

func TestReplenishConfig_MinZero(t *testing.T) {
	c := ReplenishConfig{MinHours: 0, MaxHours: 6}
	assert.Error(t, c.Validate(), "MinHours == 0 must error")
}

func TestReplenishConfig_MinGtMax(t *testing.T) {
	c := ReplenishConfig{MinHours: 8, MaxHours: 4}
	assert.Error(t, c.Validate(), "MinHours > MaxHours must error")
}

func TestReplenishConfig_MaxGt24(t *testing.T) {
	c := ReplenishConfig{MinHours: 1, MaxHours: 25}
	assert.Error(t, c.Validate(), "MaxHours > 24 must error")
}

func TestQuestGiverConfig_EmptyDialog(t *testing.T) {
	c := QuestGiverConfig{PlaceholderDialog: nil}
	assert.Error(t, c.Validate(), "empty PlaceholderDialog must error")
}

func TestQuestGiverConfig_NonEmptyDialog(t *testing.T) {
	c := QuestGiverConfig{PlaceholderDialog: []string{"Hello, stranger."}}
	assert.NoError(t, c.Validate(), "non-empty PlaceholderDialog must not error")
}

func TestProperty_ReplenishConfig_ValidRangeNeverErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		min := rapid.IntRange(1, 24).Draw(rt, "min")
		max := rapid.IntRange(min, 24).Draw(rt, "max")
		c := ReplenishConfig{MinHours: min, MaxHours: max}
		if err := c.Validate(); err != nil {
			rt.Fatalf("valid ReplenishConfig{min:%d, max:%d} must not error: %v", min, max, err)
		}
	})
}

func TestProperty_ReplenishConfig_InvalidRangeAlwaysErrors(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		min := rapid.IntRange(-10, 0).Draw(rt, "min_le_zero")
		max := rapid.IntRange(1, 24).Draw(rt, "max")
		c := ReplenishConfig{MinHours: min, MaxHours: max}
		if err := c.Validate(); err == nil {
			rt.Fatalf("invalid ReplenishConfig{min:%d, max:%d} must error", min, max)
		}
	})
}
