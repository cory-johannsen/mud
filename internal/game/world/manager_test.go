package world

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func testManagerZones() []*Zone {
	return []*Zone{validTestZone()}
}

func TestNewManager(t *testing.T) {
	mgr, err := NewManager(testManagerZones())
	require.NoError(t, err)
	assert.Equal(t, 2, mgr.RoomCount())
	assert.Equal(t, 1, mgr.ZoneCount())
}

func TestNewManager_DuplicateZone(t *testing.T) {
	zones := []*Zone{validTestZone(), validTestZone()}
	_, err := NewManager(zones)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate zone ID")
}

func TestNewManager_DuplicateRoom(t *testing.T) {
	z1 := validTestZone()
	z2 := &Zone{
		ID:        "other",
		Name:      "Other",
		StartRoom: "room_a",
		Rooms: map[string]*Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "other",
				Title:       "Duplicate",
				Description: "Duplicate room_a",
				Properties:  map[string]string{},
			},
		},
	}
	_, err := NewManager([]*Zone{z1, z2})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate room ID")
}

func TestManager_GetRoom(t *testing.T) {
	mgr, err := NewManager(testManagerZones())
	require.NoError(t, err)

	room, ok := mgr.GetRoom("room_a")
	assert.True(t, ok)
	assert.Equal(t, "Room A", room.Title)

	_, ok = mgr.GetRoom("nonexistent")
	assert.False(t, ok)
}

func TestManager_Navigate(t *testing.T) {
	mgr, err := NewManager(testManagerZones())
	require.NoError(t, err)

	room, err := mgr.Navigate("room_a", North)
	require.NoError(t, err)
	assert.Equal(t, "room_b", room.ID)

	room, err = mgr.Navigate("room_b", South)
	require.NoError(t, err)
	assert.Equal(t, "room_a", room.ID)
}

func TestManager_Navigate_NoExit(t *testing.T) {
	mgr, err := NewManager(testManagerZones())
	require.NoError(t, err)

	_, err = mgr.Navigate("room_a", West)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no exit")
}

func TestManager_Navigate_BadRoom(t *testing.T) {
	mgr, err := NewManager(testManagerZones())
	require.NoError(t, err)

	_, err = mgr.Navigate("nonexistent", North)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestManager_Navigate_Locked(t *testing.T) {
	zone := validTestZone()
	zone.Rooms["room_a"].Exits = []Exit{
		{Direction: North, TargetRoom: "room_b", Locked: true},
	}
	mgr, err := NewManager([]*Zone{zone})
	require.NoError(t, err)

	_, err = mgr.Navigate("room_a", North)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "locked")
}

func TestManager_StartRoom(t *testing.T) {
	mgr, err := NewManager(testManagerZones())
	require.NoError(t, err)

	start := mgr.StartRoom()
	require.NotNil(t, start)
	assert.Equal(t, "room_a", start.ID)
}

func TestPropertyNavigateFromStartRoomSucceeds(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		zone := genValidZone(t)
		mgr, err := NewManager([]*Zone{zone})
		if err != nil {
			t.Skip("manager creation failed (expected for some generated zones)")
		}

		start := mgr.StartRoom()
		if start == nil {
			t.Fatal("start room is nil")
		}

		// Every exit from start room should navigate successfully
		for _, exit := range start.Exits {
			if exit.Locked {
				continue
			}
			dest, err := mgr.Navigate(start.ID, exit.Direction)
			if err != nil {
				t.Fatalf("navigation from start %q via %q failed: %v", start.ID, exit.Direction, err)
			}
			if dest == nil {
				t.Fatalf("navigation returned nil room")
			}
		}
	})
}

