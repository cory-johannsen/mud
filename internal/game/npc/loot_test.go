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

// ---- OrganicDrop tests ----

func TestOrganicDrop_Validate_AcceptsValid(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "meat", Weight: 10, QuantityMin: 1, QuantityMax: 2},
		},
	}
	assert.NoError(t, lt.Validate())
}

func TestOrganicDrop_Validate_RejectsZeroWeight(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "meat", Weight: 0, QuantityMin: 1, QuantityMax: 2},
		},
	}
	assert.Error(t, lt.Validate())
}

func TestOrganicDrop_Validate_RejectsNegativeWeight(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "meat", Weight: -1, QuantityMin: 1, QuantityMax: 2},
		},
	}
	assert.Error(t, lt.Validate())
}

func TestOrganicDrop_Validate_RejectsMinQtyZero(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "meat", Weight: 5, QuantityMin: 0, QuantityMax: 2},
		},
	}
	assert.Error(t, lt.Validate())
}

func TestOrganicDrop_Validate_RejectsMinGreaterThanMax(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "meat", Weight: 5, QuantityMin: 3, QuantityMax: 2},
		},
	}
	assert.Error(t, lt.Validate())
}

// ---- SalvageDrop tests ----

func TestSalvageDrop_Validate_AcceptsValid(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs:     []string{"circuit_board", "scrap_metal"},
			QuantityMin: 1,
			QuantityMax: 2,
		},
	}
	assert.NoError(t, lt.Validate())
}

func TestSalvageDrop_Validate_RejectsEmptyItemIDs(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs:     []string{},
			QuantityMin: 1,
			QuantityMax: 2,
		},
	}
	assert.Error(t, lt.Validate())
}

func TestSalvageDrop_Validate_RejectsMinQtyZero(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs:     []string{"scrap"},
			QuantityMin: 0,
			QuantityMax: 2,
		},
	}
	assert.Error(t, lt.Validate())
}

func TestSalvageDrop_Validate_RejectsMinGreaterThanMax(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs:     []string{"scrap"},
			QuantityMin: 3,
			QuantityMax: 2,
		},
	}
	assert.Error(t, lt.Validate())
}

// ---- GenerateOrganicLoot and GenerateSalvageLoot tests ----

func TestGenerateOrganicLoot_ReturnsItem(t *testing.T) {
	lt := npc.LootTable{
		OrganicDrops: []npc.OrganicDrop{
			{ItemID: "meat", Weight: 10, QuantityMin: 1, QuantityMax: 2},
		},
	}
	result := npc.GenerateOrganicLoot(lt)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	if result.Items[0].ItemDefID != "meat" {
		t.Errorf("ItemDefID: got %q, want %q", result.Items[0].ItemDefID, "meat")
	}
	if result.Items[0].Quantity < 1 || result.Items[0].Quantity > 2 {
		t.Errorf("Quantity %d out of [1,2]", result.Items[0].Quantity)
	}
	if result.Items[0].InstanceID == "" {
		t.Error("InstanceID must not be empty")
	}
}

func TestGenerateOrganicLoot_EmptyDrops_ReturnsEmpty(t *testing.T) {
	lt := npc.LootTable{}
	result := npc.GenerateOrganicLoot(lt)
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
}

func TestGenerateSalvageLoot_ReturnsItem(t *testing.T) {
	lt := npc.LootTable{
		SalvageDrop: &npc.SalvageDrop{
			ItemIDs:     []string{"circuit_board", "scrap_metal"},
			QuantityMin: 1,
			QuantityMax: 2,
		},
	}
	result := npc.GenerateSalvageLoot(lt)
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	itemID := result.Items[0].ItemDefID
	if itemID != "circuit_board" && itemID != "scrap_metal" {
		t.Errorf("unexpected ItemDefID %q", itemID)
	}
	if result.Items[0].Quantity < 1 || result.Items[0].Quantity > 2 {
		t.Errorf("Quantity %d out of [1,2]", result.Items[0].Quantity)
	}
}

func TestGenerateSalvageLoot_NilSalvageDrop_ReturnsEmpty(t *testing.T) {
	lt := npc.LootTable{}
	result := npc.GenerateSalvageLoot(lt)
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
}

// ---- MaterialDrop tests ----

func TestGenerateLoot_MaterialDrops_ChanceBased(t *testing.T) {
	// Chance=1.0: material must always be generated.
	lt := npc.LootTable{
		MaterialDrops: []npc.MaterialDrop{
			{ID: "scrap", Chance: 1.0, QuantityMin: 1, QuantityMax: 1},
		},
	}
	for i := 0; i < 50; i++ {
		result := npc.GenerateLoot(lt)
		qty, ok := result.Materials["scrap"]
		if !ok || qty < 1 {
			t.Fatalf("iteration %d: expected scrap to always drop with Chance=1.0, got %v", i, result.Materials)
		}
	}

	// Chance=0.0 is rejected by Validate, but GenerateLoot uses rand.Float64() < Chance,
	// so Chance=0.0 means the material never drops.
	lt2 := npc.LootTable{
		MaterialDrops: []npc.MaterialDrop{
			{ID: "scrap", Chance: 0.0, QuantityMin: 1, QuantityMax: 1},
		},
	}
	for i := 0; i < 50; i++ {
		result := npc.GenerateLoot(lt2)
		if result.Materials["scrap"] != 0 {
			t.Fatalf("iteration %d: expected scrap never to drop with Chance=0.0", i)
		}
	}
}

func TestGenerateLoot_MaterialDrops_QuantityRange(t *testing.T) {
	lt := npc.LootTable{
		MaterialDrops: []npc.MaterialDrop{
			{ID: "ore", Chance: 1.0, QuantityMin: 2, QuantityMax: 4},
		},
	}
	for i := 0; i < 100; i++ {
		result := npc.GenerateLoot(lt)
		qty := result.Materials["ore"]
		assert.GreaterOrEqual(t, qty, 2, "iteration %d: quantity below minimum", i)
		assert.LessOrEqual(t, qty, 4, "iteration %d: quantity above maximum", i)
	}
}

func TestLootTable_Validate_MaterialDrop_InvalidChance(t *testing.T) {
	// Chance > 1.0 must fail.
	lt := npc.LootTable{
		MaterialDrops: []npc.MaterialDrop{
			{ID: "ore", Chance: 1.5, QuantityMin: 1, QuantityMax: 1},
		},
	}
	assert.Error(t, lt.Validate())

	// Chance <= 0 must fail.
	lt2 := npc.LootTable{
		MaterialDrops: []npc.MaterialDrop{
			{ID: "ore", Chance: 0.0, QuantityMin: 1, QuantityMax: 1},
		},
	}
	assert.Error(t, lt2.Validate())

	// Negative chance must fail.
	lt3 := npc.LootTable{
		MaterialDrops: []npc.MaterialDrop{
			{ID: "ore", Chance: -0.5, QuantityMin: 1, QuantityMax: 1},
		},
	}
	assert.Error(t, lt3.Validate())
}

func TestProperty_GenerateLoot_MaterialDrop_GuaranteedFixedQty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 20).Draw(rt, "n")
		lt := npc.LootTable{
			MaterialDrops: []npc.MaterialDrop{
				{ID: "mat", Chance: 1.0, QuantityMin: n, QuantityMax: n},
			},
		}
		result := npc.GenerateLoot(lt)
		qty := result.Materials["mat"]
		if qty != n {
			rt.Fatalf("expected Materials[mat]==%d with Chance=1.0 and QuantityMin=QuantityMax=%d, got %d", n, n, qty)
		}
	})
}
