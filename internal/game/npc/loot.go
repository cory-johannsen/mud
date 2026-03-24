// Package npc — loot table schema and loot generation.
package npc

import (
	"fmt"
	"math/rand"

	"github.com/google/uuid"
)

// CurrencyDrop defines the range of currency an NPC can drop on death.
type CurrencyDrop struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
}

// ItemDrop defines a single item entry in a loot table with a drop chance.
type ItemDrop struct {
	ItemID string  `yaml:"item"`
	Chance float64 `yaml:"chance"`
	MinQty int     `yaml:"min_qty"`
	MaxQty int     `yaml:"max_qty"`
}

// MaterialDrop defines a crafting material entry in a loot table with an independent drop chance.
type MaterialDrop struct {
	ID          string  `yaml:"id"`
	QuantityMin int     `yaml:"quantity_min"`
	QuantityMax int     `yaml:"quantity_max"`
	Chance      float64 `yaml:"chance"` // 0.0-1.0
}

// OrganicDrop defines a weighted organic item drop for animal NPCs.
type OrganicDrop struct {
	ItemID      string `yaml:"item_id"`
	Weight      int    `yaml:"weight"`
	QuantityMin int    `yaml:"quantity_min"`
	QuantityMax int    `yaml:"quantity_max"`
}

// SalvageDrop defines a salvage item drop for robot/machine NPCs.
type SalvageDrop struct {
	ItemIDs     []string `yaml:"item_ids"`
	QuantityMin int      `yaml:"quantity_min"`
	QuantityMax int      `yaml:"quantity_max"`
}

// LootTable defines the possible loot drops for an NPC template.
type LootTable struct {
	Currency      *CurrencyDrop  `yaml:"currency"`
	Items         []ItemDrop     `yaml:"items"`
	OrganicDrops  []OrganicDrop  `yaml:"organic_drops"`
	SalvageDrop   *SalvageDrop   `yaml:"salvage_drop"`
	MaterialDrops []MaterialDrop `yaml:"material_drops"`
}

// Validate checks that the loot table satisfies its invariants.
//
// Precondition: lt must not be nil.
// Postcondition: Returns nil iff all currency and item constraints hold;
// an empty loot table (no currency, no items) is valid.
func (lt *LootTable) Validate() error {
	if lt.Currency != nil {
		if lt.Currency.Min < 0 {
			return fmt.Errorf("loot table: currency min must be >= 0, got %d", lt.Currency.Min)
		}
		if lt.Currency.Min > lt.Currency.Max {
			return fmt.Errorf("loot table: currency min (%d) must be <= max (%d)", lt.Currency.Min, lt.Currency.Max)
		}
	}
	for i, item := range lt.Items {
		if item.ItemID == "" {
			return fmt.Errorf("loot table: item[%d] must have a non-empty item id", i)
		}
		if item.Chance <= 0 || item.Chance > 1.0 {
			return fmt.Errorf("loot table: item[%d] chance must be in (0, 1.0], got %f", i, item.Chance)
		}
		if item.MinQty < 1 {
			return fmt.Errorf("loot table: item[%d] min_qty must be >= 1, got %d", i, item.MinQty)
		}
		if item.MinQty > item.MaxQty {
			return fmt.Errorf("loot table: item[%d] min_qty (%d) must be <= max_qty (%d)", i, item.MinQty, item.MaxQty)
		}
	}
	for i, od := range lt.OrganicDrops {
		if od.Weight <= 0 {
			return fmt.Errorf("loot table: organic_drop[%d] weight must be > 0, got %d", i, od.Weight)
		}
		if od.QuantityMin < 1 {
			return fmt.Errorf("loot table: organic_drop[%d] quantity_min must be >= 1, got %d", i, od.QuantityMin)
		}
		if od.QuantityMin > od.QuantityMax {
			return fmt.Errorf("loot table: organic_drop[%d] quantity_min (%d) must be <= quantity_max (%d)", i, od.QuantityMin, od.QuantityMax)
		}
	}
	for i, md := range lt.MaterialDrops {
		if md.ID == "" {
			return fmt.Errorf("loot table: material_drop[%d] must have a non-empty id", i)
		}
		if md.Chance <= 0 || md.Chance > 1.0 {
			return fmt.Errorf("loot table: material_drop[%d] chance must be in (0, 1.0], got %f", i, md.Chance)
		}
		if md.QuantityMin < 1 {
			return fmt.Errorf("loot table: material_drop[%d] quantity_min must be >= 1, got %d", i, md.QuantityMin)
		}
		if md.QuantityMin > md.QuantityMax {
			return fmt.Errorf("loot table: material_drop[%d] quantity_min (%d) must be <= quantity_max (%d)", i, md.QuantityMin, md.QuantityMax)
		}
	}
	if lt.SalvageDrop != nil {
		sd := lt.SalvageDrop
		if len(sd.ItemIDs) == 0 {
			return fmt.Errorf("loot table: salvage_drop item_ids must not be empty")
		}
		if sd.QuantityMin < 1 {
			return fmt.Errorf("loot table: salvage_drop quantity_min must be >= 1, got %d", sd.QuantityMin)
		}
		if sd.QuantityMin > sd.QuantityMax {
			return fmt.Errorf("loot table: salvage_drop quantity_min (%d) must be <= quantity_max (%d)", sd.QuantityMin, sd.QuantityMax)
		}
	}
	return nil
}

