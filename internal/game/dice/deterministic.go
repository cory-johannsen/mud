package dice

import "go.uber.org/zap"

// deterministicSource implements Source by replaying a fixed sequence of values.
// Each call to Intn returns the next value in the sequence modulo n.
// If the sequence is exhausted, subsequent calls return 0.
//
// Precondition: vals must be non-nil.
// Postcondition: Each Intn(n) call consumes one value and returns it mod n.
type deterministicSource struct {
	vals []int
	idx  int
}

// NewDeterministicSource returns a Source that replays vals in order.
// Each Intn(n) call returns vals[i] % n, advancing the index.
// When the sequence is exhausted, 0 is returned.
//
// Precondition: vals must be non-nil.
// Postcondition: Returns a non-nil Source.
func NewDeterministicSource(vals []int) Source {
	return &deterministicSource{vals: vals}
}

// Intn returns the next value in the sequence modulo n.
//
// Precondition: n > 0.
// Postcondition: Returns a value in [0, n).
func (d *deterministicSource) Intn(n int) int {
	if n <= 0 {
		panic("dice: Intn called with n <= 0")
	}
	if d.idx >= len(d.vals) {
		return 0
	}
	v := d.vals[d.idx] % n
	d.idx++
	return v
}

// NewRoller creates a Roller using src and a no-op logger.
// Suitable for tests that do not need roll logging.
//
// Precondition: src must be non-nil.
// Postcondition: Returns a non-nil Roller.
func NewRoller(src Source) *Roller {
	return &Roller{src: src, logger: zap.NewNop()}
}
