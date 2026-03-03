package command

import (
	"fmt"
	"strings"
)

// WhoEntry holds the display fields for one player in the who list.
type WhoEntry struct {
	Name        string
	Level       int
	Job         string
	HealthLabel string
	Status      string
}

// HealthLabel returns a descriptive health label based on current and max HP.
//
// Precondition: max may be zero (treated as 0% HP).
// Postcondition: Returns one of: "Uninjured", "Lightly Wounded", "Wounded", "Badly Wounded", "Near Death".
func HealthLabel(current, max int) string {
	if max <= 0 {
		return "Near Death"
	}
	pct := current * 100 / max
	switch {
	case pct >= 100:
		return "Uninjured"
	case pct >= 75:
		return "Lightly Wounded"
	case pct >= 50:
		return "Wounded"
	case pct >= 25:
		return "Badly Wounded"
	default:
		return "Near Death"
	}
}

// StatusLabel returns a human-readable label for a CombatStatus int32 value.
//
// Precondition: status may be any int32.
// Postcondition: Returns one of: "Idle", "In Combat", "Resting", "Unconscious".
func StatusLabel(status int32) string {
	switch status {
	case 2:
		return "In Combat"
	case 3:
		return "Resting"
	case 4:
		return "Unconscious"
	default:
		return "Idle"
	}
}

// HandleWho returns a plain-text who listing from a slice of WhoEntry.
// Used as a fallback for telnet/dev-server clients.
//
// Precondition: entries may be nil or empty.
// Postcondition: Returns a non-empty human-readable string.
func HandleWho(entries []WhoEntry) string {
	if len(entries) == 0 {
		return "Nobody else is here."
	}
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("  %s — Lvl %d %s — %s — %s\r\n",
			e.Name, e.Level, e.Job, e.HealthLabel, e.Status))
	}
	return sb.String()
}
