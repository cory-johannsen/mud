package inventory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

func TestRarityLookup_AllTiers(t *testing.T) {
	tiers := []string{"salvage", "street", "mil_spec", "black_market", "ghost"}
	for _, id := range tiers {
		def, ok := inventory.LookupRarity(id)
		require.True(t, ok, "expected rarity %q to exist", id)
		assert.Equal(t, id, def.ID)
	}
}

func TestRarityLookup_Unknown_ReturnsFalse(t *testing.T) {
	_, ok := inventory.LookupRarity("nonexistent")
	assert.False(t, ok)
}

func TestRarityDef_StatMultipliers(t *testing.T) {
	tests := []struct {
		id   string
		mult float64
	}{
		{"salvage", 1.0},
		{"street", 1.2},
		{"mil_spec", 1.5},
		{"black_market", 1.8},
		{"ghost", 2.2},
	}
	for _, tc := range tests {
		def, ok := inventory.LookupRarity(tc.id)
		require.True(t, ok)
		assert.InDelta(t, tc.mult, def.StatMultiplier, 1e-9, "rarity %q stat multiplier", tc.id)
	}
}

func TestRarityDef_FeatureSlots(t *testing.T) {
	tests := []struct {
		id    string
		slots int
	}{
		{"salvage", 0},
		{"street", 1},
		{"mil_spec", 2},
		{"black_market", 3},
		{"ghost", 4},
	}
	for _, tc := range tests {
		def, ok := inventory.LookupRarity(tc.id)
		require.True(t, ok)
		assert.Equal(t, tc.slots, def.FeatureSlots, "rarity %q feature slots", tc.id)
	}
}

func TestRarityDef_MinLevel(t *testing.T) {
	tests := []struct {
		id       string
		minLevel int
	}{
		{"salvage", 0},
		{"street", 1},
		{"mil_spec", 5},
		{"black_market", 10},
		{"ghost", 15},
	}
	for _, tc := range tests {
		def, ok := inventory.LookupRarity(tc.id)
		require.True(t, ok)
		assert.Equal(t, tc.minLevel, def.MinLevel, "rarity %q min level", tc.id)
	}
}

func TestRarityDef_MaxDurability(t *testing.T) {
	tests := []struct {
		id            string
		maxDurability int
	}{
		{"salvage", 20},
		{"street", 40},
		{"mil_spec", 60},
		{"black_market", 80},
		{"ghost", 100},
	}
	for _, tc := range tests {
		def, ok := inventory.LookupRarity(tc.id)
		require.True(t, ok)
		assert.Equal(t, tc.maxDurability, def.MaxDurability, "rarity %q max durability", tc.id)
	}
}

func TestRarityDef_DestructionChance(t *testing.T) {
	tests := []struct {
		id      string
		chance  float64
	}{
		{"salvage", 0.50},
		{"street", 0.30},
		{"mil_spec", 0.15},
		{"black_market", 0.05},
		{"ghost", 0.01},
	}
	for _, tc := range tests {
		def, ok := inventory.LookupRarity(tc.id)
		require.True(t, ok)
		assert.InDelta(t, tc.chance, def.DestructionChance, 1e-9, "rarity %q destruction chance", tc.id)
	}
}

func TestRarityDef_ModifierProbsSumToOne(t *testing.T) {
	tiers := []string{"salvage", "street", "mil_spec", "black_market", "ghost"}
	for _, id := range tiers {
		def, ok := inventory.LookupRarity(id)
		require.True(t, ok)
		total := def.ModifierProbs.Tuned + def.ModifierProbs.Defective + def.ModifierProbs.Cursed
		normal := 1.0 - total
		assert.GreaterOrEqual(t, normal, 0.0, "rarity %q: normal probability must be >= 0", id)
		assert.LessOrEqual(t, total, 1.0, "rarity %q: total modifier probability must be <= 1", id)
	}
}

func TestRarityDef_ModifierProbsBySpec(t *testing.T) {
	tests := []struct {
		id        string
		tuned     float64
		defective float64
		cursed    float64
	}{
		{"salvage", 0.00, 0.30, 0.10},
		{"street", 0.05, 0.15, 0.05},
		{"mil_spec", 0.10, 0.10, 0.03},
		{"black_market", 0.20, 0.05, 0.02},
		{"ghost", 0.30, 0.02, 0.01},
	}
	for _, tc := range tests {
		def, ok := inventory.LookupRarity(tc.id)
		require.True(t, ok)
		assert.InDelta(t, tc.tuned, def.ModifierProbs.Tuned, 1e-9, "rarity %q tuned", tc.id)
		assert.InDelta(t, tc.defective, def.ModifierProbs.Defective, 1e-9, "rarity %q defective", tc.id)
		assert.InDelta(t, tc.cursed, def.ModifierProbs.Cursed, 1e-9, "rarity %q cursed", tc.id)
	}
}

func TestProperty_RarityDef_StatMultiplierPositive(t *testing.T) {
	tiers := []string{"salvage", "street", "mil_spec", "black_market", "ghost"}
	rapid.Check(t, func(rt *rapid.T) {
		idx := rapid.IntRange(0, len(tiers)-1).Draw(rt, "idx")
		def, ok := inventory.LookupRarity(tiers[idx])
		require.True(rt, ok)
		assert.Greater(rt, def.StatMultiplier, 0.0)
	})
}

func TestProperty_RarityDef_MaxDurabilityPositive(t *testing.T) {
	tiers := []string{"salvage", "street", "mil_spec", "black_market", "ghost"}
	rapid.Check(t, func(rt *rapid.T) {
		idx := rapid.IntRange(0, len(tiers)-1).Draw(rt, "idx")
		def, ok := inventory.LookupRarity(tiers[idx])
		require.True(rt, ok)
		assert.Greater(rt, def.MaxDurability, 0)
	})
}

func TestRollModifier_SalvageNoTuned(t *testing.T) {
	// Salvage has 0% tuned, so no tuned modifier should ever be rolled
	def, ok := inventory.LookupRarity("salvage")
	require.True(t, ok)
	// With a roll of 0.0 (below 0% tuned), should not get tuned
	modifier := inventory.RollModifier(def, 0.0)
	assert.NotEqual(t, "tuned", modifier)
}

func TestRollModifier_GhostHighTunedChance(t *testing.T) {
	def, ok := inventory.LookupRarity("ghost")
	require.True(t, ok)
	// Roll of 0.15 is within ghost's 30% tuned range
	modifier := inventory.RollModifier(def, 0.15)
	assert.Equal(t, "tuned", modifier)
}

func TestRollModifier_Normal(t *testing.T) {
	def, ok := inventory.LookupRarity("street")
	require.True(t, ok)
	// street: tuned=5%, defective=15%, cursed=5%. Roll of 0.99 is in normal range.
	modifier := inventory.RollModifier(def, 0.99)
	assert.Equal(t, "", modifier)
}
