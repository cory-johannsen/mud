package session

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/google/uuid"
)

// ExploreModeCaseIt is the Search exploration mode that enables trap detection.
const ExploreModeCaseIt = "case_it"

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
	// PendingSkillIncreases is the number of skill rank increases the player has not yet assigned.
	PendingSkillIncreases int
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
	// ActiveFeatUses tracks remaining daily uses for active feats that have a PreparedUses limit.
	// Key: feat ID. Value: uses remaining. Only feats with PreparedUses > 0 appear here.
	// 0 remaining means the feat is exhausted. Restored to PreparedUses on long rest.
	// Populated at login; nil means no limited active feats.
	ActiveFeatUses map[string]int
	// FavoredTargetMu guards FavoredTarget against concurrent reads during
	// session initialization (written by Session() during setup; read by combat).
	FavoredTargetMu sync.RWMutex
	// FavoredTarget is the NPC type favored by the predators_eye class feature.
	// Populated after the generic feature-choice loop from
	// FeatureChoices["predators_eye"]["favored_target"].
	// All reads and writes must hold FavoredTargetMu.
	FavoredTarget string
	// FeatureChoices maps feature_id → choice_key → selected value.
	// Populated at login from character_feature_choices table.
	FeatureChoices map[string]map[string]string
	// Resistances maps damage type → effective flat reduction from equipped armor (highest per type).
	// Populated from ComputedDefenses at login and after equip/unequip.
	Resistances map[string]int
	// Weaknesses maps damage type → total flat addition from equipped armor (additive).
	// Populated from ComputedDefenses at login and after equip/unequip.
	Weaknesses map[string]int
	// GrabberID is the NPC instance ID of the NPC currently grappling this player.
	// Empty string when the player is not grabbed. Set by NPC grapple; cleared by escape.
	GrabberID string
	// Gender is the character's gender identity string, loaded at login.
	Gender string
	// HeroPoints is the number of hero points available (persisted).
	HeroPoints int
	// LastCheckRoll is the dice result of the most recent ability check (session-only; 0 = none recorded).
	LastCheckRoll int
	// LastCheckDC is the DC of the most recent ability check (session-only).
	LastCheckDC int
	// LastCheckName is the display name of the most recent check (session-only).
	LastCheckName string
	// Dead is true when the character is dying and eligible for stabilize (session-only).
	Dead bool
	// BankedAP is AP banked from a delay action; added to the next round's AP pool; session-only, not persisted.
	BankedAP int
	// PendingCombatJoin holds the RoomID of a combat the player has been invited to join.
	// Empty string means no pending join offer. Protected by combatMu in the gameserver.
	PendingCombatJoin string
	// GroupID is the ID of the group this player belongs to.
	// Empty string means not in a group. Protected by Manager.mu.
	GroupID string
	// PendingGroupInvite holds the groupID of a pending group invitation.
	// Empty string means no pending invite. Protected by Manager.mu.
	PendingGroupInvite string
	// ExploreMode is the current exploration mode (see ExploreModeCaseIt).
	// "" means not in explore mode; ExploreModeCaseIt means Search mode (trap detection active).
	// Set by the exploration feature.
	ExploreMode string
	// Region is the character's home region ID (e.g. "lake_oswego").
	// Set at login from dbChar.Region. Used for Honkeypot targeting.
	Region string
	// ZoneEffectCooldowns maps "roomID:track" to an immunity value.
	// In combat: value is rounds remaining (decremented each round; 0 = ready to fire).
	// Out of combat: value is Unix timestamp (seconds) of expiry; 0 = ready to fire.
	// Nil until first use; initialized lazily on first write.
	ZoneEffectCooldowns map[string]int64
	// WantedLevel maps zone_id to the player's current wanted level (0–4) in that zone.
	// Initialized at session creation; 0 means no wanted status.
	WantedLevel map[string]int
	// SafeViolations maps zone_id to the number of safe-zone violations in the current
	// WantedLevel cycle for that zone.
	SafeViolations map[string]int
	// LastViolationDay maps zone_id to the in-game day on which the last violation occurred.
	// Used for decay calculations.
	LastViolationDay map[string]int
	// Technology slots — nil until loaded from DB at login.
	HardwiredTechs   []string                // tech IDs; unlimited use
	PreparedTechs    map[int][]*PreparedSlot // slot level → ordered slots
	SpontaneousTechs map[int][]string        // tech level → known tech IDs
	InnateTechs      map[string]*InnateSlot  // tech_id → innate slot info
	// SpontaneousUsePools tracks daily use pools per tech level.
	// Key: tech level (1-based). Value: UsePool with remaining and max uses.
	SpontaneousUsePools map[int]UsePool
	// PendingTechGrants maps character level to the technology grants that require
	// interactive player selection (pool > open slots). Populated at level-up;
	// cleared by ResolvePendingTechGrants.
	PendingTechGrants map[int]*ruleset.TechnologyGrants
	// ReactionsRemaining is the number of reactions available this round. Resets to 1 each round.
	// Not persisted. In-session only.
	ReactionsRemaining int
	// Reactions holds all registered reactions for this player, indexed by trigger type.
	// Initialised to NewReactionRegistry() at session creation.
	Reactions *reaction.ReactionRegistry
	// ReactionFn is the per-session reaction callback, set by the session handler after login.
	// Captures the player's gRPC stream for interactive prompting.
	ReactionFn reaction.ReactionCallback
	// ReadiedTrigger is the trigger alias this player is waiting for ("enemy_enters",
	// "enemy_attacks_me", "ally_attacked"). Empty string means no readied action this round.
	// Not persisted. In-session only.
	ReadiedTrigger string
	// ReadiedAction is the action to execute when ReadiedTrigger fires
	// ("strike", "step", "raise_shield"). Empty when ReadiedTrigger is empty.
	// Not persisted. In-session only.
	ReadiedAction string
	// NegotiateModifier is the session-scoped price modifier earned from a negotiate
	// attempt with a merchant. +0.2 = 20% cheaper; -0.1 = 10% penalty on buys.
	// Cleared on room transition (REQ-NPC-5a). 0.0 means no modifier active.
	NegotiateModifier float64
	// NegotiatedMerchantID is the instance ID of the merchant this player already
	// negotiated with in the current room visit. Blocks repeat negotiate. (REQ-NPC-5)
	NegotiatedMerchantID string
	// StashBalance is the player's global stash credit balance, accessible at any banker.
	StashBalance int
	// PendingBribeNPCName is the name of the NPC this player has initiated a bribe with.
	// Empty string means no pending bribe. Cleared on bribe confirm or any other command.
	PendingBribeNPCName string
	// PendingBribeAmount is the credit cost of the pending bribe.
	// Only valid when PendingBribeNPCName is non-empty.
	PendingBribeAmount int
	// DetainedUntil is non-nil when the player is serving a detention sentence.
	// nil means not detained; non-nil is the wall-clock expiry time.
	// REQ-WC-14a: persisted to DB.
	DetainedUntil *time.Time
	// DetentionGraceUntil is the 5-second window after detention completes during
	// which guard re-evaluation is suppressed (REQ-WC-14c).
	DetentionGraceUntil time.Time
	// Jobs maps job_id to the player's current level in that job.
	// Initialized to an empty map at session creation.
	// REQ-NPC-9: players acquire jobs via training; REQ-NPC-11: all held jobs grant feats/proficiencies.
	Jobs map[string]int
	// ActiveJobID is the currently active job that earns XP (REQ-NPC-10).
	// Empty string means no active job. Set to the first trained job automatically (REQ-NPC-9).
	ActiveJobID string
	// Team is the player's faction or team affiliation, loaded from characters.team DB column.
	// Empty string means no team affiliation (REQ-EM-39).
	Team string
	// SetBonusSummary is the aggregated active equipment set bonuses (REQ-EM-29).
	// Recomputed at login and whenever equipped armor changes.
	// Zero value (empty summary) is valid and means no active set bonuses.
	SetBonusSummary inventory.SetBonusSummary
	// InitDone is closed by Session() immediately before entering commandLoop,
	// signalling that all session-initialization writes to PlayerSession fields
	// are complete. Consumers (e.g. tests) that need a race-free snapshot of
	// any session field MUST receive from this channel before reading.
	// The channel is created by NewPlayerSession; it must never be written after close.
	InitDone chan struct{}
}

