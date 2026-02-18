# Content Importer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a general-purpose `import-content` CLI that reads MUD assets from external source formats and produces the project's zone YAML format, with `gomud` as the initial source adapter.

**Architecture:** A pluggable `Source` interface produces `[]*ZoneData` (a struct mirroring the project's zone YAML schema) from a source directory. The runner serializes each ZoneData to YAML, validates it with `world.LoadZoneFromBytes`, and writes to the output directory. The `gomud` adapter reads the three-tier `zones/` / `areas/` / `rooms/` layout and flattens it into ZoneData.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`, `pgregory.net/rapid` (property-based tests), `github.com/stretchr/testify`, `go.uber.org/zap`, existing `internal/game/world` package for output validation.

---

### Task 1: Core interfaces and `nameToID` utility

**Files:**
- Create: `internal/importer/source.go`
- Create: `internal/importer/converter.go`
- Create: `internal/importer/converter_test.go`

**Step 1: Write the failing test**

Create `internal/importer/converter_test.go`:

```go
package importer_test

import (
	"testing"
	"unicode"

	"github.com/cory-johannsen/mud/internal/importer"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestNameToID_Lowercase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringOf(rapid.RuneFrom(nil, unicode.Letter, unicode.Digit)).Draw(t, "name")
		id := importer.NameToID(name)
		for _, r := range id {
			assert.True(t, r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'),
				"unexpected char %q in id %q", r, id)
		}
	})
}

func TestNameToID_Idempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringOf(rapid.RuneFrom(nil, unicode.Letter, unicode.Digit)).Draw(t, "name")
		id := importer.NameToID(name)
		assert.Equal(t, id, importer.NameToID(id))
	})
}

func TestNameToID_NoSpacesOrApostrophes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringOf(rapid.RuneFrom(nil, unicode.Letter, unicode.Space)).Draw(t, "name")
		id := importer.NameToID(name)
		assert.NotContains(t, id, " ")
		assert.NotContains(t, id, "'")
	})
}

func TestNameToID_KnownValues(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Grinder's Row", "grinders_row"},
		{"The Rusty Oasis", "the_rusty_oasis"},
		{"Scrapshack 23", "scrapshack_23"},
		{"Rustbucket Ridge", "rustbucket_ridge"},
		{"Filth Court", "filth_court"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, importer.NameToID(tc.input))
		})
	}
}
```

**Step 2: Run test to verify it fails**

```
go test ./internal/importer/... -run TestNameToID -v
```
Expected: FAIL — `importer.NameToID` undefined.

**Step 3: Create `internal/importer/source.go`**

```go
package importer

// ZoneData is the common intermediate format produced by all Source
// implementations. Its YAML tags match the project's zone file schema exactly,
// so it can be marshalled directly and validated by world.LoadZoneFromBytes.
type ZoneData struct {
	Zone ZoneSpec `yaml:"zone"`
}

// ZoneSpec holds zone-level metadata and its rooms.
type ZoneSpec struct {
	ID          string     `yaml:"id"`
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	StartRoom   string     `yaml:"start_room"`
	Rooms       []RoomSpec `yaml:"rooms"`
}

// RoomSpec holds a single room's data.
type RoomSpec struct {
	ID          string            `yaml:"id"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	Exits       []ExitSpec        `yaml:"exits,omitempty"`
	Properties  map[string]string `yaml:"properties,omitempty"`
}

// ExitSpec holds a single exit's data.
type ExitSpec struct {
	Direction string `yaml:"direction"`
	Target    string `yaml:"target"`
	Locked    bool   `yaml:"locked,omitempty"`
	Hidden    bool   `yaml:"hidden,omitempty"`
}

// Source loads content from a format-specific source directory and produces
// ZoneData ready to be written as zone YAML files.
//
// Precondition: sourceDir must exist and contain the expected layout for the format.
// startRoom is an optional display-name override for the zone's start room;
// empty string means "use format default".
// Postcondition: returns at least one ZoneData, or a non-nil error.
type Source interface {
	Load(sourceDir, startRoom string) ([]*ZoneData, error)
}
```

**Step 4: Create `internal/importer/converter.go`**

```go
package importer

import "strings"

