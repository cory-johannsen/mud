// Package world provides the game world model: zones, rooms, exits, and directions.
package world

import "fmt"

// Direction represents a compass direction or named exit.
type Direction string

// Standard compass directions and vertical movements.
const (
	North     Direction = "north"
	South     Direction = "south"
	East      Direction = "east"
	West      Direction = "west"
	Northeast Direction = "northeast"
	Northwest Direction = "northwest"
	Southeast Direction = "southeast"
	Southwest Direction = "southwest"
	Up        Direction = "up"
	Down      Direction = "down"
)

// StandardDirections contains all standard compass and vertical directions.
var StandardDirections = []Direction{
	North, South, East, West,
	Northeast, Northwest, Southeast, Southwest,
	Up, Down,
}

// IsStandard reports whether d is one of the ten standard directions.
func (d Direction) IsStandard() bool {
	for _, sd := range StandardDirections {
		if d == sd {
			return true
		}
	}
	return false
}

// Opposite returns the opposite of a standard direction.
// For custom directions, it returns an empty string.
//
// Precondition: d should be a standard direction for a meaningful result.
func (d Direction) Opposite() Direction {
	switch d {
	case North:
		return South
	case South:
		return North
	case East:
		return West
	case West:
		return East
	case Northeast:
		return Southwest
	case Southwest:
		return Northeast
	case Northwest:
		return Southeast
	case Southeast:
		return Northwest
	case Up:
		return Down
	case Down:
		return Up
	default:
		return ""
	}
}

// Exit represents a passage from one room to another.
type Exit struct {
	// Direction is the compass direction or named exit (e.g., "stairs").
	Direction Direction
	// TargetRoom is the ID of the destination room.
	TargetRoom string
	// Locked indicates the exit requires a key or condition to pass.
	Locked bool
	// Hidden indicates the exit is not visible by default.
	Hidden bool
}

// RoomSpawnConfig defines how many instances of an NPC template should exist
// in a room and how long to wait before respawning a dead one.
type RoomSpawnConfig struct {
	// Template is the NPC template ID to spawn.
	Template string
	// Count is the maximum number of live instances of this template in the room.
	Count int
	// RespawnAfter is an optional duration string overriding the template's
	// respawn_delay. Empty means use the template's default.
	RespawnAfter string
}

// Room represents a location in the game world.
type Room struct {
	// ID uniquely identifies this room within the zone.
	ID string
	// ZoneID identifies the zone this room belongs to.
	ZoneID string
	// Title is the short display name of the room.
	Title string
	// Description is the multi-line room description shown to players.
	Description string
	// Exits lists all passages leading out of this room.
	Exits []Exit
	// Properties holds environment tags (lighting, atmosphere, etc.).
	Properties map[string]string
	// Spawns lists NPC templates that populate this room and their respawn config.
	Spawns []RoomSpawnConfig
}

// ExitForDirection returns the exit in the given direction, if one exists.
//
// Postcondition: Returns (exit, true) if found, or (Exit{}, false) otherwise.
func (r *Room) ExitForDirection(dir Direction) (Exit, bool) {
	for _, e := range r.Exits {
		if e.Direction == dir {
			return e, true
		}
	}
	return Exit{}, false
}

// VisibleExits returns all non-hidden exits from this room.
//
// Postcondition: Returns a slice of exits where Hidden is false.
func (r *Room) VisibleExits() []Exit {
	var visible []Exit
	for _, e := range r.Exits {
		if !e.Hidden {
			visible = append(visible, e)
		}
	}
	return visible
}

// Zone groups related rooms into a themed area.
type Zone struct {
	// ID uniquely identifies this zone.
	ID string
	// Name is the display name of the zone.
	Name string
	// Description summarizes the zone's theme.
	Description string
	// StartRoom is the ID of the default entry room.
	StartRoom string
	// Rooms contains all rooms in this zone, keyed by room ID.
	Rooms map[string]*Room
	// ScriptDir is the path to Lua scripts for this zone. Empty = no scripts.
	ScriptDir string
	// ScriptInstructionLimit overrides DefaultInstructionLimit for this zone's VM.
	// 0 = use DefaultInstructionLimit.
	ScriptInstructionLimit int
}

// Validate checks zone invariants.
//
// Postcondition: Returns nil if valid, or an error describing the first violation.
func (z *Zone) Validate() error {
	if z.ID == "" {
		return fmt.Errorf("zone ID must not be empty")
	}
	if z.Name == "" {
		return fmt.Errorf("zone %q: name must not be empty", z.ID)
	}
	if z.StartRoom == "" {
		return fmt.Errorf("zone %q: start_room must not be empty", z.ID)
	}
	if len(z.Rooms) == 0 {
		return fmt.Errorf("zone %q: must contain at least one room", z.ID)
	}
	if _, ok := z.Rooms[z.StartRoom]; !ok {
		return fmt.Errorf("zone %q: start_room %q not found in rooms", z.ID, z.StartRoom)
	}
	for id, room := range z.Rooms {
		if room.ID != id {
			return fmt.Errorf("zone %q: room key %q does not match room ID %q", z.ID, id, room.ID)
		}
		if room.Title == "" {
			return fmt.Errorf("zone %q: room %q: title must not be empty", z.ID, id)
		}
		if room.Description == "" {
			return fmt.Errorf("zone %q: room %q: description must not be empty", z.ID, id)
		}
		for _, exit := range room.Exits {
			if exit.TargetRoom == "" {
				return fmt.Errorf("zone %q: room %q: exit %q has empty target", z.ID, id, exit.Direction)
			}
			if _, ok := z.Rooms[exit.TargetRoom]; !ok {
				return fmt.Errorf("zone %q: room %q: exit %q targets unknown room %q", z.ID, id, exit.Direction, exit.TargetRoom)
			}
		}
	}
	return nil
}
