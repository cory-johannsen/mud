package session

import (
	"fmt"
	"sync"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/condition"
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
	// MaxHP is the character's maximum hit points.
	MaxHP int
	// Abilities holds the six ability scores loaded at login.
	Abilities character.AbilityScores
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
	// Experience is the character's total accumulated XP.
	Experience int
	// PendingBoosts is the number of ability boosts the player has not yet assigned.
	PendingBoosts int
	// DefaultCombatAction is the persisted default combat action; "pass" when unset.
	DefaultCombatAction string
	// LastCombatTarget is the last explicit attack/strike target; in-memory only.
	LastCombatTarget string
	// LoadoutSet holds the player's swappable weapon presets.
	LoadoutSet *inventory.LoadoutSet
	// Equipment holds the player's equipped armor and accessories.
	Equipment *inventory.Equipment
	// Entity is the bridge entity for pushing events to the player.
	Entity *BridgeEntity
	// Status is the player's current combat state.
	// Maps to gamev1.CombatStatus enum values: 0=Unspecified/Idle, 1=Idle, 2=InCombat, 3=Resting, 4=Unconscious.
	Status int32
	// AutomapCache holds discovered rooms keyed by zone ID then room ID.
	// Populated at login from the database; written through on each new discovery.
	AutomapCache map[string]map[string]bool
	// Skills maps skill_id to proficiency rank for the active character.
	// Populated after ensureSkills completes; empty map means all untrained.
	Skills map[string]string
	// Proficiencies maps proficiency category to rank for the active character.
	// Populated after backfill completes; empty map means all untrained.
	Proficiencies map[string]string
	// Conditions tracks active conditions applied outside of combat (e.g., from skill check effects).
	// Initialized at login; nil before Session() runs.
	Conditions *condition.ActiveSet
	// PassiveFeats holds the IDs of all passive class features and feats for this character.
	// Populated at login; used by combat passive checks without additional DB queries.
	PassiveFeats map[string]bool
	// FavoredTarget is the NPC type favored by the predators_eye class feature.
	// Populated after the generic feature-choice loop from
	// FeatureChoices["predators_eye"]["favored_target"].
	FavoredTarget string
	// FeatureChoices maps feature_id → choice_key → selected value.
	// Populated at login from character_feature_choices table.
	FeatureChoices map[string]map[string]string
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

// AddPlayerOptions holds all parameters for AddPlayer.
//
// Precondition: UID, Username, CharName, RoomID, and Role must be non-empty.
// Precondition: CharacterID must be >= 0.
// Precondition: CurrentHP and MaxHP must be >= 0.
// Postcondition: RegionDisplayName, Class, and Level are informational and may be zero values.
type AddPlayerOptions struct {
	UID                  string
	Username             string
	CharName             string
	CharacterID          int64
	RoomID               string
	CurrentHP            int
	MaxHP                int
	Abilities            character.AbilityScores
	Role                 string
	RegionDisplayName    string
	Class                string
	Level                int
	DefaultCombatAction  string
}

// AddPlayer registers a new player session in the given room.
//
// Precondition: opts.UID, opts.Username, opts.CharName, opts.RoomID, and opts.Role must be non-empty; opts.CharacterID must be >= 0; opts.CurrentHP and opts.MaxHP must be >= 0.
// Postcondition: Returns the created PlayerSession, or an error if the UID is already registered.
func (m *Manager) AddPlayer(opts AddPlayerOptions) (*PlayerSession, error) {
	uid := opts.UID
	username := opts.Username
	charName := opts.CharName
	characterID := opts.CharacterID
	roomID := opts.RoomID
	currentHP := opts.CurrentHP
	maxHP := opts.MaxHP
	abilities := opts.Abilities
	role := opts.Role
	regionDisplayName := opts.RegionDisplayName
	class := opts.Class
	level := opts.Level
	defaultCombatAction := opts.DefaultCombatAction
	if defaultCombatAction == "" {
		defaultCombatAction = "pass"
	}

	if uid == "" || username == "" || charName == "" || roomID == "" || role == "" {
		return nil, fmt.Errorf("AddPlayer: uid, username, charName, roomID, and role must be non-empty")
	}
	if characterID < 0 {
		return nil, fmt.Errorf("AddPlayer: characterID must be >= 0, got %d", characterID)
	}
	if currentHP < 0 || maxHP < 0 {
		return nil, fmt.Errorf("AddPlayer: currentHP and maxHP must be >= 0, got currentHP=%d maxHP=%d", currentHP, maxHP)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.players[uid]; exists {
		return nil, fmt.Errorf("player %q already connected", uid)
	}

	entity := NewBridgeEntity(uid, 64)
	sess := &PlayerSession{
		UID:                 uid,
		Username:            username,
		CharName:            charName,
		CharacterID:         characterID,
		RoomID:              roomID,
		CurrentHP:           currentHP,
		MaxHP:               maxHP,
		Abilities:           abilities,
		Role:                role,
		RegionDisplayName:   regionDisplayName,
		Class:               class,
		Level:               level,
		DefaultCombatAction: defaultCombatAction,
		Entity:              entity,
		// Status 1 = IDLE: newly connected players are idle by default.
		Status:         1,
		AutomapCache:   make(map[string]map[string]bool),
		FeatureChoices: make(map[string]map[string]string),
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

// PlayersInRoomDetails returns the full PlayerSession for each player in the given room.
//
// Precondition: roomID may be any string.
// Postcondition: Returns a non-nil slice (may be empty); each element is a non-nil *PlayerSession.
func (m *Manager) PlayersInRoomDetails(roomID string) []*PlayerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uids, ok := m.roomSets[roomID]
	if !ok {
		return []*PlayerSession{}
	}
	result := make([]*PlayerSession, 0, len(uids))
	for uid := range uids {
		if sess, ok := m.players[uid]; ok {
			result = append(result, sess)
		}
	}
	return result
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