// NameToID converts a display name to a stable snake_case identifier.
//
// Postcondition: result is lowercase, contains only [a-z0-9_], and is
// idempotent (NameToID(NameToID(s)) == NameToID(s)).
func NameToID(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "_")
	var b strings.Builder
	for _, r := range s {
		if r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```

**Step 5: Run tests to verify they pass**

```
go test ./internal/importer/... -run TestNameToID -v -count=1
```
Expected: PASS (all 4 test functions).

**Step 6: Commit**

```bash
git add internal/importer/source.go internal/importer/converter.go internal/importer/converter_test.go
git commit -m "feat(importer): core Source interface and nameToID utility"
```

---

### Task 2: Gomud model types

**Files:**
- Create: `internal/importer/gomud/model.go`

**Step 1: Create the file**

```go
package gomud

// GomudZone is the parsed form of a gomud assets/zones/<name>.yaml file.
// Rooms and Areas are lists of display names, not IDs.
type GomudZone struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Rooms       []string `yaml:"rooms"`
	Areas       []string `yaml:"areas"`
}

// GomudArea is the parsed form of a gomud assets/areas/<name>.yaml file.
// Rooms is a list of display names.
type GomudArea struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Rooms       []string `yaml:"rooms"`
}

// GomudRoom is the parsed form of a gomud assets/rooms/<name>.yaml file.
// The objects field is intentionally omitted (not supported in this project).
type GomudRoom struct {
	Name        string               `yaml:"name"`
	Description string               `yaml:"description"`
	Exits       map[string]GomudExit `yaml:"exits"`
}

// GomudExit is one exit entry in a GomudRoom.Exits map.
// Direction is the capitalized compass direction (e.g. "North", "Southwest").
// Target is the display name of the target room.
type GomudExit struct {
	Direction string `yaml:"direction"`
	Name      string `yaml:"name"`
	Target    string `yaml:"target"`
}
```

**Step 2: Verify it compiles**

```
go build ./internal/importer/gomud/...
```
Expected: no output (success).

**Step 3: Commit**

```bash
git add internal/importer/gomud/model.go
git commit -m "feat(importer/gomud): source model types"
```

---

### Task 3: Gomud parser

**Files:**
- Create: `internal/importer/gomud/parser_test.go`
- Create: `internal/importer/gomud/parser.go`

**Step 1: Write the failing tests**

Create `internal/importer/gomud/parser_test.go`:

```go
package gomud_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/importer/gomud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const zoneYAML = `
name: Test Zone
description: A test zone.
rooms:
  - Room A
  - Room B
areas:
  - Area One
`

const areaYAML = `
name: Area One
description: The first area.
rooms:
  - Room A
  - Room B
`

const roomWithExitsYAML = `
name: Room A
description: The first room.
exits:
  North:
    direction: North
    name: Room B
    description: Room B
    target: Room B
`

const roomEmptyExitsYAML = `
name: Room C
description: A dead end.
exits:
`

const roomWithObjectsYAML = `
name: Room D
description: Has objects.
objects:
  - Old couch
  - Lamp
exits:
  South:
    direction: South
    name: Room A
    target: Room A
`

func TestParseZone(t *testing.T) {
	z, err := gomud.ParseZone([]byte(zoneYAML))
	require.NoError(t, err)
	assert.Equal(t, "Test Zone", z.Name)
	assert.Equal(t, "A test zone.", z.Description)
	assert.Equal(t, []string{"Room A", "Room B"}, z.Rooms)
	assert.Equal(t, []string{"Area One"}, z.Areas)
}

func TestParseArea(t *testing.T) {
	a, err := gomud.ParseArea([]byte(areaYAML))
	require.NoError(t, err)
	assert.Equal(t, "Area One", a.Name)
	assert.Equal(t, []string{"Room A", "Room B"}, a.Rooms)
}

func TestParseRoom_WithExits(t *testing.T) {
	r, err := gomud.ParseRoom([]byte(roomWithExitsYAML))
	require.NoError(t, err)
	assert.Equal(t, "Room A", r.Name)
	assert.Equal(t, "The first room.", r.Description)
	require.Contains(t, r.Exits, "North")
	assert.Equal(t, "Room B", r.Exits["North"].Target)
}

