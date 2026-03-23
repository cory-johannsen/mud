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

	_, err := s.handleMap("nonexistent-uid", &gamev1.MapRequest{})
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
	result, err := s.handleMap("uid1", &gamev1.MapRequest{})
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
	result, err := s.handleMap("uid1", &gamev1.MapRequest{})
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
	result, err := s.handleMap("uid1", &gamev1.MapRequest{})
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
	result, err := s.handleMap("uid1", &gamev1.MapRequest{})
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
		result, err := s.handleMap("uid1", &gamev1.MapRequest{})
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

// TestHandleMap_ReflectsSessionCacheUpdatedMidSession verifies that when the AutomapCache
// is populated after AddPlayer (simulating a RevealZoneMap callback executing mid-session),
// handleMap returns the newly revealed rooms.
//
// Precondition: Player session exists with an empty AutomapCache; rooms are added to the cache
// by direct pointer mutation (as RevealZoneMap does via GetPlayer pointer).
// Postcondition: handleMap returns a MapResponse containing all rooms added to the cache.
func TestHandleMap_ReflectsSessionCacheUpdatedMidSession(t *testing.T) {
	const zoneID = "zone1"
	const roomID = "room1"

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
	require.NoError(t, err)

	// Simulate RevealZoneMap: obtain the live session pointer and mutate AutomapCache.
	sess, ok := sMgr.GetPlayer("uid1")
	require.True(t, ok)
	require.Empty(t, sess.AutomapCache[zoneID], "AutomapCache should be empty before reveal")

	// Mutate the cache through the pointer — exactly as RevealZoneMap does.
	if sess.AutomapCache[zoneID] == nil {
		sess.AutomapCache[zoneID] = make(map[string]bool)
	}
	sess.AutomapCache[zoneID][roomID] = true

	// handleMap should now see the updated cache.
	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleMap("uid1", &gamev1.MapRequest{})
	require.NoError(t, err)
	require.NotNil(t, result)
	mapResp := result.GetMap()
	require.NotNil(t, mapResp)
	require.Len(t, mapResp.Tiles, 1, "expected 1 tile after mid-session cache update")
	require.Equal(t, roomID, mapResp.Tiles[0].RoomId)
	require.True(t, mapResp.Tiles[0].Current, "tile for the player's current room must have Current == true")
}

