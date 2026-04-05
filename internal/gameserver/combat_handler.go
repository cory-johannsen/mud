package gameserver

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/faction"
	"github.com/cory-johannsen/mud/internal/game/quest"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/game/xp"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/scripting"
)

// CurrencySaver persists a player's currency to durable storage.
type CurrencySaver interface {
	SaveCurrency(ctx context.Context, characterID int64, currency int) error
}

// SubstanceService is the interface CombatHandler needs for substance application.
// REQ-AH-21: applies a poison substance to a target on a successful weapon hit.
type SubstanceService interface {
	ApplySubstanceByID(uid, substanceID string) error
}

// CombatHandler handles attack, strike, pass, flee, and round timer management
// for the 3-action-point economy (Stage 4).
//
// Precondition: All fields must be non-nil after construction.
//
// combatMu serialises all access to combat state (Combat structs, ActionQueues)
// and timer management so that the timer goroutine and caller goroutines cannot
// race on shared mutable data.
type CombatHandler struct {
	engine        *combat.Engine
	npcMgr        *npc.Manager
	sessions      *session.Manager
	dice          *dice.Roller
	broadcastFn            func(roomID string, events []*gamev1.CombatEvent)
	roundStartBroadcastFn  func(roomID string, evt *gamev1.RoundStartEvent) // optional; sends RoundStartEvent to all room sessions
	roundDuration time.Duration
	condRegistry  *condition.Registry
	worldMgr      *world.Manager
	scriptMgr     *scripting.Manager
	invRegistry   *inventory.Registry
	aiRegistry    *ai.Registry
	respawnMgr    *npc.RespawnManager
	floorMgr      *inventory.FloorManager
	onCombatEndFn      func(roomID string)                            // optional; called after combat ends; may be nil
	onCoverHit         func(roomID, attackerID, coverEquipID string) // optional; called on cover crossfire; may be nil
	// hirelingOwnerOf returns the UID of the player who has hired the given hireling instance,
	// or empty string if not hired. Used to enforce REQ-NPC-8.
	//
	// Precondition: instID is non-empty.
	// Postcondition: returns the owner UID or "".
	hirelingOwnerOf func(instID string) string // optional; enforces REQ-NPC-8; may be nil
	onCombatantMoved   func(roomID, movedCombatantID string)         // optional; called after Stride/Step/Shove resolves; may be nil
	xpSvc          *xp.Service            // optional; awards kill XP on NPC death; may be nil
	currencySaver  CurrencySaver          // optional; persists currency after loot award; may be nil
	mentalStateMgr *mentalstate.Manager   // optional; manages mental state conditions; may be nil
	featRegistry   *ruleset.FeatRegistry  // optional; used for NPC feat bonus resolution; may be nil
	logger         *zap.Logger            // optional; used for error logging; may be nil
	substanceSvc   SubstanceService       // optional; applies poison substances on weapon hit (REQ-AH-21); may be nil
	factionSvc    *faction.Service       // optional; awards faction rep on NPC kill; may be nil
	factionConfig *faction.FactionConfig // optional; holds rep economy parameters; may be nil
	questSvc      *quest.Service         // optional; records kill/fetch/explore progress; may be nil
	combatMu      sync.RWMutex
	timersMu      sync.Mutex
	timers        map[string]*combat.RoundTimer
	loadoutsMu    sync.Mutex
	loadouts      map[string]*inventory.WeaponPreset
	// roomCoverState tracks current HP for destructible cover objects.
	// Keyed by roomID+":"+equipmentID. Value is current HP (0 = destroyed).
	coverMu       sync.Mutex
	roomCoverState map[string]int
	// npcIdleTickInterval is used to convert say cooldown durations to tick counts. REQ-NB-2.
	// Zero means use 1 tick minimum.
	npcIdleTickInterval time.Duration
	// onMassiveDamage is an optional callback fired when a player takes ≥50% of their max HP in a single hit.
	// REQ-JD-10: triggers on_take_damage_in_one_hit_above_threshold drawback evaluation.
	// May be nil; no-op when nil.
	onMassiveDamage func(uid string)
	// onPlayerDeath is an optional callback fired for each player who is downed (HP=0) when all-player-down
	// combat ends. The callback receives the downed player's uid and should handle respawn.
	// May be nil; no-op when nil.
	onPlayerDeath func(uid string)
	// onNPCDamageTaken is an optional callback fired when an NPC takes damage in combat.
	// The callback receives the NPC instance ID.
	// May be nil; no-op when nil.
	onNPCDamageTaken func(instID string)
	// onNPCDeath is an optional callback fired when an NPC dies in combat, after removal.
	// The callback receives the NPC instance ID.
	// May be nil; no-op when nil.
	onNPCDeath func(instID string)
	// seduceConditions is the shared map of NPC instance ID → condition.ActiveSet used for charmed tracking.
	// Set after construction via SetSeduceConditions; nil means charmed-save processing is skipped.
	seduceConditions map[string]*condition.ActiveSet
	// weatherMgr provides active weather effects for outdoor rooms during combat.
	// May be nil when weather feature is not configured.
	weatherMgr *WeatherManager
	// pushCharacterSheetFn is an optional callback invoked after XP is awarded to push
	// an updated CharacterSheetView to the player so the web UI Stats tab refreshes.
	// May be nil; no-op when nil.
	pushCharacterSheetFn func(sess *session.PlayerSession)
	// pushInventoryFn is an optional callback invoked after currency loot is awarded to push
	// an updated InventoryView to the player so the web UI Inventory tab refreshes.
	// May be nil; no-op when nil.
	pushInventoryFn func(sess *session.PlayerSession)
	// saveInventoryFn is an optional callback invoked after items are directly granted to a
	// player's backpack to persist the new inventory state. May be nil; persistence is skipped.
	saveInventoryFn func(sess *session.PlayerSession) error
}

// NewCombatHandler creates a CombatHandler with a round timer and broadcast function.
//
// Precondition: all pointer arguments except invRegistry, respawnMgr, and floorMgr must be non-nil;
// respawnMgr may be nil (respawn scheduling is skipped when nil);
// floorMgr may be nil (floor item drops are skipped when nil); roundDuration must be > 0.
// Postcondition: Returns a non-nil CombatHandler.
func NewCombatHandler(
	engine *combat.Engine,
	npcMgr *npc.Manager,
	sessions *session.Manager,
	diceRoller *dice.Roller,
	broadcastFn func(roomID string, events []*gamev1.CombatEvent),
	roundDuration time.Duration,
	condRegistry *condition.Registry,
	worldMgr *world.Manager,
	scriptMgr *scripting.Manager,
	invRegistry *inventory.Registry,
	aiRegistry *ai.Registry,
	respawnMgr     *npc.RespawnManager,
	floorMgr       *inventory.FloorManager,
	mentalStateMgr *mentalstate.Manager,
) *CombatHandler {
	return &CombatHandler{
		engine:        engine,
		npcMgr:        npcMgr,
		sessions:      sessions,
		dice:          diceRoller,
		broadcastFn:   broadcastFn,
		roundDuration: roundDuration,
		condRegistry:  condRegistry,
		worldMgr:      worldMgr,
		scriptMgr:     scriptMgr,
		invRegistry:   invRegistry,
		aiRegistry:    aiRegistry,
		respawnMgr:     respawnMgr,
		floorMgr:       floorMgr,
		mentalStateMgr: mentalStateMgr,
		timers:         make(map[string]*combat.RoundTimer),
		loadouts:       make(map[string]*inventory.WeaponPreset),
		roomCoverState: make(map[string]int),
	}
}

// coverKey returns the map key for a room+equipment pair.
func coverKey(roomID, equipID string) string { return roomID + ":" + equipID }

// InitCoverState sets the initial HP for a destructible cover object.
//
// Precondition: hp > 0.
// Postcondition: GetCoverHP(roomID, equipID) == hp.
func (h *CombatHandler) InitCoverState(roomID, equipID string, hp int) {
	if hp <= 0 {
		return // invalid HP; do not initialize
	}
	h.coverMu.Lock()
	defer h.coverMu.Unlock()
	h.roomCoverState[coverKey(roomID, equipID)] = hp
}

// GetCoverHP returns the current HP of a cover object.
// Returns -1 when the cover object has not been initialized.
//
// Postcondition: Returns -1 (uninitialized), 0 (destroyed), or > 0 (intact).
func (h *CombatHandler) GetCoverHP(roomID, equipID string) int {
	h.coverMu.Lock()
	defer h.coverMu.Unlock()
	v, ok := h.roomCoverState[coverKey(roomID, equipID)]
	if !ok {
		return -1
	}
	return v
}

// ClearCoverForEquipment removes the cover state entry entirely.
// Called after cover is destroyed to free memory.
//
// Postcondition: GetCoverHP(roomID, equipID) == -1.
func (h *CombatHandler) ClearCoverForEquipment(roomID, equipID string) {
	h.coverMu.Lock()
	defer h.coverMu.Unlock()
	delete(h.roomCoverState, coverKey(roomID, equipID))
}

// DecrementAndCheckDestroyed decrements cover HP atomically and clears the entry if it reaches zero.
// Returns true when the cover was destroyed (HP just reached 0), false otherwise.
//
// Precondition: cover must have been initialized via InitCoverState.
// Postcondition: if returned true, the entry is removed from roomCoverState.
func (h *CombatHandler) DecrementAndCheckDestroyed(roomID, equipID string) bool {
	h.coverMu.Lock()
	defer h.coverMu.Unlock()
	k := coverKey(roomID, equipID)
	v, ok := h.roomCoverState[k]
	if !ok || v == 0 {
		return false
	}
	v--
	h.roomCoverState[k] = v
	if v == 0 {
		delete(h.roomCoverState, k)
		return true
	}
	return false
}

// SetXPService registers the XP service used to award kill XP.
//
// Precondition: svc must be non-nil.
// Postcondition: Kill XP is awarded to the first living player on NPC death.
func (h *CombatHandler) SetXPService(svc *xp.Service) {
	h.xpSvc = svc
}

// SetFactionService registers the faction service used to award rep on NPC kill.
//
// Precondition: svc and cfg must be non-nil.
// Postcondition: Faction rep is awarded to the killing player when the NPC belongs to a hostile faction.
func (h *CombatHandler) SetFactionService(svc *faction.Service, cfg *faction.FactionConfig) {
	h.factionSvc = svc
	h.factionConfig = cfg
}

// SetQuestService injects the quest service for kill-progress recording.
//
// Precondition: svc must be non-nil.
// Postcondition: RecordKill is called for each living participant when an NPC dies.
func (h *CombatHandler) SetQuestService(svc *quest.Service) {
	h.questSvc = svc
}

// SetPushCharacterSheetFn registers a callback invoked after XP is awarded in
// pushXPMessages so the web client Stats tab receives an updated CharacterSheetView.
//
// Precondition: fn may be nil (no-op when nil).
// Postcondition: fn is called once per XP award with the receiving session.
func (h *CombatHandler) SetPushCharacterSheetFn(fn func(sess *session.PlayerSession)) {
	h.pushCharacterSheetFn = fn
}

// SetPushInventoryFn registers a callback called after currency loot is distributed
// to push an updated InventoryView to the player so the web UI reflects the new balance.
//
// Precondition: fn may be nil (no-op when nil).
// Postcondition: fn is called once per currency award with the receiving session.
func (h *CombatHandler) SetPushInventoryFn(fn func(sess *session.PlayerSession)) {
	h.pushInventoryFn = fn
}

// SetSaveInventoryFn registers a callback invoked after items are directly granted to a
// player's backpack during post-combat loot distribution. The callback should persist the
// player's updated backpack to durable storage.
//
// Precondition: fn may be nil (persistence is skipped when nil).
// Postcondition: fn is called once per player after their backpack is updated with loot.
func (h *CombatHandler) SetSaveInventoryFn(fn func(sess *session.PlayerSession) error) {
	h.saveInventoryFn = fn
}

// SetCurrencySaver registers the saver used to persist player currency after loot award.
//
// Precondition: saver must be non-nil.
// Postcondition: Currency is persisted to durable storage after each NPC kill that drops currency.
func (h *CombatHandler) SetCurrencySaver(saver CurrencySaver) {
	h.currencySaver = saver
}

// SetSubstanceSvc registers the SubstanceService used to apply poison substances on hit.
//
// Precondition: svc must be non-nil.
// Postcondition: substanceSvc is set on the CombatHandler.
func (h *CombatHandler) SetSubstanceSvc(svc SubstanceService) {
	h.substanceSvc = svc
}

// SetLogger registers the logger used for error reporting inside CombatHandler.
//
// Precondition: logger must be non-nil.
// Postcondition: Errors such as AwardKill failures are logged via logger.Warn.
func (h *CombatHandler) SetLogger(logger *zap.Logger) {
	h.logger = logger
}

// SetFeatRegistry registers the feat registry used to compute NPC passive feat bonuses (REQ-AE-16).
//
// Precondition: registry must be non-nil.
// Postcondition: NPC feat bonuses are applied when building combatants.
func (h *CombatHandler) SetFeatRegistry(registry *ruleset.FeatRegistry) {
	h.featRegistry = registry
}

// SetHirelingOwnerOf registers a callback that returns the owner UID for a hireling instance.
// Used to enforce REQ-NPC-8: a player cannot attack their own bound hireling.
//
// Precondition: fn may be nil (disables the check).
// Postcondition: h.hirelingOwnerOf == fn.
func (h *CombatHandler) SetHirelingOwnerOf(fn func(instID string) string) {
	h.hirelingOwnerOf = fn
}

// SetOnCombatEnd registers a callback invoked after each combat ends.
//
// Precondition: fn may be nil (no-op after combat end).
// Postcondition: fn is called with the roomID of the ended combat.
func (h *CombatHandler) SetOnCombatEnd(fn func(roomID string)) {
	h.onCombatEndFn = fn
}

// SetRoundStartBroadcastFn registers a callback that fires when a new combat round begins.
// The callback receives the room ID and a fully-populated RoundStartEvent; it is responsible
// for delivering the event to all sessions in the room. If nil, no RoundStartEvent is sent.
func (h *CombatHandler) SetRoundStartBroadcastFn(fn func(roomID string, evt *gamev1.RoundStartEvent)) {
	h.roundStartBroadcastFn = fn
}

// SetOnCoverHit registers a callback that fires when an attack misses due to cover.
// The callback receives the room ID, attacker ID, and cover equipment ID.
// The callback may be nil; if so, no action is taken.
func (h *CombatHandler) SetOnCoverHit(fn func(roomID, attackerID, coverEquipID string)) {
	h.onCoverHit = fn
}

// SetOnCombatantMoved registers a callback that fires after any Stride, Step, or Shove resolves.
// The callback receives the room ID and the UID of the combatant that moved.
// The callback may be nil; if so, no action is taken.
func (h *CombatHandler) SetOnCombatantMoved(fn func(roomID, movedCombatantID string)) {
	h.onCombatantMoved = fn
}

// SetOnNPCDamageTaken registers a callback invoked when an NPC takes damage in combat.
//
// Precondition: fn may be nil (disables the callback).
// Postcondition: fn is called with the NPC instance ID on each damage event.
func (h *CombatHandler) SetOnNPCDamageTaken(fn func(instID string)) {
	h.onNPCDamageTaken = fn
}

// SetOnNPCDeath registers a callback invoked when an NPC dies in combat.
//
// Precondition: fn may be nil (disables the callback).
// Postcondition: fn is called with the NPC instance ID after the NPC is removed.
func (h *CombatHandler) SetOnNPCDeath(fn func(instID string)) {
	h.onNPCDeath = fn
}

// SetSeduceConditions wires the shared seduceConditions map into the CombatHandler
// so charmed NPCs can make saving throws at round end (REQ-ZN-9).
//
// Precondition: m must be the same map used by executeSeduce on GameServiceServer.
// Postcondition: h.seduceConditions is set; charmed-save processing is enabled.
func (h *CombatHandler) SetSeduceConditions(m map[string]*condition.ActiveSet) {
	h.seduceConditions = m
}

// SetWeatherManager wires the WeatherManager into the CombatHandler so outdoor
// rooms receive active weather effects during combat round evaluation.
//
// Precondition: wm may be nil (weather effects are skipped when nil).
// Postcondition: h.weatherMgr is set; outdoor weather effect processing is enabled.
func (h *CombatHandler) SetWeatherManager(wm *WeatherManager) {
	h.weatherMgr = wm
}

// SetNPCIdleTickInterval sets the idle tick interval used to convert say cooldown
// durations to tick counts. REQ-NB-2.
//
// Precondition: interval must be > 0.
func (h *CombatHandler) SetNPCIdleTickInterval(interval time.Duration) {
	if interval > 0 {
		h.npcIdleTickInterval = interval
	}
}

// FireCombatantMoved invokes the onCombatantMoved callback if registered.
// Called by grpc_service.go after Stride, Step, or Shove position changes.
//
// Precondition: none.
// Postcondition: callback is invoked synchronously if non-nil.
func (h *CombatHandler) FireCombatantMoved(roomID, uid string) {
	if h.onCombatantMoved != nil {
		h.onCombatantMoved(roomID, uid)
	}
}

// CombatantPosition returns the current combat position (feet) of the given combatant in the given room.
// Returns 0 if no combat is active for roomID or the combatant is not found.
// In 2D combat, position is expressed as GridX * 5 to maintain compatibility with the 1D trap system.
//
// Postcondition: Returns 0 when no combat is active or combatant is absent; otherwise returns the combatant's GridX*5 position.
func (h *CombatHandler) CombatantPosition(roomID, uid string) int {
	h.combatMu.RLock()
	defer h.combatMu.RUnlock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return 0
	}
	c := cbt.GetCombatant(uid)
	if c == nil {
		return 0
	}
	return c.GridX * 5
}

// CombatantsInRoom returns a copy of all combatants in the active combat for the given room.
// Returns nil if no combat is active.
//
// Postcondition: Returns nil when no combat is active; otherwise returns a non-nil slice of combatant pointers.
func (h *CombatHandler) CombatantsInRoom(roomID string) []*combat.Combatant {
	h.combatMu.RLock()
	defer h.combatMu.RUnlock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return nil
	}
	result := make([]*combat.Combatant, len(cbt.Combatants))
	copy(result, cbt.Combatants)
	return result
}

// InitiateGuardCombat finds guard NPCs in the player's current room and starts
// combat against the player. Only guards whose WantedThreshold (default 2) is
// <= wantedLevel are engaged. wantedLevel distinguishes detain (2) from kill (3-4).
// If no eligible guard NPCs are present in the room, this is a no-op.
//
// Precondition: uid MUST be a valid player UID; wantedLevel MUST be in [2, 4].
// Postcondition: if the player session exists and eligible guard NPCs are present,
// broadcastFn is called with a narrative CombatEvent and h.Attack is invoked for each
// eligible guard. If the player session is not found or no eligible guards are present,
// this is a no-op.
func (h *CombatHandler) InitiateGuardCombat(uid, zoneID string, wantedLevel int) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	npcs := h.npcMgr.InstancesInRoom(sess.RoomID)
	var guardIDs []string
	for _, n := range npcs {
		if n.NPCType != "guard" {
			continue
		}
		threshold := 2
		if tmpl := h.npcMgr.TemplateByID(n.TemplateID); tmpl != nil && tmpl.Guard != nil && tmpl.Guard.WantedThreshold > 0 {
			threshold = tmpl.Guard.WantedThreshold
		}
		if wantedLevel >= threshold {
			guardIDs = append(guardIDs, n.ID)
		}
	}
	if len(guardIDs) == 0 {
		return
	}
	var narrative string
	if wantedLevel >= 3 {
		narrative = "The guards attack on sight!"
	} else {
		narrative = "Guards shout: Drop your weapon and surrender!"
	}
	h.broadcastFn(sess.RoomID, []*gamev1.CombatEvent{
		{Type: gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK, Narrative: narrative},
	})
	for _, guardID := range guardIDs {
		_, _ = h.Attack(guardID, uid)
		// COMBATMSG-4e: push wanted-status initiation message to the player.
		if guardInst, ok := h.npcMgr.Get(guardID); ok {
			h.pushMessageToUID(uid, combat.FormatNPCInitiationMsg(guardInst.Name(), combat.ReasonWanted, ""))
		}
	}
}

