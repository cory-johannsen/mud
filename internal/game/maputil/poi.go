// Package maputil provides helpers for map rendering and POI classification.
package maputil

import (
	"sort"
	"strings"
)

// POIType describes a single Point of Interest type.
type POIType struct {
	ID     string
	Symbol rune
	Color  string // ANSI escape sequence, e.g. "\033[36m"
	Label  string
}

// POITypes is the ordered, immutable POI type table.
// Rendering and sort order are determined by position in this slice.
var POITypes = []POIType{
	{ID: "merchant", Symbol: '$', Color: "\033[36m", Label: "Merchant"},
	{ID: "healer", Symbol: '+', Color: "\033[32m", Label: "Healer"},
	{ID: "trainer", Symbol: 'T', Color: "\033[34m", Label: "Trainer"},
	{ID: "guard", Symbol: 'G', Color: "\033[33m", Label: "Guard"},
	{ID: "quest_giver", Symbol: 'Q', Color: "\033[93m", Label: "Quest"},
	{ID: "npc", Symbol: 'N', Color: "\033[37m", Label: "NPC"},
	{ID: "map", Symbol: 'M', Color: "\033[96m", Label: "Map"},
	{ID: "cover", Symbol: 'C', Color: "\033[33m", Label: "Cover"},
	{ID: "equipment", Symbol: 'E', Color: "\033[35m", Label: "Equipment"},
}

// poiOrder maps POI type ID to its index in POITypes for sort comparisons.
var poiOrder = func() map[string]int {
	m := make(map[string]int, len(POITypes))
	for i, p := range POITypes {
		m[p.ID] = i
	}
	return m
}()

// POIRoleFromNPCType derives a POI role string from an NPC's npc_type field,
// used as a fallback when the NPC has no explicit npc_role set.
//
// Precondition: npcType may be any string including empty.
// Postcondition: Returns "" for "combat" and "" (no POI contribution).
// Returns the npcType unchanged for all other non-empty types so that
// NpcRoleToPOIID can map them to their POI ID.
func POIRoleFromNPCType(npcType string) string {
	switch npcType {
	case "", "combat":
		return ""
	default:
		return npcType
	}
}

// NpcRoleToPOIID maps an npc_role string to a POI type ID.
//
// Precondition: npcRole may be any string including empty.
// Postcondition: Returns "" for empty npcRole (combat NPC; no POI contribution).
// Returns "npc" for any non-empty role not in the explicit mapping.
func NpcRoleToPOIID(npcRole string) string {
	switch strings.ToLower(npcRole) {
	case "":
		return ""
	case "merchant":
		return "merchant"
	case "healer":
		return "healer"
	case "job_trainer":
		return "trainer"
	case "guard":
		return "guard"
	case "quest_giver":
		return "quest_giver"
	default:
		return "npc"
	}
}

// SortPOIs returns a new slice of POI type IDs sorted in POITypes table order.
// Unknown IDs are sorted after all known IDs (REQ-POI-21).
//
// Precondition: pois may be nil or empty.
// Postcondition: Input slice is not mutated; returned slice is a fresh copy.
func SortPOIs(pois []string) []string {
	out := make([]string, len(pois))
	copy(out, pois)
	sort.SliceStable(out, func(i, j int) bool {
		oi, okI := poiOrder[out[i]]
		oj, okJ := poiOrder[out[j]]
		if !okI {
			oi = len(POITypes)
		}
		if !okJ {
			oj = len(POITypes)
		}
		return oi < oj
	})
	return out
}

// POISuffixRow builds a padded POI suffix string for a room cell of visible width cellW.
//
// Precondition: cellW >= 1; pois is already sorted in table order.
// Postcondition: Returns a string of exactly cellW visible display columns.
// Up to 4 colored symbols are shown. If len(pois) > 4, the 4th visible slot
// is U+2026 (…) with no color. Returns cellW spaces when pois is empty.
func POISuffixRow(pois []string, cellW int) string {
	if len(pois) == 0 {
		return strings.Repeat(" ", cellW)
	}
	var sb strings.Builder
	displayCols := 0
	for i, id := range pois {
		if displayCols >= cellW {
			break
		}
		if i == 3 && len(pois) > 4 {
			// 4th slot: show ellipsis only when 5+ types present (REQ-POI-4).
			sb.WriteString("…") // U+2026, 3 bytes but 1 display column
			displayCols++
			break
		}
		sym := '?'
		color := ""
		for _, pt := range POITypes {
			if pt.ID == id {
				sym = pt.Symbol
				color = pt.Color
				break
			}
		}
		sb.WriteString(color)
		sb.WriteRune(sym)
		sb.WriteString("\033[0m")
		displayCols++
	}
	// Pad to cellW visible columns.
	for displayCols < cellW {
		sb.WriteByte(' ')
		displayCols++
	}
	return sb.String()
}
