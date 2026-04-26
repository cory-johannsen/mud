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

// RoomTrapPayload describes the default damage and condition for a random room trap
// firing at the given danger level.
type RoomTrapPayload struct {
	DamageDice  string // dice expression; "" means no damage
	DamageType  string // e.g. "piercing"
	ConditionID string // e.g. "bleeding"; "" means no condition
}

// DefaultRoomTrapPayload returns the canonical room-trap payload by danger level.
// Safe and Sketchy levels never roll room traps (see defaultTrapPcts), so they
// have no payload. Dangerous and AllOutWar return progressively heavier payloads.
//
// Postcondition: returned payload's DamageDice is non-empty for Dangerous/AllOutWar.
func DefaultRoomTrapPayload(level DangerLevel) RoomTrapPayload {
	switch level {
	case Dangerous:
		return RoomTrapPayload{DamageDice: "2d6", DamageType: "piercing", ConditionID: "bleeding"}
	case AllOutWar:
		return RoomTrapPayload{DamageDice: "4d6", DamageType: "piercing", ConditionID: "prone"}
	default:
		return RoomTrapPayload{}
	}
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