// InitiateNPCCombat starts combat between npcInst and the player with playerUID.
// This is used for threat-based engagement where the NPC proactively attacks. REQ-NB-7.
//
// Precondition: npcInst must not be nil; playerUID must be a valid connected player.
// Postcondition: combat is started in the player's room; events are broadcast.
func (h *CombatHandler) InitiateNPCCombat(npcInst *npc.Instance, playerUID string) {
	sess, ok := h.sessions.GetPlayer(playerUID)
	if !ok {
		return
	}
	if npcInst.RoomID != sess.RoomID {
		return
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	if _, alreadyActive := h.engine.GetCombat(sess.RoomID); alreadyActive {
		return
	}

	initEvents, err := h.startPursuitCombatLocked(sess, []*npc.Instance{npcInst})
	if err != nil {
		return
	}
	h.broadcastFn(sess.RoomID, initEvents)
	// COMBATMSG-3/4/6: push NPC initiation reason message to the targeted player.
	reason := h.inferNPCReason(npcInst)
	h.pushMessageToUID(playerUID, combat.FormatNPCInitiationMsg(npcInst.Name(), reason, npcInst.ProtectedNPCName))
}

// inferNPCReason derives the InitiationReason for an NPC based on its current state.
//
// Precondition: inst must not be nil.
// Postcondition: Returns the most specific applicable InitiationReason constant.
func (h *CombatHandler) inferNPCReason(inst *npc.Instance) combat.InitiationReason {
	if inst.GrudgePlayerID != "" {
		return combat.ReasonProvoked
	}
	if inst.PendingJoinCombatRoomID != "" {
		return combat.ReasonProtecting
	}
	if inst.Disposition == "hostile" {
		return combat.ReasonOnSight
	}
	return combat.ReasonTerritory
}

// Attack queues a 1-AP ActionAttack for uid against target.
// If no combat is active in the room, it is started first.
// If AllActionsSubmitted after queuing, the round resolves immediately.
//
// Precondition: uid must be a valid connected player; target must be non-empty.
// Postcondition: Returns events to return to the requesting player, or an error.
func (h *CombatHandler) Attack(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	inst := h.npcMgr.FindInRoom(sess.RoomID, target)
	if inst == nil {
		return nil, fmt.Errorf("you don't see %q here", target)
	}
	if inst.IsDead() {
		return nil, fmt.Errorf("%s is already dead", inst.Name())
	}

	// REQ-NPC-4: non-combat NPCs cannot be attacked directly.
	// Guards enter combat via their own engage behavior (sub-project 4).
	// Hirelings are combat participants (sub-project 4).
	if inst.NPCType != "" && inst.NPCType != "combat" && inst.NPCType != "guard" && inst.NPCType != "hireling" {
		return nil, fmt.Errorf("%s is not a valid combat target", inst.Name())
	}

	// REQ-NPC-8: a hireling bound to the attacking player MUST NOT be targetable
	// by that player's own attacks.
	if inst.NPCType == "hireling" && h.hirelingOwnerOf != nil {
		if owner := h.hirelingOwnerOf(inst.ID); owner == uid {
			return nil, fmt.Errorf("you cannot attack your own hireling")
		}
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	var initEvents []*gamev1.CombatEvent
	if !ok {
		var err error
		cbt, initEvents, err = h.startCombatLocked(sess, inst)
		if err != nil {
			return nil, err
		}
		// COMBATMSG-5: push player-initiated combat message before first round output.
		h.pushMessageToUID(uid, combat.FormatPlayerInitiationMsg(inst.Name()))
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionAttack, Target: inst.Name()}); err != nil {
		return nil, fmt.Errorf("queuing attack: %w", err)
	}

	// BUG-31: include remaining AP in confirmation so the player can plan.
	apMsg := h.formatAPRemaining(uid, cbt)

	// proto has no PASS/ROUND type; ATTACK is the closest available sentinel — client uses Narrative for display
	confirmEvent := &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:  sess.CharName,
		Target:    inst.Name(),
		Narrative: fmt.Sprintf("%s readies an attack against %s.%s", sess.CharName, inst.Name(), apMsg),
	}

	if len(initEvents) > 0 {
		// Combat was just started — auto-queue NPCs and start timer.
		h.autoQueueNPCsLocked(cbt)
		h.startTimerLocked(sess.RoomID)
		return append(initEvents, confirmEvent), nil
	}

	// Combat was already active.
	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		h.resolveAndAdvanceLocked(sess.RoomID, cbt)
		return []*gamev1.CombatEvent{confirmEvent}, nil
	}

	return []*gamev1.CombatEvent{confirmEvent}, nil
}

// Strike queues a 2-AP ActionStrike for uid against target.
// Requires active combat. Early-resolves if all actions submitted.
//
// Precondition: uid must be a valid connected player in active combat; target must be non-empty.
// Postcondition: Returns events to return to the requesting player, or an error.
func (h *CombatHandler) Strike(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionStrike, Target: target}); err != nil {
		return nil, fmt.Errorf("queuing strike: %w", err)
	}

	apMsg := h.formatAPRemaining(uid, cbt)

	// proto has no PASS/ROUND type; ATTACK is the closest available sentinel — client uses Narrative for display
	confirmEvent := &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:  sess.CharName,
		Target:    target,
		Narrative: fmt.Sprintf("%s launches a full strike against %s.%s", sess.CharName, target, apMsg),
	}

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		h.resolveAndAdvanceLocked(sess.RoomID, cbt)
		return []*gamev1.CombatEvent{confirmEvent}, nil
	}

	return []*gamev1.CombatEvent{confirmEvent}, nil
}

// Aid queues an ActionAid for the combatant identified by uid targeting allyName.
//
// Precondition: uid must be a valid connected player in active combat.
// Precondition: allyName must be non-empty, must match a living player combatant in the same
// combat (case-insensitive, by Name), and must not match the actor's own CharName or UID.
// Postcondition: Returns a confirmation CombatEvent and nil error on success.
// Aid queues an Aid action for uid targeting allyName in the current combat round.
//
// Precondition: uid must identify a valid player session in active combat; allyName
// must be non-empty and must not equal the player's own name or uid.
// Postcondition: On success returns a slice containing exactly one confirmation
// CombatEvent and the action is queued for resolution at end of round; allyName
// receives an aided condition bonus when the round resolves. On failure returns
// a non-nil error and an empty slice.
func (h *CombatHandler) Aid(uid, allyName string) ([]*gamev1.CombatEvent, error) {
	if allyName == "" {
		return nil, fmt.Errorf("ally name must not be empty")
	}

	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Self-targeting check.
	if strings.EqualFold(allyName, sess.CharName) || allyName == uid {
		return nil, fmt.Errorf("you cannot aid yourself")
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in active combat")
	}

	// Find the ally: must be a living player combatant in this combat.
	var canonicalName string
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer && strings.EqualFold(c.Name, allyName) && !c.IsDead() {
			canonicalName = c.Name
			break
		}
	}
	if canonicalName == "" {
		return nil, fmt.Errorf("no living ally named %q found in this combat", allyName)
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionAid, Target: canonicalName}); err != nil {
		return nil, fmt.Errorf("queuing aid: %w", err)
	}

	apMsg := h.formatAPRemaining(uid, cbt)

	confirmEvent := &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_CONDITION,
		Attacker:  sess.CharName,
		Target:    canonicalName,
		Narrative: fmt.Sprintf("%s prepares to aid %s.%s", sess.CharName, canonicalName, apMsg),
	}

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		h.resolveAndAdvanceLocked(sess.RoomID, cbt)
	}

	return []*gamev1.CombatEvent{confirmEvent}, nil
}

// ActivateAbility queues an ActionUseAbility for the combatant identified by uid.
//
// Precondition: uid must be a valid connected player in active combat; qa.Type must be ActionUseAbility.
// Postcondition: Returns nil on success, or an error if the combatant has insufficient AP or is not found.
func (h *CombatHandler) ActivateAbility(uid string, qa combat.QueuedAction) error {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return fmt.Errorf("you are not in combat")
	}

	if err := cbt.QueueAction(uid, qa); err != nil {
		return fmt.Errorf("queuing ability: %w", err)
	}
	return nil
}

// RemainingAP returns the number of action points remaining for combatant uid.
//
// Precondition: uid must be non-empty.
// Postcondition: Returns 0 if the combatant is not found or has no remaining AP.
func (h *CombatHandler) RemainingAP(uid string) int {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return 0
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return 0
	}

	q, ok := cbt.ActionQueues[uid]
	if !ok {
		return 0
	}
	return q.RemainingPoints()
}

// formatAPRemaining returns a short string like " (2 AP remaining)" for the given
// combatant in the given combat. Returns "" if the queue is not found.
// Caller must hold combatMu.
//
// Precondition: cbt must be non-nil.
// Postcondition: Returns a non-empty string when the player has an action queue.
func (h *CombatHandler) formatAPRemaining(uid string, cbt *combat.Combat) string {
	q, ok := cbt.ActionQueues[uid]
	if !ok {
		return ""
	}
	return fmt.Sprintf(" (%d AP remaining)", q.RemainingPoints())
}

// SpendAP deducts cost AP from the combatant uid's action queue in their active combat.
//
// Precondition: uid must be non-empty; cost must be > 0.
// Postcondition: Returns nil on success; error if player not in combat or insufficient AP.
func (h *CombatHandler) SpendAP(uid string, cost int) error {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return fmt.Errorf("player %q is not in active combat", uid)
	}

	q, ok := cbt.ActionQueues[uid]
	if !ok {
		return fmt.Errorf("no action queue for player %q", uid)
	}
	return q.DeductAP(cost)
}

// SpendAllAP drains all remaining AP from uid's action queue in their active combat.
// If the player is not in combat or has no action queue, SpendAllAP is a no-op.
//
// Precondition: uid must be non-empty.
// Postcondition: uid's remaining AP is zero when they are in active combat with a queue.
func (h *CombatHandler) SpendAllAP(uid string) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return
	}
	q, ok := cbt.ActionQueues[uid]
	if !ok {
		return
	}
	remaining := q.RemainingPoints()
	if remaining > 0 {
		_ = q.DeductAP(remaining)
	}
}

// ApplyPlayerACMod adds delta to the player's own Combatant.ACMod in their active combat.
// No-op if player is not in active combat.
//
// Precondition: uid must be non-empty.
// Postcondition: Player combatant ACMod incremented by delta when player is in active combat.
func (h *CombatHandler) ApplyPlayerACMod(uid string, delta int) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return
	}
	for _, c := range cbt.Combatants {
		if c.ID == uid && c.Kind == combat.KindPlayer {
			c.ACMod += delta
			return
		}
	}
}

// PlayerACMod returns the player's Combatant.ACMod in their active combat.
// Returns 0 if player is not in active combat.
//
// Precondition: uid must be non-empty.
// Postcondition: Returns current ACMod for the player combatant, or 0 if not found.
func (h *CombatHandler) PlayerACMod(uid string) int {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return 0
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return 0
	}
	for _, c := range cbt.Combatants {
		if c.ID == uid && c.Kind == combat.KindPlayer {
			return c.ACMod
		}
	}
	return 0
}

// ApplyCombatantACMod adds mod to the named combatant's ACMod in the player's active combat.
// Use to apply mid-round AC modifiers from feint (negative) or raise_shield/take_cover (positive).
//
// Precondition: uid must be a player in active combat; targetID must be a combatant in that combat.
// Postcondition: Returns nil on success.
func (h *CombatHandler) ApplyCombatantACMod(uid, targetID string, mod int) error {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return fmt.Errorf("player %q is not in active combat", uid)
	}

	for _, c := range cbt.Combatants {
		if c.ID == targetID {
			c.ACMod += mod
			return nil
		}
	}
	return fmt.Errorf("combatant %q not found in combat", targetID)
}

// ApplyCombatantAttackMod adds mod to the named combatant's AttackMod in the player's active combat.
// Use to apply attack penalties (e.g. demoralize, frightened).
//
// Precondition: uid must be a player in active combat; targetID must be a combatant in that combat.
// Postcondition: Returns nil on success.
func (h *CombatHandler) ApplyCombatantAttackMod(uid, targetID string, mod int) error {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return fmt.Errorf("player %q is not in active combat", uid)
	}

	for _, c := range cbt.Combatants {
		if c.ID == targetID {
			c.AttackMod += mod
			return nil
		}
	}
	return fmt.Errorf("combatant %q not found in combat", targetID)
}

// GetCombatForRoom returns the active combat in roomID, or (nil, false) if none.
//
// Precondition: roomID must be a non-empty string.
// Postcondition: Returns (combat, true) if active combat exists; (nil, false) otherwise.
func (h *CombatHandler) GetCombatForRoom(roomID string) (*combat.Combat, bool) {
	h.combatMu.RLock()
	defer h.combatMu.RUnlock()
	return h.engine.GetCombat(roomID)
}

// BroadcastAllPositions broadcasts COMBAT_EVENT_TYPE_POSITION events for every combatant
// in the active combat for roomID. No-op if there is no active combat.
//
// Precondition: roomID must be non-empty.
// Postcondition: One position event per combatant is sent via broadcastFn.
func (h *CombatHandler) BroadcastAllPositions(roomID string) {
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return
	}
	events := make([]*gamev1.CombatEvent, 0, len(cbt.Combatants))
	for _, c := range cbt.Combatants {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_POSITION,
			Attacker:  c.Name,
			AttackerX: int32(c.GridX),
			AttackerY: int32(c.GridY),
		})
	}
	h.broadcastFn(roomID, events)
}

// JoinPendingNPCCombat adds inst to the active combat in pendingRoomID and pushes
// the call-for-help or protecting message to all player combatants in that combat.
//
// Precondition: inst must not be nil; pendingRoomID must be non-empty.
// Postcondition: NPC is added to active combat; message pushed to targeted player(s); no-op if no active combat.
func (h *CombatHandler) JoinPendingNPCCombat(inst *npc.Instance, pendingRoomID string) {
	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(pendingRoomID)
	if !ok {
		return
	}
	reason := combat.ReasonCallForHelp
	if inst.ProtectedNPCName != "" {
		reason = combat.ReasonProtecting
	}
	// Push message to all player combatants.
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindPlayer {
			continue
		}
		h.pushMessageToUID(c.ID, combat.FormatNPCInitiationMsg(inst.Name(), reason, inst.ProtectedNPCName))
	}
	// Build and add the NPC combatant.
	var npcFeatStats NPCEffectiveStats
	if h.featRegistry != nil {
		roomNPCs := h.npcMgr.InstancesInRoom(pendingRoomID)
		npcFeatStats = ComputeNPCAttackStats(inst, nil, h.featRegistry, roomNPCs)
	}
	npcWeaponName := ""
	if inst.WeaponID != "" && h.invRegistry != nil {
		if wDef := h.invRegistry.Weapon(inst.WeaponID); wDef != nil {
			npcWeaponName = wDef.Name
		}
	}
	npcStrMod := combat.AbilityMod(inst.Awareness) + npcFeatStats.DamageBonus + npcFeatStats.AttackBonus
	npcCbt := &combat.Combatant{
		ID:          inst.ID,
		Kind:        combat.KindNPC,
		Name:        inst.Name(),
		MaxHP:       inst.MaxHP,
		CurrentHP:   inst.CurrentHP,
		AC:          inst.AC + npcFeatStats.ACBonus,
		Level:       inst.Level,
		StrMod:      npcStrMod,
		DexMod:      1,
		NPCType:     inst.Type,
		Resistances: inst.Resistances,
		Weaknesses:  inst.Weaknesses,
		WeaponName:  npcWeaponName,
		AttackVerb:  inst.AttackVerb,
	}
	combat.RollInitiative([]*combat.Combatant{npcCbt}, h.dice.Src())
	// Assign sequential X column on row 9 (NPC side).
	existingNPCs := 0
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			existingNPCs++
		}
	}
	npcCbt.GridX = existingNPCs
	npcCbt.GridY = 9
	if addErr := h.engine.AddCombatant(pendingRoomID, npcCbt); addErr != nil {
		h.logger.Warn("JoinPendingNPCCombat: AddCombatant failed",
			zap.String("npc_id", inst.ID),
			zap.String("room_id", pendingRoomID),
			zap.Error(addErr),
		)
	}
}

// pushMessageToUID sends a plain text message event to the player identified by uid.
//
// Precondition: uid must be non-empty; content must be non-empty.
// Postcondition: Message is enqueued on the player's entity stream; no-ops if player not found or entity is nil.
func (h *CombatHandler) pushMessageToUID(uid, content string) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok || sess.Entity == nil {
		return
	}
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: content,
				Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
			},
		},
	}
	if data, err := proto.Marshal(evt); err == nil {
		_ = sess.Entity.Push(data)
	}
}

// ApplyCombatCondition applies condID (stacks=1, duration=-1) to the combatant identified by
// targetID in the active combat for the room where uid is fighting.
//
// Precondition: uid must be in active combat; targetID must be a valid combatant ID in that combat;
// condID must be registered in the combat's condition registry.
// Postcondition: The condition is active on the target combatant; returns nil on success.
func (h *CombatHandler) ApplyCombatCondition(uid, targetID, condID string) error {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return fmt.Errorf("player %q is not in active combat", uid)
	}
	return cbt.ApplyCondition(targetID, condID, 1, -1)
}

// SetCombatantHidden sets the Hidden field on the combatant identified by uid
// in the active combat for that player's room.
//
// Precondition: uid must be in active combat.
// Postcondition: The combatant's Hidden field equals hidden; returns nil on success.
func (h *CombatHandler) SetCombatantHidden(uid string, hidden bool) error {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return fmt.Errorf("player %q is not in active combat", uid)
	}
	for _, c := range cbt.Combatants {
		if c.ID == uid {
			c.Hidden = hidden
			return nil
		}
	}
	return fmt.Errorf("combatant %q not found in combat", uid)
}

// SetCombatantRevealedUntilRound sets RevealedUntilRound on the combatant with npcID
// in the active combat for roomID.
//
// Precondition: roomID and npcID must be non-empty.
// Postcondition: On success, combatant RevealedUntilRound is set to round.
func (h *CombatHandler) SetCombatantRevealedUntilRound(roomID, npcID string, round int) error {
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return fmt.Errorf("no active combat in room %q", roomID)
	}
	c := cbt.GetCombatant(npcID)
	if c == nil {
		return fmt.Errorf("combatant %q not found in room %q", npcID, roomID)
	}
	c.RevealedUntilRound = round
	return nil
}

// SetCombatantCover sets the CoverEquipmentID and CoverTier on the combatant
// identified by uid in the combat for roomID.
//
// Precondition: uid must identify an active combatant in the room's combat.
// Postcondition: combatant.CoverEquipmentID and .CoverTier are updated.
func (h *CombatHandler) SetCombatantCover(roomID, uid, equipID, tier string) error {
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return fmt.Errorf("no active combat in room %q", roomID)
	}
	for _, c := range cbt.Combatants {
		if c.ID == uid {
			c.CoverEquipmentID = equipID
			c.CoverTier = tier
			return nil
		}
	}
	return fmt.Errorf("combatant %q not found in room %q", uid, roomID)
}

// ClearCombatantCover removes cover from a combatant.
//
// Precondition: uid must identify an active combatant in the room's combat.
// Postcondition: combatant.CoverEquipmentID and .CoverTier are cleared.
func (h *CombatHandler) ClearCombatantCover(roomID, uid string) {
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return
	}
	for _, c := range cbt.Combatants {
		if c.ID == uid {
			c.CoverEquipmentID = ""
			c.CoverTier = ""
			return
		}
	}
}

// GetCombatant returns the Combatant with the given targetID from the active combat
// in the room of the player identified by uid.
//
// Precondition: uid must identify a player session; targetID must be a combatant in that room's active combat.
// Postcondition: Returns a pointer to the Combatant and true if found; nil and false otherwise.
func (h *CombatHandler) GetCombatant(uid, targetID string) (*combat.Combatant, bool) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, false
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, false
	}

	for _, c := range cbt.Combatants {
		if c.ID == targetID {
			return c, true
		}
	}
	return nil, false
}

// GetCombatConditionSet returns the condition ActiveSet for targetID from the active combat
// in the room of the player identified by uid.
//
// Precondition: uid must identify a player session; targetID must have a condition entry in that room's active combat.
// Postcondition: Returns a pointer to the ActiveSet and true if found; nil and false otherwise.
func (h *CombatHandler) GetCombatConditionSet(uid, targetID string) (*condition.ActiveSet, bool) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, false
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, false
	}

	s, ok := cbt.Conditions[targetID]
	return s, ok
}