func TestParseRoom_EmptyExits(t *testing.T) {
	r, err := gomud.ParseRoom([]byte(roomEmptyExitsYAML))
	require.NoError(t, err)
	assert.Equal(t, "Room C", r.Name)
	assert.Empty(t, r.Exits)
}

func TestParseRoom_ObjectsDropped(t *testing.T) {
	r, err := gomud.ParseRoom([]byte(roomWithObjectsYAML))
	require.NoError(t, err)
	assert.Equal(t, "Room D", r.Name)
	// objects field is silently ignored; exits still parsed
	require.Contains(t, r.Exits, "South")
	assert.Equal(t, "Room A", r.Exits["South"].Target)
}
```

**Step 2: Run tests to verify they fail**

```
go test ./internal/importer/gomud/... -run TestParse -v
```
Expected: FAIL — `gomud.ParseZone` undefined.

**Step 3: Create `internal/importer/gomud/parser.go`**

```go
package gomud

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ParseZone parses a gomud zone YAML file.
//
// Precondition: data must be valid YAML.
// Postcondition: returns a non-nil GomudZone or a non-nil error.
func ParseZone(data []byte) (*GomudZone, error) {
	var z GomudZone
	if err := yaml.Unmarshal(data, &z); err != nil {
		return nil, fmt.Errorf("parsing gomud zone: %w", err)
	}
	return &z, nil
}

// ParseArea parses a gomud area YAML file.
//
// Precondition: data must be valid YAML.
// Postcondition: returns a non-nil GomudArea or a non-nil error.
func ParseArea(data []byte) (*GomudArea, error) {
	var a GomudArea
	if err := yaml.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parsing gomud area: %w", err)
	}
	return &a, nil
}

// ParseRoom parses a gomud room YAML file.
// The objects field in the source is silently ignored.
//
// Precondition: data must be valid YAML.
// Postcondition: returns a non-nil GomudRoom or a non-nil error.
func ParseRoom(data []byte) (*GomudRoom, error) {
	var r GomudRoom
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("parsing gomud room: %w", err)
	}
	return &r, nil
}
```

**Step 4: Run tests to verify they pass**

```
go test ./internal/importer/gomud/... -run TestParse -v -count=1
```
Expected: PASS (5 tests).

**Step 5: Commit**

```bash
git add internal/importer/gomud/parser.go internal/importer/gomud/parser_test.go
git commit -m "feat(importer/gomud): YAML parser for zone, area, and room files"
```

---

### Task 4: Gomud converter

**Files:**
- Create: `internal/importer/gomud/converter_test.go`
- Create: `internal/importer/gomud/converter.go`

**Step 1: Write the failing tests**

Create `internal/importer/gomud/converter_test.go`:

```go
package gomud_test

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/importer"
	igomud "github.com/cory-johannsen/mud/internal/importer/gomud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertZone_Basic(t *testing.T) {
	zone := &igomud.GomudZone{
		Name:        "Test Zone",
		Description: "A zone.",
		Rooms:       []string{"Room A", "Room B"},
	}
	rooms := map[string]*igomud.GomudRoom{
		"Room A": {
			Name:        "Room A",
			Description: "The first room.",
			Exits: map[string]igomud.GomudExit{
				"North": {Direction: "North", Target: "Room B"},
			},
		},
		"Room B": {
			Name:        "Room B",
			Description: "The second room.",
			Exits: map[string]igomud.GomudExit{
				"South": {Direction: "South", Target: "Room A"},
			},
		},
	}
	roomArea := map[string]string{
		"Room A": "Area One",
	}

	zd, warnings := igomud.ConvertZone(zone, rooms, roomArea, "")
	assert.Empty(t, warnings)

	assert.Equal(t, "test_zone", zd.Zone.ID)
	assert.Equal(t, "Test Zone", zd.Zone.Name)
	assert.Equal(t, "room_a", zd.Zone.StartRoom)
	require.Len(t, zd.Zone.Rooms, 2)

	roomA := zd.Zone.Rooms[0]
	assert.Equal(t, "room_a", roomA.ID)
	assert.Equal(t, "Room A", roomA.Title)
	assert.Equal(t, "area_one", roomA.Properties["area"])
	require.Len(t, roomA.Exits, 1)
	assert.Equal(t, "north", roomA.Exits[0].Direction)
	assert.Equal(t, "room_b", roomA.Exits[0].Target)

	roomB := zd.Zone.Rooms[1]
	assert.Empty(t, roomB.Properties["area"])
}

