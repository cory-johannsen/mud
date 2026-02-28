package world

import (
	"fmt"
	"sync"
)

// Manager provides thread-safe access to the loaded world state.
// It indexes rooms across all zones for O(1) lookup by room ID.
type Manager struct {
	mu        sync.RWMutex
	zones     map[string]*Zone
	rooms     map[string]*Room
	startRoom string
}

// NewManager creates a Manager from the given zones.
//
// Precondition: zones must contain at least one zone; the first zone's start room is the global start room.
// Postcondition: Returns a Manager with all rooms indexed by ID, or an error on duplicate room IDs.
func NewManager(zones []*Zone) (*Manager, error) {
	m := &Manager{
		zones: make(map[string]*Zone, len(zones)),
		rooms: make(map[string]*Room),
	}

	for _, z := range zones {
		if _, exists := m.zones[z.ID]; exists {
			return nil, fmt.Errorf("duplicate zone ID: %q", z.ID)
		}
		m.zones[z.ID] = z
		for id, room := range z.Rooms {
			if existing, exists := m.rooms[id]; exists {
				return nil, fmt.Errorf("duplicate room ID %q: in zone %q and %q", id, existing.ZoneID, z.ID)
			}
			m.rooms[id] = room
		}
	}

	if len(zones) > 0 {
		m.startRoom = zones[0].StartRoom
	}

	return m, nil
}

// ValidateExits checks that every exit target in every room resolves to a
// known room across all loaded zones. Call this after NewManager to catch
// dangling cross-zone exit references.
//
// Precondition: Manager must be fully constructed with all zones loaded.
// Postcondition: Returns nil if all exits resolve, or an error listing the first dangling target.
func (m *Manager) ValidateExits() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, zone := range m.zones {
		for _, room := range zone.Rooms {
			for _, exit := range room.Exits {
				if _, ok := m.rooms[exit.TargetRoom]; !ok {
					return fmt.Errorf("zone %q: room %q: exit %q targets unknown room %q",
						zone.ID, room.ID, exit.Direction, exit.TargetRoom)
				}
			}
		}
	}
	return nil
}

// GetRoom returns the room with the given ID.
//
// Postcondition: Returns (room, true) if found, or (nil, false) otherwise.
func (m *Manager) GetRoom(id string) (*Room, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.rooms[id]
	return r, ok
}

// Navigate resolves movement from a room in a direction.
//
// Precondition: fromRoomID must exist in the world.
// Postcondition: Returns the destination room, or an error if the exit
// doesn't exist, is locked, or the target room is missing.
func (m *Manager) Navigate(fromRoomID string, dir Direction) (*Room, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	from, ok := m.rooms[fromRoomID]
	if !ok {
		return nil, fmt.Errorf("room %q not found", fromRoomID)
	}

	exit, ok := from.ExitForDirection(dir)
	if !ok {
		return nil, fmt.Errorf("no exit %q from %q", dir, fromRoomID)
	}

	if exit.Locked {
		return nil, fmt.Errorf("the way %s is locked", dir)
	}

	target, ok := m.rooms[exit.TargetRoom]
	if !ok {
		return nil, fmt.Errorf("exit %q from %q targets unknown room %q", dir, fromRoomID, exit.TargetRoom)
	}

	return target, nil
}

// StartRoom returns the global start room.
//
// Postcondition: Returns the start room or nil if the world is empty.
func (m *Manager) StartRoom() *Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.startRoom == "" {
		return nil
	}
	return m.rooms[m.startRoom]
}

// RoomCount returns the total number of rooms across all zones.
func (m *Manager) RoomCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}

// ZoneCount returns the number of loaded zones.
func (m *Manager) ZoneCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.zones)
}

// AllZones returns all loaded zones.
//
// Postcondition: Returns a non-nil slice; may be empty.
func (m *Manager) AllZones() []*Zone {
	m.mu.RLock()
	defer m.mu.RUnlock()
	zones := make([]*Zone, 0, len(m.zones))
	for _, z := range m.zones {
		zones = append(zones, z)
	}
	return zones
}