// Pass forfeits uid's remaining AP for this round.
// Requires active combat. Early-resolves if all actions submitted.
//
// Precondition: uid must be a valid connected player in active combat.
// Postcondition: Returns events to return to the requesting player, or an error.
func (h *CombatHandler) Pass(uid string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionPass}); err != nil {
		return nil, fmt.Errorf("queuing pass: %w", err)
	}

	// proto has no PASS/ROUND type; ATTACK is the closest available sentinel — client uses Narrative for display
	confirmEvent := &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:  sess.CharName,
		Narrative: fmt.Sprintf("%s holds their ground.", sess.CharName),
	}

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		h.resolveAndAdvanceLocked(sess.RoomID, cbt)
		return []*gamev1.CombatEvent{confirmEvent}, nil
	}

	return []*gamev1.CombatEvent{confirmEvent}, nil
}

// Flee attempts to remove the player from active combat using an Athletics/Acrobatics
// skill check against the highest NPC StrMod DC in the room.
//
// Precondition: uid must be a valid connected player in active combat with >= 1 AP.
// Postcondition: On success, player is removed from combat roster, moved to a random
//   valid exit (if any), and NPC pursuit is resolved. Returns fled=true on success.
func (h *CombatHandler) Flee(uid string) ([]*gamev1.CombatEvent, bool, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, false, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		h.combatMu.Unlock()
		return nil, false, fmt.Errorf("you are not in combat")
	}

	playerCbt := h.findCombatant(cbt, uid)
	if playerCbt == nil {
		h.combatMu.Unlock()
		return nil, false, fmt.Errorf("you are not a combatant")
	}

	// IMMOBILIZED: grabbed condition prevents fleeing.
	if sess.Conditions != nil && condition.IsActionRestricted(sess.Conditions, "move") {
		// IMMOBILIZED: release lock manually — all exit paths in this function manage combatMu explicitly.
		h.combatMu.Unlock()
		evt := &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
			Attacker:  uid,
			Narrative: "You are grabbed and cannot flee!",
		}
		return []*gamev1.CombatEvent{evt}, false, nil
	}

	// FLEE-1 / FLEE-2: AP guard — inline to avoid re-acquiring combatMu (SpendAP locks it).
	q, hasQ := cbt.ActionQueues[uid]
	if !hasQ || q.RemainingPoints() < 1 {
		h.combatMu.Unlock()
		return nil, false, fmt.Errorf("you need at least 1 AP to flee")
	}
	_ = q.DeductAP(q.RemainingPoints())

	// FLEE-3: skill check — auto-pick best of muscle or acrobatics.
	roll, _ := h.dice.RollExpr("d20")
	muscleBonus := skillRankBonus(sess.Skills["muscle"])
	acrobaticsBonus := skillRankBonus(sess.Skills["acrobatics"])
	bonus := muscleBonus
	if acrobaticsBonus > muscleBonus {
		bonus = acrobaticsBonus
	}
	playerTotal := roll.Total() + bonus

	// FLEE-4: DC = 10 + highest NPC StrMod.
	bestNPC := h.bestNPCCombatant(cbt)
	dc := 10
	if bestNPC != nil {
		dc = 10 + bestNPC.StrMod
	}

	var events []*gamev1.CombatEvent

	if playerTotal < dc {
		// FLEE-5: failure — stay in room, combat continues.
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
			Attacker:  sess.CharName,
			Narrative: fmt.Sprintf("%s tries to flee but can't escape! (rolled %d vs DC %d)", sess.CharName, playerTotal, dc),
		})
		h.combatMu.Unlock()
		return events, false, nil
	}

	// FLEE-6: success — remove from combat and set idle.
	origRoomID := sess.RoomID
	h.removeCombatant(cbt, uid)
	sess.Status = int32(1) // idle

	events = append(events, &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
		Attacker:  sess.CharName,
		Narrative: fmt.Sprintf("%s breaks free and runs! (rolled %d vs DC %d)", sess.CharName, playerTotal, dc),
	})

	// FLEE-7 / FLEE-8: pick a valid exit.
	var destRoomID string
	if h.worldMgr != nil {
		if room, ok := h.worldMgr.GetRoom(origRoomID); ok {
			var validExits []world.Exit
			for _, e := range room.Exits {
				if !e.Hidden && !e.Locked {
					validExits = append(validExits, e)
				}
			}
			if len(validExits) > 0 {
				idx := 0
				if len(validExits) > 1 {
					idx = h.dice.Src().Intn(len(validExits))
				}
				chosen := validExits[idx]
				sess.RoomID = chosen.TargetRoom
				destRoomID = chosen.TargetRoom
			} else {
				events = append(events, &gamev1.CombatEvent{
					Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
					Attacker:  sess.CharName,
					Narrative: "There is nowhere to run — but you are no longer in combat.",
				})
			}
		}
	}

	// FLEE-11: end original room combat if no players remain.
	// Collect the callback so it can be invoked after releasing combatMu; calling it
	// while holding the lock risks deadlock if the callback re-enters combatMu.
	var postUnlockFn func()
	if !cbt.HasLivingPlayers() {
		h.stopTimerLocked(origRoomID)
		h.engine.EndCombat(origRoomID)
		h.clearCoweringNPCsLocked(origRoomID)
		if h.onCombatEndFn != nil {
			fn := h.onCombatEndFn
			rid := origRoomID
			postUnlockFn = func() { fn(rid) }
		}
	}

	// PURSUIT-1 through PURSUIT-6: resolve NPC pursuit into the destination room.
	if destRoomID != "" {
		pursuitEvents := h.resolvePursuitLocked(cbt, sess, playerTotal, destRoomID)
		events = append(events, pursuitEvents...)
	}

	h.combatMu.Unlock()

	// Invoke post-unlock callbacks after releasing the lock (FLEE-11).
	if postUnlockFn != nil {
		postUnlockFn()
	}

	return events, true, nil
}

// resolvePursuitLocked resolves NPC pursuit checks after a successful flee.
// Caller must hold combatMu.
//
// Precondition: combatMu is held; destRoomID non-empty; playerSess.RoomID == destRoomID.
// Postcondition: Pursuing NPCs moved to destRoomID; new combat started if any pursue;
//   returned events are for deferred broadcasting by the caller.
func (h *CombatHandler) resolvePursuitLocked(cbt *combat.Combat, playerSess *session.PlayerSession, playerTotal int, destRoomID string) []*gamev1.CombatEvent {
	var events []*gamev1.CombatEvent
	var pursuers []*npc.Instance

	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindNPC || c.IsDead() {
			continue
		}
		inst, ok := h.npcMgr.Get(c.ID)
		if !ok {
			continue
		}
		pursuitRoll, _ := h.dice.RollExpr("d20")
		pursuitTotal := pursuitRoll.Total() + c.StrMod
		if pursuitTotal >= playerTotal {
			// PURSUIT-2: move NPC; skip if move fails.
			if err := h.npcMgr.Move(c.ID, destRoomID); err != nil {
				events = append(events, &gamev1.CombatEvent{
					Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
					Attacker:  c.Name,
					Narrative: fmt.Sprintf("%s gives chase but loses you!", c.Name),
				})
				continue
			}
			pursuers = append(pursuers, inst)
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
				Attacker:  c.Name,
				Narrative: fmt.Sprintf("%s gives chase! (rolled %d)", c.Name, pursuitTotal),
			})
		} else {
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
				Attacker:  c.Name,
				Narrative: fmt.Sprintf("%s can't keep up and falls behind. (rolled %d)", c.Name, pursuitTotal),
			})
		}
	}

	if len(pursuers) > 0 {
		initEvents, err := h.startPursuitCombatLocked(playerSess, pursuers)
		if err != nil {
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
				Narrative: fmt.Sprintf("Pursuit error: %v", err),
			})
		} else {
			events = append(events, initEvents...)
		}
	}

	return events
}

// startPursuitCombatLocked initiates a new combat in the player's current room
// (the destination after fleeing) with all pursuing NPC instances.
// Caller must hold combatMu. Does NOT call broadcastFn — returns init events for
// deferred broadcasting to avoid deadlock.
//
// Precondition: combatMu is held; playerSess.RoomID is the destination room;
//   insts is non-empty.
// Postcondition: combat registered in engine; StartRound(3) called; timer started.
func (h *CombatHandler) startPursuitCombatLocked(playerSess *session.PlayerSession, insts []*npc.Instance) ([]*gamev1.CombatEvent, error) {
	const dexMod = 1
	var playerAC int
	if h.invRegistry != nil {
		defStats := playerSess.Equipment.ComputedDefensesWithProficienciesAndSetBonuses(h.invRegistry, dexMod, playerSess.Proficiencies, playerSess.Level, playerSess.SetBonusSummary) // REQ-EM-35
		playerAC = 10 + defStats.ACBonus + defStats.EffectiveDex
	} else {
		playerAC = 10 + dexMod
	}

	playerCbt := &combat.Combatant{
		ID:        playerSess.UID,
		Kind:      combat.KindPlayer,
		Name:      playerSess.CharName,
		MaxHP:     playerSess.CurrentHP,
		CurrentHP: playerSess.CurrentHP,
		AC:        playerAC,
		Level:     1,
		StrMod:    2,
		DexMod:    dexMod,
	}

	h.loadoutsMu.Lock()
	if lo, ok := h.loadouts[playerSess.UID]; ok {
		playerCbt.Loadout = lo
	}
	h.loadoutsMu.Unlock()

	// Weapon proficiency rank and damage type — same pattern as startCombatLocked.
	weaponProfRank := "untrained"
	if playerCbt.Loadout != nil && playerCbt.Loadout.MainHand != nil && playerCbt.Loadout.MainHand.Def != nil {
		cat := playerCbt.Loadout.MainHand.Def.ProficiencyCategory
		if r, ok := playerSess.Proficiencies[cat]; ok {
			weaponProfRank = r
		}
	}
	playerCbt.WeaponProficiencyRank = weaponProfRank

	// Wire weapon name and damage type from equipped main-hand weapon.
	if playerCbt.Loadout != nil && playerCbt.Loadout.MainHand != nil && playerCbt.Loadout.MainHand.Def != nil {
		playerCbt.WeaponName = playerCbt.Loadout.MainHand.Def.Name
		playerCbt.WeaponDamageType = playerCbt.Loadout.MainHand.Def.DamageType
	} else {
		playerCbt.WeaponName = "fists"
		playerCbt.WeaponDamageType = "bludgeoning"
	}

	// Resistances / weaknesses.
	playerCbt.Resistances = playerSess.Resistances
	playerCbt.Weaknesses = playerSess.Weaknesses

	// Save mods and proficiency ranks.
	playerCbt.GritMod = combat.AbilityMod(playerSess.Abilities.Grit)
	playerCbt.QuicknessMod = combat.AbilityMod(playerSess.Abilities.Quickness)
	playerCbt.SavvyMod = combat.AbilityMod(playerSess.Abilities.Savvy)
	playerCbt.ToughnessRank = combat.DefaultSaveRank(playerSess.Proficiencies["toughness"])
	playerCbt.HustleRank = combat.DefaultSaveRank(playerSess.Proficiencies["hustle"])
	playerCbt.CoolRank = combat.DefaultSaveRank(playerSess.Proficiencies["cool"])

	// Player starts at row 0 (player side), column 0.
	playerCbt.GridX = 0
	playerCbt.GridY = 0

	combatants := []*combat.Combatant{playerCbt}
	for i, inst := range insts {
		npcWeaponName := ""
		if inst.WeaponID != "" && h.invRegistry != nil {
			if wDef := h.invRegistry.Weapon(inst.WeaponID); wDef != nil {
				npcWeaponName = wDef.Name
			}
		}
		npcCbt := &combat.Combatant{
			ID:          inst.ID,
			Kind:        combat.KindNPC,
			Name:        inst.Name(),
			MaxHP:       inst.MaxHP,
			CurrentHP:   inst.CurrentHP,
			AC:          inst.AC,
			Level:       inst.Level,
			StrMod:      combat.AbilityMod(inst.Awareness),
			DexMod:      1,
			NPCType:     inst.Type,
			Resistances: inst.Resistances,
			Weaknesses:  inst.Weaknesses,
			WeaponName:  npcWeaponName,
			AttackVerb:  inst.AttackVerb,
			// NPC starts at row 9 (NPC side), sequential column per NPC.
			GridX: i,
			GridY: 9,
		}
		combatants = append(combatants, npcCbt)
	}

	combat.RollInitiative(combatants, h.dice.Src())

	var scriptMgr *scripting.Manager
	var zoneID string
	if h.scriptMgr != nil && h.worldMgr != nil {
		scriptMgr = h.scriptMgr
		if room, ok := h.worldMgr.GetRoom(playerSess.RoomID); ok {
			zoneID = room.ZoneID
		}
	}

	cbt, err := h.engine.StartCombat(playerSess.RoomID, combatants, h.condRegistry, scriptMgr, zoneID)
	if err != nil {
		return nil, fmt.Errorf("startPursuitCombatLocked: %w", err)
	}
	playerSess.Status = int32(2) // in combat

	// Apply flat_footed to all pursuing NPC combatants at combat start — same
	// pattern as startCombatLocked.
	if h.condRegistry != nil {
		if def, ok := h.condRegistry.Get("flat_footed"); ok {
			for _, npcCbt := range cbt.Combatants {
				if npcCbt.Kind != combat.KindNPC {
					continue
				}
				if cbt.Conditions[npcCbt.ID] == nil {
					cbt.Conditions[npcCbt.ID] = condition.NewActiveSet()
				}
				_ = cbt.Conditions[npcCbt.ID].Apply(npcCbt.ID, def, 1, 1)
			}
		}
	}

	cbt.SetSessionGetter(func(uid string) (*session.PlayerSession, bool) {
		return h.sessions.GetPlayer(uid)
	})
	cbt.StartRound(3)

	// Fire exploration mode combat-start hook AFTER StartRound so that ACMod
	// applied by Hold Ground (REQ-EXP-11) survives round 1. StartRound resets
	// ACMod to 0 for all combatants; calling applyExploreModeOnCombatStart
	// before StartRound would zero the +2 ACMod immediately (REQ-EXP-13).
	//
	// Run Point timing note (REQ-EXP-25): Run Point applies +1 to ally Initiative
	// values after StartRound. StartRound does NOT re-sort Combatants; the slice
	// order is fixed by RollInitiative at enrollment time. The +1 modifies the
	// Initiative field (used for display and initiative-bonus calculations) but
	// does not change the round turn order for the current combat. This is
	// intentional: the benefit is reflected in the Initiative score visible to
	// players, not in turn ordering within an already-started combat.
	if combatMsgs := applyExploreModeOnCombatStart(playerSess, playerCbt, h); len(combatMsgs) > 0 {
		for _, msg := range combatMsgs {
			h.pushMessageToUID(playerSess.UID, msg)
		}
	}

	var events []*gamev1.CombatEvent
	for _, c := range cbt.Combatants {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
			Attacker:  c.Name,
			Narrative: fmt.Sprintf("%s rolls initiative: %d", c.Name, c.Initiative),
		})
	}
	events = append(events, &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
		Narrative: fmt.Sprintf("Pursuit! Round %d begins!", cbt.Round),
	})

	// Broadcast RoundStartEvent with InitialPositions so the frontend can activate combat mode (REQ-IMR-19).
	if h.roundStartBroadcastFn != nil {
		pursuitTurnOrder := make([]string, 0, len(cbt.Combatants))
		for _, c := range cbt.Combatants {
			pursuitTurnOrder = append(pursuitTurnOrder, c.Name)
		}
		initialPositions := make([]*gamev1.CombatantPosition, 0, len(cbt.Combatants))
		for _, c := range cbt.Combatants {
			initialPositions = append(initialPositions, &gamev1.CombatantPosition{
				Name: c.Name,
				X:    int32(c.GridX),
				Y:    int32(c.GridY),
			})
		}
		h.roundStartBroadcastFn(playerSess.RoomID, &gamev1.RoundStartEvent{
			Round:            int32(cbt.Round),
			ActionsPerTurn:   3,
			DurationMs:       int32(h.roundDuration.Milliseconds()),
			TurnOrder:        pursuitTurnOrder,
			InitialPositions: initialPositions,
		})
	}

	h.startTimerLocked(playerSess.RoomID)
	return events, nil
}

// SetActiveCombatDistance sets the player combatant's grid position so that the computed
// distance to the NPC equals dist.
// Returns an error if the player has no session or is not in active combat.
//
// Precondition: uid must be non-empty; dist >= 0.
// Postcondition: Player combatant's GridX is updated so CombatRange(player, npc) == dist.
func (h *CombatHandler) SetActiveCombatDistance(uid string, dist int) error {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return fmt.Errorf("no active combat in room %q", sess.RoomID)
	}
	playerCbt := cbt.GetCombatant(uid)
	if playerCbt == nil {
		return fmt.Errorf("player %q is not a combatant", uid)
	}
	// Find nearest NPC grid position (default column 5).
	npcGridX := 5
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			npcGridX = c.GridX
			break
		}
	}
	// Place the player on the same row as the NPC, dist/5 columns away.
	// Clamp to zero minimum.
	newGridX := npcGridX - dist/5
	if newGridX < 0 {
		newGridX = 0
	}
	playerCbt.GridX = newGridX
	playerCbt.GridY = 0
	return nil
}

// RegisterLoadout directly registers a pre-built WeaponPreset for uid.
// This is intended for testing and other callers that build a preset outside the
// inventory registry workflow.
//
// Precondition: uid must be non-empty; lo must not be nil.
// Postcondition: The preset is stored; any subsequent attack by uid uses lo.
func (h *CombatHandler) RegisterLoadout(uid string, lo *inventory.WeaponPreset) {
	h.loadoutsMu.Lock()
	h.loadouts[uid] = lo
	h.loadoutsMu.Unlock()
}

// Equip equips weaponID into the given slot for uid.
//
// Precondition: uid must be non-empty; weaponID must identify a registered weapon.
// Postcondition: Returns nil events and nil error on success; the loadout is updated.
func (h *CombatHandler) Equip(uid, weaponID, slotName string) ([]*gamev1.CombatEvent, error) {
	def := h.invRegistry.Weapon(weaponID)
	if def == nil {
		return nil, fmt.Errorf("weapon %q not found", weaponID)
	}

	h.loadoutsMu.Lock()
	lo, ok := h.loadouts[uid]
	if !ok {
		lo = inventory.NewWeaponPreset()
		h.loadouts[uid] = lo
	}
	var equipErr error
	if slotName == string(inventory.SlotSecondary) {
		equipErr = lo.EquipOffHand(def)
	} else {
		equipErr = lo.EquipMainHand(def)
	}
	if equipErr != nil {
		h.loadoutsMu.Unlock()
		return nil, fmt.Errorf("equipping weapon: %w", equipErr)
	}
	h.loadoutsMu.Unlock()

	// If player is in active combat, update their Combatant.Loadout.
	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	// Retrieve session to find roomID.
	sess, ok := h.sessions.GetPlayer(uid)
	if ok {
		if cbt, active := h.engine.GetCombat(sess.RoomID); active {
			if combatant := h.findCombatant(cbt, uid); combatant != nil {
				combatant.Loadout = lo
			}
		}
	}

	return nil, nil
}

// Reload queues ActionReload for uid.
//
// Precondition: uid must be a valid connected player in active combat with a primary weapon equipped.
// Postcondition: Returns resolved events if all actions submitted; nil events otherwise.
func (h *CombatHandler) Reload(uid string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	h.loadoutsMu.Lock()
	lo := h.loadouts[uid]
	h.loadoutsMu.Unlock()

	var weaponID string
	if lo != nil {
		if primary := lo.MainHand; primary != nil {
			weaponID = primary.Def.ID
		}
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionReload, WeaponID: weaponID}); err != nil {
		return nil, fmt.Errorf("queuing reload: %w", err)
	}

	h.pushMessageToUID(uid, fmt.Sprintf("Reload queued.%s", h.formatAPRemaining(uid, cbt)))

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		return h.resolveAndAdvanceLocked(sess.RoomID, cbt), nil
	}

	return nil, nil
}

// FireBurst queues ActionFireBurst for uid against target.
//
// Precondition: uid must be a valid connected player in active combat; target must be non-empty.
// Postcondition: Returns resolved events if all actions submitted; nil events otherwise.
func (h *CombatHandler) FireBurst(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	h.loadoutsMu.Lock()
	lo := h.loadouts[uid]
	h.loadoutsMu.Unlock()

	var weaponID string
	if lo != nil {
		if primary := lo.MainHand; primary != nil {
			weaponID = primary.Def.ID
		}
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionFireBurst, Target: target, WeaponID: weaponID}); err != nil {
		return nil, fmt.Errorf("queuing fire burst: %w", err)
	}

	h.pushMessageToUID(uid, fmt.Sprintf("Burst fire queued.%s", h.formatAPRemaining(uid, cbt)))

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		return h.resolveAndAdvanceLocked(sess.RoomID, cbt), nil
	}

	return nil, nil
}

