package trap

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// TrapKindConsumable is the instance key kind for player-deployed consumable traps.
const TrapKindConsumable = "consumable"

// TrapInstanceState holds runtime state for one trap instance.
type TrapInstanceState struct {
	// TemplateID references the TrapTemplate for this instance.
	TemplateID string
	// Armed is true when the trap will fire on its trigger condition.
	Armed bool
	// BeingDisarmed is true when a disarm attempt is in flight.
	// Auto re-arm is deferred while BeingDisarmed is true.
	BeingDisarmed bool
	// ResetAt is set for auto reset mode; nil otherwise.
	ResetAt *time.Time
	// DeployPosition is the combat position in feet at deploy time; 0 for world-placed traps.
	DeployPosition int
	// IsConsumable is true for player-deployed traps; enforces one-shot semantics.
	IsConsumable bool
}

// TrapManager tracks all trap instance states and per-player detection records.
//
// All methods are safe for concurrent use.
type TrapManager struct {
	mu       sync.RWMutex
	traps    map[string]*TrapInstanceState // keyed by trapInstanceID
	sessions map[string]map[string]bool    // playerUID → set of detected trapInstanceIDs
}

// NewTrapManager returns an initialised, empty TrapManager.
//
// Postcondition: The returned manager is ready to accept AddTrap/MarkDetected calls.
func NewTrapManager() *TrapManager {
	return &TrapManager{
		traps:    make(map[string]*TrapInstanceState),
		sessions: make(map[string]map[string]bool),
	}
}

// trapInstanceID constructs the stable key for a trap instance.
//
// Precondition: zoneID, roomID, kind, and id must all be non-empty.
// Panics if any component is empty.
func trapInstanceID(zoneID, roomID, kind, id string) string {
	if zoneID == "" || roomID == "" || kind == "" || id == "" {
		panic(fmt.Sprintf("trapInstanceID: all components must be non-empty; got zoneID=%q roomID=%q kind=%q id=%q", zoneID, roomID, kind, id))
	}
	return zoneID + "/" + roomID + "/" + kind + "/" + id
}

// TrapInstanceID is the public helper used by the gameserver to construct instance keys.
//
// Precondition: zoneID, roomID, kind, and id must all be non-empty.
// kind MUST be "room" for room-level traps or "equip" for equipment-level traps.
func TrapInstanceID(zoneID, roomID, kind, id string) string {
	return trapInstanceID(zoneID, roomID, kind, id)
}

// AddTrap registers a new trap instance.
//
// Precondition: instanceID must be non-empty; templateID must reference a loaded TrapTemplate.
// Postcondition: The instance is stored with Armed = armed.
func (m *TrapManager) AddTrap(instanceID, templateID string, armed bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.traps[instanceID] = &TrapInstanceState{
		TemplateID: templateID,
		Armed:      armed,
	}
}

// GetTrap returns a value copy of the instance state for the given instanceID.
//
// Postcondition: Returns (state, true) if found; (zero-value, false) otherwise.
// The returned value is a snapshot; mutations to it do not affect stored state.
func (m *TrapManager) GetTrap(instanceID string) (TrapInstanceState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.traps[instanceID]
	if !ok {
		return TrapInstanceState{}, false
	}
	return *s, true
}

// Disarm marks a trap as disarmed (Armed=false) regardless of reset mode.
// One-shot lifecycle removal is handled separately by the PlaceTraps reset flow.
//
// Precondition: instanceID should reference an existing trap (no-op if not found).
// Postcondition: Armed is false; BeingDisarmed is cleared. The trap entry remains in the map.
func (m *TrapManager) Disarm(instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok := m.traps[instanceID]
	if !ok {
		return
	}
	state.Armed = false
	state.BeingDisarmed = false
}

// MarkDetected flags a trap instance as detected for a player.
//
// Precondition: playerUID and instanceID must be non-empty.
// Postcondition: IsDetected(playerUID, instanceID) returns true.
func (m *TrapManager) MarkDetected(playerUID, instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessions[playerUID] == nil {
		m.sessions[playerUID] = make(map[string]bool)
	}
	m.sessions[playerUID][instanceID] = true
}

// IsDetected reports whether playerUID has detected the given trap instance in this session.
//
// Postcondition: Returns true iff MarkDetected was previously called with these arguments.
func (m *TrapManager) IsDetected(playerUID, instanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[playerUID][instanceID]
}

// TrapsForRoom returns all trap instance IDs whose key begins with "zoneID/roomID/".
// Used by checkEntryTraps and detectTrapsInRoom to enumerate both room-level and
// equipment-level traps in a single pass without exposing the internal map.
//
// Precondition: zoneID and roomID must be non-empty.
// Postcondition: Returns a slice of instance IDs (may be empty); caller must not modify the states.
func (m *TrapManager) TrapsForRoom(zoneID, roomID string) []string {
	prefix := zoneID + "/" + roomID + "/"
	m.mu.RLock()
	defer m.mu.RUnlock()
	var ids []string
	for id := range m.traps {
		if strings.HasPrefix(id, prefix) {
			ids = append(ids, id)
		}
	}
	return ids
}

// ClearDetectionForRoom removes all detection records for the given room trap instance IDs
// for all players. Used on room reset to satisfy REQ-TR-12.
//
// Precondition: roomInstanceIDs contains all trap instance IDs that belong to the resetting room.
// Postcondition: No player will have detection state for any of the given instance IDs.
func (m *TrapManager) ClearDetectionForRoom(roomInstanceIDs []string) {
	if len(roomInstanceIDs) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for uid, detected := range m.sessions {
		for _, iid := range roomInstanceIDs {
			delete(detected, iid)
		}
		if len(detected) == 0 {
			delete(m.sessions, uid)
		}
	}
}

// AddConsumableTrap arms a player-deployed consumable trap at the given deploy position.
//
// Precondition: instanceID must be unique within this TrapManager; tmpl must be non-nil.
// Postcondition: GetTrap(instanceID) returns a state with Armed=true, IsConsumable=true, DeployPosition=deployPos.
// Returns an error if instanceID already exists.
func (m *TrapManager) AddConsumableTrap(instanceID string, tmpl *TrapTemplate, deployPos int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.traps[instanceID]; exists {
		return fmt.Errorf("trap instance %q already exists", instanceID)
	}
	m.traps[instanceID] = &TrapInstanceState{
		TemplateID:     tmpl.ID,
		Armed:          true,
		DeployPosition: deployPos,
		IsConsumable:   true,
	}
	return nil
}
