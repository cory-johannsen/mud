package danger

// Roller is a source of random integers.
// Roll(max) returns a value in [0, max).
type Roller interface {
	Roll(max int) int
}

// defaultTrapPcts maps danger level to [roomPct, coverPct].
var defaultTrapPcts = map[DangerLevel][2]int{
	Safe:      {0, 0},
	Sketchy:   {0, 15},
	Dangerous: {35, 50},
	AllOutWar: {60, 75},
}

// RollRoomTrap returns true if a room trap should trigger.
// override is nil to use the danger level default; non-nil uses the explicit value (including 0).
// Precondition: rng MUST NOT be nil.
// Postcondition: returns false when effective pct <= 0; returns rng.Roll(100) < pct otherwise.
func RollRoomTrap(level DangerLevel, override *int, rng Roller) bool {
	pct := defaultTrapPcts[level][0]
	if override != nil {
		pct = *override
	}
	if pct <= 0 {
		return false
	}
	return rng.Roll(100) < pct
}

// RollCoverTrap returns true if a cover trap should trigger.
// override is nil to use the danger level default; non-nil uses the explicit value (including 0).
// Precondition: rng MUST NOT be nil.
// Postcondition: returns false when effective pct <= 0; returns rng.Roll(100) < pct otherwise.
func RollCoverTrap(level DangerLevel, override *int, rng Roller) bool {
	pct := defaultTrapPcts[level][1]
	if override != nil {
		pct = *override
	}
	if pct <= 0 {
		return false
	}
	return rng.Roll(100) < pct
}
