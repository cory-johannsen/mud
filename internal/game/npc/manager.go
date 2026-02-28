package npc

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// Manager tracks all live NPC instances by ID and by room.
// All methods are safe for concurrent use.
type Manager struct {
	mu        sync.RWMutex
	instances map[string]*Instance       // instanceID → Instance
	roomSets  map[string]map[string]bool // roomID → set of instanceIDs
	counter   atomic.Uint64
}

// NewManager creates an empty NPC Manager.
func NewManager() *Manager {
	return &Manager{
		instances: make(map[string]*Instance),
		roomSets:  make(map[string]map[string]bool),
	}
}

// Spawn creates a new Instance from tmpl and places it in roomID.
// If multiple instances of the same template occupy the room, each is assigned
// a unique uppercase letter suffix (A, B, C, …). A single instance has no suffix.
//
// Precondition: tmpl must be non-nil; roomID must be non-empty.
// Postcondition: Returns a new Instance with a unique ID registered in roomID.
//
//	Existing instances of the same template in the room may be renamed.
func (m *Manager) Spawn(tmpl *Template, roomID string) (*Instance, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("npc.Manager.Spawn: tmpl must not be nil")
	}
	if roomID == "" {
		return nil, fmt.Errorf("npc.Manager.Spawn: roomID must not be empty")
	}

	n := m.counter.Add(1)
	id := fmt.Sprintf("%s-%s-%d", tmpl.ID, roomID, n)
	inst := NewInstance(id, tmpl, roomID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// Count existing live instances of the same template in this room.
	var sameTemplate []*Instance
	if ids, ok := m.roomSets[roomID]; ok {
		for existingID := range ids {
			if existing, ok := m.instances[existingID]; ok && existing.TemplateID == tmpl.ID {
				sameTemplate = append(sameTemplate, existing)
			}
		}
	}

	count := len(sameTemplate)
	switch count {
	case 0:
		// Single instance — no suffix.
	case 1:
		// Second instance arriving: rename the first to A, assign B to new.
		sameTemplate[0].Name = tmpl.Name + " A"
		inst.Name = tmpl.Name + " B"
	default:
		// Third or beyond: existing instances already have suffixes.
		inst.Name = fmt.Sprintf("%s %c", tmpl.Name, 'A'+rune(count))
	}

	m.instances[id] = inst
	if m.roomSets[roomID] == nil {
		m.roomSets[roomID] = make(map[string]bool)
	}
	m.roomSets[roomID][id] = true

	return inst, nil
}

// Remove deletes an instance by ID.
//
// Precondition: id must be non-empty.
// Postcondition: Returns an error if the instance is not found.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("npc instance %q not found", id)
	}

	if rs, ok := m.roomSets[inst.RoomID]; ok {
		delete(rs, id)
		if len(rs) == 0 {
			delete(m.roomSets, inst.RoomID)
		}
	}
	delete(m.instances, id)
	return nil
}

// Get returns the instance with the given ID.
//
// Postcondition: Returns (inst, true) if found, or (nil, false) otherwise.
func (m *Manager) Get(id string) (*Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.instances[id]
	return inst, ok
}

// InstancesInRoom returns a snapshot of all live instances in roomID.
//
// Postcondition: Returns a non-nil slice (may be empty).
func (m *Manager) InstancesInRoom(roomID string) []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids, ok := m.roomSets[roomID]
	if !ok {
		return []*Instance{}
	}

	out := make([]*Instance, 0, len(ids))
	for id := range ids {
		if inst, ok := m.instances[id]; ok {
			out = append(out, inst)
		}
	}
	return out
}

// Move relocates an instance from its current room to newRoomID.
//
// Precondition: id must identify an existing instance; newRoomID must be non-empty.
// Postcondition: instance.RoomID equals newRoomID; room index is updated accordingly.
func (m *Manager) Move(id, newRoomID string) error {
	if newRoomID == "" {
		return fmt.Errorf("npc.Manager.Move: newRoomID must not be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	inst, ok := m.instances[id]
	if !ok {
		return fmt.Errorf("npc.Manager.Move: instance %q not found", id)
	}

	oldRoomID := inst.RoomID
	if rs, ok := m.roomSets[oldRoomID]; ok {
		delete(rs, id)
		if len(rs) == 0 {
			delete(m.roomSets, oldRoomID)
		}
	}

	inst.RoomID = newRoomID
	if m.roomSets[newRoomID] == nil {
		m.roomSets[newRoomID] = make(map[string]bool)
	}
	m.roomSets[newRoomID][id] = true

	return nil
}

// FindInRoom returns the first instance in roomID whose Name has target as a
// case-insensitive prefix. Returns nil if no match is found.
//
// Precondition: roomID and target must be non-empty for meaningful results.
func (m *Manager) FindInRoom(roomID, target string) *Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids, ok := m.roomSets[roomID]
	if !ok {
		return nil
	}

	lower := strings.ToLower(target)
	for id := range ids {
		inst, ok := m.instances[id]
		if !ok {
			continue
		}
		if strings.HasPrefix(strings.ToLower(inst.Name), lower) {
			return inst
		}
	}
	return nil
}
