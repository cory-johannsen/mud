package importer_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/importer"
	igomud "github.com/cory-johannsen/mud/internal/importer/gomud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
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

// TestImporter_Run_NZonesProducesNFiles is a property-based test verifying that
// Run with N zone files in the source produces exactly N output YAML files.
//
// This satisfies SWENG-5a (Property-Based Testing).
func TestImporter_Run_NZonesProducesNFiles(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate between 1 and 5 distinct zone indices.
		n := rapid.IntRange(1, 5).Draw(rt, "numZones")

		srcRoot := t.TempDir()
		if err := os.MkdirAll(filepath.Join(srcRoot, "zones"), 0755); err != nil {
			rt.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(srcRoot, "areas"), 0755); err != nil {
			rt.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(srcRoot, "rooms"), 0755); err != nil {
			rt.Fatal(err)
		}

		// Write a shared room so every zone has a valid room definition.
		sharedRoom := `
name: Shared Room
description: A shared room for testing.
exits:
`
		if err := os.WriteFile(
			filepath.Join(srcRoot, "rooms", "shared_room.yaml"),
			[]byte(sharedRoom), 0644,
		); err != nil {
			rt.Fatal(err)
		}

		// Write N zone files, each referencing the shared room.
		for i := 0; i < n; i++ {
			zoneContent := fmt.Sprintf(`
name: Zone %d
description: Generated zone %d.
rooms:
  - Shared Room
areas: []
`, i, i)
			filename := fmt.Sprintf("zone_%d.yaml", i)
			if err := os.WriteFile(
				filepath.Join(srcRoot, "zones", filename),
				[]byte(zoneContent), 0644,
			); err != nil {
				rt.Fatal(err)
			}
		}

		outDir := t.TempDir()
		imp := importer.New(igomud.NewSource())
		if err := imp.Run(srcRoot, outDir, ""); err != nil {
			rt.Fatal(err)
		}

		entries, err := os.ReadDir(outDir)
		if err != nil {
			rt.Fatal(err)
		}
		assert.Equal(rt, n, len(entries),
			"Run with %d zone file(s) must produce exactly %d output file(s)", n, n)
	})
}
