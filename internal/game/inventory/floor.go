package inventory

import "sync"

// FloorManager tracks item instances on the floor of rooms.
// It is thread-safe via sync.RWMutex.
type FloorManager struct {
	mu    sync.RWMutex
	rooms map[string][]ItemInstance
}

// NewFloorManager creates a FloorManager with no items on any floor.
//
// Postcondition: returned FloorManager is ready for use with zero items.
func NewFloorManager() *FloorManager {
	return &FloorManager{
		rooms: make(map[string][]ItemInstance),
	}
}

// Drop places an item instance on the floor of the given room.
//
// Precondition: roomID is non-empty; inst is a valid ItemInstance.
// Postcondition: inst is appended to the room's floor items.
func (fm *FloorManager) Drop(roomID string, inst ItemInstance) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.rooms[roomID] = append(fm.rooms[roomID], inst)
}

// Pickup removes and returns the item with the given instanceID from the room.
// Returns false if the item is not found.
//
// Precondition: roomID and instanceID are non-empty.
// Postcondition: on success, the item is removed from the room's floor and returned;
// on failure, room state is unchanged.
func (fm *FloorManager) Pickup(roomID, instanceID string) (ItemInstance, bool) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	items := fm.rooms[roomID]
	for i, inst := range items {
		if inst.InstanceID == instanceID {
			fm.rooms[roomID] = append(items[:i], items[i+1:]...)
			return inst, true
		}
	}
	return ItemInstance{}, false
}

// PickupAll removes and returns all items from the room's floor.
//
// Postcondition: the room's floor is empty; returned slice contains all previously held items.
func (fm *FloorManager) PickupAll(roomID string) []ItemInstance {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	items := fm.rooms[roomID]
	if len(items) == 0 {
		return []ItemInstance{}
	}
	delete(fm.rooms, roomID)
	return items
}

// ItemsInRoom returns a snapshot copy of all items on the floor of the given room.
//
// Postcondition: returned slice is a copy; mutations do not affect internal state.
func (fm *FloorManager) ItemsInRoom(roomID string) []ItemInstance {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	items := fm.rooms[roomID]
	out := make([]ItemInstance, len(items))
	copy(out, items)
	return out
}