// FireAutomatic queues ActionFireAutomatic for uid against target.
//
// Precondition: uid must be a valid connected player in active combat; target must be non-empty.
// Postcondition: Returns resolved events if all actions submitted; nil events otherwise.
func (h *CombatHandler) FireAutomatic(uid, target string) ([]*gamev1.CombatEvent, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	h.loadoutsMu.Lock()
	lo := h.loadouts[uid]
	h.loadoutsMu.Unlock()

	var weaponID string
	if lo != nil {
		if primary := lo.MainHand; primary != nil {
			weaponID = primary.Def.ID
		}
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionFireAutomatic, Target: target, WeaponID: weaponID}); err != nil {
		return nil, fmt.Errorf("queuing fire automatic: %w", err)
	}

	h.pushMessageToUID(uid, fmt.Sprintf("Full auto queued.%s", h.formatAPRemaining(uid, cbt)))

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		return h.resolveAndAdvanceLocked(sess.RoomID, cbt), nil
	}

	return nil, nil
}

// Throw queues ActionThrow for uid using explosiveID.
//
// Precondition: uid must be a valid connected player in active combat; explosiveID must identify a registered explosive.
// Postcondition: Returns resolved events if all actions submitted; nil events otherwise.
func (h *CombatHandler) Throw(uid, explosiveID string) ([]*gamev1.CombatEvent, error) {
	if h.invRegistry.Explosive(explosiveID) == nil {
		return nil, fmt.Errorf("explosive %q not found", explosiveID)
	}

	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, fmt.Errorf("you are not in combat")
	}

	if err := cbt.QueueAction(uid, combat.QueuedAction{Type: combat.ActionThrow, ExplosiveID: explosiveID}); err != nil {
		return nil, fmt.Errorf("queuing throw: %w", err)
	}

	h.pushMessageToUID(uid, fmt.Sprintf("Throw queued.%s", h.formatAPRemaining(uid, cbt)))

	if cbt.AllActionsSubmitted() {
		h.stopTimerLocked(sess.RoomID)
		return h.resolveAndAdvanceLocked(sess.RoomID, cbt), nil
	}

	return nil, nil
}

// resolveAndAdvance is the timer-fired entry point. It acquires combatMu, then
// auto-queues any players who have not yet submitted their action (using their
// DefaultCombatAction), and delegates to resolveAndAdvanceLocked.
//
// Precondition: a combat must be active in roomID.
// Postcondition: round events are broadcast; combat is ended or next round is started;
//   SwappedThisRound is reset to false for all player sessions in this combat.
func (h *CombatHandler) resolveAndAdvance(roomID string) {
	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return
	}
	// Auto-queue defaults for any players who did not submit before the timer expired.
	// This must happen here (timer-fired path only) so that players who submit their
	// own action before the timer are not overridden.
	h.autoQueuePlayersLocked(cbt)
	h.resolveAndAdvanceLocked(roomID, cbt)
}

// resolveAndAdvanceLocked resolves the current round and either ends combat or
// starts the next round. Caller must hold combatMu.
//
// Precondition: combatMu is held; cbt is the active Combat for roomID.
// Postcondition: round events are broadcast; combat is ended or next round is started;
//   SwappedThisRound is reset to false for all player sessions in this combat.
func (h *CombatHandler) resolveAndAdvanceLocked(roomID string, cbt *combat.Combat) []*gamev1.CombatEvent {
	targetUpdater := func(id string, hp int) {
		if inst, found := h.npcMgr.Get(id); found {
			inst.CurrentHP = hp
		}
		if sess, found := h.sessions.GetPlayer(id); found {
			sess.CurrentHP = hp
			h.checkHPThresholdFear(id)
		}
	}

	coverDegrader := func(rID, equipID string) bool {
		destroyed := h.DecrementAndCheckDestroyed(rID, equipID)
		if destroyed {
			for _, c := range cbt.Combatants {
				if c.CoverEquipmentID == equipID {
					c.CoverEquipmentID = ""
					c.CoverTier = ""
					if sess, ok := h.sessions.GetPlayer(c.ID); ok && sess.Conditions != nil {
						for _, condID := range []string{"greater_cover", "standard_cover", "lesser_cover"} {
							sess.Conditions.Remove(c.ID, condID)
						}
					}
				}
			}
		}
		return destroyed
	}
	// Build a per-round dispatch wrapper: for each player in this combat, call their stored ReactionFn.
	reactionFn := reaction.ReactionCallback(func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) (bool, error) {
		if sess, ok := h.sessions.GetPlayer(uid); ok && sess.ReactionFn != nil {
			return sess.ReactionFn(uid, trigger, ctx)
		}
		return false, nil
	})
	roundEvents := combat.ResolveRound(cbt, h.dice.Src(), targetUpdater, reactionFn, coverDegrader)

	// REQ-JD-10: Fire on_take_damage_in_one_hit_above_threshold drawback trigger for players
	// that received ≥50% of their max HP in a single hit this round.
	if h.onMassiveDamage != nil {
		for _, ev := range roundEvents {
			if ev.AttackResult == nil {
				continue
			}
			r := ev.AttackResult
			dmg := r.EffectiveDamage()
			if dmg <= 0 {
				continue
			}
			if _, playerOK := h.sessions.GetPlayer(r.TargetID); !playerOK {
				continue
			}
			var maxHP int
			for _, c := range cbt.Combatants {
				if c.ID == r.TargetID {
					maxHP = c.MaxHP
					break
				}
			}
			if maxHP > 0 && dmg*2 >= maxHP {
				h.onMassiveDamage(r.TargetID)
			}
		}
	}

	// REQ-AH-21: Apply poison substance from attacker's weapon to player targets on hit.
	if h.substanceSvc != nil && h.invRegistry != nil {
		for _, ev := range roundEvents {
			if ev.AttackResult == nil {
				continue
			}
			r := ev.AttackResult
			if r.Outcome != combat.Success && r.Outcome != combat.CritSuccess {
				continue
			}
			// Only apply poison when the target is a player.
			if _, playerOK := h.sessions.GetPlayer(r.TargetID); !playerOK {
				continue
			}
			// Determine attacker's weapon ID (NPC or player).
			var weaponDefID string
			if inst, npcOK := h.npcMgr.Get(r.AttackerID); npcOK && inst.WeaponID != "" {
				weaponDefID = inst.WeaponID
			} else if actorSess, playerActorOK := h.sessions.GetPlayer(r.AttackerID); playerActorOK && actorSess.LoadoutSet != nil {
				if preset := actorSess.LoadoutSet.ActivePreset(); preset != nil && preset.MainHand != nil {
					weaponDefID = preset.MainHand.Def.ID
				}
			}
			if weaponDefID == "" {
				continue
			}
			if itemDef, itemOK := h.invRegistry.ItemByWeaponRef(weaponDefID); itemOK && itemDef.PoisonSubstanceID != "" {
				_ = h.substanceSvc.ApplySubstanceByID(r.TargetID, itemDef.PoisonSubstanceID)
			}
		}
	}

	// REQ-NB-13: Set GrudgePlayerID and OnDamageTaken for NPCs hit by players this round.
	for _, ev := range roundEvents {
		if ev.AttackResult == nil || ev.AttackResult.EffectiveDamage() <= 0 {
			continue
		}
		// Actor must be a player; target must be an NPC.
		if _, isPlayer := h.sessions.GetPlayer(ev.ActorID); !isPlayer {
			continue
		}
		if ev.TargetID == "" {
			continue
		}
		if npcInst, found := h.npcMgr.Get(ev.TargetID); found {
			npcInst.GrudgePlayerID = ev.ActorID
			npcInst.OnDamageTaken = true
			if h.onNPCDamageTaken != nil {
				h.onNPCDamageTaken(npcInst.ID)
			}
		}
	}

	// REQ-NB-14: Evaluate FleeHPPct for all living NPCs that took damage this round.
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindNPC || c.IsDead() {
			continue
		}
		npcInst, found := h.npcMgr.Get(c.ID)
		if !found || npcInst == nil || !npcInst.OnDamageTaken || npcInst.FleeHPPct == 0 {
			continue
		}
		maxHP := c.MaxHP
		if maxHP <= 0 {
			maxHP = 1
		}
		hpPct := c.CurrentHP * 100 / maxHP
		if hpPct < npcInst.FleeHPPct {
			npcInst.PendingFlee = true
		}
	}

	// Fire cover crossfire trap callbacks for ActionCoverHit events.
	if h.onCoverHit != nil {
		for _, ev := range roundEvents {
			if ev.ActionType == combat.ActionCoverHit {
				h.onCoverHit(cbt.RoomID, ev.ActorID, ev.CoverEquipmentID)
			}
		}
	}

	// REQ-EM-5/6/10: deduct durability from weapons (per attack) and armor (per hit).
	durRnd := rand.New(rand.NewSource(int64(cbt.Round))) //nolint:gosec
	durRoller := NewDurabilityRoller(rand.NewSource(int64(cbt.Round + 1)))
	ApplyRoundDurability(
		roundEvents,
		func(actorID string) *inventory.EquippedWeapon {
			sess, ok := h.sessions.GetPlayer(actorID)
			if !ok || sess.LoadoutSet == nil {
				return nil
			}
			preset := sess.LoadoutSet.ActivePreset()
			if preset == nil {
				return nil
			}
			return preset.MainHand
		},
		func(targetID string) *inventory.Equipment {
			sess, ok := h.sessions.GetPlayer(targetID)
			if !ok {
				return nil
			}
			return sess.Equipment
		},
		func(targetID string, slot inventory.ArmorSlot) {
			sess, ok := h.sessions.GetPlayer(targetID)
			if !ok {
				return
			}
			if sess.Equipment != nil {
				sess.Equipment.Armor[slot] = nil
			}
		},
		func(actorID, msg string) {
			sess, ok := h.sessions.GetPlayer(actorID)
			if !ok || sess.Entity == nil {
				return
			}
			evt := &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{Content: msg},
				},
			}
			if data, err := proto.Marshal(evt); err == nil {
				_ = sess.Entity.Push(data)
			}
		},
		durRoller,
		durRnd,
	)

	// REQ-AH-21: apply poison substance on weapon hit for player attackers.
	if h.substanceSvc != nil && h.invRegistry != nil {
		for _, re := range roundEvents {
			if re.ActionType != combat.ActionAttack {
				continue
			}
			if re.AttackResult == nil {
				continue
			}
			if re.AttackResult.Outcome != combat.CritSuccess && re.AttackResult.Outcome != combat.Success {
				continue
			}
			if re.TargetID == "" {
				continue
			}
			// Check if the actor is a player with a poisoned weapon.
			actSess, actOK := h.sessions.GetPlayer(re.ActorID)
			if !actOK || actSess.LoadoutSet == nil {
				continue
			}
			preset := actSess.LoadoutSet.ActivePreset()
			if preset == nil || preset.MainHand == nil {
				continue
			}
			// Resolve ItemDefID from the InstanceID in the backpack.
			if preset.MainHand.InstanceID == "" {
				continue
			}
			inst := actSess.Backpack.GetByInstanceID(preset.MainHand.InstanceID)
			if inst == nil {
				continue
			}
			weapItemDef, wOK := h.invRegistry.Item(inst.ItemDefID)
			if !wOK || weapItemDef.PoisonSubstanceID == "" {
				continue
			}
			if err := h.substanceSvc.ApplySubstanceByID(re.TargetID, weapItemDef.PoisonSubstanceID); err != nil {
				if h.logger != nil {
					h.logger.Warn("ApplySubstanceByID failed", zap.Error(err))
				}
			}
		}
	}

	// Reset per-round loadout swap flag for all players in this combat.
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer {
			if sess, found := h.sessions.GetPlayer(c.ID); found {
				if sess.LoadoutSet != nil {
					sess.LoadoutSet.ResetRound()
				}
				// REQ-READY-1: Clear readied action at end of every round; notify player if it expired unfired.
				if sess.ReadiedTrigger != "" && sess.Entity != nil {
					actionName := sess.ReadiedAction
					switch actionName {
					case "raise_shield":
						actionName = "Raise Shield"
					case "step":
						actionName = "Step"
					case "strike":
						actionName = "Strike"
					}
					evt := &gamev1.ServerEvent{
						Payload: &gamev1.ServerEvent_Message{
							Message: &gamev1.MessageEvent{Content: "Your readied " + actionName + " expires. (No refund.)"},
						},
					}
					if data, err := proto.Marshal(evt); err == nil {
						_ = sess.Entity.Push(data)
					}
				}
				clearReadiedAction(sess)
				sess.ReactionsRemaining = 1
			}
		}
	}

	// REQ-ZN-9: charmed NPCs make a Savvy saving throw vs DC 15 at end of each round.
	// On success, the charmed condition is removed from seduceConditions.
	if h.seduceConditions != nil {
		const charmedSaveDC = 15
		for _, c := range cbt.Combatants {
			if c.Kind != combat.KindNPC || c.IsDead() {
				continue
			}
			cs, ok := h.seduceConditions[c.ID]
			if !ok || !cs.Has("charmed") {
				continue
			}
			inst, found := h.npcMgr.Get(c.ID)
			if !found {
				continue
			}
			var roll int
			if h.dice != nil {
				r, err := h.dice.RollExpr("d20")
				if err == nil {
					roll = r.Total()
				} else {
					roll = 10
				}
			} else {
				roll = 10
			}
			total := roll + inst.Savvy
			if total >= charmedSaveDC {
				cs.Remove(c.ID, "charmed")
			}
		}
	}

	// Advance mental state for all players in this combat and collect narrative messages.
	var mentalStateEvents []*gamev1.CombatEvent
	if h.mentalStateMgr != nil {
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindPlayer {
				changes := h.mentalStateMgr.AdvanceRound(c.ID)
				msgs := h.applyMentalStateChanges(c.ID, changes)
				for _, msg := range msgs {
					mentalStateEvents = append(mentalStateEvents, &gamev1.CombatEvent{
						Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
						Narrative: msg,
					})
				}
			}
		}
	}

	var events []*gamev1.CombatEvent
	for _, re := range roundEvents {
		events = append(events, h.roundEventToProto(re))
	}

	// proto has no PASS/ROUND type; ATTACK is the closest available sentinel — client uses Narrative for display
	events = append(events, &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Narrative: fmt.Sprintf("Round %d complete.", cbt.Round),
	})
	events = append(events, mentalStateEvents...)

	if !cbt.HasLivingNPCs() || !cbt.HasUndowedPlayers() {
		var endNarrative string
		if !cbt.HasLivingNPCs() {
			endNarrative = "Combat is over. You stand victorious."
		} else {
			endNarrative = "Everything goes dark."
			// Mark all downed player combatants so sess.Dead == true.
			// This is required for the heropoint stabilize subcommand to work.
			for _, c := range cbt.Combatants {
				if c.Kind == combat.KindPlayer && c.CurrentHP <= 0 {
					if sess, ok := h.sessions.GetPlayer(c.ID); ok {
						sess.Dead = true
					}
				}
			}
			// Rob defeated players before broadcasting the end event.
			robEvents := h.robPlayersLocked(cbt)
			events = append(events, robEvents...)
		}
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_END,
			Narrative: endNarrative,
		})
		h.broadcastFn(roomID, events)
		h.removeDeadNPCsLocked(cbt)
		// REQ-NB-41: set ReturningHome for surviving NPCs not in their home room; clear grudge.
		for _, c := range cbt.Combatants {
			if c.Kind != combat.KindNPC || c.IsDead() {
				continue
			}
			if npcInst, found := h.npcMgr.Get(c.ID); found && npcInst != nil {
				npcInst.GrudgePlayerID = ""
				if npcInst.HomeRoomID != "" && npcInst.RoomID != npcInst.HomeRoomID {
					npcInst.ReturningHome = true
				}
			}
		}
		h.stopTimerLocked(roomID)
		h.engine.EndCombat(roomID)
		h.clearCoweringNPCsLocked(roomID)
		// Collect downed player UIDs for respawn before releasing the lock.
		var downedUIDs []string
		if !cbt.HasLivingNPCs() {
			// Victory — no downed players to respawn.
		} else {
			for _, c := range cbt.Combatants {
				if c.Kind == combat.KindPlayer && c.CurrentHP <= 0 {
					downedUIDs = append(downedUIDs, c.ID)
				}
			}
		}
		// Fire onCombatEndFn in a goroutine: calling it while holding combatMu
		// deadlocks because the callback (pushRoomViewToAllInRoom) re-acquires combatMu.
		// See the same pattern at the flee path (postUnlockFn above).
		if h.onCombatEndFn != nil {
			fn := h.onCombatEndFn
			rid := roomID
			go fn(rid)
		}
		// Fire onPlayerDeath for each downed player after releasing the lock.
		if h.onPlayerDeath != nil && len(downedUIDs) > 0 {
			deathFn := h.onPlayerDeath
			uids := downedUIDs
			go func() {
				for _, uid := range uids {
					deathFn(uid)
				}
			}()
		}
		return events
	}

	h.broadcastFn(roomID, events)

	// Start the next round.
	condEvents := cbt.StartRound(3)
	condCombatEvents := conditionEventsToProto(condEvents, h.condRegistry)

	// Apply MotiveBonus from sense motive critical failures to NPC AttackMod.
	// Precondition: combatMu held; cbt.StartRound has reset per-round state.
	// Postcondition: NPC combatants with MotiveBonus > 0 have AttackMod incremented; MotiveBonus zeroed.
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindNPC {
			continue
		}
		inst, instOK := h.npcMgr.Get(c.ID)
		if !instOK || inst == nil || inst.MotiveBonus <= 0 {
			continue
		}
		c.AttackMod += inst.MotiveBonus
		inst.MotiveBonus = 0
	}

	// Inject banked AP from delayed players into their new ActionQueue.
	// Precondition: combatMu held; cbt.StartRound has reset ActionQueues.
	// Postcondition: Each delayed player's ActionQueue has extra AP added; BankedAP zeroed.
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindPlayer {
			continue
		}
		playerSess, playerOK := h.sessions.GetPlayer(c.ID)
		if !playerOK || playerSess.BankedAP <= 0 {
			continue
		}
		q := cbt.ActionQueues[c.ID]
		if q != nil {
			q.AddAP(playerSess.BankedAP)
		}
		playerSess.BankedAP = 0
	}

	// BUG-31: Send per-player AP notification BEFORE auto-queue consumes AP.
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindPlayer || c.IsDead() {
			continue
		}
		if q, qOK := cbt.ActionQueues[c.ID]; qOK {
			h.pushMessageToUID(c.ID, fmt.Sprintf("You have %d AP this round.", q.RemainingPoints()))
		}
	}

	h.autoQueueNPCsLocked(cbt)

	// Apply per-round drowning damage to any submerged player combatants (TERRAIN-13).
	// Precondition: combatMu is held; cbt is non-nil.
	// Postcondition: Each submerged player session has CurrentHP decremented by 1d6 (min 1).
	var drowningEvents []*gamev1.CombatEvent
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindPlayer {
			continue
		}
		sess, ok := h.sessions.GetPlayer(c.ID)
		if !ok || sess.Conditions == nil || !sess.Conditions.Has("submerged") {
			continue
		}
		dmgResult, _ := h.dice.RollExpr("1d6")
		dmg := dmgResult.Total()
		if dmg < 1 {
			dmg = 1
		}
		sess.CurrentHP -= dmg
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}
		drowningEvents = append(drowningEvents, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
			Attacker:  "Drowning",
			Target:    sess.CharName,
			Damage:    int32(dmg),
			TargetHp:  int32(sess.CurrentHP),
			Narrative: fmt.Sprintf("%s takes %d drowning damage from being submerged!", sess.CharName, dmg),
		})
	}

	// Apply round_start hazards for all players in the room (REQ-AE-29).
	var hazardEvents []*gamev1.CombatEvent
	if h.worldMgr != nil {
		if room, ok := h.worldMgr.GetRoom(roomID); ok && len(room.Hazards) > 0 {
			for _, c := range cbt.Combatants {
				if c.Kind != combat.KindPlayer || c.IsDead() {
					continue
				}
				playerSess, playerOK := h.sessions.GetPlayer(c.ID)
				if !playerOK {
					continue
				}
				ApplyHazards(room, playerSess, "round_start", h.dice, h.condRegistry,
					func(msg string) {
						hazardEvents = append(hazardEvents, &gamev1.CombatEvent{
							Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
							Attacker:  "Hazard",
							Target:    playerSess.CharName,
							Narrative: msg,
						})
					},
					h.logger,
				)
			}
		}
	}

	// Build per-player range status events so each player knows their distance
	// to their primary NPC target at the start of each round (BUG-29).
	// Precondition: cbt.Combatants is non-nil.
	// Postcondition: One INITIATIVE event per living player-NPC pair is appended.
	var rangeEvents []*gamev1.CombatEvent
	for _, pc := range cbt.Combatants {
		if pc.Kind != combat.KindPlayer || pc.IsDead() {
			continue
		}
		npcTarget := h.bestNPCCombatant(cbt)
		if npcTarget == nil {
			continue
		}
		dist := combat.CombatRange(*pc, *npcTarget)
		var rangeLabel string
		if dist <= 5 {
			rangeLabel = fmt.Sprintf("%d ft (melee range)", dist)
		} else {
			rangeLabel = fmt.Sprintf("%d ft (ranged)", dist)
		}
		rangeEvents = append(rangeEvents, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
			Narrative: fmt.Sprintf("Range to %s: %s", npcTarget.Name, rangeLabel),
		})
	}

	roundStartEvents := []*gamev1.CombatEvent{
		{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
			Narrative: fmt.Sprintf("Round %d begins!", cbt.Round),
		},
	}
	roundStartEvents = append(roundStartEvents, rangeEvents...)
	roundStartEvents = append(roundStartEvents, condCombatEvents...)
	roundStartEvents = append(roundStartEvents, drowningEvents...)
	roundStartEvents = append(roundStartEvents, hazardEvents...)
	h.broadcastFn(roomID, roundStartEvents)

	// Broadcast RoundStartEvent with InitialPositions so the frontend can activate/update combat mode (REQ-IMR-19).
	if h.roundStartBroadcastFn != nil {
		nextTurnOrder := make([]string, 0, len(cbt.Combatants))
		for _, c := range cbt.Combatants {
			nextTurnOrder = append(nextTurnOrder, c.Name)
		}
		initialPositions := make([]*gamev1.CombatantPosition, 0, len(cbt.Combatants))
		for _, c := range cbt.Combatants {
			initialPositions = append(initialPositions, &gamev1.CombatantPosition{
				Name: c.Name,
				X:    int32(c.GridX),
				Y:    int32(c.GridY),
			})
		}
		h.roundStartBroadcastFn(roomID, &gamev1.RoundStartEvent{
			Round:            int32(cbt.Round),
			ActionsPerTurn:   3,
			DurationMs:       int32(h.roundDuration.Milliseconds()),
			TurnOrder:        nextTurnOrder,
			InitialPositions: initialPositions,
		})
	}

	h.startTimerLocked(roomID)
	return events
}

