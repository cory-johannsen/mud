// internal/game/reaction/budget.go
package reaction

// Budget tracks a combatant's per-round reaction spending.
//
// Invariants (maintained at all observation points):
//   - Max >= 0
//   - 0 <= Spent <= Max
//
// TrySpend and Refund are the only public mutators post-construction.
type Budget struct {
	Max   int
	Spent int
}

// Remaining returns the number of unspent reactions (Max - Spent).
func (b *Budget) Remaining() int { return b.Max - b.Spent }

// TrySpend attempts to spend one reaction.
// Returns true and increments Spent when Spent < Max.
// Returns false without mutation when Spent >= Max.
func (b *Budget) TrySpend() bool {
	if b.Spent >= b.Max {
		return false
	}
	b.Spent++
	return true
}

// Refund decrements Spent by one, flooring at 0.
func (b *Budget) Refund() {
	if b.Spent > 0 {
		b.Spent--
	}
}

// Reset sets Max = n (clamped to 0 if negative) and Spent = 0.
func (b *Budget) Reset(n int) {
	if n < 0 {
		n = 0
	}
	b.Max = n
	b.Spent = 0
}
