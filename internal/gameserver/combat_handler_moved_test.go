package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/character"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMovedCombat spawns an NPC named npcName and adds a player in roomID, then starts
// combat via h.Attack. Returns the player session.
//
// Precondition: h.npcMgr and h.sessions must be the provided npcMgr and sessMgr.
// Postcondition: Active combat exists in roomID; timer is cancelled.
func setupMovedCombat(t *testing.T, h *CombatHandler, npcMgr *npc.Manager, sessMgr *session.Manager, roomID, playerUID, npcName string) {
	t.Helper()
	_, err := npcMgr.Spawn(&npc.Template{
		ID: npcName + "-tmpl", Name: npcName, Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       playerUID,
		Username:  playerUID,
		CharName:  playerUID,
		Role:      "player",
		RoomID:    roomID,
		CurrentHP: 10,
		MaxHP:     10,
		Abilities: character.AbilityScores{},
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	_, err = h.Attack(playerUID, npcName)
	require.NoError(t, err)
	h.cancelTimer(roomID)
}

// makeHandlerWithManagers constructs a CombatHandler using the provided npcMgr and sessMgr.
//
// Precondition: npcMgr and sessMgr must be non-nil.
// Postcondition: Returns a non-nil CombatHandler backed by those managers.
func makeHandlerWithManagers(t *testing.T, npcMgr *npc.Manager, sessMgr *session.Manager) *CombatHandler {
	t.Helper()
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	h.npcMgr = npcMgr
	h.sessions = sessMgr
	return h
}

// TestSetOnCombatantMoved_CallbackStoredAndInvokable verifies that SetOnCombatantMoved
// stores the callback and FireCombatantMoved invokes it with the correct arguments.
//
// Precondition: CombatHandler is initialized.
// Postcondition: callback receives the exact roomID and uid passed to FireCombatantMoved.
func TestSetOnCombatantMoved_CallbackStoredAndInvokable(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})

	var gotRoom, gotUID string
	h.SetOnCombatantMoved(func(roomID, uid string) {
		gotRoom = roomID
		gotUID = uid
	})

	h.FireCombatantMoved("room_x", "player_42")

	assert.Equal(t, "room_x", gotRoom)
	assert.Equal(t, "player_42", gotUID)
}

// TestSetOnCombatantMoved_NilCallbackNoPanic verifies that FireCombatantMoved is a
// no-op when no callback has been registered.
//
// Precondition: onCombatantMoved is nil (default).
// Postcondition: FireCombatantMoved does not panic.
func TestSetOnCombatantMoved_NilCallbackNoPanic(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	assert.NotPanics(t, func() {
		h.FireCombatantMoved("room_x", "player_1")
	})
}

// TestCombatantPosition_PlayerStartsAtZero verifies that a newly enrolled player
// combatant has position 0.
//
// Precondition: Combat started in room_cp with one player and one NPC.
// Postcondition: CombatantPosition returns 0 for the player.
func TestCombatantPosition_PlayerStartsAtZero(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := makeHandlerWithManagers(t, npcMgr, sessMgr)

	setupMovedCombat(t, h, npcMgr, sessMgr, "room_cp", "cp_player", "GuardCP")

	pos := h.CombatantPosition("room_cp", "cp_player")
	assert.Equal(t, 0, pos, "player starts at position 0")
}

// TestCombatantPosition_NotInCombat_ReturnsZero verifies that CombatantPosition returns
// 0 when there is no active combat in the given room.
//
// Precondition: No combat is active in room "no_room".
// Postcondition: Returns 0.
func TestCombatantPosition_NotInCombat_ReturnsZero(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	assert.Equal(t, 0, h.CombatantPosition("no_room", "nobody"))
}

// TestCombatantsInRoom_ReturnsAllCombatants verifies that CombatantsInRoom returns at
// least the player and the NPC once combat is active.
//
// Precondition: Combat started with one player and one NPC in room_cir.
// Postcondition: len(result) >= 2.
func TestCombatantsInRoom_ReturnsAllCombatants(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := makeHandlerWithManagers(t, npcMgr, sessMgr)

	setupMovedCombat(t, h, npcMgr, sessMgr, "room_cir", "cir_player", "GuardCIR")

	combatants := h.CombatantsInRoom("room_cir")
	assert.GreaterOrEqual(t, len(combatants), 2, "must include player and NPC")
}

// TestCombatantsInRoom_NoActiveCombat_ReturnsNil verifies that CombatantsInRoom returns
// nil when no combat is active.
//
// Precondition: No combat is active in "empty_room".
// Postcondition: Returns nil.
func TestCombatantsInRoom_NoActiveCombat_ReturnsNil(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	combatants := h.CombatantsInRoom("empty_room")
	assert.Nil(t, combatants)
}

// TestCombatantsInRoom_ReturnsCopy verifies that the returned slice is a defensive copy —
// mutating it does not affect the internal combat state.
//
// Precondition: Combat started with at least one combatant.
// Postcondition: Modifying the returned slice does not change the combat's Combatants slice.
func TestCombatantsInRoom_ReturnsCopy(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := makeHandlerWithManagers(t, npcMgr, sessMgr)

	setupMovedCombat(t, h, npcMgr, sessMgr, "room_copy", "copy_player", "GuardCopy")

	first := h.CombatantsInRoom("room_copy")
	require.NotEmpty(t, first)
	original := first[0]
	first[0] = nil // mutate the copy

	second := h.CombatantsInRoom("room_copy")
	require.NotEmpty(t, second)
	assert.Equal(t, original, second[0], "internal combat state must be unchanged after copy mutation")
}

// TestCombatantPosition_NPCStartsAt50 verifies that an NPC combatant is initialized at
// position 50 (the default NPC starting position per the combat rules).
//
// Precondition: Combat started with one player and one NPC.
// Postcondition: CombatantPosition returns 50 for the NPC combatant.
func TestCombatantPosition_NPCStartsAt50(t *testing.T) {
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	h := makeHandlerWithManagers(t, npcMgr, sessMgr)

	setupMovedCombat(t, h, npcMgr, sessMgr, "room_npc50", "npc50_player", "GuardNPC50")

	combatants := h.CombatantsInRoom("room_npc50")
	require.NotEmpty(t, combatants)

	var npcCombatant *combat.Combatant
	for _, c := range combatants {
		if c.Kind == combat.KindNPC {
			npcCombatant = c
			break
		}
	}
	require.NotNil(t, npcCombatant, "NPC combatant must be present")
	assert.Equal(t, 50, h.CombatantPosition("room_npc50", npcCombatant.ID))
}
