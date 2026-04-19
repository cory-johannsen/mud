package world

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cory-johannsen/mud/internal/game/skillcheck"
	"gopkg.in/yaml.v3"
)

// yamlZoneFile is the top-level YAML structure for zone files.
type yamlZoneFile struct {
	Zone yamlZone `yaml:"zone"`
}

// yamlZone is the YAML representation of a zone.
type yamlZone struct {
	ID                     string                 `yaml:"id"`
	Name                   string                 `yaml:"name"`
	Description            string                 `yaml:"description"`
	StartRoom              string                 `yaml:"start_room"`
	ScriptDir              string                 `yaml:"script_dir"`
	ScriptInstructionLimit int                    `yaml:"script_instruction_limit"`
	Rooms                  []yamlRoom             `yaml:"rooms"`
	DangerLevel            string                 `yaml:"danger_level"`
	RoomTrapChance         *int                   `yaml:"room_trap_chance,omitempty"`
	CoverTrapChance        *int                   `yaml:"cover_trap_chance,omitempty"`
	TrapProbabilities      *yamlTrapProbabilities `yaml:"trap_probabilities,omitempty"`
	WorldX                 *int                   `yaml:"world_x,omitempty"`
	WorldY                 *int                   `yaml:"world_y,omitempty"`
	MinLevel               int                    `yaml:"min_level,omitempty"`
	MaxLevel               int                    `yaml:"max_level,omitempty"`
	ZoneEffects            []RoomEffect           `yaml:"zone_effects,omitempty"`
	FactionID              string                 `yaml:"faction_id,omitempty"`
}

// yamlTrapProbabilities is the YAML representation of zone trap placement config.
type yamlTrapProbabilities struct {
	RoomTrapChance  *float64        `yaml:"room_trap_chance,omitempty"`
	CoverTrapChance *float64        `yaml:"cover_trap_chance,omitempty"`
	TrapPool        []yamlTrapEntry `yaml:"trap_pool,omitempty"`
}

// yamlTrapEntry is one weighted pool entry in the YAML.
type yamlTrapEntry struct {
	Template string `yaml:"template"`
	Weight   int    `yaml:"weight"`
}

// yamlRoomSpawn is the YAML representation of a room spawn config.
type yamlRoomSpawn struct {
	Template     string `yaml:"template"`
	Count        int    `yaml:"count"`
	RespawnAfter string `yaml:"respawn_after"`
}

// yamlRoomTrap is the YAML representation of a static room trap config.
type yamlRoomTrap struct {
	Template string `yaml:"template"`
	Position string `yaml:"position"`
}

// yamlRoomEquipment is the YAML representation of a room equipment config.
type yamlRoomEquipment struct {
	ItemID            string                  `yaml:"item_id"`
	Description       string                  `yaml:"description"`
	MaxCount          int                     `yaml:"max_count"`
	RespawnAfter      string                  `yaml:"respawn_after"`
	Immovable         bool                    `yaml:"immovable"`
	Script            string                  `yaml:"script"`
	SkillChecks       []skillcheck.TriggerDef `yaml:"skill_checks"`
	TrapTemplate      string                  `yaml:"trap_template,omitempty"`
	CoverTier         string                  `yaml:"cover_tier,omitempty"`
	CoverDestructible bool                    `yaml:"cover_destructible"`
	CoverHP           int                     `yaml:"cover_hp"`
}

// yamlRoom is the YAML representation of a room.
type yamlRoom struct {
	ID              string                  `yaml:"id"`
	Title           string                  `yaml:"title"`
	Description     string                  `yaml:"description"`
	Exits           []yamlExit              `yaml:"exits"`
	Properties      map[string]string       `yaml:"properties"`
	Spawns          []yamlRoomSpawn         `yaml:"spawns"`
	Equipment       []yamlRoomEquipment     `yaml:"equipment"`
	Traps           []yamlRoomTrap          `yaml:"traps"`
	SkillChecks     []skillcheck.TriggerDef `yaml:"skill_checks"`
	Effects         []RoomEffect            `yaml:"effects"`
	MapX            *int                    `yaml:"map_x"`
	MapY            *int                    `yaml:"map_y"`
	DangerLevel     string                  `yaml:"danger_level,omitempty"`
	RoomTrapChance  *int                    `yaml:"room_trap_chance,omitempty"`
	CoverTrapChance *int                    `yaml:"cover_trap_chance,omitempty"`
	Indoor           bool                    `yaml:"indoor"`
	AmbientSubstance string                  `yaml:"ambient_substance,omitempty"`
	BossRoom         bool                    `yaml:"boss_room,omitempty"`
	Hazards          []HazardDef             `yaml:"hazards,omitempty"`
	MinFactionTierID string                  `yaml:"min_faction_tier_id,omitempty"`
}

// yamlExit is the YAML representation of an exit.
type yamlExit struct {
	Direction string `yaml:"direction"`
	Target    string `yaml:"target"`
	Locked    bool   `yaml:"locked"`
	Hidden    bool   `yaml:"hidden"`
}