// buildPlayerCombatant constructs a *combat.Combatant from a player session, loading
// equipment stats, loadout, proficiency ranks, resistances/weaknesses, and ability mods.
//
// Precondition: sess must be non-nil; h must be non-nil.
// Postcondition: Returns a fully populated *combat.Combatant for the player.
func buildPlayerCombatant(sess *session.PlayerSession, h *CombatHandler) *combat.Combatant {
	// Compute player AC from equipped armor. dexMod is a placeholder until character sheet stats are integrated.
	const dexMod = 1
	var playerAC int
	if h.invRegistry != nil {
		defStats := sess.Equipment.ComputedDefensesWithProficienciesAndSetBonuses(h.invRegistry, dexMod, sess.Proficiencies, sess.Level, sess.SetBonusSummary) // REQ-EM-35
		playerAC = 10 + defStats.ACBonus + defStats.EffectiveDex
	} else {
		playerAC = 10 + dexMod
	}

	playerCbt := &combat.Combatant{
		ID:        sess.UID,
		Kind:      combat.KindPlayer,
		Name:      sess.CharName,
		MaxHP:     sess.MaxHP,
		CurrentHP: sess.CurrentHP,
		AC:        playerAC,
		Level:     1,
		StrMod:    2,
		DexMod:    dexMod,
	}

	h.loadoutsMu.Lock()
	lo, ok := h.loadouts[sess.UID]
	h.loadoutsMu.Unlock()
	if !ok && sess.LoadoutSet != nil {
		lo = sess.LoadoutSet.ActivePreset()
	}
	if lo != nil {
		playerCbt.Loadout = lo
	}

	// Determine weapon proficiency rank from equipped main-hand weapon.
	weaponProfRank := "untrained"
	if playerCbt.Loadout != nil && playerCbt.Loadout.MainHand != nil && playerCbt.Loadout.MainHand.Def != nil {
		cat := playerCbt.Loadout.MainHand.Def.ProficiencyCategory
		if r, ok := sess.Proficiencies[cat]; ok {
			weaponProfRank = r
		}
	}
	playerCbt.WeaponProficiencyRank = weaponProfRank

	// Wire weapon name and damage type from equipped main-hand weapon.
	if playerCbt.Loadout != nil && playerCbt.Loadout.MainHand != nil && playerCbt.Loadout.MainHand.Def != nil {
		playerCbt.WeaponName = playerCbt.Loadout.MainHand.Def.Name
		playerCbt.WeaponDamageType = playerCbt.Loadout.MainHand.Def.DamageType
	} else {
		playerCbt.WeaponName = "fists"
		playerCbt.WeaponDamageType = "bludgeoning"
	}

	// Wire player resistances/weaknesses from equipped armor.
	playerCbt.Resistances = sess.Resistances
	playerCbt.Weaknesses = sess.Weaknesses

	// Wire save ability mods from character ability scores.
	playerCbt.GritMod = combat.AbilityMod(sess.Abilities.Grit)
	playerCbt.QuicknessMod = combat.AbilityMod(sess.Abilities.Quickness)
	playerCbt.SavvyMod = combat.AbilityMod(sess.Abilities.Savvy)

	// Wire save proficiency ranks from session.
	playerCbt.ToughnessRank = combat.DefaultSaveRank(sess.Proficiencies["toughness"])
	playerCbt.HustleRank = combat.DefaultSaveRank(sess.Proficiencies["hustle"])
	playerCbt.CoolRank = combat.DefaultSaveRank(sess.Proficiencies["cool"])

	return playerCbt
}

// startCombatLocked initialises a new combat encounter for sess attacking inst.
// Caller must hold combatMu.
//
// Precondition: combatMu is held; sess and inst must be non-nil.
// Postcondition: combat is registered in the engine; StartRound(3) is called.
func (h *CombatHandler) startCombatLocked(sess *session.PlayerSession, inst *npc.Instance) (*combat.Combat, []*gamev1.CombatEvent, error) {
	playerCbt := buildPlayerCombatant(sess, h)

	// Resolve NPC weapon name for combat narrative.
	npcWeaponName := ""
	if inst.WeaponID != "" && h.invRegistry != nil {
		if wDef := h.invRegistry.Weapon(inst.WeaponID); wDef != nil {
			npcWeaponName = wDef.Name
		}
	}

	// Compute NPC feat bonuses (REQ-AE-16, REQ-AE-17, REQ-AE-11).
	// Target is a player session (not an NPC instance), so target is nil and
	// TargetTags-gated feats are skipped per REQ-AE-11.
	var npcFeatStats NPCEffectiveStats
	if h.featRegistry != nil {
		roomNPCs := h.npcMgr.InstancesInRoom(inst.RoomID)
		npcFeatStats = ComputeNPCAttackStats(inst, nil, h.featRegistry, roomNPCs)
	}

	// StrMod serves as both the attack modifier and damage modifier for NPCs.
	// feat bonuses from brutal_strike (+2 damage) and pack_tactics (+2 attack)
	// are both added to StrMod since it feeds both the attack roll and the damage roll.
	npcStrMod := combat.AbilityMod(inst.Awareness) + npcFeatStats.DamageBonus + npcFeatStats.AttackBonus

	npcCbt := &combat.Combatant{
		ID:          inst.ID,
		Kind:        combat.KindNPC,
		Name:        inst.Name(),
		MaxHP:       inst.MaxHP,
		CurrentHP:   inst.CurrentHP,
		AC:          inst.AC + npcFeatStats.ACBonus,
		Level:       inst.Level,
		StrMod:      npcStrMod,
		DexMod:      1,
		NPCType:     inst.Type,
		Resistances: inst.Resistances,
		Weaknesses:  inst.Weaknesses,
		WeaponName:  npcWeaponName,
		AttackVerb:  inst.AttackVerb,
	}

	combatants := []*combat.Combatant{playerCbt, npcCbt}
	combat.RollInitiative(combatants, h.dice.Src())

	var scriptMgr *scripting.Manager
	var zoneID string
	if h.scriptMgr != nil && h.worldMgr != nil {
		scriptMgr = h.scriptMgr
		if room, ok := h.worldMgr.GetRoom(sess.RoomID); ok {
			zoneID = room.ZoneID
		}
	}
	// Player starts at row 0 (player side), column 0.
	playerCbt.GridX = 0
	playerCbt.GridY = 0
	// NPC starts at row 9 (NPC side), column 5 (middle).
	npcCbt.GridX = 5
	npcCbt.GridY = 9
	cbt, err := h.engine.StartCombat(sess.RoomID, combatants, h.condRegistry, scriptMgr, zoneID)
	if err != nil {
		return nil, nil, fmt.Errorf("starting combat: %w", err)
	}
	sess.Status = int32(2) // gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT

	// Auto-join group members in the same room.
	roomID := sess.RoomID
	if group := h.sessions.GroupByUID(sess.UID); group != nil {
		for _, memberUID := range group.MemberUIDs {
			if memberUID == sess.UID {
				continue
			}
			memberSess, ok := h.sessions.GetPlayer(memberUID)
			if !ok {
				continue
			}
			if memberSess.Status == statusInCombat {
				continue
			}
			memberCbt := buildPlayerCombatant(memberSess, h)
			combat.RollInitiative([]*combat.Combatant{memberCbt}, h.dice.Src())
			if memberSess.RoomID == roomID {
				// Assign sequential X column on row 0 (player side).
				existingPlayers := 0
				for _, c := range cbt.Combatants {
					if c.Kind == combat.KindPlayer {
						existingPlayers++
					}
				}
				memberCbt.GridX = existingPlayers
				memberCbt.GridY = 0
				if addErr := h.engine.AddCombatant(roomID, memberCbt); addErr != nil {
					h.logger.Warn("auto-join group member failed",
						zap.String("uid", memberUID),
						zap.Error(addErr),
					)
					continue
				}
				memberSess.Status = statusInCombat
				joinMsg := fmt.Sprintf("Your group entered combat! You join the fight (initiative %d).", memberCbt.Initiative)
				joinEvt := &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_Message{
						Message: &gamev1.MessageEvent{Content: joinMsg},
					},
				}
				if memberSess.Entity != nil {
					if data, marshalErr := proto.Marshal(joinEvt); marshalErr == nil {
						_ = memberSess.Entity.Push(data)
					}
				}
			} else {
				notifyMsg := "Your group is under attack!"
				notifyEvt := &gamev1.ServerEvent{
					Payload: &gamev1.ServerEvent_Message{
						Message: &gamev1.MessageEvent{Content: notifyMsg},
					},
				}
				if memberSess.Entity != nil {
					if data, marshalErr := proto.Marshal(notifyEvt); marshalErr == nil {
						_ = memberSess.Entity.Push(data)
					}
				}
			}
		}
	}

	// Apply flat_footed to all NPC combatants at combat start (sucker_punch window).
	if h.condRegistry != nil {
		if def, ok := h.condRegistry.Get("flat_footed"); ok {
			if cbt.Conditions[npcCbt.ID] == nil {
				cbt.Conditions[npcCbt.ID] = condition.NewActiveSet()
			}
			_ = cbt.Conditions[npcCbt.ID].Apply(npcCbt.ID, def, 1, 1)
		}
	}

	// Register session getter so ResolveRound can look up passive feats.
	cbt.SetSessionGetter(func(uid string) (*session.PlayerSession, bool) {
		return h.sessions.GetPlayer(uid)
	})

	initCondEvents := cbt.StartRound(3)
	_ = initCondEvents // round 1 starts with no active conditions; events are empty

	// Fire exploration mode combat-start hook AFTER StartRound so that ACMod
	// applied by Hold Ground (REQ-EXP-11) survives round 1. StartRound resets
	// ACMod to 0 for all combatants; calling applyExploreModeOnCombatStart
	// before StartRound would zero the +2 ACMod immediately (REQ-EXP-13).
	//
	// Run Point timing note (REQ-EXP-25): see the single-player combat path above
	// for a full explanation. The +1 Initiative modifies the ally's Initiative field
	// (display/calculation) but does not re-sort the Combatants slice.
	if combatMsgs := applyExploreModeOnCombatStart(sess, playerCbt, h); len(combatMsgs) > 0 {
		for _, msg := range combatMsgs {
			h.pushMessageToUID(sess.UID, msg)
		}
	}

	// Build initiative events.
	var events []*gamev1.CombatEvent
	for _, c := range cbt.Combatants {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
			Attacker:  c.Name,
			Narrative: fmt.Sprintf("%s rolls initiative: %d", c.Name, c.Initiative),
		})
	}

	// Announce initiative bonus if the player won.
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindPlayer && c.InitiativeBonus > 0 {
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
				Attacker:  c.Name,
				Narrative: fmt.Sprintf("You win initiative! +%d to attack and AC this combat.", c.InitiativeBonus),
			})
		}
	}

	turnOrder := make([]string, 0, len(cbt.Combatants))
	for _, c := range cbt.Combatants {
		turnOrder = append(turnOrder, c.Name)
	}

	events = append(events, &gamev1.CombatEvent{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
		Narrative: fmt.Sprintf("Round %d begins! Turn order: %v", cbt.Round, turnOrder),
	})

	// BUG-29: Include range info at combat start so the player sees distance on round 1.
	for _, pc := range cbt.Combatants {
		if pc.Kind != combat.KindPlayer || pc.IsDead() {
			continue
		}
		npcTarget := h.bestNPCCombatant(cbt)
		if npcTarget == nil {
			continue
		}
		dist := combat.CombatRange(*pc, *npcTarget)
		var rangeLabel string
		if dist <= 5 {
			rangeLabel = fmt.Sprintf("%d ft (melee range)", dist)
		} else {
			rangeLabel = fmt.Sprintf("%d ft (ranged)", dist)
		}
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
			Narrative: fmt.Sprintf("Range to %s: %s", npcTarget.Name, rangeLabel),
		})
	}

	// Trigger flee/cower for all non-combat NPCs in the room.
	h.applyCombatStartBehaviorsLocked(sess.RoomID)

	// Include initial position events so the battle map populates immediately.
	for _, c := range cbt.Combatants {
		events = append(events, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_POSITION,
			Attacker:  c.Name,
			AttackerX: int32(c.GridX),
			AttackerY: int32(c.GridY),
		})
	}

	// Broadcast RoundStartEvent with InitialPositions so the frontend can activate combat mode (REQ-IMR-19).
	if h.roundStartBroadcastFn != nil {
		initialPositions := make([]*gamev1.CombatantPosition, 0, len(cbt.Combatants))
		for _, c := range cbt.Combatants {
			initialPositions = append(initialPositions, &gamev1.CombatantPosition{
				Name: c.Name,
				X:    int32(c.GridX),
				Y:    int32(c.GridY),
			})
		}
		h.roundStartBroadcastFn(sess.RoomID, &gamev1.RoundStartEvent{
			Round:            int32(cbt.Round),
			ActionsPerTurn:   3,
			DurationMs:       int32(h.roundDuration.Milliseconds()),
			TurnOrder:        turnOrder,
			InitialPositions: initialPositions,
		})
	}

	return cbt, events, nil
}

// defaultCombatResponse returns "flee", "cower", or "engage" for a non-combat NPC.
//
// Personality "cowardly" always maps to "flee"; "brave" always maps to "cower".
// "neutral", "opportunistic", and empty all fall through to the type-specific default.
//
// Type defaults:
//
//	merchant, quest_giver, job_trainer → "cower"
//	healer, banker, crafter           → "flee"
//	guard                             → "engage"
//	hireling                          → "engage"
func defaultCombatResponse(npcType, personality string) string {
	switch personality {
	case "cowardly":
		return "flee"
	case "brave":
		return "cower"
	// "neutral", "opportunistic", and "" fall through to type default.
	}
	switch npcType {
	case "merchant", "quest_giver", "job_trainer":
		return "cower"
	case "healer", "banker", "crafter":
		return "flee"
	case "guard":
		return "engage"
	case "hireling":
		return "engage"
	default:
		return "cower"
	}
}

// applyCombatStartBehaviorsLocked fires flee/cower for all non-combat NPCs in roomID
// when combat starts. Must be called with h.combatMu already held.
//
// Precondition: combatMu is held; roomID is non-empty.
// Postcondition: Each non-combat NPC in roomID is either moved to an adjacent room or has Cowering set to true.
func (h *CombatHandler) applyCombatStartBehaviorsLocked(roomID string) {
	var room *world.Room
	if h.worldMgr != nil {
		r, ok := h.worldMgr.GetRoom(roomID)
		if ok {
			room = r
		}
	}
	for _, inst := range h.npcMgr.InstancesInRoom(roomID) {
		if inst.NPCType == "" || inst.NPCType == "combat" || inst.NPCType == "guard" || inst.NPCType == "hireling" {
			continue
		}
		switch defaultCombatResponse(inst.NPCType, inst.Personality) {
		case "flee":
			if room != nil {
				h.fleeNPCLocked(inst, room)
			} else {
				inst.Cowering = true
			}
		default: // "cower"
			inst.Cowering = true
		}
	}
}

// fleeNPCLocked moves inst to a random valid adjacent room.
// A valid exit is non-hidden, non-locked, and does not lead to an AllOutWar room.
// Falls back to cower if no valid exits exist.
// Must be called with h.combatMu held.
//
// Precondition: combatMu is held; inst and room are non-nil.
// Postcondition: inst is moved to a valid adjacent room, or inst.Cowering is set to true.
func (h *CombatHandler) fleeNPCLocked(inst *npc.Instance, room *world.Room) {
	// REQ-NHN-22: Immobile NPCs must never be moved.
	if inst.Immobile {
		inst.Cowering = true
		return
	}
	var validExits []world.Exit
	for _, exit := range room.Exits {
		if exit.Hidden || exit.Locked {
			continue
		}
		if dest, ok := h.worldMgr.GetRoom(exit.TargetRoom); ok {
			dl := danger.EffectiveDangerLevel(dest.ZoneID, dest.DangerLevel)
			if dl == danger.AllOutWar {
				continue
			}
		}
		validExits = append(validExits, exit)
	}
	if len(validExits) == 0 {
		inst.Cowering = true
		return
	}
	target := validExits[rand.Intn(len(validExits))]
	_ = h.npcMgr.Move(inst.ID, target.TargetRoom)
}

// clearCoweringNPCsLocked resets the Cowering flag for all NPCs in roomID.
// Must be called when combat in roomID ends.
//
// Precondition: combatMu is held; roomID is non-empty.
// Postcondition: All NPCs in roomID have Cowering set to false.
func (h *CombatHandler) clearCoweringNPCsLocked(roomID string) {
	for _, inst := range h.npcMgr.InstancesInRoom(roomID) {
		if inst.Cowering {
			inst.Cowering = false
		}
	}
}

