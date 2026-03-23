package behavior_test

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/npc/behavior"
	"github.com/cory-johannsen/mud/internal/game/world"
)

func TestBFSDistanceMap_LinearChain(t *testing.T) {
	// A -> B -> C
	rooms := []*world.Room{
		{ID: "A", Exits: []world.Exit{{TargetRoom: "B"}}},
		{ID: "B", Exits: []world.Exit{{TargetRoom: "A"}, {TargetRoom: "C"}}},
		{ID: "C", Exits: []world.Exit{{TargetRoom: "B"}}},
	}
	dm, err := behavior.BFSDistanceMap(rooms, "A")
	if err != nil {
		t.Fatal(err)
	}
	if dm["A"] != 0 {
		t.Errorf("A distance expected 0, got %d", dm["A"])
	}
	if dm["B"] != 1 {
		t.Errorf("B distance expected 1, got %d", dm["B"])
	}
	if dm["C"] != 2 {
		t.Errorf("C distance expected 2, got %d", dm["C"])
	}
}

func TestBFSDistanceMap_OriginNotInRooms_ReturnsError(t *testing.T) {
	rooms := []*world.Room{
		{ID: "A", Exits: []world.Exit{{TargetRoom: "B"}}},
	}
	_, err := behavior.BFSDistanceMap(rooms, "X")
	if err == nil {
		t.Fatal("expected error for origin not in rooms")
	}
}

func TestBFSDistanceMap_Disconnected_NotReachable(t *testing.T) {
	rooms := []*world.Room{
		{ID: "A"},
		{ID: "B"},
	}
	dm, err := behavior.BFSDistanceMap(rooms, "A")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := dm["B"]; ok {
		t.Error("disconnected room B should not be in distance map")
	}
}

func TestProperty_BFSDistanceMap_OriginAlwaysZero(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Build a simple linear chain of n rooms
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		rooms := make([]*world.Room, n)
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("room%d", i)
			var exits []world.Exit
			if i+1 < n {
				exits = append(exits, world.Exit{TargetRoom: fmt.Sprintf("room%d", i+1)})
			}
			rooms[i] = &world.Room{ID: id, Exits: exits}
		}
		dm, err := behavior.BFSDistanceMap(rooms, "room0")
		if err != nil {
			rt.Fatal(err)
		}
		if dm["room0"] != 0 {
			rt.Fatalf("origin must have distance 0, got %d", dm["room0"])
		}
	})
}
