package inventory_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"pgregory.net/rapid"
)

func TestFloorManager_Drop_And_ItemsInRoom(t *testing.T) {
	fm := inventory.NewFloorManager()
	inst := inventory.ItemInstance{InstanceID: "i1", ItemDefID: "sword", Quantity: 1}

	fm.Drop("room1", inst)

	items := fm.ItemsInRoom("room1")
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].InstanceID != "i1" {
		t.Fatalf("expected instance i1, got %s", items[0].InstanceID)
	}

	// Verify snapshot isolation: mutating returned slice must not affect internal state.
	items[0].InstanceID = "mutated"
	items2 := fm.ItemsInRoom("room1")
	if items2[0].InstanceID != "i1" {
		t.Fatalf("ItemsInRoom must return a copy; internal state was mutated")
	}
}

func TestFloorManager_Pickup_RemovesItem(t *testing.T) {
	fm := inventory.NewFloorManager()
	fm.Drop("room1", inventory.ItemInstance{InstanceID: "i1", ItemDefID: "sword", Quantity: 1})
	fm.Drop("room1", inventory.ItemInstance{InstanceID: "i2", ItemDefID: "shield", Quantity: 1})

	got, ok := fm.Pickup("room1", "i1")
	if !ok {
		t.Fatal("expected Pickup to succeed")
	}
	if got.InstanceID != "i1" {
		t.Fatalf("expected i1, got %s", got.InstanceID)
	}

	remaining := fm.ItemsInRoom("room1")
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(remaining))
	}
	if remaining[0].InstanceID != "i2" {
		t.Fatalf("expected i2 remaining, got %s", remaining[0].InstanceID)
	}
}

func TestFloorManager_Pickup_NotFound(t *testing.T) {
	fm := inventory.NewFloorManager()

	// Empty room.
	_, ok := fm.Pickup("room1", "nope")
	if ok {
		t.Fatal("expected Pickup on empty room to return false")
	}

	// Room with items but wrong ID.
	fm.Drop("room1", inventory.ItemInstance{InstanceID: "i1", ItemDefID: "sword", Quantity: 1})
	_, ok = fm.Pickup("room1", "nope")
	if ok {
		t.Fatal("expected Pickup with wrong ID to return false")
	}
}

func TestFloorManager_PickupAll_ReturnsAndClears(t *testing.T) {
	fm := inventory.NewFloorManager()
	fm.Drop("room1", inventory.ItemInstance{InstanceID: "i1", ItemDefID: "sword", Quantity: 1})
	fm.Drop("room1", inventory.ItemInstance{InstanceID: "i2", ItemDefID: "shield", Quantity: 1})

	all := fm.PickupAll("room1")
	if len(all) != 2 {
		t.Fatalf("expected 2 items, got %d", len(all))
	}

	remaining := fm.ItemsInRoom("room1")
	if len(remaining) != 0 {
		t.Fatalf("expected 0 items after PickupAll, got %d", len(remaining))
	}
}

func TestFloorManager_EmptyRoom(t *testing.T) {
	fm := inventory.NewFloorManager()

	items := fm.ItemsInRoom("nonexistent")
	if items == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}

	all := fm.PickupAll("nonexistent")
	if len(all) != 0 {
		t.Fatalf("expected 0 items from PickupAll on empty room, got %d", len(all))
	}
}

func TestProperty_FloorManager_DropPickup_Roundtrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		fm := inventory.NewFloorManager()
		roomID := rapid.StringMatching(`^room[0-9]{1,3}$`).Draw(t, "roomID")

		n := rapid.IntRange(1, 20).Draw(t, "n")
		var ids []string
		for i := 0; i < n; i++ {
			id := rapid.StringMatching(`^inst[0-9]{1,6}$`).Draw(t, "id")
			ids = append(ids, id)
			fm.Drop(roomID, inventory.ItemInstance{
				InstanceID: id,
				ItemDefID:  "def",
				Quantity:   rapid.IntRange(1, 100).Draw(t, "qty"),
			})
		}

		items := fm.ItemsInRoom(roomID)
		if len(items) != n {
			t.Fatalf("expected %d items, got %d", n, len(items))
		}

		// Pick up a random item and verify it's removed.
		pickIdx := rapid.IntRange(0, n-1).Draw(t, "pickIdx")
		pickID := ids[pickIdx]
		got, ok := fm.Pickup(roomID, pickID)
		if !ok {
			t.Fatalf("expected Pickup(%q) to succeed", pickID)
		}
		if got.InstanceID != pickID {
			t.Fatalf("expected %q, got %q", pickID, got.InstanceID)
		}

		after := fm.ItemsInRoom(roomID)
		if len(after) != n-1 {
			t.Fatalf("expected %d items after pickup, got %d", n-1, len(after))
		}
	})
}
