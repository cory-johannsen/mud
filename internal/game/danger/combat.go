package danger

// CanInitiateCombat reports whether the given initiator may start combat
// at this danger level. initiator is "player" or "npc".
// Precondition: initiator MUST be "player" or "npc".
// Postcondition: returns false for all initiators in Safe rooms;
//   NPCs cannot initiate in Sketchy rooms; both may initiate in Dangerous and AllOutWar.
// Note: guard enforcement via InitiateGuardCombat bypasses this function.
func CanInitiateCombat(level DangerLevel, initiator string) bool {
	switch level {
	case Safe:
		return false
	case Sketchy:
		return initiator == "player"
	case Dangerous, AllOutWar:
		return true
	default:
		return false
	}
}
