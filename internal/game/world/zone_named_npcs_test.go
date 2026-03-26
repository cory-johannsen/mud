package world_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadRustbucketRidge(t *testing.T) *world.Zone {
	t.Helper()
	zone, err := world.LoadZoneFromFile("../../../content/zones/rustbucket_ridge.yaml")
	require.NoError(t, err)
	return zone
}

func TestRustbucketRidge_WayneDawgsTrailer_DangerLevelSafe(t *testing.T) {
	zone := loadRustbucketRidge(t)
	room, ok := zone.Rooms["wayne_dawgs_trailer"]
	require.True(t, ok, "wayne_dawgs_trailer must exist")
	assert.Equal(t, "safe", room.DangerLevel, "REQ-NN-7: danger_level must be safe")
}

func TestRustbucketRidge_WayneDawgsTrailer_UpdatedDescription(t *testing.T) {
	zone := loadRustbucketRidge(t)
	room := zone.Rooms["wayne_dawgs_trailer"]
	assert.Contains(t, room.Description, "Wayne and Jennifer Dawg",
		"REQ-NN-8: description must reference both Wayne and Jennifer Dawg")
	assert.Contains(t, room.Description, "makeshift lab",
		"REQ-NN-8: description must mention makeshift lab")
}

func TestRustbucketRidge_WayneDawgsTrailer_HasWestExitToDwayne(t *testing.T) {
	zone := loadRustbucketRidge(t)
	room := zone.Rooms["wayne_dawgs_trailer"]
	exit, ok := room.ExitForDirection(world.West)
	require.True(t, ok, "REQ-NN-10: wayne_dawgs_trailer must have a west exit")
	assert.Equal(t, "dwayne_dawgs_trailer", exit.TargetRoom,
		"REQ-NN-10: west exit must target dwayne_dawgs_trailer")
}

func TestRustbucketRidge_WayneDawgsTrailer_HasSpawnsForWayneAndJennifer(t *testing.T) {
	zone := loadRustbucketRidge(t)
	room := zone.Rooms["wayne_dawgs_trailer"]
	templates := make(map[string]bool)
	for _, sp := range room.Spawns {
		templates[sp.Template] = true
	}
	assert.True(t, templates["wayne_dawg"],
		"REQ-NN-9: wayne_dawgs_trailer must have spawn for wayne_dawg")
	assert.True(t, templates["jennifer_dawg"],
		"REQ-NN-9: wayne_dawgs_trailer must have spawn for jennifer_dawg")
}

func TestRustbucketRidge_DwayneDawgsTrailer_Exists(t *testing.T) {
	zone := loadRustbucketRidge(t)
	room, ok := zone.Rooms["dwayne_dawgs_trailer"]
	require.True(t, ok, "REQ-NN-11: dwayne_dawgs_trailer must exist")
	assert.Equal(t, "safe", room.DangerLevel, "REQ-NN-11: danger_level must be safe")
	assert.Equal(t, -4, room.MapX, "REQ-NN-11/REQ-NN-13: map_x must be -4")
	assert.Equal(t, 4, room.MapY, "REQ-NN-11/REQ-NN-13: map_y must be 4")
}

func TestRustbucketRidge_DwayneDawgsTrailer_HasEastExitToWayne(t *testing.T) {
	zone := loadRustbucketRidge(t)
	room := zone.Rooms["dwayne_dawgs_trailer"]
	exit, ok := room.ExitForDirection(world.East)
	require.True(t, ok, "REQ-NN-12: dwayne_dawgs_trailer must have an east exit")
	assert.Equal(t, "wayne_dawgs_trailer", exit.TargetRoom,
		"REQ-NN-12: east exit must target wayne_dawgs_trailer")
}

func TestRustbucketRidge_DwayneDawgsTrailer_HasSpawnForDwayne(t *testing.T) {
	zone := loadRustbucketRidge(t)
	room := zone.Rooms["dwayne_dawgs_trailer"]
	templates := make(map[string]bool)
	for _, sp := range room.Spawns {
		templates[sp.Template] = true
	}
	assert.True(t, templates["dwayne_dawg"],
		"REQ-NN-12: dwayne_dawgs_trailer must have spawn for dwayne_dawg")
}

func TestRustbucketRidge_MapCoordinates_NoOverlap(t *testing.T) {
	zone := loadRustbucketRidge(t)
	type coord struct{ x, y int }
	seen := make(map[coord]string)
	for id, room := range zone.Rooms {
		c := coord{room.MapX, room.MapY}
		if existing, ok := seen[c]; ok {
			assert.Failf(t, "duplicate map coordinates",
				"rooms %q and %q both have map_x=%d map_y=%d (REQ-NN-13)",
				existing, id, room.MapX, room.MapY)
		}
		seen[c] = id
	}
}
