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

// LootTable defines the possible loot drops for an NPC template.
type LootTable struct {
	Currency *CurrencyDrop `yaml:"currency"`
	Items    []ItemDrop    `yaml:"items"`
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
	Currency int
	Items    []LootItem
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

	return result
}
