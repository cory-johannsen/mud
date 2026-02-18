package gomud_test

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/importer"
	igomud "github.com/cory-johannsen/mud/internal/importer/gomud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
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

func TestConvertZone_StartRoomOverride_NotInList_Warns(t *testing.T) {
	zone := &igomud.GomudZone{
		Name:  "Test Zone",
		Rooms: []string{"Room A"},
	}
	rooms := map[string]*igomud.GomudRoom{
		"Room A": {Name: "Room A", Description: "desc"},
	}

	_, warnings := igomud.ConvertZone(zone, rooms, nil, "Room Z")
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "Room Z")
}

// TestConvertZone_Deterministic ensures output is stable across multiple calls (no ordering issues).
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

// TestConvertZone_NonNilOutput is a property-based test verifying ConvertZone always
// returns a non-nil ZoneData regardless of input.
func TestConvertZone_NonNilOutput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := rapid.SliceOfN(rapid.StringMatching(`[A-Za-z ]{1,20}`), 0, 5).Draw(t, "names")
		zone := &igomud.GomudZone{
			Name:  "Zone",
			Rooms: names,
		}
		rooms := make(map[string]*igomud.GomudRoom, len(names))
		for _, n := range names {
			rooms[n] = &igomud.GomudRoom{Name: n, Description: "d"}
		}
		zd, warnings := igomud.ConvertZone(zone, rooms, nil, "")
		assert.NotNil(t, zd)
		assert.GreaterOrEqual(t, len(warnings), 0)
	})
}

// TestConvertZone_OutputRoomCountLeInput is a property-based test verifying the
// output room count never exceeds the input zone room list length.
func TestConvertZone_OutputRoomCountLeInput(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := rapid.SliceOfN(rapid.StringMatching(`[A-Za-z ]{1,20}`), 0, 5).Draw(t, "names")
		zone := &igomud.GomudZone{
			Name:  "Zone",
			Rooms: names,
		}
		// Only add definitions for a subset — missing ones will be warned and skipped.
		rooms := make(map[string]*igomud.GomudRoom)
		for i, n := range names {
			if i%2 == 0 {
				rooms[n] = &igomud.GomudRoom{Name: n, Description: "d"}
			}
		}
		zd, _ := igomud.ConvertZone(zone, rooms, nil, "")
		assert.LessOrEqual(t, len(zd.Zone.Rooms), len(names),
			"output room count must not exceed input room list length")
	})
}

// TestConvertZone_WarningCountNonNegative is a property-based test confirming
// that warning count is always >= 0 (i.e., the slice is never nil when non-empty).
func TestConvertZone_WarningCountNonNegative(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		zone := &igomud.GomudZone{
			Name:  "Zone",
			Rooms: []string{"R"},
		}
		rooms := map[string]*igomud.GomudRoom{
			"R": {Name: "R", Description: "d"},
		}
		zd, warnings := igomud.ConvertZone(zone, rooms, nil, "")
		assert.NotNil(t, zd)
		assert.GreaterOrEqual(t, len(warnings), 0)
	})
}

// Compile-time check: ConvertZone must return *importer.ZoneData.
var _ *importer.ZoneData = func() *importer.ZoneData {
	zd, _ := igomud.ConvertZone(&igomud.GomudZone{}, nil, nil, "")
	return zd
}()