func TestConvertZone_StartRoomOverride(t *testing.T) {
	zone := &igomud.GomudZone{
		Name:  "Test Zone",
		Rooms: []string{"Room A", "Room B"},
	}
	rooms := map[string]*igomud.GomudRoom{
		"Room A": {Name: "Room A", Description: "desc"},
		"Room B": {Name: "Room B", Description: "desc"},
	}

	zd, _ := igomud.ConvertZone(zone, rooms, nil, "Room B")
	assert.Equal(t, "room_b", zd.Zone.StartRoom)
}

func TestConvertZone_UnknownExitTarget_WarnAndDrop(t *testing.T) {
	zone := &igomud.GomudZone{
		Name:  "Test Zone",
		Rooms: []string{"Room A"},
	}
	rooms := map[string]*igomud.GomudRoom{
		"Room A": {
			Name:        "Room A",
			Description: "desc",
			Exits: map[string]igomud.GomudExit{
				"North": {Direction: "North", Target: "Nonexistent Room"},
			},
		},
	}

	zd, warnings := igomud.ConvertZone(zone, rooms, nil, "")
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "Nonexistent Room")
	assert.Empty(t, zd.Zone.Rooms[0].Exits)
}

func TestConvertZone_MissingRoom_WarnAndSkip(t *testing.T) {
	zone := &igomud.GomudZone{
		Name:  "Test Zone",
		Rooms: []string{"Room A", "Missing Room"},
	}
	rooms := map[string]*igomud.GomudRoom{
		"Room A": {Name: "Room A", Description: "desc"},
	}

	zd, warnings := igomud.ConvertZone(zone, rooms, nil, "")
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "Missing Room")
	assert.Len(t, zd.Zone.Rooms, 1)
}

func TestConvertZone_DirectionLowercased(t *testing.T) {
	zone := &igomud.GomudZone{
		Name:  "Test Zone",
		Rooms: []string{"Room A", "Room B"},
	}
	rooms := map[string]*igomud.GomudRoom{
		"Room A": {
			Name:        "Room A",
			Description: "desc",
			Exits: map[string]igomud.GomudExit{
				"Northeast": {Direction: "Northeast", Target: "Room B"},
			},
		},
		"Room B": {Name: "Room B", Description: "desc"},
	}

	zd, _ := igomud.ConvertZone(zone, rooms, nil, "")
	require.Len(t, zd.Zone.Rooms[0].Exits, 1)
	assert.Equal(t, "northeast", zd.Zone.Rooms[0].Exits[0].Direction)
}

// Ensure ConvertZone output is stable across multiple calls (no ordering issues).
func TestConvertZone_Deterministic(t *testing.T) {
	zone := &igomud.GomudZone{
		Name:  "Test Zone",
		Rooms: []string{"Room A", "Room B"},
	}
	rooms := map[string]*igomud.GomudRoom{
		"Room A": {Name: "Room A", Description: "d"},
		"Room B": {Name: "Room B", Description: "d"},
	}
	first, _ := igomud.ConvertZone(zone, rooms, nil, "")
	for i := 0; i < 10; i++ {
		got, _ := igomud.ConvertZone(zone, rooms, nil, "")
		assert.Equal(t, first.Zone.Rooms[0].ID, got.Zone.Rooms[0].ID,
			fmt.Sprintf("iteration %d", i))
	}
}
```

**Step 2: Run tests to verify they fail**

```
go test ./internal/importer/gomud/... -run TestConvert -v
```
Expected: FAIL — `igomud.ConvertZone` undefined.

**Step 3: Create `internal/importer/gomud/converter.go`**

```go
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
```

**Step 4: Run tests to verify they pass**

```
go test ./internal/importer/gomud/... -run TestConvert -v -count=1
```
Expected: PASS (6 tests).

**Step 5: Commit**

```bash
git add internal/importer/gomud/converter.go internal/importer/gomud/converter_test.go
git commit -m "feat(importer/gomud): zone converter with exit resolution and area properties"
```

---

### Task 5: Gomud source (filesystem integration)

**Files:**
- Create: `internal/importer/gomud/source_test.go`
- Create: `internal/importer/gomud/source.go`

**Step 1: Write the failing integration test**

Create `internal/importer/gomud/source_test.go`:

```go
package gomud_test

