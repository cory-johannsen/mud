// Package world provides the game world model: zones, rooms, exits, and directions.
package world

import (
	"fmt"
	"time"

	"github.com/cory-johannsen/mud/internal/game/skillcheck"
)

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
	ClimbDC int `yaml:"climb_dc"` // 0 = not climbable (unless terrain default applies)
	Height  int `yaml:"height"`   // feet; used for fall damage: max(1, floor(Height/10)) d6
	SwimDC  int `yaml:"swim_dc"`  // 0 = not swimmable (unless terrain default applies)
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

// RoomEquipmentConfig defines a static or respawning item present in a room.
//
// Precondition: ItemID must reference a valid ItemDef ID.
// Postcondition: Immovable items with RespawnAfter==0 persist indefinitely.
type RoomEquipmentConfig struct {
	ItemID       string                  // references inventory.ItemDef.ID
	Description  string                  // player-visible name for interact command matching
	MaxCount     int                     // max live instances allowed in this room
	RespawnAfter time.Duration           // 0 = permanent (never despawn); >0 = respawn after pickup
	Immovable    bool                    // if true, cannot be picked up
	Script       string                  // path to Lua script for use effect; empty = no effect
	SkillChecks  []skillcheck.TriggerDef // skill check triggers fired on_use
	// CoverTier specifies the cover tier this equipment provides: "lesser", "standard",
	// "greater", or "" (no cover). Only meaningful when Immovable is true.
	CoverTier         string `yaml:"cover_tier"`
	// CoverDestructible indicates whether this cover object can be degraded and destroyed.
	CoverDestructible bool   `yaml:"cover_destructible"`
	// CoverHP is the number of hits this cover object can absorb before being destroyed.
	// Only meaningful when CoverDestructible is true.
	CoverHP           int    `yaml:"cover_hp"`
}

// RoomEffect declares a persistent mental-state aura for a room.
// Effects fire on room entry and at the start of each combat round.
// Save resolution is binary: d20 + GritMod vs BaseDC (no proficiency bonus).
type RoomEffect struct {
	// Track is the mental state track to trigger.
	// One of "rage", "despair", "delirium", "fear".
	Track string `yaml:"track"`

	// Severity is the minimum severity to apply.
	// One of "mild", "moderate", "severe".
	Severity string `yaml:"severity"`

	// BaseDC is the Grit save difficulty. Effective save: d20 + GritMod vs BaseDC.
	BaseDC int `yaml:"base_dc"`

	// CooldownRounds is rounds of immunity after a successful in-combat save.
	CooldownRounds int `yaml:"cooldown_rounds"`

	// CooldownMinutes is minutes of immunity after a successful out-of-combat save.
	CooldownMinutes int `yaml:"cooldown_minutes"`
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
	// Equipment lists static or respawning items present in this room.
	Equipment []RoomEquipmentConfig
	// MapX is the column position of this room on the zone map grid (required).
	MapX int
	// MapY is the row position of this room on the zone map grid (required).
	MapY int
	// SkillChecks holds all trigger-based skill check definitions declared for this room.
	SkillChecks []skillcheck.TriggerDef
	// Effects lists persistent mental-state auras that apply to players in this room.
	Effects []RoomEffect
	// Terrain is an optional terrain type tag: rubble, cliff, wall, sewer, river, ocean, flooded.
	Terrain string `yaml:"terrain"`
	// DangerLevel overrides the zone's danger level for this specific room.
	// Empty string means inherit from the zone.
	DangerLevel string `yaml:"danger_level,omitempty"`
	// RoomTrapChance overrides the zone's room trap chance for this specific room.
	// nil means inherit from the zone.
	RoomTrapChance *int `yaml:"room_trap_chance,omitempty"`
	// CoverTrapChance overrides the zone's cover trap chance for this specific room.
	// nil means inherit from the zone.
	CoverTrapChance *int `yaml:"cover_trap_chance,omitempty"`
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
	// DangerLevel sets the default danger level for all rooms in this zone.
	// One of "safe", "risky", "dangerous", "deadly", or "" (unset).
	DangerLevel string `yaml:"danger_level"`
	// RoomTrapChance sets the default percentage chance (0-100) that a room in
	// this zone contains a trap. nil means no trap chance configured.
	RoomTrapChance *int `yaml:"room_trap_chance,omitempty"`
	// CoverTrapChance sets the default percentage chance (0-100) that cover
	// objects in this zone contain a trap. nil means no trap chance configured.
	CoverTrapChance *int `yaml:"cover_trap_chance,omitempty"`
}

// ExternalExitTargets returns exit targets that reference rooms outside this zone.
// These targets must be validated at the world-manager level once all zones are loaded.
//
// Postcondition: Returns a (possibly empty) slice of room IDs not found in this zone.
func (z *Zone) ExternalExitTargets() []string {
	var external []string
	for _, room := range z.Rooms {
		for _, exit := range room.Exits {
			if _, ok := z.Rooms[exit.TargetRoom]; !ok {
				external = append(external, exit.TargetRoom)
			}
		}
	}
	return external
}

// Validate checks zone invariants. Exit targets that reference rooms outside
// this zone are permitted; they must be validated at the Manager level via
// ValidateExits once all zones are loaded.
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
			// Cross-zone exits are validated at the Manager level via ValidateExits.
		}
		for i, s := range room.Spawns {
			if s.Template == "" {
				return fmt.Errorf("zone %q: room %q: spawn[%d]: template must not be empty", z.ID, id, i)
			}
			if s.Count < 1 {
				return fmt.Errorf("zone %q: room %q: spawn[%d]: count must be >= 1", z.ID, id, i)
			}
			if s.RespawnAfter != "" {
				if _, err := time.ParseDuration(s.RespawnAfter); err != nil {
					return fmt.Errorf("zone %q: room %q: spawn[%d]: respawn_after %q is not a valid duration: %w", z.ID, id, i, s.RespawnAfter, err)
				}
			}
		}
	}
	// Enforce unique map coordinates across all rooms.
	coordSeen := make(map[[2]int]string) // (x,y) → first room ID
	for _, r := range z.Rooms {
		key := [2]int{r.MapX, r.MapY}
		if first, dup := coordSeen[key]; dup {
			return fmt.Errorf("zone %q: rooms %q and %q share map coordinates (%d, %d)", z.ID, first, r.ID, r.MapX, r.MapY)
		}
		coordSeen[key] = r.ID
	}
	return nil
}
