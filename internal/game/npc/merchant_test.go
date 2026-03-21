package npc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestInitRuntimeState_SetsStockAndBudget(t *testing.T) {
	cfg := &MerchantConfig{
		Budget: 500,
		Inventory: []MerchantItem{
			{ItemID: "sword", BasePrice: 100, InitStock: 3, MaxStock: 5},
			{ItemID: "shield", BasePrice: 80, InitStock: 1, MaxStock: 3},
		},
		ReplenishRate: ReplenishConfig{MinHours: 1, MaxHours: 4},
	}
	state := InitRuntimeState(cfg, time.Unix(0, 0))
	assert.Equal(t, 3, state.Stock["sword"])
	assert.Equal(t, 1, state.Stock["shield"])
	assert.Equal(t, 500, state.CurrentBudget)
	assert.False(t, state.NextReplenishAt.IsZero())
}

func TestComputeBuyPrice_NoModifiers(t *testing.T) {
	got := ComputeBuyPrice(100, 1.2, 1.0, 0.0)
	assert.Equal(t, 120, got)
}

func TestComputeBuyPrice_WantedSurcharge(t *testing.T) {
	// floor(100 * 1.0 * 1.1 * (1-0.1)) = floor(99.0) = 99
	got := ComputeBuyPrice(100, 1.0, 1.1, 0.1)
	assert.Equal(t, 99, got)
}

func TestComputeBuyPrice_CritFailPenalty(t *testing.T) {
	// floor(100 * 1.0 * 1.0 * (1-(-0.1))) = floor(110) = 110
	got := ComputeBuyPrice(100, 1.0, 1.0, -0.1)
	assert.Equal(t, 110, got)
}

func TestComputeSellPayout_Basic(t *testing.T) {
	got := ComputeSellPayout(100, 0.5, 2, 0.0)
	assert.Equal(t, 100, got)
}

func TestComputeSellPayout_NegotiateBoost(t *testing.T) {
	got := ComputeSellPayout(100, 0.5, 1, 0.1)
	assert.Equal(t, 55, got)
}

func TestComputeSellPayout_CritFailUnaffected(t *testing.T) {
	// REQ-NPC-5b: crit fail does NOT penalise sells
	got := ComputeSellPayout(100, 0.5, 1, -0.1)
	assert.Equal(t, 50, got)
}

func TestApplyReplenish_StockRefill_Increments(t *testing.T) {
	cfg := &MerchantConfig{
		Budget: 500,
		Inventory: []MerchantItem{
			{ItemID: "sword", BasePrice: 100, InitStock: 0, MaxStock: 5},
		},
		ReplenishRate: ReplenishConfig{MinHours: 1, MaxHours: 1, StockRefill: 2, BudgetRefill: 100},
	}
	state := &MerchantRuntimeState{
		Stock:         map[string]int{"sword": 1},
		CurrentBudget: 200,
	}
	next := ApplyReplenish(cfg, state, 0)
	assert.Equal(t, 3, next.Stock["sword"])
	assert.Equal(t, 300, next.CurrentBudget)
}

func TestApplyReplenish_StockRefill_Zero_FullReset(t *testing.T) {
	cfg := &MerchantConfig{
		Budget: 500,
		Inventory: []MerchantItem{
			{ItemID: "sword", BasePrice: 100, InitStock: 0, MaxStock: 5},
		},
		ReplenishRate: ReplenishConfig{MinHours: 1, MaxHours: 1, StockRefill: 0, BudgetRefill: 0},
	}
	state := &MerchantRuntimeState{
		Stock:         map[string]int{"sword": 2},
		CurrentBudget: 100,
	}
	next := ApplyReplenish(cfg, state, 0)
	assert.Equal(t, 5, next.Stock["sword"])
	assert.Equal(t, 500, next.CurrentBudget)
}

func TestApplyNegotiateOutcome_CritSuccess(t *testing.T) {
	assert.Equal(t, 0.2, ApplyNegotiateOutcome("crit_success"))
}

func TestApplyNegotiateOutcome_Success(t *testing.T) {
	assert.Equal(t, 0.1, ApplyNegotiateOutcome("success"))
}

func TestApplyNegotiateOutcome_Failure(t *testing.T) {
	assert.Equal(t, 0.0, ApplyNegotiateOutcome("failure"))
}

func TestApplyNegotiateOutcome_CritFailure(t *testing.T) {
	assert.Equal(t, -0.1, ApplyNegotiateOutcome("crit_failure"))
}

func TestProperty_ComputeBuyPrice_NeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		base := rapid.IntRange(1, 10000).Draw(rt, "base")
		margin := rapid.Float64Range(0.1, 5.0).Draw(rt, "margin")
		surcharge := rapid.Float64Range(1.0, 2.0).Draw(rt, "surcharge")
		mod := rapid.Float64Range(-0.1, 0.2).Draw(rt, "mod")
		got := ComputeBuyPrice(base, margin, surcharge, mod)
		assert.GreaterOrEqual(t, got, 0)
	})
}

func TestProperty_ComputeSellPayout_NeverNegative(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		base := rapid.IntRange(1, 10000).Draw(rt, "base")
		margin := rapid.Float64Range(0.0, 1.0).Draw(rt, "margin")
		qty := rapid.IntRange(1, 100).Draw(rt, "qty")
		mod := rapid.Float64Range(-0.1, 0.2).Draw(rt, "mod")
		got := ComputeSellPayout(base, margin, qty, mod)
		assert.GreaterOrEqual(t, got, 0)
	})
}
