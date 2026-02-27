package npc_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func validLootTable() npc.LootTable {
	return npc.LootTable{
		Currency: &npc.CurrencyDrop{Min: 5, Max: 20},
		Items: []npc.ItemDrop{
			{ItemID: "sword", Chance: 0.5, MinQty: 1, MaxQty: 1},
			{ItemID: "potion", Chance: 1.0, MinQty: 1, MaxQty: 3},
		},
	}
}

func TestLootTable_Validate_AcceptsValid(t *testing.T) {
	lt := validLootTable()
	assert.NoError(t, lt.Validate())
}

func TestLootTable_Validate_RejectsNegativeMinCurrency(t *testing.T) {
	lt := npc.LootTable{Currency: &npc.CurrencyDrop{Min: -1, Max: 10}}
	assert.Error(t, lt.Validate())
}

func TestLootTable_Validate_RejectsMinGreaterThanMax(t *testing.T) {
	lt := npc.LootTable{Currency: &npc.CurrencyDrop{Min: 20, Max: 10}}
	assert.Error(t, lt.Validate())
}

func TestLootTable_Validate_RejectsInvalidChance(t *testing.T) {
	lt := npc.LootTable{Items: []npc.ItemDrop{
		{ItemID: "sword", Chance: 1.5, MinQty: 1, MaxQty: 1},
	}}
	assert.Error(t, lt.Validate())
}

func TestLootTable_Validate_RejectsZeroChance(t *testing.T) {
	lt := npc.LootTable{Items: []npc.ItemDrop{
		{ItemID: "sword", Chance: 0.0, MinQty: 1, MaxQty: 1},
	}}
	assert.Error(t, lt.Validate())
}

func TestLootTable_Validate_RejectsMinQtyGreaterThanMaxQty(t *testing.T) {
	lt := npc.LootTable{Items: []npc.ItemDrop{
		{ItemID: "sword", Chance: 0.5, MinQty: 5, MaxQty: 2},
	}}
	assert.Error(t, lt.Validate())
}

func TestLootTable_Validate_Empty(t *testing.T) {
	lt := npc.LootTable{}
	assert.NoError(t, lt.Validate())
}

func TestGenerateLoot_CurrencyInRange(t *testing.T) {
	lt := npc.LootTable{Currency: &npc.CurrencyDrop{Min: 10, Max: 20}}
	for i := 0; i < 100; i++ {
		result := npc.GenerateLoot(lt)
		assert.GreaterOrEqual(t, result.Currency, 10)
		assert.LessOrEqual(t, result.Currency, 20)
	}
}

func TestGenerateLoot_NoCurrency(t *testing.T) {
	lt := npc.LootTable{Items: []npc.ItemDrop{
		{ItemID: "sword", Chance: 1.0, MinQty: 1, MaxQty: 1},
	}}
	result := npc.GenerateLoot(lt)
	assert.Equal(t, 0, result.Currency)
}

func TestGenerateLoot_GuaranteedItem(t *testing.T) {
	lt := npc.LootTable{Items: []npc.ItemDrop{
		{ItemID: "sword", Chance: 1.0, MinQty: 1, MaxQty: 1},
	}}
	for i := 0; i < 100; i++ {
		result := npc.GenerateLoot(lt)
		require.Len(t, result.Items, 1)
		assert.Equal(t, "sword", result.Items[0].ItemDefID)
		assert.NotEmpty(t, result.Items[0].InstanceID)
		assert.Equal(t, 1, result.Items[0].Quantity)
	}
}

func TestGenerateLoot_ZeroChance_NeverDrops(t *testing.T) {
	// Chance must be > 0 per validation, but GenerateLoot uses < comparison,
	// so an extremely small chance effectively never drops. We test with the
	// smallest valid chance over many iterations.
	lt := npc.LootTable{Items: []npc.ItemDrop{
		{ItemID: "rare", Chance: 0.000001, MinQty: 1, MaxQty: 1},
	}}
	drops := 0
	for i := 0; i < 1000; i++ {
		result := npc.GenerateLoot(lt)
		drops += len(result.Items)
	}
	// With chance 0.000001 over 1000 trials, drops should be ~0.
	assert.LessOrEqual(t, drops, 5, "extremely low chance item should almost never drop")
}

func TestProperty_GenerateLoot_CurrencyAlwaysInRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		min := rapid.IntRange(0, 100).Draw(rt, "min")
		max := rapid.IntRange(min, min+100).Draw(rt, "max")
		lt := npc.LootTable{Currency: &npc.CurrencyDrop{Min: min, Max: max}}
		result := npc.GenerateLoot(lt)
		if min >= result.Currency || result.Currency > max {
			// allow exact bounds
			assert.GreaterOrEqual(rt, result.Currency, min)
			assert.LessOrEqual(rt, result.Currency, max)
		}
	})
}

func TestProperty_GenerateLoot_ItemQuantityInRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		minQty := rapid.IntRange(1, 10).Draw(rt, "minQty")
		maxQty := rapid.IntRange(minQty, minQty+10).Draw(rt, "maxQty")
		lt := npc.LootTable{Items: []npc.ItemDrop{
			{ItemID: "item", Chance: 1.0, MinQty: minQty, MaxQty: maxQty},
		}}
		result := npc.GenerateLoot(lt)
		require.Len(rt, result.Items, 1)
		assert.GreaterOrEqual(rt, result.Items[0].Quantity, minQty)
		assert.LessOrEqual(rt, result.Items[0].Quantity, maxQty)
	})
}