import (
	"os"
	"path/filepath"
	"testing"

	igomud "github.com/cory-johannsen/mud/internal/importer/gomud"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// buildTestAssets writes a minimal 3-room / 2-area / 1-zone gomud asset tree
// into a temp directory and returns the root path.
func buildTestAssets(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "zones"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "areas"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "rooms"), 0755))

	writeFile := func(path, content string) {
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	writeFile(filepath.Join(root, "zones", "test_zone.yaml"), `
name: Test Zone
description: A zone for testing.
rooms:
  - Room Alpha
  - Room Beta
  - Room Gamma
areas:
  - Area One
  - Area Two
`)
	writeFile(filepath.Join(root, "areas", "area_one.yaml"), `
name: Area One
description: First area.
rooms:
  - Room Alpha
  - Room Beta
`)
	writeFile(filepath.Join(root, "areas", "area_two.yaml"), `
name: Area Two
description: Second area.
rooms:
  - Room Gamma
`)
	writeFile(filepath.Join(root, "rooms", "room_alpha.yaml"), `
name: Room Alpha
description: The alpha room.
exits:
  East:
    direction: East
    name: Room Beta
    target: Room Beta
`)
	writeFile(filepath.Join(root, "rooms", "room_beta.yaml"), `
name: Room Beta
description: The beta room.
exits:
  West:
    direction: West
    name: Room Alpha
    target: Room Alpha
  East:
    direction: East
    name: Room Gamma
    target: Room Gamma
`)
	writeFile(filepath.Join(root, "rooms", "room_gamma.yaml"), `
name: Room Gamma
description: The gamma room.
exits:
  West:
    direction: West
    name: Room Beta
    target: Room Beta
`)
	return root
}

func TestGomudSource_Load(t *testing.T) {
	root := buildTestAssets(t)
	src := igomud.NewSource()
	zones, err := src.Load(root, "")
	require.NoError(t, err)
	require.Len(t, zones, 1)

	zd := zones[0]
	assert.Equal(t, "test_zone", zd.Zone.ID)
	assert.Equal(t, "room_alpha", zd.Zone.StartRoom)
	assert.Len(t, zd.Zone.Rooms, 3)

	// Verify area properties
	roomsByID := make(map[string]struct{ props map[string]string })
	for _, r := range zd.Zone.Rooms {
		roomsByID[r.ID] = struct{ props map[string]string }{r.Properties}
	}
	assert.Equal(t, "area_one", roomsByID["room_alpha"].props["area"])
	assert.Equal(t, "area_one", roomsByID["room_beta"].props["area"])
	assert.Equal(t, "area_two", roomsByID["room_gamma"].props["area"])
}

func TestGomudSource_OutputValidatesWithWorldLoader(t *testing.T) {
	root := buildTestAssets(t)
	src := igomud.NewSource()
	zones, err := src.Load(root, "")
	require.NoError(t, err)
	require.Len(t, zones, 1)

	data, err := yaml.Marshal(zones[0])
	require.NoError(t, err)

	_, err = world.LoadZoneFromBytes(data)
	require.NoError(t, err, "produced YAML must be loadable by world.LoadZoneFromBytes")
}

func TestGomudSource_StartRoomOverride(t *testing.T) {
	root := buildTestAssets(t)
	src := igomud.NewSource()
	zones, err := src.Load(root, "Room Gamma")
	require.NoError(t, err)
	assert.Equal(t, "room_gamma", zones[0].Zone.StartRoom)
}

func TestGomudSource_MissingSubdirError(t *testing.T) {
	root := t.TempDir()
	src := igomud.NewSource()
	_, err := src.Load(root, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zones")
}
```

**Step 2: Run tests to verify they fail**

```
go test ./internal/importer/gomud/... -run TestGomudSource -v
```
Expected: FAIL — `igomud.NewSource` undefined.

**Step 3: Create `internal/importer/gomud/source.go`**

```go
package gomud

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cory-johannsen/mud/internal/importer"
)

