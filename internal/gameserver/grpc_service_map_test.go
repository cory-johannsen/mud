package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newWorldWithRoom creates a world.Manager containing a single zone with one room.
//
// Precondition: zoneID and roomID must be non-empty strings.
// Postcondition: Returns a Manager where GetZone(zoneID) and GetRoom(roomID) both succeed.
func newWorldWithRoom(zoneID, roomID string) *world.Manager {
	r := &world.Room{
		ID:          roomID,
		ZoneID:      zoneID,
		Title:       "Test Room",
		Description: "A test room.",
		MapX:        0,
		MapY:        0,
	}
	z := &world.Zone{
		ID:        zoneID,
		Name:      "Test Zone",
		StartRoom: roomID,
		Rooms:     map[string]*world.Room{roomID: r},
	}
	mgr, err := world.NewManager([]*world.Zone{z})
	if err != nil {
		panic("newWorldWithRoom: " + err.Error())
	}
	return mgr
}

// newWorldWithDiscoveredRoom creates a world.Manager and a session.Manager with a player whose
// AutomapCache already contains one discovered room in the given zone.
//
// Precondition: zoneID and roomID must be non-empty strings.
// Postcondition: The player session for "uid1" has AutomapCache[zoneID][roomID] == true.
func newWorldAndSessionWithDiscovery(zoneID, roomID string) (*world.Manager, *session.Manager) {
	wMgr := newWorldWithRoom(zoneID, roomID)
	sMgr := session.NewManager()
	_, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "uid1",
		Username:          "user1",
		CharName:          "Hero",
		CharacterID:       1,
		RoomID:            roomID,
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "the Northeast",
		Class:             "Gunner",
		Level:             1,
	})
	if err != nil {
		panic("newWorldAndSessionWithDiscovery AddPlayer: " + err.Error())
	}
	sess, _ := sMgr.GetPlayer("uid1")
	sess.AutomapCache[zoneID] = map[string]bool{roomID: true}
	return wMgr, sMgr
}

// TestHandleMap_PlayerNotFound verifies that handleMap returns an error when the uid
// does not map to any active player session.
//
// Precondition: No player session exists for the given uid.
// Postcondition: Returns a non-nil error.
func TestHandleMap_PlayerNotFound(t *testing.T) {
	mgr := session.NewManager()
	s := &GameServiceServer{sessions: mgr}

	_, err := s.handleMap("nonexistent-uid")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// TestHandleMap_RoomNotFound verifies that handleMap returns a "You are nowhere." message
// when the player's RoomID does not exist in the world.
//
// Precondition: Player session exists but RoomID is not in the world.
// Postcondition: Returns a ServerEvent with a message containing "You are nowhere.".
func TestHandleMap_RoomNotFound(t *testing.T) {
	// Build a world that does NOT contain "ghost_room"
	wMgr := newWorldWithRoom("zone1", "real_room")

	sMgr := session.NewManager()
	_, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "uid1",
		Username:          "user1",
		CharName:          "Hero",
		CharacterID:       1,
		RoomID:            "ghost_room",
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "the Northeast",
		Class:             "Gunner",
		Level:             1,
	})
	require.NoError(t, err)

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleMap("uid1")
	require.NoError(t, err)
	require.NotNil(t, result)
	msg := result.GetMessage()
	require.NotNil(t, msg)
	require.Contains(t, msg.Content, "You are nowhere.")
}

// TestHandleMap_ZoneNotFound verifies that handleMap returns a "No map available." message
// when the room's ZoneID does not exist in the world.
//
// Precondition: The player's room exists in the world index but its ZoneID resolves to no zone.
// Postcondition: Returns a ServerEvent with a message containing "No map available.".
func TestHandleMap_ZoneNotFound(t *testing.T) {
	// Create a room whose ZoneID is "missing_zone", but only register "other_zone" in the manager.
	orphanRoom := &world.Room{
		ID:          "orphan_room",
		ZoneID:      "missing_zone",
		Title:       "Orphan Room",
		Description: "A room whose zone is absent.",
		MapX:        0,
		MapY:        0,
	}
	// Build a zone that holds the orphan room so it appears in the room index.
	z := &world.Zone{
		ID:        "missing_zone",
		Name:      "Missing Zone",
		StartRoom: "orphan_room",
		Rooms:     map[string]*world.Room{"orphan_room": orphanRoom},
	}
	// Remove the zone from the manager after construction so GetRoom succeeds but GetZone fails.
	// We achieve this by constructing via a private wrapper: create with zone, then verify,
	// then use a second Manager that only has a different zone.
	//
	// Simpler approach: create the Manager with the zone present, but override the player's
	// RoomID to a room whose ZoneID doesn't match any loaded zone.
	// We do this by adding the room to a zone that the world index holds, but under a zone ID
	// that GetZone will not find. The easiest way is to rely on the fact that the world.Manager
	// indexes rooms globally but zones by their Zone.ID. If we create a Zone with ID "z_carrier"
	// that holds orphan_room (ZoneID="missing_zone"), GetRoom("orphan_room") works but
	// GetZone("missing_zone") fails because the registered zone ID is "z_carrier".
	carrier := &world.Zone{
		ID:        "z_carrier",
		Name:      "Carrier Zone",
		StartRoom: "orphan_room",
		Rooms:     map[string]*world.Room{"orphan_room": orphanRoom},
	}
	_ = z // suppress unused warning
	wMgr, err := world.NewManager([]*world.Zone{carrier})
	require.NoError(t, err)

	sMgr := session.NewManager()
	_, err = sMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "uid1",
		Username:          "user1",
		CharName:          "Hero",
		CharacterID:       1,
		RoomID:            "orphan_room",
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "the Northeast",
		Class:             "Gunner",
		Level:             1,
	})
	require.NoError(t, err)

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleMap("uid1")
	require.NoError(t, err)
	require.NotNil(t, result)
	msg := result.GetMessage()
	require.NotNil(t, msg)
	require.Contains(t, msg.Content, "No map available.")
}

