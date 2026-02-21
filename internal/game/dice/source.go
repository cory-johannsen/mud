package dice

import (
	"crypto/rand"
	"math/big"
)

// cryptoSource implements Source using crypto/rand.
//
// Invariant: All values produced are cryptographically secure and uniformly
// distributed in [0, n) for any n > 0.
type cryptoSource struct{}

// NewCryptoSource returns a Source backed by crypto/rand.
//
// Postcondition: Every value returned by Intn is in [0, n).
func NewCryptoSource() Source {
	return &cryptoSource{}
}

// Intn returns a cryptographically secure random int in [0, n).
//
// Precondition: n > 0. Panics with "dice: Intn called with n <= 0" if n <= 0.
// Panics with "dice: crypto/rand failure: <err>" if crypto/rand fails.
func (c *cryptoSource) Intn(n int) int {
	if n <= 0 {
		panic("dice: Intn called with n <= 0")
	}
	val, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic("dice: crypto/rand failure: " + err.Error())
	}
	return int(val.Int64())
}