// RecomputeSetBonuses recomputes and stores the active set bonuses for sess
// using equippedItemDefIDs gathered from the session's current equipment (REQ-EM-29).
// Must be called at login and whenever equipped armor changes.
//
// Precondition: sess must not be nil; setReg may be nil (treated as no set bonuses).
// Postcondition: sess.SetBonusSummary is updated.
func RecomputeSetBonuses(sess *PlayerSession, setReg *inventory.SetRegistry) {
	if setReg == nil || sess == nil {
		return
	}
	ids := make([]string, 0, 8)
	// Collect armor slot item def IDs.
	for _, slotted := range sess.Equipment.Armor {
		if slotted != nil && slotted.ItemDefID != "" {
			ids = append(ids, slotted.ItemDefID)
		}
	}
	// Collect weapon item def IDs from active loadout preset.
	if sess.LoadoutSet != nil && int(sess.LoadoutSet.Active) < len(sess.LoadoutSet.Presets) {
		active := sess.LoadoutSet.Presets[sess.LoadoutSet.Active]
		if active != nil {
			if active.MainHand != nil && active.MainHand.Def != nil {
				ids = append(ids, active.MainHand.Def.ID)
			}
			if active.OffHand != nil && active.OffHand.Def != nil {
				ids = append(ids, active.OffHand.Def.ID)
			}
		}
	}
	bonuses := setReg.ActiveBonuses(ids)
	sess.SetBonusSummary = inventory.ComputeSetBonusSummary(bonuses)
}

