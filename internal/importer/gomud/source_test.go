package gomud_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/world"
	igomud "github.com/cory-johannsen/mud/internal/importer/gomud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"pgregory.net/rapid"
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
	roomAreas := make(map[string]string)
	for _, r := range zd.Zone.Rooms {
		if area, ok := r.Properties["area"]; ok {
			roomAreas[r.ID] = area
		}
	}
	assert.Equal(t, "area_one", roomAreas["room_alpha"])
	assert.Equal(t, "area_one", roomAreas["room_beta"])
	assert.Equal(t, "area_two", roomAreas["room_gamma"])
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

// TestGomudSource_ZoneCountMatchesFiles is a property-based test verifying that
// the number of zones returned by Load equals the number of .yaml files written
// into the zones/ subdirectory.
func TestGomudSource_ZoneCountMatchesFiles(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate between 1 and 4 distinct zone names to avoid collisions.
		n := rapid.IntRange(1, 4).Draw(rt, "numZones")

		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "zones"), 0755); err != nil {
			rt.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "areas"), 0755); err != nil {
			rt.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "rooms"), 0755); err != nil {
			rt.Fatal(err)
		}

		// Write one room so every zone has at least one valid room.
		roomYAML := `
name: Room Omega
description: The omega room.
exits:
`
		if err := os.WriteFile(
			filepath.Join(root, "rooms", "room_omega.yaml"),
			[]byte(roomYAML), 0644,
		); err != nil {
			rt.Fatal(err)
		}

		for i := 0; i < n; i++ {
			zoneContent := fmt.Sprintf(`
name: Zone %d
description: Generated zone %d.
rooms:
  - Room Omega
areas: []
`, i, i)
			filename := fmt.Sprintf("zone_%d.yaml", i)
			if err := os.WriteFile(
				filepath.Join(root, "zones", filename),
				[]byte(zoneContent), 0644,
			); err != nil {
				rt.Fatal(err)
			}
		}

		src := igomud.NewSource()
		zones, err := src.Load(root, "")
		if err != nil {
			rt.Fatal(err)
		}
		assert.Equal(t, n, len(zones),
			"number of zones returned must equal number of .yaml files in zones/")
	})
}
