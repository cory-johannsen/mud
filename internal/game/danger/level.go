package danger

// DangerLevel represents the threat level of a zone or room.
type DangerLevel string

const (
	Safe      DangerLevel = "safe"
	Sketchy   DangerLevel = "sketchy"
	Dangerous DangerLevel = "dangerous"
	AllOutWar DangerLevel = "all_out_war"
)

// EffectiveDangerLevel returns roomDanger if non-empty, else zoneDanger.
// Precondition: zoneDanger MUST be a valid DangerLevel string.
// Postcondition: returns the effective DangerLevel for the room.
func EffectiveDangerLevel(zoneDanger, roomDanger string) DangerLevel {
	if roomDanger != "" {
		return DangerLevel(roomDanger)
	}
	return DangerLevel(zoneDanger)
}
