package gameserver

// FlavorText returns an atmospheric sentence appended to outdoor room descriptions.
// Returns empty string for indoor rooms (isOutdoor == false).
//
// Precondition: period is one of the eight TimePeriod constants.
// Postcondition: Returns a non-empty string for outdoor rooms, empty for indoor.
func FlavorText(period TimePeriod, isOutdoor bool) string {
	if !isOutdoor {
		return ""
	}
	switch period {
	case PeriodMidnight:
		return "The world is cloaked in deep darkness; only faint starlight remains."
	case PeriodLateNight:
		return "The night presses close, silent and still."
	case PeriodDawn:
		return "A pale blush of light edges the horizon as dawn breaks."
	case PeriodMorning:
		return "Morning light floods the area, casting long shadows."
	case PeriodAfternoon:
		return "The sun hangs high overhead, bright and relentless."
	case PeriodDusk:
		return "The sky burns orange and red as the sun sinks toward the horizon."
	case PeriodEvening:
		return "Twilight settles softly, the first stars beginning to appear."
	default: // PeriodNight
		return "A canopy of stars fills the night sky above."
	}
}

// IsDarkPeriod reports whether a period reduces outdoor visibility.
//
// Postcondition: Returns true for Midnight, LateNight, Night.
func IsDarkPeriod(period TimePeriod) bool {
	return period == PeriodMidnight || period == PeriodLateNight || period == PeriodNight
}
