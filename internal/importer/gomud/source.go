package gomud

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cory-johannsen/mud/internal/importer"
)

var _ importer.Source = (*GomudSource)(nil)

// GomudSource implements importer.Source for the gomud asset layout:
//
//	sourceDir/
//	  zones/   <- one YAML file per zone
//	  areas/   <- one YAML file per area
//	  rooms/   <- one YAML file per room
type GomudSource struct{}

// NewSource constructs a GomudSource.
func NewSource() *GomudSource { return &GomudSource{} }

// Load reads the gomud asset tree rooted at sourceDir and returns one ZoneData
// per zone file. Warnings for missing rooms and unresolvable exit targets are
// printed to stderr. startRoom overrides the zone's default start room (first
// listed room) when non-empty.
//
// Precondition: sourceDir must contain zones/, areas/, and rooms/ subdirs.
// Postcondition: returns at least one ZoneData or a non-nil error.
func (s *GomudSource) Load(sourceDir, startRoom string) ([]*importer.ZoneData, error) {
	zonesDir := filepath.Join(sourceDir, "zones")
	areasDir := filepath.Join(sourceDir, "areas")
	roomsDir := filepath.Join(sourceDir, "rooms")

	for _, dir := range []string{zonesDir, areasDir, roomsDir} {
		if _, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("required subdirectory %q not accessible in source: %w", filepath.Base(dir), err)
		}
	}

	allRooms, err := loadRooms(roomsDir)
	if err != nil {
		return nil, err
	}

	roomArea, err := loadRoomAreaMap(areasDir)
	if err != nil {
		return nil, err
	}

	zoneFiles, err := yamlFiles(zonesDir)
	if err != nil {
		return nil, err
	}

	var results []*importer.ZoneData
	for _, path := range zoneFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading zone file %s: %w", path, err)
		}
		zone, err := ParseZone(data)
		if err != nil {
			return nil, fmt.Errorf("parsing zone file %s: %w", path, err)
		}
		zd, warnings := ConvertZone(zone, allRooms, roomArea, startRoom)
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
		}
		results = append(results, zd)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no zone files found in %s", zonesDir)
	}
	return results, nil
}

func loadRooms(dir string) (map[string]*GomudRoom, error) {
	files, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}
	rooms := make(map[string]*GomudRoom, len(files))
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading room file %s: %w", path, err)
		}
		room, err := ParseRoom(data)
		if err != nil {
			return nil, fmt.Errorf("parsing room file %s: %w", path, err)
		}
		rooms[strings.TrimSpace(room.Name)] = room
	}
	return rooms, nil
}

func loadRoomAreaMap(dir string) (map[string]string, error) {
	files, err := yamlFiles(dir)
	if err != nil {
		return nil, err
	}
	roomArea := make(map[string]string)
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading area file %s: %w", path, err)
		}
		area, err := ParseArea(data)
		if err != nil {
			return nil, fmt.Errorf("parsing area file %s: %w", path, err)
		}
		for _, name := range area.Rooms {
			roomArea[strings.TrimSpace(name)] = area.Name
		}
	}
	return roomArea, nil
}

func yamlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	return paths, nil
}
