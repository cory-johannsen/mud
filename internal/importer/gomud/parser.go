package gomud

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ParseZone parses a gomud zone YAML file.
//
// Precondition: data must be valid YAML.
// Postcondition: returns a non-nil GomudZone or a non-nil error.
func ParseZone(data []byte) (*GomudZone, error) {
	var z GomudZone
	if err := yaml.Unmarshal(data, &z); err != nil {
		return nil, fmt.Errorf("parsing gomud zone: %w", err)
	}
	return &z, nil
}

// ParseArea parses a gomud area YAML file.
//
// Precondition: data must be valid YAML.
// Postcondition: returns a non-nil GomudArea or a non-nil error.
func ParseArea(data []byte) (*GomudArea, error) {
	var a GomudArea
	if err := yaml.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parsing gomud area: %w", err)
	}
	return &a, nil
}

// ParseRoom parses a gomud room YAML file.
// The objects field in the source is silently ignored.
//
// Precondition: data must be valid YAML.
// Postcondition: returns a non-nil GomudRoom or a non-nil error.
func ParseRoom(data []byte) (*GomudRoom, error) {
	var r GomudRoom
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parsing gomud room: %w", err)
	}
	return &r, nil
}