// autoQueuePlayersLocked queues default actions for all living players in cbt
// who have not yet submitted their action queue for this round.
//
// Precondition: h.combatMu is held; cbt must not be nil.
// Postcondition: Each unsubmitted player receives their DefaultCombatAction
// queued against the best available target, and is notified via their entity.
func (h *CombatHandler) autoQueuePlayersLocked(cbt *combat.Combat) {
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindPlayer || c.IsDead() {
			continue
		}
		q, ok := cbt.ActionQueues[c.ID]
		if !ok {
			continue
		}

		sess, ok := h.sessions.GetPlayer(c.ID)
		if !ok {
			continue
		}

		forcedAction := condition.ForcedActionType(sess.Conditions)
		if forcedAction == "" && len(q.QueuedActions()) > 0 {
			continue // player submitted and no forced override
		}

		if forcedAction != "" {
			q.ClearActions()
			var forcedTarget string
			switch forcedAction {
			case "random_attack":
				// Collect all alive combatants (any faction, including the acting player).
				// Self-targeting is intentional — panic causes indiscriminate lashing out.
				var targets []string
				for _, combatant := range cbt.Combatants {
					if !combatant.IsDead() {
						targets = append(targets, combatant.Name)
					}
				}
				if len(targets) > 0 {
					forcedTarget = targets[h.dice.Src().Intn(len(targets))]
				}
			case "lowest_hp_attack":
				lowestHP := int(^uint(0) >> 1) // MaxInt
				for _, combatant := range cbt.Combatants {
					if !combatant.IsDead() && combatant.CurrentHP < lowestHP {
						lowestHP = combatant.CurrentHP
						forcedTarget = combatant.Name
					}
				}
			}
			if forcedTarget == "" {
				forcedTarget = c.Name
			}
			if err := cbt.QueueAction(c.ID, combat.QueuedAction{Type: combat.ActionAttack, Target: forcedTarget}); err != nil {
				continue
			}
			var msg string
			switch forcedAction {
			case "random_attack":
				msg = fmt.Sprintf("Panic grips you — you lash out wildly at %s!", forcedTarget)
			case "lowest_hp_attack":
				msg = fmt.Sprintf("Berserker rage drives you to destroy the weakest target — you attack %s!", forcedTarget)
			default:
				h.logger.Warn("unknown forced action type — no player notification sent", zap.String("forced_action", forcedAction), zap.String("uid", c.ID))
			}
			notifyEvt := &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{
						Content: msg,
						Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
					},
				},
			}
			if data, marshalErr := proto.Marshal(notifyEvt); marshalErr == nil {
				_ = sess.Entity.Push(data)
			}
			continue
		}

		// Resolve target: prefer LastCombatTarget if that NPC is still alive in the room.
		var targetName string
		if sess.LastCombatTarget != "" {
			for _, combatant := range cbt.Combatants {
				if combatant.Kind == combat.KindNPC && !combatant.IsDead() && combatant.Name == sess.LastCombatTarget {
					targetName = combatant.Name
					break
				}
			}
		}
		// Fallback: first living NPC in combat.
		if targetName == "" {
			for _, combatant := range cbt.Combatants {
				if combatant.Kind == combat.KindNPC && !combatant.IsDead() {
					targetName = combatant.Name
					break
				}
			}
		}

		// If a condition requires skipping the turn entirely, force a pass and notify.
		if condition.SkipTurn(sess.Conditions) {
			if err := cbt.QueueAction(c.ID, combat.QueuedAction{Type: combat.ActionPass}); err != nil {
				continue
			}
			skipEvt := &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{
						Content: "You are overwhelmed and cannot act this round.",
						Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
					},
				},
			}
			if data, marshalErr := proto.Marshal(skipEvt); marshalErr == nil {
				_ = sess.Entity.Push(data)
			}
			continue
		}

		// Determine the action type from DefaultCombatAction.
		action := sess.DefaultCombatAction
		if action == "" {
			action = "pass"
		}

		var qa combat.QueuedAction
		switch action {
		case "attack":
			qa = combat.QueuedAction{Type: combat.ActionAttack, Target: targetName}
		case "strike":
			qa = combat.QueuedAction{Type: combat.ActionStrike, Target: targetName}
		default:
			qa = combat.QueuedAction{Type: combat.ActionPass}
		}

		if err := cbt.QueueAction(c.ID, qa); err != nil {
			continue
		}

		// Notify player of the auto-queued action.
		narrative := fmt.Sprintf("[Auto] Your default action: %s", action)
		if targetName != "" && action != "pass" {
			narrative = fmt.Sprintf("[Auto] Your default action: %s → %s", action, targetName)
		}
		notifyEvt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{
					Content: narrative,
					Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
				},
			},
		}
		if data, marshalErr := proto.Marshal(notifyEvt); marshalErr == nil {
			_ = sess.Entity.Push(data)
		}
	}
}

// FilterAnimalPlanActions removes "say" actions from the plan when isAnimal is true.
// Animals cannot speak, so any HTN "say" action must be suppressed.
//
// Precondition: actions may be nil (treated as empty).
// Postcondition: Returns the original slice unmodified when isAnimal is false;
// returns a new slice with "say" actions removed when isAnimal is true.
func FilterAnimalPlanActions(actions []ai.PlannedAction, isAnimal bool) []ai.PlannedAction {
	if !isAnimal {
		return actions
	}
	filtered := actions[:0:0]
	for _, a := range actions {
		if a.Action != "say" {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// autoQueueNPCsLocked queues actions for all living NPCs using the HTN planner
// when available, falling back to a simple attack for NPCs without an AI domain.
//
// Precondition: h.combatMu is held; cbt must not be nil.
func (h *CombatHandler) autoQueueNPCsLocked(cbt *combat.Combat) {
	// Decrement ability cooldowns for all living NPCs before planning.
	// Also evaluate boss abilities for boss-tier NPCs (REQ-AE-35).
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindNPC || c.IsDead() {
			continue
		}
		if inst, ok := h.npcMgr.Get(c.ID); ok {
			for k := range inst.AbilityCooldowns {
				if inst.AbilityCooldowns[k] > 0 {
					inst.AbilityCooldowns[k]--
				}
			}
			// Evaluate boss abilities at the start of each NPC's round.
			if inst.Tier == "boss" {
				h.evaluateBossAbilitiesLocked(inst, cbt.RoomID, cbt.Round, false)
			}
		}
	}

	// Always decrement zone effect cooldowns for all living players each round
	// (regardless of current room's effects — prevents gaming immunity by room-swapping).
	if h.mentalStateMgr != nil && h.worldMgr != nil {
		for _, c := range cbt.Combatants {
			if c.Kind != combat.KindPlayer || c.IsDead() {
				continue
			}
			sess, ok := h.sessions.GetPlayer(c.ID)
			if !ok {
				continue
			}
			for k := range sess.ZoneEffectCooldowns {
				if sess.ZoneEffectCooldowns[k] > 0 {
					sess.ZoneEffectCooldowns[k]--
				}
				if sess.ZoneEffectCooldowns[k] < 0 {
					sess.ZoneEffectCooldowns[k] = 0
				}
			}
		}
		// Apply room effects only if current room has effects.
		if zoneRoom, ok := h.worldMgr.GetRoom(cbt.RoomID); ok {
			roomEffects := zoneRoom.Effects
			if h.weatherMgr != nil {
				roomEffects = append(roomEffects, h.weatherMgr.ActiveEffects(zoneRoom.Indoor)...)
			}
			for _, c := range cbt.Combatants {
				if c.Kind != combat.KindPlayer || c.IsDead() {
					continue
				}
				sess, ok := h.sessions.GetPlayer(c.ID)
				if !ok {
					continue
				}
				// Check each room effect.
				for _, effect := range roomEffects {
					key := cbt.RoomID + ":" + effect.Track
					if sess.ZoneEffectCooldowns[key] > 0 {
						continue // immune
					}
					track, trackOK := abilityTrack(effect.Track)
					sev, sevOK := abilitySeverity(effect.Severity)
					if !trackOK || !sevOK {
						continue
					}
					// Binary Will save: d20 + GritMod vs BaseDC (no proficiency bonus).
					roll := h.dice.Src().Intn(20) + 1
					total := roll + c.GritMod
					if total < effect.BaseDC {
						// Failed save — apply trigger; no cooldown set.
						changes := h.mentalStateMgr.ApplyTrigger(c.ID, track, sev)
						h.applyMentalStateChanges(c.ID, changes)
					} else {
						// Successful save — set cooldown immunity.
						if sess.ZoneEffectCooldowns == nil {
							sess.ZoneEffectCooldowns = make(map[string]int64)
						}
						sess.ZoneEffectCooldowns[key] = int64(effect.CooldownRounds)
					}
				}
			}
		}
	}

	// Fetch room once for NPC auto-cover checks.
	var room *world.Room
	if h.worldMgr != nil {
		if r, ok := h.worldMgr.GetRoom(cbt.RoomID); ok {
			room = r
		}
	}

	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindNPC || c.IsDead() {
			continue
		}

		// REQ-ZN-10: charmed NPCs treat players as allied — skip their entire action queue.
		if h.seduceConditions != nil {
			if cs, ok := h.seduceConditions[c.ID]; ok && cs.Has("charmed") {
				continue
			}
		}

		// NPC-initiated seduction (Phase 2): neutral NPCs with SeductionProbability > 0
		// may attempt to seduce player combatants at the start of their turn.
		if h.condRegistry != nil {
			if inst, ok := h.npcMgr.Get(c.ID); ok && inst.SeductionProbability > 0 && inst.Disposition != "hostile" {
				seducedDef, hasDef := h.condRegistry.Get("seduced")
				if hasDef {
					for _, pc := range cbt.Combatants {
						if pc.Kind != combat.KindPlayer || pc.IsDead() {
							continue
						}
						if inst.SeductionRejected != nil && inst.SeductionRejected[pc.ID] {
							continue
						}
						sess, ok := h.sessions.GetPlayer(pc.ID)
						if !ok {
							continue
						}
						if h.ResolveNPCSeductionGenderCheck(inst, pc.ID, sess.Gender) {
							continue
						}
						roll := h.dice.Src().Intn(20) + 1
						if float64(roll) > inst.SeductionProbability*20 {
							continue
						}
						npcRoll := h.dice.Src().Intn(20) + 1
						playerRoll := h.dice.Src().Intn(20) + 1
						if cbt.Conditions[pc.ID] == nil {
							cbt.Conditions[pc.ID] = condition.NewActiveSet()
						}
						h.ResolveNPCSeductionContest(inst, pc.ID, sess.Abilities.Savvy, seducedDef, cbt.Conditions[pc.ID], npcRoll, playerRoll)
					}
				}
			}
		}

		// Auto-use-cover: apply cover at start of NPC turn when strategy enables it
		// and the NPC is not already in cover.
		if c.CoverTier == "" {
			if inst, ok := h.npcMgr.Get(c.ID); ok && inst.UseCover && room != nil {
				if bestEquip, bestTier := bestCoverInRoom(room); bestTier != "" {
					c.CoverEquipmentID = bestEquip.ItemID
					c.CoverTier = bestTier
					condID := bestTier + "_cover"
					if h.condRegistry != nil {
						if def, ok := h.condRegistry.Get(condID); ok {
							if cbt.Conditions[c.ID] == nil {
								cbt.Conditions[c.ID] = condition.NewActiveSet()
							}
							_ = cbt.Conditions[c.ID].Apply(c.ID, def, 1, -1)
						}
					}
					if bestEquip.CoverDestructible && bestEquip.CoverHP > 0 && h.GetCoverHP(cbt.RoomID, bestEquip.ItemID) < 0 {
						h.InitCoverState(cbt.RoomID, bestEquip.ItemID, bestEquip.CoverHP)
					}
				}
			}
		}

		// Attempt HTN planning.
		if h.aiRegistry != nil {
			inst, ok := h.npcMgr.Get(c.ID)
			if ok && inst.AIDomain != "" {
				if planner, ok := h.aiRegistry.PlannerFor(inst.AIDomain); ok {
					zoneID := h.zoneIDForRoom(cbt.RoomID)
					ws := ai.BuildCombatWorldState(cbt, inst, zoneID)
					actions, err := planner.Plan(ws)
					if err == nil {
						actions = FilterAnimalPlanActions(actions, inst.IsAnimal())
						if len(actions) > 0 {
							h.applyPlanLocked(cbt, c, actions)
							h.maybeBroadcastTauntLocked(cbt, c)
							continue
						}
						// Empty plan after filtering (e.g. animal with only say tasks):
						// fall through to legacyAutoQueueLocked below.
					}
				}
			}
		}
		// Fallback: attack first living player.
		h.legacyAutoQueueLocked(cbt, c)
		h.maybeBroadcastTauntLocked(cbt, c)
	}
}

// applyPlanLocked converts PlannedActions to QueuedActions and enqueues them.
//
// Precondition: h.combatMu is held.
// Postcondition: actions queued until AP budget exhausted.
func (h *CombatHandler) applyPlanLocked(cbt *combat.Combat, actor *combat.Combatant, actions []ai.PlannedAction) {
	// REQ-NB-7B, REQ-NB-14: Evaluate pending flee from FleeHPPct threshold.
	if npcInst, found := h.npcMgr.Get(actor.ID); found && npcInst != nil && npcInst.PendingFlee {
		npcInst.PendingFlee = false
		// Insert flee action at front so it executes before any other planned actions.
		actions = append([]ai.PlannedAction{{Action: "flee", OperatorID: "__flee_threshold"}}, actions...)
	}
	// Prepend stride action if the NPC needs to close or open range.
	if strideDir := h.npcMovementStrideLocked(cbt, actor); strideDir != "" {
		cbt.QueueAction(actor.ID, combat.QueuedAction{Type: combat.ActionStride, Direction: strideDir})
	}
	for _, a := range actions {
		var qa combat.QueuedAction
		switch a.Action {
		case "attack":
			qa = combat.QueuedAction{Type: combat.ActionAttack, Target: a.Target}
		case "strike":
			qa = combat.QueuedAction{Type: combat.ActionStrike, Target: a.Target}
		case "pass":
			qa = combat.QueuedAction{Type: combat.ActionPass}
		case "apply_mental_state":
			// Resolve target selector.
			targetUID := h.resolveAbilityTarget(cbt, a.Target)
			if targetUID == "" {
				continue // no valid target; no AP deducted, no cooldown set
			}

			// Parse track and severity.
			track, ok := abilityTrack(a.Track)
			if !ok {
				if h.logger != nil {
					h.logger.Warn("apply_mental_state: unknown track", zap.String("track", a.Track))
				}
				continue
			}
			sev, ok := abilitySeverity(a.Severity)
			if !ok {
				if h.logger != nil {
					h.logger.Warn("apply_mental_state: unknown severity", zap.String("severity", a.Severity))
				}
				continue
			}

			// Look up NPC instance; skip if not found.
			inst, ok := h.npcMgr.Get(actor.ID)
			if !ok {
				if h.logger != nil {
					h.logger.Warn("apply_mental_state: NPC instance not found", zap.String("id", actor.ID))
				}
				continue // no AP deducted
			}

			// Cooldown gate — reading a nil map returns zero value (safe).
			if inst.AbilityCooldowns[a.OperatorID] > 0 {
				continue // still on cooldown; no AP deducted
			}

			// Apply mental state trigger.
			if h.mentalStateMgr != nil {
				changes := h.mentalStateMgr.ApplyTrigger(targetUID, track, sev)
				msgs := h.applyMentalStateChanges(targetUID, changes)
				for _, msg := range msgs {
					h.pushMessageToUID(targetUID, msg)
				}
			}

			// Push taunt message to target.
			taunt := h.pickTaunt(inst)
			if taunt != "" {
				h.pushMessageToUID(targetUID, taunt)
			}

			// Set cooldown (lazy-initialize map on first write).
			if inst.AbilityCooldowns == nil {
				inst.AbilityCooldowns = make(map[string]int)
			}
			inst.AbilityCooldowns[a.OperatorID] = a.CooldownRounds

			// Deduct AP: queue APCost pass actions (each costs 1 AP slot).
			apCost := a.APCost
			if apCost == 0 {
				apCost = 1
			}
			for i := 0; i < apCost; i++ {
				if err := cbt.QueueAction(actor.ID, combat.QueuedAction{Type: combat.ActionPass}); err != nil {
					return // AP budget exhausted
				}
			}
			continue
		case "say":
			if len(a.Strings) == 0 {
				continue
			}
			inst, ok := h.npcMgr.Get(actor.ID)
			if !ok {
				continue
			}
			if inst.AbilityCooldowns == nil {
				inst.AbilityCooldowns = make(map[string]int)
			}
			if inst.AbilityCooldowns[a.OperatorID] > 0 {
				continue
			}
			if a.Cooldown != "" {
				if d, err := time.ParseDuration(a.Cooldown); err == nil && d > 0 {
					// Compute tick count from cooldown duration and idle tick interval. REQ-NB-2.
					ticks := 1
					if h.npcIdleTickInterval > 0 {
						ticks = int(d / h.npcIdleTickInterval)
						if ticks < 1 {
							ticks = 1
						}
					}
					inst.AbilityCooldowns[a.OperatorID] = ticks
				}
			}
			line := a.Strings[rand.Intn(len(a.Strings))]
			h.broadcastFn(cbt.RoomID, []*gamev1.CombatEvent{{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
				Narrative: fmt.Sprintf("%s says \"%s\"", actor.Name, line),
			}})
			continue
		case "flee":
			// REQ-NB-25: NPC flees combat, exits to a random adjacent room. REQ-NB-26, REQ-NB-27.
			if h.worldMgr == nil {
				continue
			}
			room, roomOK := h.worldMgr.GetRoom(cbt.RoomID)
			if !roomOK {
				continue
			}
			exits := room.VisibleExits()
			if len(exits) == 0 {
				// REQ-NB-28: no visible exits → fail silently; NPC stays in combat.
				continue
			}
			// Remove NPC from combatants before moving. REQ-NB-26.
			cbt.RemoveCombatant(actor.ID)
			target := exits[rand.Intn(len(exits))]
			_ = h.npcMgr.Move(actor.ID, target.TargetRoom)
			h.broadcastFn(cbt.RoomID, []*gamev1.CombatEvent{{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_FLEE,
				Narrative: fmt.Sprintf("%s flees!", actor.Name),
			}})
			// REQ-NB-27: remaining participants continue combat.
			return

		case "target_weakest":
			// REQ-NB-29, REQ-NB-30: queue attack against lowest HP% living player.
			// REQ-NB-31: silently skip when fewer than 2 living players exist.
			var livingPlayers []*combat.Combatant
			for _, c := range cbt.Combatants {
				if c.Kind == combat.KindPlayer && !c.IsDead() {
					livingPlayers = append(livingPlayers, c)
				}
			}
			if len(livingPlayers) < 2 {
				continue
			}
			weakest := livingPlayers[0]
			for _, p := range livingPlayers[1:] {
				weakestPct := float64(0)
				if weakest.MaxHP > 0 {
					weakestPct = float64(weakest.CurrentHP) / float64(weakest.MaxHP)
				}
				pPct := float64(0)
				if p.MaxHP > 0 {
					pPct = float64(p.CurrentHP) / float64(p.MaxHP)
				}
				if pPct < weakestPct {
					weakest = p
				}
			}
			qa = combat.QueuedAction{Type: combat.ActionAttack, Target: weakest.Name}

		case "call_for_help":
			// REQ-NB-32: recruit adjacent idle hostile NPCs of same type. REQ-NB-35: once per combat.
			callerInst, callerOK := h.npcMgr.Get(actor.ID)
			if !callerOK {
				continue
			}
			if callerInst.AbilityCooldowns == nil {
				callerInst.AbilityCooldowns = make(map[string]int)
			}
			const callForHelpKey = "__call_for_help_used"
			if callerInst.AbilityCooldowns[callForHelpKey] > 0 {
				continue
			}
			callerInst.AbilityCooldowns[callForHelpKey] = 999999 // permanent until respawn

			if h.worldMgr == nil {
				continue
			}
			room, roomOK := h.worldMgr.GetRoom(cbt.RoomID)
			if !roomOK {
				continue
			}
			exits := room.VisibleExits()
			type recruitEntry struct {
				inst   *npc.Instance
				roomID string
			}
			var recruits []recruitEntry
			for _, exit := range exits {
				for _, candidate := range h.npcMgr.InstancesInRoom(exit.TargetRoom) {
					if candidate.Type != actor.NPCType {
						continue
					}
					if candidate.Disposition != "hostile" {
						continue
					}
					if h.engine.IsNPCInCombat(candidate.ID) {
						continue
					}
					recruits = append(recruits, recruitEntry{inst: candidate, roomID: exit.TargetRoom})
				}
			}
			if len(recruits) == 0 {
				continue
			}
			h.broadcastFn(cbt.RoomID, []*gamev1.CombatEvent{{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
				Narrative: fmt.Sprintf("%s calls for help!", actor.Name),
			}})
			for _, r := range recruits {
				_ = h.npcMgr.Move(r.inst.ID, cbt.RoomID)
				// REQ-NB-34: join on next tick via PendingJoinCombatRoomID.
				r.inst.PendingJoinCombatRoomID = cbt.RoomID
				// COMBATMSG-4f: store recruiter name for "protecting [NPC name]" message.
				r.inst.ProtectedNPCName = actor.Name
			}
			continue

		default:
			qa = combat.QueuedAction{Type: combat.ActionPass}
		}
		if err := cbt.QueueAction(actor.ID, qa); err != nil {
			break // AP budget exhausted
		}
	}
}

// resolveAbilityTarget resolves a target selector to a living player UID.
// Returns "" if no valid target exists.
func (h *CombatHandler) resolveAbilityTarget(cbt *combat.Combat, selector string) string {
	switch selector {
	case "nearest_enemy":
		for _, c := range cbt.Combatants {
			if c.Kind == combat.KindPlayer && !c.IsDead() {
				return c.ID
			}
		}
	case "lowest_hp_enemy":
		var best *combat.Combatant
		for _, c := range cbt.Combatants {
			if c.Kind != combat.KindPlayer || c.IsDead() {
				continue
			}
			if best == nil || c.CurrentHP < best.CurrentHP {
				best = c
			}
		}
		if best != nil {
			return best.ID
		}
	case "highest_damage_enemy":
		var bestUID string
		bestDmg := -1
		for _, c := range cbt.Combatants {
			if c.Kind != combat.KindPlayer || c.IsDead() {
				continue
			}
			dmg := cbt.DamageDealt[c.ID]
			if dmg > bestDmg {
				bestUID = c.ID
				bestDmg = dmg
			}
		}
		return bestUID
	}
	return ""
}

// abilityTrack converts a string track name to a mentalstate.Track.
func abilityTrack(s string) (mentalstate.Track, bool) {
	switch s {
	case "rage":
		return mentalstate.TrackRage, true
	case "despair":
		return mentalstate.TrackDespair, true
	case "delirium":
		return mentalstate.TrackDelirium, true
	case "fear":
		return mentalstate.TrackFear, true
	}
	return 0, false
}

// abilitySeverity converts a string severity name to a mentalstate.Severity.
func abilitySeverity(s string) (mentalstate.Severity, bool) {
	switch s {
	case "mild":
		return mentalstate.SeverityMild, true
	case "moderate":
		return mentalstate.SeverityMod, true
	case "severe":
		return mentalstate.SeveritySevere, true
	}
	return 0, false
}

// pickTaunt returns a random taunt from the NPC's taunt list, or a generic fallback.
func (h *CombatHandler) pickTaunt(inst *npc.Instance) string {
	if len(inst.Taunts) > 0 {
		return inst.Taunts[rand.Intn(len(inst.Taunts))]
	}
	return fmt.Sprintf("The %s unsettles you.", inst.Name())
}

// maybeBroadcastTauntLocked broadcasts a random taunt from the NPC with 25% probability.
// Precondition: h.combatMu is held; c must be a living NPC combatant.
func (h *CombatHandler) maybeBroadcastTauntLocked(cbt *combat.Combat, c *combat.Combatant) {
	inst, ok := h.npcMgr.Get(c.ID)
	if !ok || len(inst.Taunts) == 0 {
		return
	}
	if rand.Float64() >= 0.25 {
		return
	}
	taunt := inst.Taunts[rand.Intn(len(inst.Taunts))]
	h.broadcastFn(cbt.RoomID, []*gamev1.CombatEvent{{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Narrative: fmt.Sprintf("%s: \"%s\"", inst.Name(), taunt),
	}})
}

// npcMovementStrideLocked returns the stride direction needed for the NPC combatant
// based on its current weapon type and distance to the nearest living player.
//
// Rules:
//   - Melee weapon (RangeIncrement == 0): stride "toward" when distance > 5 feet.
//   - Ranged weapon (RangeIncrement > 0): stride "away" when distance <= 5 feet.
//   - Returns "" when no stride is needed.
//
// Precondition: h.combatMu is held; c must not be nil.
// Postcondition: Returns "toward", "away", or "".
func (h *CombatHandler) npcMovementStrideLocked(cbt *combat.Combat, c *combat.Combatant) string {
	isRanged := false
	if inst, ok := h.npcMgr.Get(c.ID); ok && inst.WeaponID != "" && h.invRegistry != nil {
		if wDef := h.invRegistry.Weapon(inst.WeaponID); wDef != nil && wDef.RangeIncrement > 0 {
			isRanged = true
		}
	}
	playerDist := 25 // fallback if no living player found
	for _, comb := range cbt.Combatants {
		if comb.Kind == combat.KindPlayer && !comb.IsDead() {
			playerDist = combat.CombatRange(*c, *comb)
			break
		}
	}
	if !isRanged && playerDist > 5 {
		return "toward"
	}
	if isRanged && playerDist <= 5 && playerDist < combat.MaxCombatRange {
		return "away"
	}
	return ""
}

// legacyAutoQueueLocked queues ActionAttack for c targeting the first living player.
// Prepends an ActionStride when movement is needed based on weapon type and distance.
func (h *CombatHandler) legacyAutoQueueLocked(cbt *combat.Combat, c *combat.Combatant) {
	// REQ-ZN-10: charmed NPCs treat players as allied — do not attack.
	if h.seduceConditions != nil {
		if cs, ok := h.seduceConditions[c.ID]; ok && cs.Has("charmed") {
			return
		}
	}

	if dir := h.npcMovementStrideLocked(cbt, c); dir != "" {
		_ = cbt.QueueAction(c.ID, combat.QueuedAction{Type: combat.ActionStride, Direction: dir})
	}
	for _, combatant := range cbt.Combatants {
		if combatant.Kind == combat.KindPlayer && !combatant.IsDead() {
			_ = cbt.QueueAction(c.ID, combat.QueuedAction{
				Type:   combat.ActionAttack,
				Target: combatant.Name,
			})
			return
		}
	}
}

// bestCoverInRoom returns the highest-tier cover equipment in the room.
// Returns a zero RoomEquipmentConfig and empty string if no cover equipment is present.
//
// Precondition: room must not be nil.
// Postcondition: Returns the RoomEquipmentConfig with the highest CoverTier, or (zero, "").
func bestCoverInRoom(room *world.Room) (world.RoomEquipmentConfig, string) {
	bestTier := ""
	var bestEquip world.RoomEquipmentConfig
	for _, eq := range room.Equipment {
		if eq.CoverTier == "" {
			continue
		}
		if coverTierRank(eq.CoverTier) > coverTierRank(bestTier) {
			bestTier = eq.CoverTier
			bestEquip = eq
		}
	}
	return bestEquip, bestTier
}

// zoneIDForRoom looks up the zone ID for a room via the world manager.
func (h *CombatHandler) zoneIDForRoom(roomID string) string {
	if h.worldMgr == nil {
		return ""
	}
	room, ok := h.worldMgr.GetRoom(roomID)
	if !ok {
		return ""
	}
	return room.ZoneID
}

// startTimerLocked starts or replaces the round timer for roomID.
// Caller must hold combatMu. The timer callback acquires combatMu independently.
//
// Precondition: combatMu is held; roomID must be non-empty.
// Postcondition: A running RoundTimer is stored for roomID.
func (h *CombatHandler) startTimerLocked(roomID string) {
	h.timersMu.Lock()
	if existing, ok := h.timers[roomID]; ok {
		existing.Stop()
	}
	h.timers[roomID] = combat.NewRoundTimer(h.roundDuration, func() {
		h.resolveAndAdvance(roomID)
	})
	h.timersMu.Unlock()
}

// stopTimerLocked stops and removes the round timer for roomID.
// Caller must hold combatMu.
//
// Precondition: combatMu is held; roomID must be non-empty.
// Postcondition: No running timer for roomID remains.
func (h *CombatHandler) stopTimerLocked(roomID string) {
	h.timersMu.Lock()
	if t, ok := h.timers[roomID]; ok {
		t.Stop()
		delete(h.timers, roomID)
	}
	h.timersMu.Unlock()
}

// IsRoomInCombat reports whether roomID currently has an active combat round timer.
// Safe to call from any goroutine.
//
// Postcondition: Returns true if and only if a running timer exists for roomID.
func (h *CombatHandler) IsRoomInCombat(roomID string) bool {
	h.timersMu.Lock()
	_, ok := h.timers[roomID]
	h.timersMu.Unlock()
	return ok
}

// cancelTimer stops and removes the round timer for roomID without requiring
// combatMu to be held. Safe to call from tests or external code.
//
// Postcondition: No running timer for roomID remains.
func (h *CombatHandler) cancelTimer(roomID string) {
	h.timersMu.Lock()
	if t, ok := h.timers[roomID]; ok {
		t.Stop()
		delete(h.timers, roomID)
	}
	h.timersMu.Unlock()
}

// roundEventToProto converts a combat.RoundEvent to a gamev1.CombatEvent.
//
// Postcondition: Returns a non-nil CombatEvent.
func (h *CombatHandler) roundEventToProto(re combat.RoundEvent) *gamev1.CombatEvent {
	if re.AttackResult == nil {
		return &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
			Attacker:  re.ActorName,
			Narrative: re.Narrative,
		}
	}

	r := re.AttackResult
	dmg := r.EffectiveDamage()

	evt := &gamev1.CombatEvent{
		Type:        gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
		Attacker:    re.ActorName,
		AttackRoll:  int32(r.AttackRoll),
		AttackTotal: int32(r.AttackTotal),
		Outcome:     r.Outcome.String(),
		Damage:      int32(dmg),
		Narrative:   re.Narrative,
	}

	// Resolve target name and HP from managers.
	if r.TargetID != "" {
		if inst, ok := h.npcMgr.Get(r.TargetID); ok {
			evt.Target = inst.Name()
			evt.TargetHp = int32(inst.CurrentHP)
			evt.TargetMaxHp = int32(inst.MaxHP)
		}
		if sess, ok := h.sessions.GetPlayer(r.TargetID); ok {
			evt.Target = sess.CharName
			evt.TargetHp = int32(sess.CurrentHP)
			evt.TargetMaxHp = int32(sess.MaxHP)
		}
	}

	return evt
}