// LootItem represents a single item instance in a loot result.
type LootItem struct {
	ItemDefID  string
	InstanceID string
	Quantity   int
}

// LootResult holds the generated loot from a single NPC kill.
type LootResult struct {
	Currency  int
	Items     []LootItem
	Materials map[string]int // materialID → quantity
}

// GenerateOrganicLoot selects one organic item using weighted random from the
// OrganicDrops list and returns it with a quantity in [QuantityMin, QuantityMax].
//
// Precondition: lt must have passed Validate().
// Postcondition: Returns empty LootResult if OrganicDrops is empty.
func GenerateOrganicLoot(lt LootTable) LootResult {
	if len(lt.OrganicDrops) == 0 {
		return LootResult{}
	}
	total := 0
	for _, od := range lt.OrganicDrops {
		total += od.Weight
	}
	roll := rand.Intn(total)
	var selected OrganicDrop
	for _, od := range lt.OrganicDrops {
		roll -= od.Weight
		if roll < 0 {
			selected = od
			break
		}
	}
	qty := selected.QuantityMin
	if spread := selected.QuantityMax - selected.QuantityMin; spread > 0 {
		qty += rand.Intn(spread + 1)
	}
	return LootResult{Items: []LootItem{{
		ItemDefID:  selected.ItemID,
		InstanceID: uuid.New().String(),
		Quantity:   qty,
	}}}
}

// GenerateSalvageLoot selects one salvage item at random from SalvageDrop.ItemIDs
// and returns it with a quantity in [QuantityMin, QuantityMax].
//
// Precondition: lt must have passed Validate().
// Postcondition: Returns empty LootResult if SalvageDrop is nil or has no item IDs.
func GenerateSalvageLoot(lt LootTable) LootResult {
	if lt.SalvageDrop == nil || len(lt.SalvageDrop.ItemIDs) == 0 {
		return LootResult{}
	}
	sd := lt.SalvageDrop
	itemID := sd.ItemIDs[rand.Intn(len(sd.ItemIDs))]
	qty := sd.QuantityMin
	if spread := sd.QuantityMax - sd.QuantityMin; spread > 0 {
		qty += rand.Intn(spread + 1)
	}
	return LootResult{Items: []LootItem{{
		ItemDefID:  itemID,
		InstanceID: uuid.New().String(),
		Quantity:   qty,
	}}}
}

// GenerateLoot rolls loot from the given LootTable using math/rand.
//
// Precondition: lt must have passed Validate().
// Postcondition: Currency is in [Currency.Min, Currency.Max] if currency is set;
// each item's Quantity is in [MinQty, MaxQty] for items that pass the chance roll.
func GenerateLoot(lt LootTable) LootResult {
	var result LootResult

	if lt.Currency != nil && lt.Currency.Max > 0 {
		spread := lt.Currency.Max - lt.Currency.Min
		if spread == 0 {
			result.Currency = lt.Currency.Min
		} else {
			result.Currency = lt.Currency.Min + rand.Intn(spread+1)
		}
	}

	for _, item := range lt.Items {
		if rand.Float64() < item.Chance {
			qty := item.MinQty
			spread := item.MaxQty - item.MinQty
			if spread > 0 {
				qty += rand.Intn(spread + 1)
			}
			result.Items = append(result.Items, LootItem{
				ItemDefID:  item.ItemID,
				InstanceID: uuid.New().String(),
				Quantity:   qty,
			})
		}
	}

	for _, md := range lt.MaterialDrops {
		if rand.Float64() < md.Chance {
			qty := md.QuantityMin
			if md.QuantityMax > md.QuantityMin {
				qty += rand.Intn(md.QuantityMax - md.QuantityMin + 1)
			}
			if result.Materials == nil {
				result.Materials = make(map[string]int)
			}
			result.Materials[md.ID] += qty
		}
	}

	return result
}
