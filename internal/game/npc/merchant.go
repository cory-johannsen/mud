// Package npc — merchant pure-logic helpers.
package npc

import (
	"math"
	"math/rand"
	"time"
)

// InitRuntimeState creates a MerchantRuntimeState from a MerchantConfig at first
// zone initialization.
//
// Precondition: cfg must be non-nil; now must be non-zero.
// Postcondition: Stock contains one entry per Inventory item; CurrentBudget == cfg.Budget.
func InitRuntimeState(cfg *MerchantConfig, now time.Time) *MerchantRuntimeState {
	stock := make(map[string]int, len(cfg.Inventory))
	for _, item := range cfg.Inventory {
		stock[item.ItemID] = item.InitStock
	}
	return &MerchantRuntimeState{
		Stock:           stock,
		CurrentBudget:   cfg.Budget,
		NextReplenishAt: now.Add(nextReplenishInterval(cfg.ReplenishRate)),
	}
}

// nextReplenishInterval returns a random duration in [MinHours, MaxHours].
func nextReplenishInterval(r ReplenishConfig) time.Duration {
	span := r.MaxHours - r.MinHours
	h := r.MinHours
	if span > 0 {
		h += rand.Intn(span)
	}
	return time.Duration(h) * time.Hour
}

// ApplyReplenish advances a MerchantRuntimeState by one replenishment cycle.
// The unused int parameter is reserved for future danger-level-based scaling.
//
// Precondition: cfg and state must be non-nil.
// Postcondition: Returns a new MerchantRuntimeState; original is unchanged.
func ApplyReplenish(cfg *MerchantConfig, state *MerchantRuntimeState, _ int) *MerchantRuntimeState {
	next := &MerchantRuntimeState{
		Stock:           make(map[string]int, len(state.Stock)),
		CurrentBudget:   state.CurrentBudget,
		NextReplenishAt: state.NextReplenishAt.Add(nextReplenishInterval(cfg.ReplenishRate)),
	}
	for k, v := range state.Stock {
		next.Stock[k] = v
	}
	for _, item := range cfg.Inventory {
		cur := next.Stock[item.ItemID]
		if cfg.ReplenishRate.StockRefill == 0 {
			next.Stock[item.ItemID] = item.MaxStock
		} else {
			added := cur + cfg.ReplenishRate.StockRefill
			if added > item.MaxStock {
				added = item.MaxStock
			}
			next.Stock[item.ItemID] = added
		}
	}
	if cfg.ReplenishRate.BudgetRefill == 0 {
		next.CurrentBudget = cfg.Budget
	} else {
		next.CurrentBudget = state.CurrentBudget + cfg.ReplenishRate.BudgetRefill
	}
	return next
}

// ComputeBuyPrice returns the credit cost for the player to buy one unit of an
// item at basePrice, applying sellMargin, wantedSurcharge, then negotiate modifier.
//
// REQ-NPC-5b: negotiateMod is signed: +0.2 = 20% discount, -0.1 = 10% penalty.
//
// Precondition: basePrice >= 1; all multipliers > 0.
// Postcondition: Returns floor of final price; always >= 0.
func ComputeBuyPrice(basePrice int, sellMargin, wantedSurcharge, negotiateMod float64) int {
	price := float64(basePrice) * sellMargin * wantedSurcharge * (1.0 - negotiateMod)
	if price < 0 {
		price = 0
	}
	return int(math.Floor(price))
}

// ComputeSellPayout returns the credits paid to the player for selling qty units.
// Critical failure modifier (negative mod) does NOT penalise sells — clamped to 0.
//
// REQ-NPC-5b: sells are unaffected by critical failure.
// Precondition: basePrice >= 1; buyMargin in [0,1]; qty >= 1.
// Postcondition: Returns floor of payout; always >= 0.
func ComputeSellPayout(basePrice int, buyMargin float64, qty int, negotiateMod float64) int {
	mod := negotiateMod
	if mod < 0 {
		mod = 0
	}
	payout := float64(basePrice) * buyMargin * float64(qty) * (1.0 + mod)
	if payout < 0 {
		payout = 0
	}
	return int(math.Floor(payout))
}

// ApplyNegotiateOutcome converts a skill-check outcome to a session-scoped modifier.
//
// Outcomes: "crit_success"→0.2, "success"→0.1, "failure"→0.0, "crit_failure"→-0.1.
//
// Precondition: outcome is one of the four defined strings.
// Postcondition: Returns a value in {-0.1, 0.0, 0.1, 0.2}.
func ApplyNegotiateOutcome(outcome string) float64 {
	switch outcome {
	case "crit_success":
		return 0.2
	case "success":
		return 0.1
	case "crit_failure":
		return -0.1
	default:
		return 0.0
	}
}

// BrowseItem is one row in a browse listing.
type BrowseItem struct {
	ItemID    string
	BuyPrice  int
	SellPrice int
	Stock     int
}

// BrowseLines returns browse display rows for all inventory items, prices
// adjusted for the active negotiate modifier and wanted surcharge.
//
// Precondition: cfg and state must be non-nil.
// Postcondition: Returns one BrowseItem per Inventory entry in config order.
func BrowseLines(cfg *MerchantConfig, state *MerchantRuntimeState, wantedSurcharge, negotiateMod float64) []BrowseItem {
	rows := make([]BrowseItem, 0, len(cfg.Inventory))
	for _, item := range cfg.Inventory {
		rows = append(rows, BrowseItem{
			ItemID:    item.ItemID,
			BuyPrice:  ComputeBuyPrice(item.BasePrice, cfg.SellMargin, wantedSurcharge, negotiateMod),
			SellPrice: ComputeSellPayout(item.BasePrice, cfg.BuyMargin, 1, negotiateMod),
			Stock:     state.Stock[item.ItemID],
		})
	}
	return rows
}