// Manager tracks all active player sessions and room occupancy.
// All methods are safe for concurrent use.
type Manager struct {
	mu       sync.RWMutex
	players  map[string]*PlayerSession  // uid → session
	roomSets map[string]map[string]bool // roomID → set of UIDs
	groups   map[string]*Group          // groupID → group
}

// NewManager creates an empty session Manager.
func NewManager() *Manager {
	return &Manager{
		players:  make(map[string]*PlayerSession),
		roomSets: make(map[string]map[string]bool),
		groups:   make(map[string]*Group),
	}
}

// AddPlayerOptions holds all parameters for AddPlayer.
//
// Precondition: UID, Username, CharName, RoomID, and Role must be non-empty.
// Precondition: CharacterID must be >= 0.
// Precondition: CurrentHP and MaxHP must be >= 0.
// Postcondition: RegionDisplayName, Class, and Level are informational and may be zero values.
type AddPlayerOptions struct {
	UID                 string
	Username            string
	CharName            string
	CharacterID         int64
	RoomID              string
	CurrentHP           int
	MaxHP               int
	Abilities           character.AbilityScores
	Role                string
	RegionDisplayName   string
	Class               string
	Level               int
	DefaultCombatAction string
	Gender              string
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

	entity := NewBridgeEntity(uid, 256)
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
		Gender:              opts.Gender,
		Entity:              entity,
		// Status 1 = IDLE: newly connected players are idle by default.
		Status:             1,
		AutomapCache:       make(map[string]map[string]bool),
		FeatureChoices:     make(map[string]map[string]string),
		ReactionsRemaining: 1,
		Reactions:          reaction.NewReactionRegistry(),
		WantedLevel:        make(map[string]int),
		SafeViolations:     make(map[string]int),
		LastViolationDay:   make(map[string]int),
		Jobs:               make(map[string]int),
		InitDone:           make(chan struct{}),
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

// GetPlayerByCharNameCI does a case-insensitive lookup for a player by CharName.
// Returns nil if not found. Uses read lock.
func (m *Manager) GetPlayerByCharNameCI(charName string) *PlayerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	lower := strings.ToLower(charName)
	for _, sess := range m.players {
		if strings.ToLower(sess.CharName) == lower {
			return sess
		}
	}
	return nil
}

