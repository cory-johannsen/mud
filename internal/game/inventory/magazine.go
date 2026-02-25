package inventory

import (
	"errors"
	"fmt"
)

// Magazine tracks loaded round count for one firearm instance.
// Invariant: 0 <= Loaded <= Capacity.
type Magazine struct {
	// WeaponID identifies the firearm this magazine belongs to.
	WeaponID string
	// Loaded is the number of rounds currently available.
	Loaded int
	// Capacity is the maximum number of rounds the magazine can hold.
	Capacity int
}

// NewMagazine returns a fully loaded Magazine for the given weaponID.
//
// Precondition:  capacity > 0 (panics otherwise).
// Postcondition: Loaded == Capacity == capacity.
func NewMagazine(weaponID string, capacity int) *Magazine {
	if capacity <= 0 {
		panic(fmt.Sprintf("inventory: NewMagazine: capacity must be > 0, got %d", capacity))
	}
	return &Magazine{
		WeaponID: weaponID,
		Loaded:   capacity,
		Capacity: capacity,
	}
}

// IsEmpty returns true when Loaded <= 0.
//
// Postcondition: result == (Loaded <= 0).
func (m *Magazine) IsEmpty() bool {
	return m.Loaded <= 0
}

// Consume removes n rounds from the magazine.
//
// Precondition:  n > 0 (panics if n <= 0).
// Postcondition: on success Loaded decreases by n; returns error if Loaded < n.
func (m *Magazine) Consume(n int) error {
	if n <= 0 {
		panic(fmt.Sprintf("inventory: Magazine.Consume: n must be > 0, got %d", n))
	}
	if m.Loaded < n {
		return errors.New("inventory: Magazine.Consume: insufficient rounds loaded")
	}
	m.Loaded -= n
	return nil
}

// Reload restores Loaded to Capacity.
//
// Postcondition: Loaded == Capacity.
func (m *Magazine) Reload() {
	m.Loaded = m.Capacity
}
