package world

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlZoneFile is the top-level YAML structure for zone files.
type yamlZoneFile struct {
	Zone yamlZone `yaml:"zone"`
}

// yamlZone is the YAML representation of a zone.
type yamlZone struct {
	ID                     string     `yaml:"id"`
	Name                   string     `yaml:"name"`
	Description            string     `yaml:"description"`
	StartRoom              string     `yaml:"start_room"`
	ScriptDir              string     `yaml:"script_dir"`
	ScriptInstructionLimit int        `yaml:"script_instruction_limit"`
	Rooms                  []yamlRoom `yaml:"rooms"`
}

// yamlRoom is the YAML representation of a room.
type yamlRoom struct {
	ID          string            `yaml:"id"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	Exits       []yamlExit        `yaml:"exits"`
	Properties  map[string]string `yaml:"properties"`
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

	zone := convertYAMLZone(file.Zone)
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
func convertYAMLZone(yz yamlZone) *Zone {
	zone := &Zone{
		ID:                     yz.ID,
		Name:                   yz.Name,
		Description:            yz.Description,
		StartRoom:              yz.StartRoom,
		ScriptDir:              yz.ScriptDir,
		ScriptInstructionLimit: yz.ScriptInstructionLimit,
		Rooms:                  make(map[string]*Room, len(yz.Rooms)),
	}

	for _, yr := range yz.Rooms {
		room := &Room{
			ID:          yr.ID,
			ZoneID:      yz.ID,
			Title:       yr.Title,
			Description: strings.TrimSpace(yr.Description),
			Properties:  yr.Properties,
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
		zone.Rooms[room.ID] = room
	}

	return zone
}