// SetPendingGroupInvite atomically sets or clears the PendingGroupInvite field for uid.
// groupID "" clears the invite. Protected by mu.Lock().
func (m *Manager) SetPendingGroupInvite(uid, groupID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sess, ok := m.players[uid]; ok {
		sess.PendingGroupInvite = groupID
	}
}

// PlayerCount returns the total number of connected players.
func (m *Manager) PlayerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.players)
}

// AllPlayers returns a snapshot of all active player sessions.
//
// Postcondition: Returns a non-nil slice (may be empty); each element is non-nil.
func (m *Manager) AllPlayers() []*PlayerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*PlayerSession, 0, len(m.players))
	for _, sess := range m.players {
		out = append(out, sess)
	}
	return out
}

// CreateGroup creates a new group with leaderUID as the sole member and leader.
//
// Precondition: leaderUID must be non-empty.
// Postcondition: Returns a non-nil *Group stored in the manager; sets leader session's GroupID if the session exists.
func (m *Manager) CreateGroup(leaderUID string) *Group {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New().String()
	g := &Group{
		ID:         id,
		LeaderUID:  leaderUID,
		MemberUIDs: []string{leaderUID},
	}
	m.groups[id] = g
	if sess, ok := m.players[leaderUID]; ok {
		sess.GroupID = id
	}
	return g
}

// DisbandGroup removes the group and clears GroupID on all member sessions.
//
// Postcondition: No-op if groupID is not found.
func (m *Manager) DisbandGroup(groupID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok {
		return
	}
	for _, uid := range g.MemberUIDs {
		if sess, ok := m.players[uid]; ok {
			sess.GroupID = ""
		}
	}
	delete(m.groups, groupID)
}

// AddGroupMember appends uid to the group's MemberUIDs.
//
// Precondition: groupID must identify an existing group.
// Postcondition: Returns an error if the group is not found, uid is already a member, or the group is at capacity (8).
func (m *Manager) AddGroupMember(groupID, uid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("group not found")
	}
	for _, existing := range g.MemberUIDs {
		if existing == uid {
			return fmt.Errorf("already a member")
		}
	}
	if len(g.MemberUIDs) >= 8 {
		return fmt.Errorf("Group is full (max 8 members).")
	}
	g.MemberUIDs = append(g.MemberUIDs, uid)
	if sess, ok := m.players[uid]; ok {
		sess.GroupID = groupID
	}
	return nil
}

// RemoveGroupMember removes uid from the group's MemberUIDs and clears the session's GroupID.
//
// Postcondition: No-op if groupID is not found or uid is not a member. The group itself is not disbanded.
func (m *Manager) RemoveGroupMember(groupID, uid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.groups[groupID]
	if !ok {
		return
	}
	filtered := make([]string, 0, len(g.MemberUIDs))
	for _, existing := range g.MemberUIDs {
		if existing != uid {
			filtered = append(filtered, existing)
		}
	}
	g.MemberUIDs = filtered
	if sess, ok := m.players[uid]; ok {
		sess.GroupID = ""
	}
}

// GroupByUID returns the group that contains uid, or nil if the player is not in any group.
//
// Postcondition: Returns nil if uid is not a member of any group.
func (m *Manager) GroupByUID(uid string) *Group {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, g := range m.groups {
		for _, member := range g.MemberUIDs {
			if member == uid {
				return g
			}
		}
	}
	return nil
}

// GroupByID returns the group with the given ID.
//
// Postcondition: Returns (group, true) if found, or (nil, false) otherwise.
func (m *Manager) GroupByID(groupID string) (*Group, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.groups[groupID]
	return g, ok
}

// ForEachPlayer calls fn once for each connected player session under a read lock.
//
// Postcondition: fn must not call any Manager method that acquires mu (would deadlock).
func (m *Manager) ForEachPlayer(fn func(*PlayerSession)) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, sess := range m.players {
		fn(sess)
	}
}
