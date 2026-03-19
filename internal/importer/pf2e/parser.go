package pf2e

import (
	"encoding/json"
	"fmt"
)

// ParseSpell unmarshals a PF2E compendium spell JSON byte slice into a PF2ESpell.
//
// Precondition: data must be a valid JSON byte slice.
// Postcondition: returns a non-nil PF2ESpell or a non-nil error.
func ParseSpell(data []byte) (*PF2ESpell, error) {
	var spell PF2ESpell
	if err := json.Unmarshal(data, &spell); err != nil {
		return nil, fmt.Errorf("parsing pf2e spell: %w", err)
	}
	return &spell, nil
}