// findCombatant returns the Combatant with the given id, or nil.
func (h *CombatHandler) findCombatant(cbt *combat.Combat, id string) *combat.Combatant {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// bestNPCCombatant returns the living NPC combatant with the highest StrMod.
func (h *CombatHandler) bestNPCCombatant(cbt *combat.Combat) *combat.Combatant {
	var best *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC && !c.IsDead() {
			if best == nil || c.StrMod > best.StrMod {
				best = c
			}
		}
	}
	return best
}

// removeCombatant sets a combatant's HP to 0, marking them as dead/removed.
//
// Setting CurrentHP=0 marks the combatant as dead so ResolveRound skips them
// in subsequent resolution passes. This is safe because the entire Flee path
// holds combatMu; any pending timer callback will no-op because GetCombat
// returns false after EndCombat is called.
func (h *CombatHandler) removeCombatant(cbt *combat.Combat, id string) {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			c.CurrentHP = 0
			c.Dead = true
			return
		}
	}
}

// removeDeadNPCsLocked removes all dead NPC combatants from npcMgr and
// schedules their respawn via respawnMgr.
// Caller must hold combatMu.
//
// Precondition: combatMu is held; cbt must not be nil.
// Postcondition: dead NPC instances are removed from npcMgr; respawn
// entries are enqueued in respawnMgr when respawnMgr is non-nil.
func (h *CombatHandler) removeDeadNPCsLocked(cbt *combat.Combat) {
	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindNPC || !c.IsDead() {
			continue
		}
		inst, ok := h.npcMgr.Get(c.ID)
		if !ok {
			continue
		}
		templateID := inst.TemplateID
		roomID := inst.RoomID
		// Generate loot from NPC's loot table before removal so that
		// the instance data is still accessible and removal serves as
		// a happens-before signal for tests polling npcMgr.Get.
		if inst.Loot != nil {
			var result npc.LootResult
			switch {
			case inst.IsAnimal():
				result = npc.GenerateOrganicLoot(*inst.Loot)
			case inst.IsRobot():
				salvage := npc.GenerateSalvageLoot(*inst.Loot)
				std := npc.GenerateLoot(*inst.Loot)
				result.Currency = std.Currency
				result.Items = append(salvage.Items, std.Items...)
			default:
				result = npc.GenerateLoot(*inst.Loot)
			}
			// Distribute currency equally among all living participants.
			totalCurrency := result.Currency + inst.Currency
			inst.Currency = 0
			livingParticipants := h.livingParticipantSessions(cbt)
			h.distributeCurrencyLocked(context.Background(), livingParticipants, totalCurrency)
			if totalCurrency > 0 {
				h.pushCurrencyMessages(livingParticipants, totalCurrency, c.Name)
			}
			// Grant items directly to each living participant's backpack (REQ-BUG96-1).
			if len(result.Items) > 0 {
				h.distributeItemsLocked(livingParticipants, result.Items)
				h.pushLootMessages(livingParticipants, result.Items)
			}
			// Materials still drop to the floor (not directly granted).
			if h.floorMgr != nil && len(result.Materials) > 0 {
				h.floorMgr.DropMaterials(roomID, result.Materials)
			}
		} else if inst.Currency > 0 {
			// No loot table but NPC has rob wallet — distribute to living participants.
			totalCurrency := inst.Currency
			inst.Currency = 0
			livingParticipants := h.livingParticipantSessions(cbt)
			h.distributeCurrencyLocked(context.Background(), livingParticipants, totalCurrency)
			h.pushCurrencyMessages(livingParticipants, totalCurrency, c.Name)
		}
		// Announce NPC death in the console.
		deathNarrative := fmt.Sprintf("%s is dead!", c.Name)
		if inst.IsMachine() {
			deathNarrative = fmt.Sprintf("%s is destroyed.", c.Name)
		}
		h.broadcastFn(inst.RoomID, []*gamev1.CombatEvent{{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH,
			Attacker:  c.Name,
			Narrative: deathNarrative,
		}})

		// Award kill XP split among all living participants.
		if h.xpSvc != nil {
			cfg := h.xpSvc.Config()
			livingParticipants := h.livingParticipantSessions(cbt)
			if len(livingParticipants) > 0 {
				effectiveTier := inst.Tier
			if effectiveTier == "" {
				effectiveTier = "standard"
			}
			totalXP := inst.Level * cfg.Awards.KillXPPerNPCLevel
			if mult, ok := cfg.TierMultipliers[effectiveTier]; ok {
				totalXP = int(math.Ceil(float64(totalXP) * mult.XP))
			}
				share := totalXP / len(livingParticipants)
				if share == 0 && totalXP > 0 {
					p := livingParticipants[0]
					xpMsgs, xpErr := h.xpSvc.AwardXPAmount(context.Background(), p, p.CharacterID, 1)
					if xpErr == nil {
						h.pushXPMessages(p, xpMsgs, 1, c.Name)
					} else if h.logger != nil {
						h.logger.Warn("AwardXPAmount failed",
							zap.String("uid", p.UID),
							zap.Error(xpErr),
						)
					}
				} else {
					for _, p := range livingParticipants {
						xpMsgs, xpErr := h.xpSvc.AwardXPAmount(context.Background(), p, p.CharacterID, share)
						if xpErr == nil {
							h.pushXPMessages(p, xpMsgs, share, c.Name)
						} else if h.logger != nil {
							h.logger.Warn("AwardXPAmount failed",
								zap.String("uid", p.UID),
								zap.Error(xpErr),
							)
						}
					}
				}
			}
		}

		// Award boss kill bonus XP to all living participants when a boss-tier NPC dies (REQ-AE-22).
		if h.xpSvc != nil {
			cfg := h.xpSvc.Config()
			bonusXP := cfg.Awards.BossKillBonusXP
			if bonusXP > 0 {
				tierForBonus := inst.Tier
				if tierForBonus == "" {
					tierForBonus = "standard"
				}
				if tierForBonus == "boss" {
					allRoomPlayers := h.sessions.PlayersInRoomDetails(roomID)
					for _, p := range allRoomPlayers {
						xpMsgs, xpErr := h.xpSvc.AwardXPAmount(context.Background(), p, p.CharacterID, bonusXP)
						if xpErr == nil {
							h.pushXPMessages(p, xpMsgs, bonusXP, "boss kill bonus")
						} else if h.logger != nil {
							h.logger.Warn("boss kill bonus XP failed",
								zap.String("uid", p.UID),
								zap.Error(xpErr),
							)
						}
					}
				}
			}
		}

		// Award faction rep for killing enemy faction NPC (REQ-FA-20).
		if h.factionSvc != nil && h.factionConfig != nil && inst.FactionID != "" {
			livingParticipants := h.livingParticipantSessions(cbt)
			for _, p := range livingParticipants {
				if p.FactionID != "" && h.factionSvc.IsHostile(inst.FactionID, p.FactionID) {
					repAmt := inst.Level * h.factionConfig.RepPerNPCLevel
					if repAmt > 0 {
						msg, repErr := h.factionSvc.AwardRep(context.Background(), p, p.CharacterID, p.FactionID, repAmt)
						if repErr != nil && h.logger != nil {
							h.logger.Warn("faction.AwardRep failed", zap.Error(repErr))
						} else if msg != "" {
							h.broadcastFn(inst.RoomID, []*gamev1.CombatEvent{{
								Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
								Narrative: msg,
							}})
						}
					}
				}
			}
		}

		// Record kill progress for all living quest participants (REQ-QU-19).
		if h.questSvc != nil {
			for _, p := range h.livingParticipantSessions(cbt) {
				questMsgs, questErr := h.questSvc.RecordKill(context.Background(), p, p.CharacterID, templateID)
				if questErr != nil && h.logger != nil {
					h.logger.Warn("RecordKill failed", zap.String("uid", p.UID), zap.Error(questErr))
				}
				h.pushQuestMessages(p, questMsgs)
			}
		}

		// Remove cannot fail: Get confirmed existence above, and combatMu prevents concurrent removal.
		_ = h.npcMgr.Remove(c.ID)
		if h.onNPCDeath != nil {
			h.onNPCDeath(c.ID)
		}
		if h.respawnMgr != nil {
			delay := h.respawnMgr.ResolvedDelay(templateID, roomID)
			h.respawnMgr.Schedule(templateID, roomID, time.Now(), delay)
		}
	}
}

// livingParticipantSessions returns []*session.PlayerSession for all combat participants
// whose Dead field is false, in Combatants order.
//
// Precondition: combatMu is held; cbt must not be nil.
// Postcondition: Returns a non-nil (possibly empty) slice of living player sessions.
func (h *CombatHandler) livingParticipantSessions(cbt *combat.Combat) []*session.PlayerSession {
	participantSet := make(map[string]bool, len(cbt.Participants))
	for _, uid := range cbt.Participants {
		participantSet[uid] = true
	}
	var result []*session.PlayerSession
	for _, c := range cbt.Combatants {
		if !participantSet[c.ID] || c.IsDead() {
			continue
		}
		if sess, ok := h.sessions.GetPlayer(c.ID); ok {
			result = append(result, sess)
		}
	}
	return result
}

// distributeCurrencyLocked distributes totalCurrency equally among livingParticipants.
// When share == 0 (more participants than currency units), only the first participant
// receives 1 unit. SaveCurrency errors are logged as warnings and do not propagate.
//
// Precondition: combatMu is held.
// Postcondition: Each participant's Currency is incremented by their share; persisted when currencySaver is non-nil.
func (h *CombatHandler) distributeCurrencyLocked(ctx context.Context, livingParticipants []*session.PlayerSession, totalCurrency int) {
	if totalCurrency == 0 || len(livingParticipants) == 0 {
		return
	}
	share := totalCurrency / len(livingParticipants)
	if share == 0 {
		livingParticipants[0].Currency++
		if h.currencySaver != nil {
			if err := h.currencySaver.SaveCurrency(ctx, livingParticipants[0].CharacterID, livingParticipants[0].Currency); err != nil && h.logger != nil {
				h.logger.Warn("SaveCurrency failed (share=0 fallback)",
					zap.String("uid", livingParticipants[0].UID),
					zap.Error(err),
				)
			}
		}
		return
	}
	for _, p := range livingParticipants {
		p.Currency += share
		if h.currencySaver != nil {
			if err := h.currencySaver.SaveCurrency(ctx, p.CharacterID, p.Currency); err != nil && h.logger != nil {
				h.logger.Warn("SaveCurrency failed",
					zap.String("uid", p.UID),
					zap.Error(err),
				)
			}
		}
	}
}