// TestHandleMap_NoDiscoveredRooms verifies that handleMap returns a MapResponse with an empty
// tile list when the player has not yet discovered any rooms in the current zone.
//
// Precondition: Zone and room exist; player's AutomapCache for the zone is empty.
// Postcondition: Returns a MapResponse with Tiles == nil or len(Tiles) == 0.
func TestHandleMap_NoDiscoveredRooms(t *testing.T) {
	wMgr := newWorldWithRoom("zone1", "room1")
	sMgr := session.NewManager()
	_, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:               "uid1",
		Username:          "user1",
		CharName:          "Hero",
		CharacterID:       1,
		RoomID:            "room1",
		CurrentHP:         10,
		MaxHP:             10,
		Abilities:         character.AbilityScores{},
		Role:              "player",
		RegionDisplayName: "the Northeast",
		Class:             "Gunner",
		Level:             1,
	})
	require.NoError(t, err)
	// AutomapCache is initialised to an empty map — do not add any entries.

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleMap("uid1")
	require.NoError(t, err)
	require.NotNil(t, result)
	mapResp := result.GetMap()
	require.NotNil(t, mapResp)
	require.Empty(t, mapResp.Tiles)
}

// TestHandleMap_WithDiscoveredRooms verifies that handleMap returns a MapResponse whose
// tile list contains exactly the discovered rooms with correct coordinates and current flag.
//
// Precondition: Zone and room exist; player's AutomapCache contains the room.
// Postcondition: Returns a MapResponse with one tile matching the room's coordinates and
// Current == true (because the player is in that room).
func TestHandleMap_WithDiscoveredRooms(t *testing.T) {
	wMgr, sMgr := newWorldAndSessionWithDiscovery("zone1", "room1")

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleMap("uid1")
	require.NoError(t, err)
	require.NotNil(t, result)
	mapResp := result.GetMap()
	require.NotNil(t, mapResp)
	require.Len(t, mapResp.Tiles, 1)

	tile := mapResp.Tiles[0]
	require.Equal(t, "room1", tile.RoomId)
	require.Equal(t, "Test Room", tile.RoomName)
	require.Equal(t, int32(0), tile.X)
	require.Equal(t, int32(0), tile.Y)
	require.True(t, tile.Current, "tile for the player's current room must have Current == true")
}

// TestProperty_HandleMap_DiscoveredRoomsAlwaysInResponse is a property-based test verifying
// that any room added to AutomapCache always appears in the MapResponse tiles.
//
// Precondition: Zone and room exist; room is added to AutomapCache before calling handleMap.
// Postcondition: The returned tile list always contains the discovered room.
func TestProperty_HandleMap_DiscoveredRoomsAlwaysInResponse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		roomID := rapid.StringMatching(`[a-z][a-z0-9_]{2,8}`).Draw(t, "roomID")
		zoneID := rapid.StringMatching(`[a-z][a-z0-9_]{2,8}`).Draw(t, "zoneID")

		r := &world.Room{
			ID:          roomID,
			ZoneID:      zoneID,
			Title:       "Prop Room",
			Description: "A room for property testing.",
			MapX:        rapid.Int().Draw(t, "mapX"),
			MapY:        rapid.Int().Draw(t, "mapY"),
		}
		z := &world.Zone{
			ID:        zoneID,
			Name:      "Prop Zone",
			StartRoom: roomID,
			Rooms:     map[string]*world.Room{roomID: r},
		}
		wMgr, err := world.NewManager([]*world.Zone{z})
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}

		sMgr := session.NewManager()
		_, addErr := sMgr.AddPlayer(session.AddPlayerOptions{
			UID:               "uid1",
			Username:          "user1",
			CharName:          "Hero",
			CharacterID:       1,
			RoomID:            roomID,
			CurrentHP:         10,
			MaxHP:             10,
			Abilities:         character.AbilityScores{},
			Role:              "player",
			RegionDisplayName: "the Northeast",
			Class:             "Gunner",
			Level:             1,
		})
		if addErr != nil {
			t.Fatalf("AddPlayer: %v", addErr)
		}
		sess, _ := sMgr.GetPlayer("uid1")
		sess.AutomapCache[zoneID] = map[string]bool{roomID: true}

		s := &GameServiceServer{sessions: sMgr, world: wMgr}
		result, err := s.handleMap("uid1")
		if err != nil {
			t.Fatalf("handleMap: %v", err)
		}
		mapResp := result.GetMap()
		if mapResp == nil {
			t.Fatalf("expected MapResponse payload, got %T", result.Payload)
		}
		if len(mapResp.Tiles) != 1 {
			t.Fatalf("expected 1 tile, got %d", len(mapResp.Tiles))
		}
		tile := mapResp.Tiles[0]
		if tile.RoomId != roomID {
			t.Fatalf("tile.RoomId = %q, want %q", tile.RoomId, roomID)
		}
		if tile.X != int32(r.MapX) {
			t.Fatalf("tile.X = %d, want %d", tile.X, r.MapX)
		}
		if tile.Y != int32(r.MapY) {
			t.Fatalf("tile.Y = %d, want %d", tile.Y, r.MapY)
		}
		if !tile.Current {
			t.Fatalf("tile.Current must be true for the player's current room")
		}
	})
}

// Compile-time check that gamev1 is used.
var _ *gamev1.MapResponse
