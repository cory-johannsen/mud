package behavior

import (
	"fmt"
	"strconv"
	"strings"
)

// ScheduleEntry defines one time-of-day behavior window for an NPC.
type ScheduleEntry struct {
	// Hours is a range ("6-18") or comma-separated ("8,12,20") hour specification.
	Hours string `yaml:"hours"`
	// PreferredRoom is the room ID the NPC moves toward during this window.
	PreferredRoom string `yaml:"preferred_room"`
	// BehaviorMode is one of "idle", "patrol", "aggressive".
	BehaviorMode string `yaml:"behavior_mode"`
}

// ParseHours parses a schedule hours string into a slice of hour values (0–23).
// Supports range format ("6-18", wrapping when end < start) and comma-separated ("8,12,20").
//
// Precondition: s must be non-empty.
// Postcondition: all returned values are in [0, 23]; returns an error on parse failure.
func ParseHours(s string) ([]int, error) {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "-") {
		parts := strings.SplitN(s, "-", 2)
		start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("behavior.ParseHours: invalid start hour %q: %w", parts[0], err)
		}
		end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("behavior.ParseHours: invalid end hour %q: %w", parts[1], err)
		}
		if start < 0 || start > 23 || end < 0 || end > 23 {
			return nil, fmt.Errorf("behavior.ParseHours: hours must be in [0,23], got %d-%d", start, end)
		}
		var hours []int
		if end >= start {
			for h := start; h <= end; h++ {
				hours = append(hours, h)
			}
		} else {
			// wrap midnight
			for h := start; h <= 23; h++ {
				hours = append(hours, h)
			}
			for h := 0; h <= end; h++ {
				hours = append(hours, h)
			}
		}
		return hours, nil
	}
	// comma-separated
	parts := strings.Split(s, ",")
	hours := make([]int, 0, len(parts))
	for _, p := range parts {
		h, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("behavior.ParseHours: invalid hour %q: %w", p, err)
		}
		if h < 0 || h > 23 {
			return nil, fmt.Errorf("behavior.ParseHours: hour %d out of range [0,23]", h)
		}
		hours = append(hours, h)
	}
	if len(hours) == 0 {
		return nil, fmt.Errorf("behavior.ParseHours: no hours parsed from %q", s)
	}
	return hours, nil
}

// ActiveEntry returns the first schedule entry whose hours include the given game hour,
// or nil if no entry matches.
//
// Precondition: hour must be in [0, 23].
// Postcondition: returns nil when entries is empty or no entry matches hour.
func ActiveEntry(entries []ScheduleEntry, hour int) *ScheduleEntry {
	for i := range entries {
		hours, err := ParseHours(entries[i].Hours)
		if err != nil {
			continue
		}
		for _, h := range hours {
			if h == hour {
				return &entries[i]
			}
		}
	}
	return nil
}