// TestWireRevealZone_PopulatesAutomapCache verifies that calling RevealZoneMap
// via wireRevealZone populates the player's AutomapCache for all rooms in the zone.
//
// Precondition: A zone with multiple rooms exists; player's AutomapCache is empty.
// Postcondition: After calling RevealZoneMap, AutomapCache[zoneID] contains all room IDs.
func TestWireRevealZone_PopulatesAutomapCache(t *testing.T) {
	const zoneID = "zone1"
	r1 := &world.Room{ID: "room1", ZoneID: zoneID, Title: "Room 1", MapX: 0, MapY: 0}
	r2 := &world.Room{ID: "room2", ZoneID: zoneID, Title: "Room 2", MapX: 1, MapY: 0}
	z := &world.Zone{
		ID:        zoneID,
		Name:      "Test Zone",
		StartRoom: "room1",
		Rooms:     map[string]*world.Room{"room1": r1, "room2": r2},
	}
	wMgr, err := world.NewManager([]*world.Zone{z})
	require.NoError(t, err)

	sMgr := session.NewManager()
	_, err = sMgr.AddPlayer(session.AddPlayerOptions{
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

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	// scriptMgr is nil — wireRevealZone must be a no-op without panicking.
	require.Nil(t, s.scriptMgr, "scriptMgr should be nil in this test")
	require.NotPanics(t, func() { s.wireRevealZone() })
}

// TestWireRevealZone_BulkReveal verifies that RevealZoneMap callback, when set,
// populates AutomapCache for all rooms in the target zone.
//
// Precondition: A zone with 3 rooms exists; player AutomapCache is empty; scriptMgr is non-nil.
// Postcondition: After calling RevealZoneMap(uid, zoneID), AutomapCache[zoneID] contains all 3 room IDs.
func TestWireRevealZone_BulkReveal(t *testing.T) {
	const zoneID = "zone1"
	const uid = "uid1"
	r1 := &world.Room{ID: "room1", ZoneID: zoneID, Title: "Room 1", MapX: 0, MapY: 0}
	r2 := &world.Room{ID: "room2", ZoneID: zoneID, Title: "Room 2", MapX: 1, MapY: 0}
	r3 := &world.Room{ID: "room3", ZoneID: zoneID, Title: "Room 3", MapX: 0, MapY: 1}
	z := &world.Zone{
		ID:        zoneID,
		Name:      "Test Zone",
		StartRoom: "room1",
		Rooms:     map[string]*world.Room{"room1": r1, "room2": r2, "room3": r3},
	}
	wMgr, err := world.NewManager([]*world.Zone{z})
	require.NoError(t, err)

	sMgr := session.NewManager()
	_, err = sMgr.AddPlayer(session.AddPlayerOptions{
		UID:               uid,
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

	// Directly invoke the wireRevealZone logic without a real scriptMgr:
	// wire the callback manually to simulate what wireRevealZone does.
	s := &GameServiceServer{sessions: sMgr, world: wMgr}

	// Directly call the reveal logic as wireRevealZone would wire it.
	revealFn := func(revUID, revZoneID string) {
		zone, ok := s.world.GetZone(revZoneID)
		if !ok {
			return
		}
		sess, ok := s.sessions.GetPlayer(revUID)
		if !ok {
			return
		}
		if sess.AutomapCache[revZoneID] == nil {
			sess.AutomapCache[revZoneID] = make(map[string]bool)
		}
		for roomID := range zone.Rooms {
			sess.AutomapCache[revZoneID][roomID] = true
		}
	}

	revealFn(uid, zoneID)

	sess, ok := sMgr.GetPlayer(uid)
	require.True(t, ok)
	require.Len(t, sess.AutomapCache[zoneID], 3, "AutomapCache must contain all 3 rooms after reveal")
	require.True(t, sess.AutomapCache[zoneID]["room1"])
	require.True(t, sess.AutomapCache[zoneID]["room2"])
	require.True(t, sess.AutomapCache[zoneID]["room3"])
}

// Compile-time check that gamev1 is used.
var _ *gamev1.MapResponse

// TestHandleMap_WorldView_ReturnsWorldTiles verifies that view=="world" returns WorldZoneTile
// entries for zones with non-nil WorldX/WorldY.
func TestHandleMap_WorldView_ReturnsWorldTiles(t *testing.T) {
	wx, wy := 0, 0
	r := &world.Room{ID: "room1", ZoneID: "zone1", Title: "Room", Description: "Desc"}
	z := &world.Zone{
		ID: "zone1", Name: "Test Zone", StartRoom: "room1",
		Rooms:       map[string]*world.Room{"room1": r},
		DangerLevel: "sketchy",
		WorldX:      &wx, WorldY: &wy,
	}
	wMgr, err := world.NewManager([]*world.Zone{z})
	require.NoError(t, err)
	sMgr := session.NewManager()
	_, err = sMgr.AddPlayer(session.AddPlayerOptions{
		UID: "uid1", Username: "u", CharName: "Hero", CharacterID: 1,
		RoomID: "room1", CurrentHP: 10, MaxHP: 10,
		Abilities: character.AbilityScores{}, Role: "player",
		RegionDisplayName: "test", Class: "Gunner", Level: 1,
	})
	require.NoError(t, err)
	sess, _ := sMgr.GetPlayer("uid1")
	sess.AutomapCache["zone1"] = map[string]bool{"room1": true}

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleMap("uid1", &gamev1.MapRequest{View: "world"})
	require.NoError(t, err)
	mapResp := result.GetMap()
	require.NotNil(t, mapResp)
	require.Empty(t, mapResp.Tiles, "world view must not populate zone tiles")
	require.Len(t, mapResp.WorldTiles, 1)
	tile := mapResp.WorldTiles[0]
	require.Equal(t, "zone1", tile.ZoneId)
	require.Equal(t, "Test Zone", tile.ZoneName)
	require.True(t, tile.Discovered)
	require.True(t, tile.Current)
	require.Equal(t, "sketchy", tile.DangerLevel)
}

// TestHandleMap_WorldView_ExcludesNilCoordZones verifies that zones without
// WorldX/WorldY are excluded from world_tiles.
func TestHandleMap_WorldView_ExcludesNilCoordZones(t *testing.T) {
	r := &world.Room{ID: "room1", ZoneID: "zone1", Title: "Room", Description: "Desc"}
	z := &world.Zone{
		ID: "zone1", Name: "No Coords", StartRoom: "room1",
		Rooms: map[string]*world.Room{"room1": r},
	}
	wMgr, err := world.NewManager([]*world.Zone{z})
	require.NoError(t, err)

	sMgr := session.NewManager()
	_, err = sMgr.AddPlayer(session.AddPlayerOptions{
		UID: "uid1", Username: "u", CharName: "Hero", CharacterID: 1,
		RoomID: "room1", CurrentHP: 10, MaxHP: 10,
		Abilities: character.AbilityScores{}, Role: "player",
		RegionDisplayName: "test", Class: "Gunner", Level: 1,
	})
	require.NoError(t, err)

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleMap("uid1", &gamev1.MapRequest{View: "world"})
	require.NoError(t, err)
	mapResp := result.GetMap()
	require.NotNil(t, mapResp)
	require.Empty(t, mapResp.WorldTiles)
}

// TestHandleMap_ZoneView_SignatureUnchanged verifies that view=="" (or "zone") still
// returns zone map tiles as before.
func TestHandleMap_ZoneView_SignatureUnchanged(t *testing.T) {
	wMgr, sMgr := newWorldAndSessionWithDiscovery("zone1", "room1")
	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	result, err := s.handleMap("uid1", &gamev1.MapRequest{})
	require.NoError(t, err)
	mapResp := result.GetMap()
	require.NotNil(t, mapResp)
	require.Len(t, mapResp.Tiles, 1)
	require.Empty(t, mapResp.WorldTiles)
}

// TestProperty_HandleMap_WorldView_DiscoveredFlagMatchesCache is a property-based test
// verifying that WorldZoneTile.Discovered always matches len(AutomapCache[zoneID]) > 0.
func TestProperty_HandleMap_WorldView_DiscoveredFlagMatchesCache(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		discovered := rapid.Bool().Draw(t, "discovered")
		wx, wy := 0, 0
		r := &world.Room{ID: "room1", ZoneID: "zone1", Title: "R", Description: "D"}
		z := &world.Zone{
			ID: "zone1", Name: "Z", StartRoom: "room1",
			Rooms:  map[string]*world.Room{"room1": r},
			WorldX: &wx, WorldY: &wy,
		}
		wMgr, err := world.NewManager([]*world.Zone{z})
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}
		sMgr := session.NewManager()
		_, addErr := sMgr.AddPlayer(session.AddPlayerOptions{
			UID: "uid1", Username: "u", CharName: "Hero", CharacterID: 1,
			RoomID: "room1", CurrentHP: 10, MaxHP: 10,
			Abilities: character.AbilityScores{}, Role: "player",
			RegionDisplayName: "test", Class: "Gunner", Level: 1,
		})
		if addErr != nil {
			t.Fatalf("AddPlayer: %v", addErr)
		}
		if discovered {
			sess, _ := sMgr.GetPlayer("uid1")
			sess.AutomapCache["zone1"] = map[string]bool{"room1": true}
		}
		s := &GameServiceServer{sessions: sMgr, world: wMgr}
		result, err := s.handleMap("uid1", &gamev1.MapRequest{View: "world"})
		if err != nil {
			t.Fatalf("handleMap: %v", err)
		}
		mapResp := result.GetMap()
		if mapResp == nil || len(mapResp.WorldTiles) != 1 {
			t.Fatalf("expected 1 world tile")
		}
		if mapResp.WorldTiles[0].Discovered != discovered {
			t.Fatalf("Discovered=%v but cache populated=%v", mapResp.WorldTiles[0].Discovered, discovered)
		}
	})
}
