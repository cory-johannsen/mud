package npc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestNewCurrentRate_ClampedLow(t *testing.T) {
	cfg := &BankerConfig{BaseRate: 0.5, RateVariance: 1.0}
	rate := NewCurrentRateFromDelta(cfg, -1.0)
	assert.GreaterOrEqual(t, rate, 0.5)
}

func TestNewCurrentRate_ClampedHigh(t *testing.T) {
	cfg := &BankerConfig{BaseRate: 1.0, RateVariance: 0.5}
	rate := NewCurrentRateFromDelta(cfg, 0.5)
	assert.LessOrEqual(t, rate, 1.0)
}

func TestComputeDeposit_Basic(t *testing.T) {
	assert.Equal(t, 95, ComputeDeposit(100, 0.95))
}

func TestComputeWithdrawal_Basic(t *testing.T) {
	assert.Equal(t, 100, ComputeWithdrawal(95, 0.95))
}

func TestComputeDeposit_RateOne_NoFee(t *testing.T) {
	assert.Equal(t, 200, ComputeDeposit(200, 1.0))
}

func TestComputeWithdrawal_RateOne_NoFee(t *testing.T) {
	assert.Equal(t, 200, ComputeWithdrawal(200, 1.0))
}

func TestProperty_NewCurrentRate_InRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		base := rapid.Float64Range(0.5, 1.0).Draw(rt, "base")
		variance := rapid.Float64Range(0.0, 0.5).Draw(rt, "variance")
		delta := rapid.Float64Range(-variance, variance).Draw(rt, "delta")
		cfg := &BankerConfig{BaseRate: base, RateVariance: variance}
		rate := NewCurrentRateFromDelta(cfg, delta)
		assert.GreaterOrEqual(t, rate, 0.5)
		assert.LessOrEqual(t, rate, 1.0)
	})
}

func TestProperty_ComputeDeposit_NeverExceedsAmount(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		amount := rapid.IntRange(1, 100000).Draw(rt, "amount")
		rate := rapid.Float64Range(0.5, 1.0).Draw(rt, "rate")
		stash := ComputeDeposit(amount, rate)
		assert.LessOrEqual(t, stash, amount)
		assert.GreaterOrEqual(t, stash, 0)
	})
}