// pushXPMessages sends XP narrative messages to sess after an AwardXPAmount call.
//
// Precondition: sess must not be nil; xpAmount must be >= 0.
// Postcondition: XP grant message and any level-up messages are pushed to the player's entity stream.
func (h *CombatHandler) pushXPMessages(sess *session.PlayerSession, levelMsgs []string, xpAmount int, npcName string) {
	xpGrantEvt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: fmt.Sprintf("You gain %d XP for killing %s.", xpAmount, npcName),
				Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
			},
		},
	}
	if data, marshalErr := proto.Marshal(xpGrantEvt); marshalErr == nil {
		_ = sess.Entity.Push(data)
	}
	for _, msg := range levelMsgs {
		xpEvt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{
					Content: msg,
					Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
				},
			},
		}
		if data, marshalErr := proto.Marshal(xpEvt); marshalErr == nil {
			_ = sess.Entity.Push(data)
		}
	}
	if len(levelMsgs) > 0 {
		ciEvt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_CharacterInfo{
				CharacterInfo: &gamev1.CharacterInfo{
					CurrentHp: int32(sess.CurrentHP),
					MaxHp:     int32(sess.MaxHP),
				},
			},
		}
		if data, marshalErr := proto.Marshal(ciEvt); marshalErr == nil {
			_ = sess.Entity.Push(data)
		}
	}
	// Push an updated CharacterSheetView so the web UI Stats tab reflects the new XP total.
	if h.pushCharacterSheetFn != nil {
		h.pushCharacterSheetFn(sess)
	}
}

// pushQuestMessages sends quest completion messages to the player's event stream.
//
// Precondition: sess must not be nil.
// Postcondition: each message in msgs is pushed as a MessageEvent to sess.Entity.
func (h *CombatHandler) pushQuestMessages(sess *session.PlayerSession, msgs []string) {
	for _, msg := range msgs {
		evt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{
					Content: msg,
					Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
				},
			},
		}
		if data, marshalErr := proto.Marshal(evt); marshalErr == nil {
			_ = sess.Entity.Push(data)
		}
	}
}

// pushLootMessages sends a loot notification message to each living combat participant
// listing the items that were dropped by the defeated NPC.
//
// Precondition: livingParticipants must not be nil; items must be non-empty.
// Postcondition: Each participant whose Entity is non-nil receives one MessageEvent
// with content "You looted: <name> (xN), ..." using display names from invRegistry
// when available, falling back to ItemDefID when the registry is nil or the item is unknown.
// distributeItemsLocked adds each loot item directly to every living participant's
// backpack, persists via saveInventoryFn, and pushes an InventoryView refresh.
//
// Precondition: livingParticipants and items must both be non-empty.
// Postcondition: Each participant's Backpack contains the looted items; persistence
// and UI refresh are attempted for each participant (errors are logged, not returned).
func (h *CombatHandler) distributeItemsLocked(livingParticipants []*session.PlayerSession, items []npc.LootItem) {
	if len(items) == 0 || len(livingParticipants) == 0 || h.invRegistry == nil {
		return
	}
	for _, p := range livingParticipants {
		if p.Backpack == nil {
			continue
		}
		for _, lootItem := range items {
			if _, err := p.Backpack.Add(lootItem.ItemDefID, lootItem.Quantity, h.invRegistry); err != nil {
				if h.logger != nil {
					h.logger.Warn("distributeItemsLocked: failed to add item to backpack",
						zap.String("uid", p.UID),
						zap.String("item", lootItem.ItemDefID),
						zap.Error(err),
					)
				}
			}
		}
		if h.saveInventoryFn != nil {
			if err := h.saveInventoryFn(p); err != nil && h.logger != nil {
				h.logger.Warn("distributeItemsLocked: save inventory failed",
					zap.String("uid", p.UID),
					zap.Error(err),
				)
			}
		}
		if h.pushInventoryFn != nil {
			h.pushInventoryFn(p)
		}
	}
}

func (h *CombatHandler) pushLootMessages(livingParticipants []*session.PlayerSession, items []npc.LootItem) {
	if len(items) == 0 || len(livingParticipants) == 0 {
		return
	}
	// Build display names for each item.
	parts := make([]string, 0, len(items))
	for _, lootItem := range items {
		name := lootItem.ItemDefID
		if h.invRegistry != nil {
			if def, ok := h.invRegistry.Item(lootItem.ItemDefID); ok {
				name = def.Name
			}
		}
		if lootItem.Quantity > 1 {
			parts = append(parts, fmt.Sprintf("%s (x%d)", name, lootItem.Quantity))
		} else {
			parts = append(parts, name)
		}
	}
	content := "You looted: " + strings.Join(parts, ", ")
	for _, p := range livingParticipants {
		if p.Entity == nil {
			continue
		}
		evt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{
					Content: content,
					Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
				},
			},
		}
		if data, marshalErr := proto.Marshal(evt); marshalErr == nil {
			_ = p.Entity.Push(data)
		}
	}
}

// pushCurrencyMessages sends a currency grant notification and an InventoryView refresh
// to each living combat participant after currency loot is distributed.
//
// Precondition: livingParticipants must not be nil; totalCurrency must be > 0.
// Postcondition: Each participant whose Entity is non-nil receives one MessageEvent
// describing their individual currency share, and pushInventoryFn is called so the
// web UI Inventory tab reflects the updated balance.
func (h *CombatHandler) pushCurrencyMessages(livingParticipants []*session.PlayerSession, totalCurrency int, npcName string) {
	if totalCurrency == 0 || len(livingParticipants) == 0 {
		return
	}
	share := totalCurrency / len(livingParticipants)
	if share == 0 {
		share = 1
	}
	for _, p := range livingParticipants {
		if p.Entity == nil {
			continue
		}
		evt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{
					Content: fmt.Sprintf("You gain %s from %s.", inventory.FormatCrypto(share), npcName),
					Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
				},
			},
		}
		if data, marshalErr := proto.Marshal(evt); marshalErr == nil {
			_ = p.Entity.Push(data)
		}
		if h.pushInventoryFn != nil {
			h.pushInventoryFn(p)
		}
	}
}

// robPlayersLocked executes the robbery sequence when all players are defeated.
// For each living NPC with RobPercent > 0, a fraction of each dead player's
// remaining currency is transferred to the NPC's Currency wallet.
// Rob messages are returned as CombatEvents for broadcast.
//
// Precondition: combatMu is held; cbt must not be nil.
// Postcondition: Each living robbing NPC has inst.Currency incremented by stolen amount;
// each dead player session has Currency decremented by same; returned events contain
// one narrative event per robbery that occurred.
func (h *CombatHandler) robPlayersLocked(cbt *combat.Combat) []*gamev1.CombatEvent {
	var events []*gamev1.CombatEvent
	var robbedSessions []*session.PlayerSession

	for _, c := range cbt.Combatants {
		if c.Kind != combat.KindNPC || c.IsDead() {
			continue
		}
		inst, ok := h.npcMgr.Get(c.ID)
		if !ok || inst.RobPercent <= 0 {
			continue
		}
		for _, pc := range cbt.Combatants {
			if pc.Kind != combat.KindPlayer || !pc.IsDead() {
				continue
			}
			sess, ok := h.sessions.GetPlayer(pc.ID)
			if !ok {
				continue
			}
			stolen := int(math.Floor(float64(sess.Currency) * inst.RobPercent / 100.0))
			if stolen <= 0 {
				continue
			}
			inst.Currency += stolen
			sess.Currency -= stolen
			robbedSessions = append(robbedSessions, sess)
			events = append(events, &gamev1.CombatEvent{
				Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
				Narrative: fmt.Sprintf("The %s rifles through your pockets, taking %d rounds.", inst.Name(), stolen),
			})
		}
	}

	// Persist updated currency, deduplicating by UID.
	if h.currencySaver != nil {
		seen := make(map[string]bool)
		for _, sess := range robbedSessions {
			if seen[sess.UID] {
				continue
			}
			seen[sess.UID] = true
			if saveErr := h.currencySaver.SaveCurrency(context.Background(), sess.CharacterID, sess.Currency); saveErr != nil && h.logger != nil {
				h.logger.Warn("robPlayersLocked: SaveCurrency failed",
					zap.String("uid", sess.UID),
					zap.Int64("character_id", sess.CharacterID),
					zap.Error(saveErr),
				)
			}
		}
	}
	return events
}

// conditionEventsToProto converts a slice of RoundConditionEvents into CombatEvents
// for broadcast using the narrative channel.
//
// Precondition: reg must not be nil.
// Postcondition: Returns one CombatEvent per RoundConditionEvent.
func conditionEventsToProto(events []combat.RoundConditionEvent, reg *condition.Registry) []*gamev1.CombatEvent {
	result := make([]*gamev1.CombatEvent, 0, len(events))
	for _, ce := range events {
		def, _ := reg.Get(ce.ConditionID)
		name := ce.ConditionID
		if def != nil {
			name = def.Name
		}
		var narrative string
		if ce.Applied {
			narrative = fmt.Sprintf("%s is now %s (stacks: %d).", ce.Name, name, ce.Stacks)
		} else {
			narrative = fmt.Sprintf("%s fades from %s.", name, ce.Name)
		}
		result = append(result, &gamev1.CombatEvent{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_CONDITION,
			Narrative: narrative,
		})
	}
	return result
}

// IsInCombat returns true when the NPC with npcID is currently in an active combat.
//
// Postcondition: equivalent to engine.IsNPCInCombat(npcID).
func (h *CombatHandler) IsInCombat(npcID string) bool {
	return h.engine.IsNPCInCombat(npcID)
}

// FightingTargetName returns the name of the player combatant this NPC is currently fighting,
// or "" if the NPC is not in active combat.
//
// Precondition: npcID must be non-empty.
// Postcondition: Returns the player's Name, or "" if not found.
func (h *CombatHandler) FightingTargetName(npcID string) string {
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	for _, cbt := range h.engine.AllCombats() {
		for _, c := range cbt.Combatants {
			if c.ID == npcID {
				for _, other := range cbt.Combatants {
					if other.Kind == combat.KindPlayer {
						return other.Name
					}
				}
				return ""
			}
		}
	}
	return ""
}

// ActiveCombatForRoom returns the active combat in roomID, or nil if none.
//
// Precondition: roomID must be non-empty.
// Postcondition: Returns a non-nil *combat.Combat if active; nil otherwise.
func (h *CombatHandler) ActiveCombatForRoom(roomID string) *combat.Combat {
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(roomID)
	if !ok {
		return nil
	}
	return cbt
}

// ActiveCombatForPlayer returns the active combat in the player's current room, or nil if none.
//
// Precondition: uid must be a valid connected player.
// Postcondition: Returns a non-nil *combat.Combat if an active combat exists in the player's room; nil otherwise.
func (h *CombatHandler) ActiveCombatForPlayer(uid string) *combat.Combat {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil
	}
	return cbt
}

// DisarmNPC clears the weapon from the NPC combatant identified by npcInstID
// in the active combat for the room where uid is fighting.
// Returns the weapon item ID that was cleared (empty string if NPC was already unarmed).
//
// Precondition: uid must be in active combat; npcInstID must be a valid combatant ID.
// Postcondition: WeaponName is cleared on the combatant; WeaponID is cleared on the NPC instance.
func (h *CombatHandler) DisarmNPC(uid, npcInstID string) (weaponItemID string, err error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return "", fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return "", fmt.Errorf("player %q is not in active combat", uid)
	}

	// Clear WeaponName on the combatant.
	for _, c := range cbt.Combatants {
		if c.ID == npcInstID {
			c.WeaponName = ""
			break
		}
	}

	// Clear WeaponID on the NPC instance; capture old value.
	inst, found := h.npcMgr.Get(npcInstID)
	if found {
		weaponItemID = inst.WeaponID
		inst.WeaponID = ""
	}

	return weaponItemID, nil
}

// ShoveNPC adjusts the NPC combatant's grid position by the given push distance (in feet).
// The direction of the push is away from the player along the X axis.
//
// Precondition: uid must be a valid connected player in active combat; npcInstID must be a combatant in that combat.
// Postcondition: NPC GridX is increased by pushFt/5 cells if NPC.GridX >= player.GridX, otherwise decreased (floored at 0).
func (h *CombatHandler) ShoveNPC(uid, npcInstID string, pushFt int) error {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("player %q not found", uid)
	}

	h.combatMu.Lock()
	defer h.combatMu.Unlock()

	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return fmt.Errorf("player %q is not in active combat", uid)
	}

	var playerCbt *combat.Combatant
	var npcCbt *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.ID == uid {
			playerCbt = c
		}
		if c.ID == npcInstID {
			npcCbt = c
		}
	}
	if playerCbt == nil {
		return fmt.Errorf("player combatant %q not found", uid)
	}
	if npcCbt == nil {
		return fmt.Errorf("NPC combatant %q not found", npcInstID)
	}

	// Convert feet to grid cells (1 cell = 5 ft), minimum 1 cell.
	pushCells := pushFt / 5
	if pushCells < 1 {
		pushCells = 1
	}
	width := 10
	if cbt.GridWidth > 0 {
		width = cbt.GridWidth
	}
	if npcCbt.GridX >= playerCbt.GridX {
		npcCbt.GridX += pushCells
		if npcCbt.GridX >= width {
			npcCbt.GridX = width - 1
		}
	} else {
		npcCbt.GridX -= pushCells
		if npcCbt.GridX < 0 {
			npcCbt.GridX = 0
		}
	}
	return nil
}

// applyMentalStateChanges applies condition swaps from mental state transitions to the player session.
// Returns narrative messages for broadcast.
//
// Precondition: uid is a valid player session; changes may be nil or empty.
// Postcondition: conditions applied/removed on session; messages returned.
func (h *CombatHandler) applyMentalStateChanges(uid string, changes []mentalstate.StateChange) []string {
	if len(changes) == 0 || h.mentalStateMgr == nil {
		return nil
	}
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok || sess.Conditions == nil {
		return nil
	}
	var messages []string
	for _, ch := range changes {
		if ch.OldConditionID != "" {
			sess.Conditions.Remove(uid, ch.OldConditionID)
		}
		if ch.NewConditionID != "" {
			def, ok := h.condRegistry.Get(ch.NewConditionID)
			if ok {
				_ = sess.Conditions.Apply(uid, def, 1, -1) // -1 = permanent duration for mental state
			}
		}
		if ch.Message != "" {
			messages = append(messages, ch.Message)
		}
	}
	return messages
}

// checkHPThresholdFear triggers Fear (Uneasy) if player HP is at or below 25% of MaxHP.
// Call after player takes damage during combat.
//
// Precondition: uid is a valid player session.
// Postcondition: if HP <= 25% of MaxHP, ApplyTrigger(TrackFear, SeverityMild) is called and
//
//	resulting messages are broadcast as narrative combat events.
func (h *CombatHandler) checkHPThresholdFear(uid string) {
	if h.mentalStateMgr == nil {
		return
	}
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok || sess.MaxHP == 0 {
		return
	}
	if float64(sess.CurrentHP)/float64(sess.MaxHP) <= 0.25 {
		changes := h.mentalStateMgr.ApplyTrigger(uid, mentalstate.TrackFear, mentalstate.SeverityMild)
		msgs := h.applyMentalStateChanges(uid, changes)
		if len(msgs) > 0 && h.broadcastFn != nil {
			var events []*gamev1.CombatEvent
			for _, msg := range msgs {
				events = append(events, &gamev1.CombatEvent{
					Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK,
					Narrative: msg,
				})
			}
			h.broadcastFn(sess.RoomID, events)
		}
	}
}

// Precondition: uid must be a valid connected player.
// Postcondition: Returns active conditions or nil if not in combat.
func (h *CombatHandler) Status(uid string) ([]*condition.ActiveCondition, error) {
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}
	h.combatMu.Lock()
	defer h.combatMu.Unlock()
	cbt, ok := h.engine.GetCombat(sess.RoomID)
	if !ok {
		return nil, nil // no combat active; return empty
	}
	return cbt.GetConditions(uid), nil
}

// clearReadiedAction clears ReadiedTrigger and ReadiedAction on a session.
// Called at end-of-round to enforce REQ-READY-1.
func clearReadiedAction(sess *session.PlayerSession) {
	sess.ReadiedTrigger = ""
	sess.ReadiedAction = ""
}

// evaluateBossAbilitiesLocked checks and fires all eligible boss abilities for inst.
// Called at the start of each combat round, before normal NPC attack resolution.
//
// Precondition: combatMu is held; inst must not be nil; roomID must be non-empty.
// Postcondition: eligible abilities are fired; BossAbilityCooldowns updated in inst.
func (h *CombatHandler) evaluateBossAbilitiesLocked(
	inst *npc.Instance,
	roomID string,
	round int,
	tookDamageThisRound bool,
) {
	tmpl := h.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || len(tmpl.BossAbilities) == 0 {
		return
	}
	now := time.Now()
	for _, ability := range tmpl.BossAbilities {
		// Check cooldown.
		if deadline, hasCooldown := inst.BossAbilityCooldowns[ability.ID]; hasCooldown {
			if now.Before(deadline) {
				continue
			}
		}
		// Check trigger condition.
		fires := false
		switch ability.Trigger {
		case "hp_pct_below":
			pct := 0
			if inst.MaxHP > 0 {
				pct = int(100 * inst.CurrentHP / inst.MaxHP)
			}
			fires = pct < ability.TriggerValue
		case "round_start":
			fires = ability.TriggerValue == 0 || round == ability.TriggerValue
		case "on_damage_taken":
			fires = tookDamageThisRound
		}
		if !fires {
			continue
		}

		// Announce ability.
		msg := fmt.Sprintf("%s uses %s!", inst.Name(), ability.Name)
		h.broadcastFn(roomID, []*gamev1.CombatEvent{{
			Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_INITIATIVE,
			Narrative: msg,
		}})

		// Execute effect.
		switch {
		case ability.Effect.AoeDamageExpr != "":
			if dmgResult, err := h.dice.RollExpr(ability.Effect.AoeDamageExpr); err == nil {
				dmg := dmgResult.Total()
				if dmg < 0 {
					dmg = 0
				}
				players := h.sessions.PlayersInRoomDetails(roomID)
				for _, p := range players {
					p.CurrentHP -= dmg
					if p.CurrentHP < 0 {
						p.CurrentHP = 0
					}
				}
			}
		case ability.Effect.AoeCondition != "":
			if h.condRegistry != nil {
				players := h.sessions.PlayersInRoomDetails(roomID)
				for _, p := range players {
					if cond, ok := h.condRegistry.Get(ability.Effect.AoeCondition); ok && p.Conditions != nil {
						_ = p.Conditions.Apply(p.UID, cond, 1, -1)
					}
				}
			}
		case ability.Effect.HealPct != 0:
			heal := int(math.Ceil(float64(inst.MaxHP) * float64(ability.Effect.HealPct) / 100.0))
			inst.CurrentHP += heal
			if inst.CurrentHP > inst.MaxHP {
				inst.CurrentHP = inst.MaxHP
			}
		}

		// Record cooldown.
		if ability.Cooldown != "" {
			if dur, parseErr := time.ParseDuration(ability.Cooldown); parseErr == nil {
				inst.BossAbilityCooldowns[ability.ID] = now.Add(dur)
			}
		}
	}
}

// ResolveNPCSeductionGenderCheck checks whether inst's SeductionGender restriction
// blocks seduction for the given player. When blocked, inst.Disposition is set to
// "hostile" and true is returned.
//
// Precondition: inst must not be nil.
// Postcondition: Returns true and sets inst.Disposition="hostile" when the player's
// gender does not match inst.SeductionGender; returns false otherwise.
func (h *CombatHandler) ResolveNPCSeductionGenderCheck(inst *npc.Instance, playerUID, playerGender string) bool {
	if inst.SeductionGender != "" && inst.SeductionGender != playerGender {
		inst.Disposition = "hostile"
		return true
	}
	return false
}

// ResolveNPCSeductionContest runs the seduction opposed check between inst and the player.
// npcRoll and playerRoll are raw d20 results; callers pass fixed values for testing.
// Uses abilityModFrom(inst.Flair) and abilityModFrom(playerSavvy).
// On NPC win: applies seducedDef to condSet (1 round, 1 stack), returns true.
// On player win: inst.Disposition="hostile", inst.SeductionRejected[playerUID]=true, returns false.
//
// Precondition: inst must not be nil; seducedDef must not be nil; condSet must not be nil.
// Postcondition: Returns true with seduced condition applied on NPC win;
// returns false with inst turned hostile on player win.
func (h *CombatHandler) ResolveNPCSeductionContest(inst *npc.Instance, playerUID string, playerSavvy int, seducedDef *condition.ConditionDef, condSet *condition.ActiveSet, npcRoll, playerRoll int) bool {
	npcTotal := npcRoll + abilityModFrom(inst.Flair)
	playerTotal := playerRoll + abilityModFrom(playerSavvy)
	if npcTotal >= playerTotal {
		_ = condSet.Apply(playerUID, seducedDef, 1, 1)
		return true
	}
	inst.Disposition = "hostile"
	if inst.SeductionRejected == nil {
		inst.SeductionRejected = make(map[string]bool)
	}
	inst.SeductionRejected[playerUID] = true
	return false
}
