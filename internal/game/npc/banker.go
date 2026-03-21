// Package npc — banker pure-logic helpers.
package npc

import "math"

// BankerRuntimeState holds the mutable daily state for a banker NPC.
type BankerRuntimeState struct {
	// CurrentRate is the current exchange rate. Updated daily.
	// Deposit adds floor(amount * CurrentRate) to stash.
	// Withdrawal gives floor(stash / CurrentRate) carried credits.
	CurrentRate float64
}

// NewCurrentRateFromDelta computes the banker's exchange rate given a variance delta.
// Formula: clamp(BaseRate + delta, 0.5, 1.0).
//
// Precondition: cfg must be non-nil.
// Postcondition: Returns a value in [0.5, 1.0].
func NewCurrentRateFromDelta(cfg *BankerConfig, delta float64) float64 {
	rate := cfg.BaseRate + delta
	if rate < 0.5 {
		rate = 0.5
	}
	if rate > 1.0 {
		rate = 1.0
	}
	return rate
}

// ComputeDeposit returns stash credits added when a player deposits amount at rate.
// Formula: floor(amount * rate).
//
// Precondition: amount >= 0; rate in [0.5, 1.0].
// Postcondition: Returns a value in [0, amount].
func ComputeDeposit(amount int, rate float64) int {
	return int(math.Floor(float64(amount) * rate))
}

// ComputeWithdrawal returns carried credits received when withdrawing stashAmount at rate.
// Formula: floor(stashAmount / rate).
//
// Precondition: stashAmount >= 0; rate in (0, 1.0].
// Postcondition: Returns a value >= stashAmount when rate < 1.0.
func ComputeWithdrawal(stashAmount int, rate float64) int {
	if rate <= 0 {
		return stashAmount
	}
	return int(math.Floor(float64(stashAmount) / rate))
}
