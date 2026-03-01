package session

import (
	"fmt"
	"sync"

	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// PlayerSession tracks a connected player's state.
type PlayerSession struct {
	// UID is the unique player identifier (character ID as string).
	UID string
	// Username is the account username (for logging).
	Username string
	// CharName is the character display name shown in-game.
	CharName string
	// CharacterID is the database ID of the character for persistence.
	CharacterID int64
	// RoomID is the current room the player occupies.
	RoomID string
	// CurrentHP is the character's current hit points.
	CurrentHP int
	// Backpack is the player's inventory container.
	Backpack *inventory.Backpack
	// Currency is the player's total rounds (ammunition-as-currency).
	Currency int
	// Role is the account privilege level (player, editor, admin).
	Role string
	// RegionDisplayName is the human-readable region display name (e.g. "the Northeast").
	RegionDisplayName string
	// Class is the character's job/class ID.
	Class string
	// Level is the character's current level.
	Level int
	// LoadoutSet holds the player's swappable weapon presets.
	LoadoutSet *inventory.LoadoutSet
	// Equipment holds the player's equipped armor and accessories.
	Equipment *inventory.Equipment
	// Entity is the bridge entity for pushing events to the player.
	Entity *BridgeEntity
}

// Manager tracks all active player sessions and room occupancy.
// All methods are safe for concurrent use.
type Manager struct {
	mu       sync.RWMutex
	players  map[string]*PlayerSession  // uid → session
	roomSets map[string]map[string]bool // roomID → set of UIDs
}

// NewManager creates an empty session Manager.
func NewManager() *Manager {
	return &Manager{
		players:  make(map[string]*PlayerSession),
		roomSets: make(map[string]map[string]bool),
	}
}

// AddPlayer registers a new player session in the given room.
//
// Precondition: uid, username, charName, and roomID must be non-empty; characterID must be >= 0; currentHP must be >= 0; role must be non-empty; regionDisplayName, class, and level are informational and may be zero values.
// Postcondition: Returns the created PlayerSession, or an error if the UID is already registered.
func (m *Manager) AddPlayer(uid, username, charName string, characterID int64, roomID string, currentHP int, role string, regionDisplayName string, class string, level int) (*PlayerSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.players[uid]; exists {
		return nil, fmt.Errorf("player %q already connected", uid)
	}

	entity := NewBridgeEntity(uid, 64)
	sess := &PlayerSession{
		UID:               uid,
		Username:          username,
		CharName:          charName,
		CharacterID:       characterID,
		RoomID:            roomID,
		CurrentHP:         currentHP,
		Role:              role,
		RegionDisplayName: regionDisplayName,
		Class:             class,
		Level:             level,
		Entity:            entity,
	}

	sess.Backpack = inventory.NewBackpack(20, 50.0)
	sess.Currency = 0
	sess.LoadoutSet = inventory.NewLoadoutSet()
	sess.Equipment = inventory.NewEquipment()

	m.players[uid] = sess
	if m.roomSets[roomID] == nil {
		m.roomSets[roomID] = make(map[string]bool)
	}
	m.roomSets[roomID][uid] = true

	return sess, nil
}

// RemovePlayer removes a player session and cleans up room occupancy.
//
// Precondition: uid must be non-empty.
// Postcondition: The player is removed from all tracking. Returns an error if not found.
func (m *Manager) RemovePlayer(uid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, exists := m.players[uid]
	if !exists {
		return fmt.Errorf("player %q not found", uid)
	}

	// Remove from room
	if rs, ok := m.roomSets[sess.RoomID]; ok {
		delete(rs, uid)
		if len(rs) == 0 {
			delete(m.roomSets, sess.RoomID)
		}
	}

	// Close entity
	_ = sess.Entity.Close()

	delete(m.players, uid)
	return nil
}

// MovePlayer moves a player from their current room to a new room.
//
// Precondition: uid and newRoomID must be non-empty.
// Postcondition: Returns the old room ID, or an error if the player is not found.
func (m *Manager) MovePlayer(uid, newRoomID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, exists := m.players[uid]
	if !exists {
		return "", fmt.Errorf("player %q not found", uid)
	}

	oldRoomID := sess.RoomID

	// Remove from old room
	if rs, ok := m.roomSets[oldRoomID]; ok {
		delete(rs, uid)
		if len(rs) == 0 {
			delete(m.roomSets, oldRoomID)
		}
	}

	// Add to new room
	sess.RoomID = newRoomID
	if m.roomSets[newRoomID] == nil {
		m.roomSets[newRoomID] = make(map[string]bool)
	}
	m.roomSets[newRoomID][uid] = true

	return oldRoomID, nil
}

// PlayersInRoom returns the character display names of all players in the given room.
//
// Postcondition: Returns a slice of character names (may be empty).
func (m *Manager) PlayersInRoom(roomID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uids, ok := m.roomSets[roomID]
	if !ok {
		return nil
	}

	names := make([]string, 0, len(uids))
	for uid := range uids {
		if sess, ok := m.players[uid]; ok {
			names = append(names, sess.CharName)
		}
	}
	return names
}

// PlayerUIDsInRoom returns the UIDs of all players in the given room.
//
// Postcondition: Returns a slice of UIDs (may be empty).
func (m *Manager) PlayerUIDsInRoom(roomID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uids, ok := m.roomSets[roomID]
	if !ok {
		return nil
	}

	result := make([]string, 0, len(uids))
	for uid := range uids {
		result = append(result, uid)
	}
	return result
}

// GetPlayer returns the session for the given UID.
//
// Postcondition: Returns (session, true) if found, or (nil, false) otherwise.
func (m *Manager) GetPlayer(uid string) (*PlayerSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sess, ok := m.players[uid]
	return sess, ok
}

// GetPlayerByCharName returns the session for the player with the given character name.
//
// Postcondition: Returns (session, true) if found, or (nil, false) otherwise.
func (m *Manager) GetPlayerByCharName(charName string) (*PlayerSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, sess := range m.players {
		if sess.CharName == charName {
			return sess, true
		}
	}
	return nil, false
}

// PlayerCount returns the total number of connected players.
func (m *Manager) PlayerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.players)
}