// LoadZoneFromFile reads and validates a single zone YAML file.
//
// Precondition: path must point to a valid YAML zone file.
// Postcondition: Returns a validated Zone or a non-nil error.
func LoadZoneFromFile(path string) (*Zone, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading zone file %s: %w", path, err)
	}
	return LoadZoneFromBytes(data)
}

// LoadZoneFromBytes parses and validates a zone from YAML bytes.
//
// Precondition: data must be valid YAML conforming to the zone schema.
// Postcondition: Returns a validated Zone or a non-nil error.
func LoadZoneFromBytes(data []byte) (*Zone, error) {
	var file yamlZoneFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parsing zone YAML: %w", err)
	}

	zone, err := convertYAMLZone(file.Zone)
	if err != nil {
		return nil, err
	}
	if err := zone.Validate(); err != nil {
		return nil, fmt.Errorf("validating zone: %w", err)
	}

	return zone, nil
}

// LoadZonesFromDir loads all YAML files in a directory as zones.
//
// Precondition: dir must be a valid directory path.
// Postcondition: Returns all validated zones or the first error encountered.
func LoadZonesFromDir(dir string) ([]*Zone, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading zone directory %s: %w", dir, err)
	}

	var zones []*Zone
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		zone, err := LoadZoneFromFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("loading zone from %s: %w", name, err)
		}
		zones = append(zones, zone)
	}

	if len(zones) == 0 {
		return nil, fmt.Errorf("no zone files found in %s", dir)
	}

	return zones, nil
}

// convertYAMLZone converts the parsed YAML structures into domain types.
//
// Precondition: yz must be a populated yamlZone.
// Postcondition: Returns a fully populated Zone or a non-nil error if any room
// is missing required map_x or map_y coordinates.
func convertYAMLZone(yz yamlZone) (*Zone, error) {
	zone := &Zone{
		ID:                     yz.ID,
		Name:                   yz.Name,
		Description:            yz.Description,
		StartRoom:              yz.StartRoom,
		ScriptDir:              yz.ScriptDir,
		ScriptInstructionLimit: yz.ScriptInstructionLimit,
		Rooms:                  make(map[string]*Room, len(yz.Rooms)),
		DangerLevel:            yz.DangerLevel,
		RoomTrapChance:         yz.RoomTrapChance,
		CoverTrapChance:        yz.CoverTrapChance,
		WorldX:                 yz.WorldX,
		WorldY:                 yz.WorldY,
		MinLevel:               yz.MinLevel,
		MaxLevel:               yz.MaxLevel,
		ZoneEffects:            yz.ZoneEffects,
		FactionID:              yz.FactionID,
	}
	if yz.TrapProbabilities != nil {
		tp := &TrapProbabilities{
			RoomTrapChance:  yz.TrapProbabilities.RoomTrapChance,
			CoverTrapChance: yz.TrapProbabilities.CoverTrapChance,
		}
		for _, e := range yz.TrapProbabilities.TrapPool {
			tp.TrapPool = append(tp.TrapPool, TrapPoolEntry{Template: e.Template, Weight: e.Weight})
		}
		zone.TrapProbabilities = tp
	}

	for _, yr := range yz.Rooms {
		if yr.MapX == nil {
			return nil, fmt.Errorf("zone %q: room %q: missing required field map_x", yz.ID, yr.ID)
		}
		if yr.MapY == nil {
			return nil, fmt.Errorf("zone %q: room %q: missing required field map_y", yz.ID, yr.ID)
		}
		room := &Room{
			ID:               yr.ID,
			ZoneID:           yz.ID,
			Title:            yr.Title,
			Description:      strings.TrimSpace(yr.Description),
			Properties:       yr.Properties,
			SkillChecks:      yr.SkillChecks,
			Effects:          yr.Effects,
			MapX:             *yr.MapX,
			MapY:             *yr.MapY,
			DangerLevel:      yr.DangerLevel,
			RoomTrapChance:   yr.RoomTrapChance,
			CoverTrapChance:  yr.CoverTrapChance,
			Indoor:           yr.Indoor,
			AmbientSubstance: yr.AmbientSubstance,
			BossRoom:         yr.BossRoom,
			Hazards:          yr.Hazards,
			MinFactionTierID: yr.MinFactionTierID,
		}
		if room.Properties == nil {
			room.Properties = make(map[string]string)
		}
		for _, ye := range yr.Exits {
			room.Exits = append(room.Exits, Exit{
				Direction:  Direction(ye.Direction),
				TargetRoom: ye.Target,
				Locked:     ye.Locked,
				Hidden:     ye.Hidden,
			})
		}
		for _, ys := range yr.Spawns {
			room.Spawns = append(room.Spawns, RoomSpawnConfig{
				Template:     ys.Template,
				Count:        ys.Count,
				RespawnAfter: ys.RespawnAfter,
			})
		}
		for _, e := range yr.Equipment {
			dur, err := time.ParseDuration(e.RespawnAfter)
			if err != nil {
				dur = 0
			}
			eq := RoomEquipmentConfig{
				ItemID:            e.ItemID,
				Description:       e.Description,
				MaxCount:          e.MaxCount,
				RespawnAfter:      dur,
				Immovable:         e.Immovable,
				Script:            e.Script,
				SkillChecks:       e.SkillChecks,
				TrapTemplate:      e.TrapTemplate,
				CoverTier:         e.CoverTier,
				CoverDestructible: e.CoverDestructible,
				CoverHP:           e.CoverHP,
			}
			room.Equipment = append(room.Equipment, eq)
		}
		for _, yt := range yr.Traps {
			room.Traps = append(room.Traps, RoomTrapConfig{
				TemplateID: yt.Template,
				Position:   yt.Position,
			})
		}
		zone.Rooms[room.ID] = room
	}

	for _, room := range zone.Rooms {
		room.Effects = append(room.Effects, zone.ZoneEffects...)
	}

	return zone, nil
}

