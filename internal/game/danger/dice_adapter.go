package danger

import "math/rand"

// RandRoller wraps math/rand as a Roller.
// Use danger.RandRoller{} wherever a production Roller is required.
// This avoids importing the internal dice package from within the danger package.
type RandRoller struct{}

func (RandRoller) Roll(max int) int { return rand.Intn(max) }