// GomudSource implements importer.Source for the gomud asset layout:
//   sourceDir/
//     zones/   ← one YAML file per zone
//     areas/   ← one YAML file per area
//     rooms/   ← one YAML file per room
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
			return nil, fmt.Errorf("required subdirectory %q not found in source: %w", filepath.Base(dir), err)
		}
	}

	// Load all rooms into a map keyed by display name.
	allRooms, err := loadRooms(roomsDir)
	if err != nil {
		return nil, err
	}

	// Build room → area display name map from area files.
	roomArea, err := loadRoomAreaMap(areasDir)
	if err != nil {
		return nil, err
	}

	// Load and convert each zone.
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
```

**Step 4: Run tests to verify they pass**

```
go test ./internal/importer/gomud/... -v -count=1
```
Expected: PASS (all tests in package).

**Step 5: Commit**

```bash
git add internal/importer/gomud/source.go internal/importer/gomud/source_test.go
git commit -m "feat(importer/gomud): GomudSource filesystem integration"
```

---

### Task 6: Importer runner

**Files:**
- Create: `internal/importer/importer_test.go`
- Create: `internal/importer/importer.go`

**Step 1: Write the failing test**

Create `internal/importer/importer_test.go`:

```go
package importer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/importer"
	igomud "github.com/cory-johannsen/mud/internal/importer/gomud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImporter_Run_WritesZoneFile(t *testing.T) {
	// Build minimal source tree.
	srcRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "zones"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "areas"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(srcRoot, "rooms"), 0755))

	write := func(path, s string) {
		require.NoError(t, os.WriteFile(path, []byte(s), 0644))
	}
	write(filepath.Join(srcRoot, "zones", "z.yaml"), `
name: My Zone
description: A zone.
rooms:
  - Room One
  - Room Two
areas: []
`)
	write(filepath.Join(srcRoot, "rooms", "room_one.yaml"), `
name: Room One
description: First.
exits:
  North:
    direction: North
    name: Room Two
    target: Room Two
`)
	write(filepath.Join(srcRoot, "rooms", "room_two.yaml"), `
name: Room Two
description: Second.
exits:
  South:
    direction: South
    name: Room One
    target: Room One
`)

	outDir := t.TempDir()
	imp := importer.New(igomud.NewSource())
	err := imp.Run(srcRoot, outDir, "")
	require.NoError(t, err)

	entries, err := os.ReadDir(outDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "my_zone.yaml", entries[0].Name())
}

func TestImporter_Run_InvalidSourceDir(t *testing.T) {
	imp := importer.New(igomud.NewSource())
	err := imp.Run("/nonexistent/dir", t.TempDir(), "")
	require.Error(t, err)
}
```

**Step 2: Run tests to verify they fail**

```
go test ./internal/importer/... -run TestImporter -v
```
Expected: FAIL — `importer.New` undefined.

**Step 3: Create `internal/importer/importer.go`**

```go
package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cory-johannsen/mud/internal/game/world"
	"gopkg.in/yaml.v3"
)

// Importer orchestrates content import from a Source to an output directory.
type Importer struct {
	source Source
}

// New constructs an Importer backed by the given Source.
func New(source Source) *Importer {
	return &Importer{source: source}
}