func TestPropertyAllRoomsReachableFromStart(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		zone := genConnectedZone(t)
		mgr, err := NewManager([]*Zone{zone})
		if err != nil {
			t.Skip("manager creation failed")
		}

		start := mgr.StartRoom()
		if start == nil {
			t.Fatal("start room is nil")
		}

		// BFS from start
		visited := make(map[string]bool)
		queue := []string{start.ID}
		visited[start.ID] = true

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			room, ok := mgr.GetRoom(current)
			if !ok {
				continue
			}
			for _, exit := range room.Exits {
				if !visited[exit.TargetRoom] {
					visited[exit.TargetRoom] = true
					queue = append(queue, exit.TargetRoom)
				}
			}
		}

		if len(visited) != mgr.RoomCount() {
			t.Fatalf("only %d/%d rooms reachable from start", len(visited), mgr.RoomCount())
		}
	})
}

func TestManager_ValidateExits_Valid(t *testing.T) {
	mgr, err := NewManager(testManagerZones())
	require.NoError(t, err)
	assert.NoError(t, mgr.ValidateExits())
}

func TestManager_ValidateExits_CrossZoneValid(t *testing.T) {
	z1 := &Zone{
		ID: "zone_a", Name: "Zone A", Description: "A", StartRoom: "a1",
		Rooms: map[string]*Room{
			"a1": {ID: "a1", ZoneID: "zone_a", Title: "A1", Description: "Room A1",
				Exits: []Exit{{Direction: North, TargetRoom: "b1"}}, Properties: map[string]string{}},
		},
	}
	z2 := &Zone{
		ID: "zone_b", Name: "Zone B", Description: "B", StartRoom: "b1",
		Rooms: map[string]*Room{
			"b1": {ID: "b1", ZoneID: "zone_b", Title: "B1", Description: "Room B1",
				Exits: []Exit{{Direction: South, TargetRoom: "a1"}}, Properties: map[string]string{}},
		},
	}
	mgr, err := NewManager([]*Zone{z1, z2})
	require.NoError(t, err)
	assert.NoError(t, mgr.ValidateExits())
}

func TestManager_ValidateExits_DanglingTarget(t *testing.T) {
	z1 := &Zone{
		ID: "zone_a", Name: "Zone A", Description: "A", StartRoom: "a1",
		Rooms: map[string]*Room{
			"a1": {ID: "a1", ZoneID: "zone_a", Title: "A1", Description: "Room A1",
				Exits: []Exit{{Direction: North, TargetRoom: "nonexistent"}}, Properties: map[string]string{}},
		},
	}
	mgr, err := NewManager([]*Zone{z1})
	require.NoError(t, err)
	err = mgr.ValidateExits()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown room")
}

// genConnectedZone generates a zone where all rooms are reachable from start.
func genConnectedZone(t *rapid.T) *Zone {
	numRooms := rapid.IntRange(2, 6).Draw(t, "num_rooms")
	roomIDs := make([]string, numRooms)
	for i := range roomIDs {
		roomIDs[i] = rapid.StringMatching(`r_[a-z]{3,5}`).Draw(t, "room_id")
		for j := 0; j < i; j++ {
			if roomIDs[j] == roomIDs[i] {
				roomIDs[i] = roomIDs[i] + rapid.StringMatching(`[0-9]{2}`).Draw(t, "suffix")
			}
		}
	}

	rooms := make(map[string]*Room, numRooms)

	// Create rooms with a chain of exits to guarantee connectivity
	for i, id := range roomIDs {
		room := &Room{
			ID:          id,
			ZoneID:      "gen",
			Title:       "Room " + id,
			Description: "Generated room " + id,
			Properties:  map[string]string{},
		}
		if i < numRooms-1 {
			dirIdx := i % len(StandardDirections)
			room.Exits = append(room.Exits, Exit{
				Direction:  StandardDirections[dirIdx],
				TargetRoom: roomIDs[i+1],
			})
		}
		if i > 0 {
			dirIdx := (i + 5) % len(StandardDirections)
			room.Exits = append(room.Exits, Exit{
				Direction:  StandardDirections[dirIdx],
				TargetRoom: roomIDs[i-1],
			})
		}
		rooms[id] = room
	}

	return &Zone{
		ID:          "gen",
		Name:        "Generated",
		Description: "Generated zone",
		StartRoom:   roomIDs[0],
		Rooms:       rooms,
	}
}
