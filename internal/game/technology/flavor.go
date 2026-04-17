package technology

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// TraditionFlavor holds the player-facing copy for a technology tradition.
type TraditionFlavor struct {
	LoadoutTitle string
	PrepVerb     string
	PrepGerund   string // gerund form of PrepVerb, e.g. "Configuring"
	SlotNoun     string
	RestMessage  string
}

var fallbackFlavor = TraditionFlavor{
	LoadoutTitle: "Loadout",
	PrepVerb:     "Prepare",
	PrepGerund:   "Preparing",
	SlotNoun:     "slot",
	RestMessage:  "Technologies prepared.",
}

var traditionFlavors = map[string]TraditionFlavor{
	"technical": {
		LoadoutTitle: "Field Loadout",
		PrepVerb:     "Configure",
		PrepGerund:   "Configuring",
		SlotNoun:     "slot",
		RestMessage:  "Field loadout configured.",
	},
	"bio_synthetic": {
		LoadoutTitle: "Chem Kit",
		PrepVerb:     "Mix",
		PrepGerund:   "Mixing",
		SlotNoun:     "dose",
		RestMessage:  "Chem kit mixed.",
	},
	"neural": {
		LoadoutTitle: "Neural Profile",
		PrepVerb:     "Queue",
		PrepGerund:   "Queuing",
		SlotNoun:     "routine",
		RestMessage:  "Neural profile written.",
	},
	"fanatic_doctrine": {
		LoadoutTitle: "Doctrine",
		PrepVerb:     "Prepare",
		PrepGerund:   "Preparing",
		SlotNoun:     "rite",
		RestMessage:  "Doctrine prepared.",
	},
}

var jobTradition = map[string]string{
	"nerd":       "technical",
	"naturalist": "bio_synthetic",
	"drifter":    "bio_synthetic",
	"schemer":    "neural",
	"influencer": "neural",
	"zealot":     "fanatic_doctrine",
}

// FlavorFor returns the TraditionFlavor for the given tradition string.
// For any unknown or empty tradition, the fallback flavor is returned.
func FlavorFor(tradition string) TraditionFlavor {
	if f, ok := traditionFlavors[tradition]; ok {
		return f
	}
	return fallbackFlavor
}

// DominantTradition returns the primary technology tradition for an archetype ID.
// Returns "" for unknown or non-tech archetype IDs (e.g. "aggressor", "criminal").
func DominantTradition(archetypeID string) string {
	return jobTradition[archetypeID]
}

// FormatPreparedTechs formats the prepared tech slots grouped by level in
// ascending level order.
//
// Precondition: flavor is a valid TraditionFlavor (zero value produces empty labels).
// Postcondition: Returns "No <LoadoutTitle> configured." when slots is empty or all levels
// have zero entries; otherwise returns the full formatted display string.
func FormatPreparedTechs(slots map[int][]*session.PreparedSlot, flavor TraditionFlavor, reg *Registry) string {
	if len(slots) == 0 {
		return fmt.Sprintf("No %s configured.", flavor.LoadoutTitle)
	}

	levels := make([]int, 0, len(slots))
	for lvl, s := range slots {
		if len(s) > 0 {
			levels = append(levels, lvl)
		}
	}
	if len(levels) == 0 {
		return fmt.Sprintf("No %s configured.", flavor.LoadoutTitle)
	}
	sort.Ints(levels)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s]", flavor.LoadoutTitle))

	for _, lvl := range levels {
		s := slots[lvl]
		noun := flavor.SlotNoun
		count := len(s)
		plural := "s"
		if count == 1 {
			plural = ""
		}
		sb.WriteString(fmt.Sprintf("\n  Level %d — %d %s%s", lvl, count, noun, plural))
		for _, slot := range s {
			state := "ready"
			if slot.Expended {
				state = "expended"
			}
			name := slot.TechID
			if reg != nil {
				if def, ok := reg.Get(slot.TechID); ok {
					name = def.Name
				}
			}
			sb.WriteString(fmt.Sprintf("\n    %s    %s", name, state))
		}
	}

	return sb.String()
}