// Run loads zones from sourceDir, validates each, and writes them as YAML
// files to outputDir. Each output file is named <zone_id>.yaml.
//
// Precondition: sourceDir must satisfy the source's layout requirements;
// outputDir must exist or be creatable.
// Postcondition: one zone YAML per zone is written to outputDir, or an error
// is returned and no partial output is left.
func (imp *Importer) Run(sourceDir, outputDir, startRoom string) error {
	overall := time.Now()

	t0 := time.Now()
	zones, err := imp.source.Load(sourceDir, startRoom)
	if err != nil {
		return fmt.Errorf("loading source: %w", err)
	}
	fmt.Printf("load    %d zone(s) in %s\n", len(zones), time.Since(t0).Round(time.Millisecond))

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory %s: %w", outputDir, err)
	}

	for _, zd := range zones {
		t1 := time.Now()

		data, err := yaml.Marshal(zd)
		if err != nil {
			return fmt.Errorf("serialising zone %q: %w", zd.Zone.ID, err)
		}

		// Validate output is loadable before writing.
		if _, err := world.LoadZoneFromBytes(data); err != nil {
			return fmt.Errorf("zone %q failed validation: %w", zd.Zone.ID, err)
		}

		outPath := filepath.Join(outputDir, zd.Zone.ID+".yaml")
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("writing zone %q to %s: %w", zd.Zone.ID, outPath, err)
		}

		fmt.Printf("wrote   %s  (%d rooms)  in %s\n",
			outPath, len(zd.Zone.Rooms), time.Since(t1).Round(time.Millisecond))
	}

	fmt.Printf("total   %s\n", time.Since(overall).Round(time.Millisecond))
	return nil
}
```

**Step 4: Run all importer tests**

```
go test ./internal/importer/... -v -count=1 -race
```
Expected: PASS (all tests).

**Step 5: Commit**

```bash
git add internal/importer/importer.go internal/importer/importer_test.go
git commit -m "feat(importer): runner with YAML serialisation and world validation"
```

---

### Task 7: CLI and Makefile

**Files:**
- Create: `cmd/import-content/main.go`
- Modify: `Makefile`

**Step 1: Create `cmd/import-content/main.go`**

```go
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cory-johannsen/mud/internal/importer"
	"github.com/cory-johannsen/mud/internal/importer/gomud"
)

func main() {
	format    := flag.String("format", "", "source format: gomud")
	sourceDir := flag.String("source", "", "path to source asset directory")
	outputDir := flag.String("output", "", "path to output zone directory")
	startRoom := flag.String("start-room", "", "optional display-name override for zone start room")
	flag.Parse()

	if *format == "" || *sourceDir == "" || *outputDir == "" {
		fmt.Fprintln(os.Stderr, "usage: import-content -format <fmt> -source <dir> -output <dir> [-start-room <name>]")
		os.Exit(1)
	}

	var src importer.Source
	switch *format {
	case "gomud":
		src = gomud.NewSource()
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q (supported: gomud)\n", *format)
		os.Exit(1)
	}

	start := time.Now()
	imp := importer.New(src)
	if err := imp.Run(*sourceDir, *outputDir, *startRoom); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("import complete in %s\n", time.Since(start).Round(time.Millisecond))
}
```

**Step 2: Update Makefile**

Add to the `build` target and add new entries:

```makefile
build: build-frontend build-gameserver build-migrate build-import-content

build-import-content:
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/import-content ./cmd/import-content
```

Also add to the `.PHONY` line: `build-import-content`

**Step 3: Build and smoke test**

```
make build-import-content
./bin/import-content -help
```
Expected: usage output.

**Step 4: Commit**

```bash
git add cmd/import-content/main.go Makefile
git commit -m "feat(cmd): import-content CLI"
```

---

### Task 8: Import the actual gomud assets

**Step 1: Clone the gomud repo alongside this project**

```bash
git clone https://github.com/cory-johannsen/gomud.git /tmp/gomud
```

**Step 2: Run the importer**

```
go run ./cmd/import-content \
  -format gomud \
  -source /tmp/gomud/assets \
  -output content/zones
```

Expected output: load/write timing lines plus warnings for any unresolvable exit targets (e.g. `"Grinder's Way"`). A `content/zones/rustbucket_ridge.yaml` file should be created.

**Step 3: Verify the zone loads**

```
go test ./internal/game/world/... -v -count=1
```
Expected: PASS — the world loader exercises all zone files in `content/zones/`.

**Step 4: Spot-check the output**

```bash
head -40 content/zones/rustbucket_ridge.yaml
```
Verify: `zone.id: rustbucket_ridge`, rooms present, exits use lowercase directions and snake_case target IDs.

**Step 5: Run full test suite**

```
go test ./... -race -count=1
```
Expected: PASS.

**Step 6: Commit**

```bash
git add content/zones/rustbucket_ridge.yaml
git commit -m "content: import Rustbucket Ridge zone from gomud"
```