// zoneToYAML converts a Zone domain object back to its YAML serialization form.
//
// Precondition: zone must be non-nil.
// Postcondition: Returns a yamlZoneFile that, when marshaled and passed to LoadZoneFromBytes,
// produces a Zone equal to the input (modulo floating-point trap fields).
func zoneToYAML(zone *Zone) yamlZoneFile {
	yrooms := make([]yamlRoom, 0, len(zone.Rooms))
	for _, room := range zone.Rooms {
		yr := yamlRoom{
			ID:               room.ID,
			Title:            room.Title,
			Description:      room.Description,
			Properties:       room.Properties,
			SkillChecks:      room.SkillChecks,
			Effects:          room.Effects,
			MapX:             new(room.MapX),
			MapY:             new(room.MapY),
			DangerLevel:      room.DangerLevel,
			RoomTrapChance:   room.RoomTrapChance,
			CoverTrapChance:  room.CoverTrapChance,
			Indoor:           room.Indoor,
			AmbientSubstance: room.AmbientSubstance,
			BossRoom:         room.BossRoom,
			Hazards:          room.Hazards,
			MinFactionTierID: room.MinFactionTierID,
		}
		for _, exit := range room.Exits {
			yr.Exits = append(yr.Exits, yamlExit{
				Direction: string(exit.Direction),
				Target:    exit.TargetRoom,
				Locked:    exit.Locked,
				Hidden:    exit.Hidden,
			})
		}
		for _, sp := range room.Spawns {
			yr.Spawns = append(yr.Spawns, yamlRoomSpawn{
				Template:     sp.Template,
				Count:        sp.Count,
				RespawnAfter: sp.RespawnAfter,
			})
		}
		for _, eq := range room.Equipment {
			respawnStr := ""
			if eq.RespawnAfter > 0 {
				respawnStr = eq.RespawnAfter.String()
			}
			yr.Equipment = append(yr.Equipment, yamlRoomEquipment{
				ItemID:            eq.ItemID,
				Description:       eq.Description,
				MaxCount:          eq.MaxCount,
				RespawnAfter:      respawnStr,
				Immovable:         eq.Immovable,
				Script:            eq.Script,
				SkillChecks:       eq.SkillChecks,
				TrapTemplate:      eq.TrapTemplate,
				CoverTier:         eq.CoverTier,
				CoverDestructible: eq.CoverDestructible,
				CoverHP:           eq.CoverHP,
			})
		}
		for _, tr := range room.Traps {
			yr.Traps = append(yr.Traps, yamlRoomTrap{
				Template: tr.TemplateID,
				Position: tr.Position,
			})
		}
		yrooms = append(yrooms, yr)
	}

	yz := yamlZone{
		ID:                     zone.ID,
		Name:                   zone.Name,
		Description:            zone.Description,
		StartRoom:              zone.StartRoom,
		ScriptDir:              zone.ScriptDir,
		ScriptInstructionLimit: zone.ScriptInstructionLimit,
		Rooms:                  yrooms,
		DangerLevel:            zone.DangerLevel,
		RoomTrapChance:         zone.RoomTrapChance,
		CoverTrapChance:        zone.CoverTrapChance,
		FactionID:              zone.FactionID,
	}
	if zone.TrapProbabilities != nil {
		tp := &yamlTrapProbabilities{
			RoomTrapChance:  zone.TrapProbabilities.RoomTrapChance,
			CoverTrapChance: zone.TrapProbabilities.CoverTrapChance,
		}
		for _, e := range zone.TrapProbabilities.TrapPool {
			tp.TrapPool = append(tp.TrapPool, yamlTrapEntry{Template: e.Template, Weight: e.Weight})
		}
		yz.TrapProbabilities = tp
	}
	return yamlZoneFile{Zone: yz}
}
