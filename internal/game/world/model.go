// Package world provides the game world model: zones, rooms, exits, and directions.
package world

import (
	"fmt"
	"strings"
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
	CoverTier         string `yaml:"cover_tier,omitempty"`
	// CoverDestructible indicates whether this cover object can be degraded and destroyed.
	CoverDestructible bool   `yaml:"cover_destructible"`
	// CoverHP is the number of hits this cover object can absorb before being destroyed.
	// Only meaningful when CoverDestructible is true.
	CoverHP           int    `yaml:"cover_hp"`
	// TrapTemplate is the trap template ID assigned to this equipment item.
	// "" means no trap. Set by static YAML authoring or procedural generation.
	TrapTemplate string `yaml:"trap_template,omitempty"`
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

// RoomTrapConfig declares a statically placed trap in a room.
type RoomTrapConfig struct {
	// TemplateID references the TrapTemplate in content/traps/.
	TemplateID string `yaml:"template"`
	// Position is "room" for a room-level trap or a RoomEquipmentConfig.Description
	// string to attach the trap to a specific equipment item.
	Position string `yaml:"position"`
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
	// Traps lists statically declared traps for this room.
	// Procedurally placed traps are managed by the TrapManager at runtime.
	Traps []RoomTrapConfig `yaml:"traps,omitempty"`
	// BossRoom marks this room as a boss room. When true:
	//   - the map renderer displays the tile as <BB> instead of [BB].
	//   - boss NPC respawns trigger coordinated respawn of all room spawns.
	BossRoom bool `yaml:"boss_room,omitempty"`
	// Indoor marks this room as an enclosed indoor space. When true,
	// weather effects (temperature modifiers, precipitation, wind) do not apply.
	Indoor bool `yaml:"indoor,omitempty"`
	// Hazards lists environmental hazards that fire on player entry or each combat round.
	Hazards []HazardDef `yaml:"hazards,omitempty"`
	// MinFactionTierID is the minimum faction tier ID required to enter this room.
	// Empty string means no gating.
	MinFactionTierID string `yaml:"min_faction_tier_id"`
	// AmbientSubstance is the substance ID dosed to players in this room every 60s
	// by the ambient substance ticker. Empty string means no ambient dosing.
	AmbientSubstance string `yaml:"ambient_substance,omitempty"`
}

// HazardDef defines an environmental hazard in a room.
type HazardDef struct {
	ID          string `yaml:"id"`
	Trigger     string `yaml:"trigger"`
	DamageExpr  string `yaml:"damage_expr"`
	DamageType  string `yaml:"damage_type"`
	ConditionID string `yaml:"condition_id"`
	Message     string `yaml:"message"`
}

// Validate checks HazardDef invariants.
func (h HazardDef) Validate() error {
	if h.Trigger != "on_enter" && h.Trigger != "round_start" {
		return fmt.Errorf("hazard %q: trigger must be \"on_enter\" or \"round_start\", got %q", h.ID, h.Trigger)
	}
	if h.DamageExpr == "" {
		return fmt.Errorf("hazard %q: damage_expr must not be empty", h.ID)
	}
	return nil
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

// LoreFacts returns the room's lore fact strings from Properties["lore_facts"].
// Facts are stored as a newline-separated string. Returns nil if not set.
//
// Postcondition: Returns nil or a slice of non-empty strings.
func (r *Room) LoreFacts() []string {
	if r.Properties == nil {
		return nil
	}
	raw := r.Properties["lore_facts"]
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "\n")
}

// TrapPoolEntry is one entry in a weighted trap selection pool.
type TrapPoolEntry struct {
	Template string `yaml:"template"`
	Weight   int    `yaml:"weight"`
}

// TrapProbabilities configures per-zone trap placement chances and the weighted pool.
// Overrides global danger-level defaults when set.
type TrapProbabilities struct {
	// RoomTrapChance overrides the danger-level default chance (0.0–1.0) for room-level traps.
	RoomTrapChance *float64 `yaml:"room_trap_chance,omitempty"`
	// CoverTrapChance overrides the danger-level default chance (0.0–1.0) for cover item traps.
	CoverTrapChance *float64 `yaml:"cover_trap_chance,omitempty"`
	// TrapPool is the weighted selection pool for this zone.
	// If empty, content/traps/defaults.yaml pool is used.
	TrapPool []TrapPoolEntry `yaml:"trap_pool,omitempty"`
}

// MaterialPoolDrop is one weighted entry in a zone's material scavenge pool.
type MaterialPoolDrop struct {
	// ID references a material ID in the MaterialRegistry.
	ID string `yaml:"id"`
	// Weight is the relative probability weight for this drop; higher means more likely.
	Weight int `yaml:"weight"`
}

// MaterialPool defines the scavenge yield configuration for a zone.
type MaterialPool struct {
	// DC is the skill check difficulty for a scavenge attempt.
	DC int `yaml:"dc"`
	// Drops is the weighted pool of materials that may be found.
	Drops []MaterialPoolDrop `yaml:"drops"`
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
	// TrapProbabilities configures per-zone procedural trap placement.
	// nil means use the global danger-level defaults.
	TrapProbabilities *TrapProbabilities `yaml:"trap_probabilities,omitempty"`
	// FactionID is the faction that controls this zone.
	// Empty string means no faction ownership.
	FactionID string `yaml:"faction_id"`
	// WorldX is the zone's column position on the world map grid.
	// nil means the zone has no world map position and is excluded from the world map.
	WorldX *int `yaml:"world_x,omitempty"`
	// WorldY is the zone's row position on the world map grid. Lower values are further north.
	// nil means the zone has no world map position and is excluded from the world map.
	WorldY *int `yaml:"world_y,omitempty"`
	// MinLevel is the minimum NPC level expected in this zone.
	// 0 means no minimum enforced.
	MinLevel int `yaml:"min_level,omitempty"`
	// MaxLevel is the maximum NPC level expected in this zone.
	// 0 means no maximum enforced.
	MaxLevel int `yaml:"max_level,omitempty"`
	// MaterialPool defines the scavenge yield configuration for this zone.
	// nil means scavenging yields nothing here (REQ-CRAFT-9).
	MaterialPool *MaterialPool `yaml:"material_pool,omitempty"`
	SettlementDC int `yaml:"settlement_dc"` // default 15 if absent
	// ZoneEffects defines persistent mental-state auras applied to every room in the zone.
	// At world load time these are appended to each room's Effects slice.
	ZoneEffects []RoomEffect `yaml:"zone_effects,omitempty"`
}

// NPCLevelRegistry is the minimal interface needed to look up NPC template levels.
//
// Precondition: id must be non-empty.
// Postcondition: returns (level, true) if found, (0, false) if not.
type NPCLevelRegistry interface {
	TemplateLevel(id string) (int, bool)
}

// ValidateNPCLevels checks that every NPC spawn template's level falls within
// [MinLevel, MaxLevel]. Zones with MinLevel == 0 and MaxLevel == 0 are skipped.
// Templates not found in the registry are silently skipped.
//
// Precondition: reg must not be nil.
// Postcondition: returns nil if all found template levels are in-range, or an
// error describing the first violation.
func (z *Zone) ValidateNPCLevels(reg NPCLevelRegistry) error {
	if z.MinLevel == 0 && z.MaxLevel == 0 {
		return nil
	}
	for roomID, room := range z.Rooms {
		for _, spawn := range room.Spawns {
			lvl, ok := reg.TemplateLevel(spawn.Template)
			if !ok {
				continue
			}
			if z.MinLevel > 0 && lvl > 0 && lvl < z.MinLevel {
				return fmt.Errorf("zone %q: room %q: NPC template %q level %d is below zone min_level %d",
					z.ID, roomID, spawn.Template, lvl, z.MinLevel)
			}
			if z.MaxLevel > 0 && lvl > z.MaxLevel {
				return fmt.Errorf("zone %q: room %q: NPC template %q level %d exceeds zone max_level %d",
					z.ID, roomID, spawn.Template, lvl, z.MaxLevel)
			}
		}
	}
	return nil
}

// ValidateWithConditions checks that every RoomEffect.Track value exists in reg.
// reg is any type with Has(id string) bool — typically *condition.Registry.
//
// Precondition: reg must not be nil.
// Postcondition: Returns nil if all effect track IDs are found in reg, or an error describing
// the first violation.
func (z *Zone) ValidateWithConditions(reg interface {
	Has(id string) bool
}) error {
	for _, eff := range z.ZoneEffects {
		if !reg.Has(eff.Track) {
			return fmt.Errorf("zone %q: zone effect track %q not found in condition registry", z.ID, eff.Track)
		}
	}
	for id, room := range z.Rooms {
		for _, eff := range room.Effects {
			if !reg.Has(eff.Track) {
				return fmt.Errorf("zone %q: room %q: effect track %q not found in condition registry", z.ID, id, eff.Track)
			}
		}
	}
	return nil
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
