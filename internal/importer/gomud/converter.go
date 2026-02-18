package gomud

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/importer"
)

// ConvertZone transforms a parsed GomudZone and its supporting data into a
// ZoneData ready for serialisation and validation.
//
// Precondition: zone must be non-nil; rooms is the full map of known room
// display names to GomudRoom; roomArea maps room display names to area display
// names (may be nil); startRoom is an optional display-name override for the
// zone's start room.
//
// Postcondition: returns a non-nil ZoneData and a (possibly empty) slice of
// warning strings for recoverable issues (missing rooms, unknown exit targets).
func ConvertZone(
	zone *GomudZone,
	rooms map[string]*GomudRoom,
	roomArea map[string]string,
	startRoom string,
) (*importer.ZoneData, []string) {
	var warnings []string

	zoneID := importer.NameToID(zone.Name)

	// Build the name→ID lookup from all rooms known to this zone.
	nameToID := make(map[string]string, len(zone.Rooms))
	for _, name := range zone.Rooms {
		nameToID[strings.TrimSpace(name)] = importer.NameToID(strings.TrimSpace(name))
	}

	// Determine start room ID.
	startRoomID := ""
	if startRoom != "" {
		startRoomID = importer.NameToID(startRoom)
	} else if len(zone.Rooms) > 0 {
		startRoomID = importer.NameToID(strings.TrimSpace(zone.Rooms[0]))
	}

	var roomSpecs []importer.RoomSpec
	for _, rawName := range zone.Rooms {
		name := strings.TrimSpace(rawName)
		room, ok := rooms[name]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("zone %q: room %q has no definition file; skipping", zone.Name, name))
			continue
		}

		props := make(map[string]string)
		if roomArea != nil {
			if area, found := roomArea[name]; found {
				props["area"] = importer.NameToID(area)
			}
		}

		var exits []importer.ExitSpec
		for _, exit := range room.Exits {
			target := strings.TrimSpace(exit.Target)
			targetID, known := nameToID[target]
			if !known {
				warnings = append(warnings, fmt.Sprintf(
					"room %q: exit target %q has no room definition; dropping exit",
					name, target,
				))
				continue
			}
			exits = append(exits, importer.ExitSpec{
				Direction: strings.ToLower(exit.Direction),
				Target:    targetID,
			})
		}

		roomSpecs = append(roomSpecs, importer.RoomSpec{
			ID:          importer.NameToID(name),
			Title:       room.Name,
			Description: strings.TrimSpace(room.Description),
			Exits:       exits,
			Properties:  props,
		})
	}

	return &importer.ZoneData{
		Zone: importer.ZoneSpec{
			ID:          zoneID,
			Name:        zone.Name,
			Description: zone.Description,
			StartRoom:   startRoomID,
			Rooms:       roomSpecs,
		},
	}, warnings
}
