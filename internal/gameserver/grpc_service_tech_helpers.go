package gameserver

import "github.com/cory-johannsen/mud/internal/game/session"

// snapshotPreparedTechIDs returns a set of all tech IDs currently in PreparedTechs.
//
// Precondition: pt may be nil.
// Postcondition: Returns a non-nil map of tech IDs present in any prepared slot.
func snapshotPreparedTechIDs(pt map[int][]*session.PreparedSlot) map[string]bool {
	out := make(map[string]bool)
	for _, slots := range pt {
		for _, s := range slots {
			if s != nil {
				out[s.TechID] = true
			}
		}
	}
	return out
}

// snapshotSpontaneousTechIDs returns a set of all tech IDs in SpontaneousTechs.
//
// Precondition: st may be nil.
// Postcondition: Returns a non-nil map of tech IDs present in any spontaneous slot.
func snapshotSpontaneousTechIDs(st map[int][]string) map[string]bool {
	out := make(map[string]bool)
	for _, ids := range st {
		for _, id := range ids {
			out[id] = true
		}
	}
	return out
}

// newTechIDs returns IDs in after that are not in before.
//
// Precondition: before and after may be nil.
// Postcondition: Returns a slice of IDs that appear in after but not in before.
func newTechIDs(before []string, after []string) []string {
	s := make(map[string]bool, len(before))
	for _, id := range before {
		s[id] = true
	}
	var result []string
	for _, id := range after {
		if !s[id] {
			result = append(result, id)
		}
	}
	return result
}

// newTechIDsFromPrepared returns tech IDs in after that are not in the before snapshot.
//
// Precondition: before is a non-nil map; after may be nil.
// Postcondition: Returns IDs present in after slots that were absent in before.
func newTechIDsFromPrepared(before map[string]bool, after map[int][]*session.PreparedSlot) []string {
	var result []string
	for _, slots := range after {
		for _, slot := range slots {
			if slot != nil && !before[slot.TechID] {
				result = append(result, slot.TechID)
			}
		}
	}
	return result
}

// newTechIDsFromSpontaneous returns tech IDs in after that are not in the before snapshot.
//
// Precondition: before is a non-nil map; after may be nil.
// Postcondition: Returns IDs present in after that were absent in before.
func newTechIDsFromSpontaneous(before map[string]bool, after map[int][]string) []string {
	var result []string
	for _, ids := range after {
		for _, id := range ids {
			if !before[id] {
				result = append(result, id)
			}
		}
	}
	return result
}
