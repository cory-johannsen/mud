package npc_test

import (
	"testing"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/world"
)

func TestNewPopulatedRespawnManager_HomeRoomBFSPopulated(t *testing.T) {
	// Build a minimal zone with 2 rooms; NPC spawned in room_a with home_room room_a.
	zone := &world.Zone{
		ID: "test_zone",
		Rooms: map[string]*world.Room{
			"room_a": {
				ID: "room_a",
				Exits: []world.Exit{{TargetRoom: "room_b"}},
				Spawns: []world.RoomSpawnConfig{{Template: "guard", Count: 1}},
			},
			"room_b": {
				ID:    "room_b",
				Exits: []world.Exit{{TargetRoom: "room_a"}},
			},
		},
	}
	worldMgr, err := world.NewManager([]*world.Zone{zone})
	if err != nil {
		t.Fatal(err)
	}

	templates := []*npc.Template{
		{
			ID:       "guard",
			Name:     "Guard",
			Level:    1,
			MaxHP:    10,
			AC:       10,
			HomeRoom: "room_a",
		},
	}
	// Validate to set CourageThreshold default.
	for _, tmpl := range templates {
		if err := tmpl.Validate(); err != nil {
			t.Fatal(err)
		}
	}

	npcMgr := npc.NewWiredManager(&inventory.Registry{})
	logger, _ := zap.NewDevelopment()

	respawnMgr, err := npc.NewPopulatedRespawnManager(templates, worldMgr, npcMgr, logger)
	if err != nil {
		t.Fatal(err)
	}
	_ = respawnMgr

	instances := npcMgr.AllInstances()
	if len(instances) == 0 {
		t.Fatal("expected at least one NPC instance")
	}

	for _, inst := range instances {
		if inst.HomeRoomBFS == nil {
			t.Errorf("NPC %q HomeRoomBFS is nil", inst.ID)
			continue
		}
		d, ok := inst.HomeRoomBFS["room_a"]
		if !ok || d != 0 {
			t.Errorf("NPC %q HomeRoomBFS[room_a] = %d (ok=%v), want 0", inst.ID, d, ok)
		}
		d, ok = inst.HomeRoomBFS["room_b"]
		if !ok || d != 1 {
			t.Errorf("NPC %q HomeRoomBFS[room_b] = %d (ok=%v), want 1", inst.ID, d, ok)
		}
	}
}
