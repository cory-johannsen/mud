package world

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestDirection_IsStandard(t *testing.T) {
	for _, d := range StandardDirections {
		assert.True(t, d.IsStandard(), "expected %q to be standard", d)
	}
	assert.False(t, Direction("stairs").IsStandard())
	assert.False(t, Direction("portal").IsStandard())
}

func TestDirection_Opposite(t *testing.T) {
	pairs := [][2]Direction{
		{North, South},
		{East, West},
		{Northeast, Southwest},
		{Northwest, Southeast},
		{Up, Down},
	}
	for _, pair := range pairs {
		assert.Equal(t, pair[1], pair[0].Opposite())
		assert.Equal(t, pair[0], pair[1].Opposite())
	}
	assert.Equal(t, Direction(""), Direction("stairs").Opposite())
}

func TestPropertyOppositeIsInvolution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		idx := rapid.IntRange(0, len(StandardDirections)-1).Draw(t, "dir_idx")
		d := StandardDirections[idx]
		assert.Equal(t, d, d.Opposite().Opposite(), "opposite should be an involution for %q", d)
	})
}

func TestRoom_ExitForDirection(t *testing.T) {
	room := &Room{
		ID: "test",
		Exits: []Exit{
			{Direction: North, TargetRoom: "north_room"},
			{Direction: East, TargetRoom: "east_room"},
		},
	}

	exit, ok := room.ExitForDirection(North)
	assert.True(t, ok)
	assert.Equal(t, "north_room", exit.TargetRoom)

	_, ok = room.ExitForDirection(South)
	assert.False(t, ok)
}

func TestRoom_VisibleExits(t *testing.T) {
	room := &Room{
		ID: "test",
		Exits: []Exit{
			{Direction: North, TargetRoom: "a"},
			{Direction: South, TargetRoom: "b", Hidden: true},
			{Direction: East, TargetRoom: "c"},
		},
	}

	visible := room.VisibleExits()
	assert.Len(t, visible, 2)
	assert.Equal(t, North, visible[0].Direction)
	assert.Equal(t, East, visible[1].Direction)
}

func TestZone_Validate_Valid(t *testing.T) {
	zone := validTestZone()
	assert.NoError(t, zone.Validate())
}

func TestZone_Validate_EmptyID(t *testing.T) {
	zone := validTestZone()
	zone.ID = ""
	assert.Error(t, zone.Validate())
}

func TestZone_Validate_EmptyName(t *testing.T) {
	zone := validTestZone()
	zone.Name = ""
	assert.Error(t, zone.Validate())
}

func TestZone_Validate_MissingStartRoom(t *testing.T) {
	zone := validTestZone()
	zone.StartRoom = "nonexistent"
	assert.Error(t, zone.Validate())
}

func TestZone_Validate_ExitTargetMissing(t *testing.T) {
	zone := validTestZone()
	zone.Rooms["room_a"].Exits = []Exit{
		{Direction: North, TargetRoom: "nonexistent"},
	}
	assert.Error(t, zone.Validate())
}

func TestZone_Validate_EmptyRoomTitle(t *testing.T) {
	zone := validTestZone()
	zone.Rooms["room_a"].Title = ""
	assert.Error(t, zone.Validate())
}

func TestZone_Validate_EmptyRoomDescription(t *testing.T) {
	zone := validTestZone()
	zone.Rooms["room_a"].Description = ""
	assert.Error(t, zone.Validate())
}

func TestZone_Validate_NoRooms(t *testing.T) {
	zone := validTestZone()
	zone.Rooms = map[string]*Room{}
	assert.Error(t, zone.Validate())
}

func TestZone_Validate_RoomKeyMismatch(t *testing.T) {
	zone := validTestZone()
	room := zone.Rooms["room_a"]
	room.ID = "wrong_id"
	assert.Error(t, zone.Validate())
}

func TestPropertyAllExitTargetsExist(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		zone := genValidZone(t)
		for _, room := range zone.Rooms {
			for _, exit := range room.Exits {
				_, ok := zone.Rooms[exit.TargetRoom]
				if !ok {
					t.Fatalf("room %q exit %q targets unknown room %q", room.ID, exit.Direction, exit.TargetRoom)
				}
			}
		}
	})
}

func TestPropertyNoDuplicateRoomIDs(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		zone := genValidZone(t)
		seen := make(map[string]bool)
		for id := range zone.Rooms {
			if seen[id] {
				t.Fatalf("duplicate room ID: %q", id)
			}
			seen[id] = true
		}
	})
}

// genValidZone generates a random valid zone for property testing.
func genValidZone(t *rapid.T) *Zone {
	numRooms := rapid.IntRange(2, 8).Draw(t, "num_rooms")
	roomIDs := make([]string, numRooms)
	for i := range roomIDs {
		roomIDs[i] = rapid.StringMatching(`room_[a-z]{3,6}`).Draw(t, "room_id")
		// Ensure uniqueness
		for j := 0; j < i; j++ {
			if roomIDs[j] == roomIDs[i] {
				roomIDs[i] = roomIDs[i] + rapid.StringMatching(`[0-9]{2}`).Draw(t, "suffix")
			}
		}
	}

	rooms := make(map[string]*Room, numRooms)
	for i, id := range roomIDs {
		room := &Room{
			ID:          id,
			ZoneID:      "test_zone",
			Title:       "Room " + id,
			Description: "Description of " + id,
			Properties:  map[string]string{},
		}
		// Add a random exit to another room
		if numRooms > 1 {
			targetIdx := (i + 1) % numRooms
			dirIdx := rapid.IntRange(0, len(StandardDirections)-1).Draw(t, "dir_idx")
			room.Exits = append(room.Exits, Exit{
				Direction:  StandardDirections[dirIdx],
				TargetRoom: roomIDs[targetIdx],
			})
		}
		rooms[id] = room
	}

	return &Zone{
		ID:          "test_zone",
		Name:        "Test Zone",
		Description: "A test zone",
		StartRoom:   roomIDs[0],
		Rooms:       rooms,
	}
}

func validTestZone() *Zone {
	return &Zone{
		ID:          "test",
		Name:        "Test Zone",
		Description: "A test zone",
		StartRoom:   "room_a",
		Rooms: map[string]*Room{
			"room_a": {
				ID:          "room_a",
				ZoneID:      "test",
				Title:       "Room A",
				Description: "The first room.",
				Exits: []Exit{
					{Direction: North, TargetRoom: "room_b"},
				},
				Properties: map[string]string{},
			},
			"room_b": {
				ID:          "room_b",
				ZoneID:      "test",
				Title:       "Room B",
				Description: "The second room.",
				Exits: []Exit{
					{Direction: South, TargetRoom: "room_a"},
				},
				Properties: map[string]string{},
			},
		},
	}
}
