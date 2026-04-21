package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newConditionTargetingCombatHandler constructs a CombatHandler suitable for
// NPC condition targeting tests. Uses the shared makeTestConditionRegistry so
// frightened and related conditions are registered.
func newConditionTargetingCombatHandler(t *testing.T) (*CombatHandler, *npc.Manager, *session.Manager) {
	t.Helper()
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	return h, h.npcMgr, h.sessions
}

// startCombatForConditionTest spawns an NPC, registers a player session, starts
// combat, and returns the NPC instance and combat.
func startCombatForConditionTest(
	t *testing.T,
	h *CombatHandler,
	npcMgr *npc.Manager,
	sessMgr *session.Manager,
	roomID, playerUID string,
) (*npc.Instance, *combat.Combat) {
	t.Helper()
	inst, err := npcMgr.Spawn(&npc.Template{
		ID: "goblin-cond", Name: "Goblin", Level: 1, MaxHP: 20, AC: 13, Awareness: 2,
	}, roomID)
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: playerUID, Username: "hero", CharName: "Hero",
		RoomID: roomID, CurrentHP: 20, MaxHP: 20, Role: "player",
	})
	require.NoError(t, err)

	cbt, _, err := h.startCombatLocked(sess, inst)
	require.NoError(t, err)
	require.NotNil(t, cbt)
	return inst, cbt
}

// TestFindNPCInCombat_ReturnNilWhenNoCombat verifies FindNPCInCombat returns nil
// when no active combat exists in the room.
//
// Precondition: No combat is active in roomID.
// Postcondition: FindNPCInCombat returns nil.
func TestFindNPCInCombat_ReturnNilWhenNoCombat(t *testing.T) {
	t.Parallel()
	h, _, _ := newConditionTargetingCombatHandler(t)

	result := h.FindNPCInCombat("no-combat-room", "Goblin")
	assert.Nil(t, result, "FindNPCInCombat must return nil when no combat is active")
}

// TestFindNPCInCombat_MatchesByNamePrefix verifies that FindNPCInCombat returns
// the correct NPC instance by case-insensitive prefix match.
//
// Precondition: Active combat in roomID contains an NPC named "Goblin".
// Postcondition: FindNPCInCombat("gob") returns the matching NPC instance.
func TestFindNPCInCombat_MatchesByNamePrefix(t *testing.T) {
	t.Parallel()
	const roomID = "room-find-npc"
	h, npcMgr, sessMgr := newConditionTargetingCombatHandler(t)
	inst, _ := startCombatForConditionTest(t, h, npcMgr, sessMgr, roomID, "u-find-npc")

	found := h.FindNPCInCombat(roomID, "gob")
	require.NotNil(t, found, "FindNPCInCombat must return the matching NPC")
	assert.Equal(t, inst.ID, found.ID)
}

// TestApplyConditionToNPC_StoresConditionInCombatMap verifies that applying a
// condition to an NPC in combat stores it in the combat's Conditions map.
//
// Precondition: Active combat in roomID; frightened is a registered condition.
// Postcondition: cbt.HasCondition(npcID, "frightened") is true after application.
func TestApplyConditionToNPC_StoresConditionInCombatMap(t *testing.T) {
	t.Parallel()
	const roomID = "room-apply-cond"
	h, npcMgr, sessMgr := newConditionTargetingCombatHandler(t)
	inst, cbt := startCombatForConditionTest(t, h, npcMgr, sessMgr, roomID, "u-apply-cond")

	err := h.ApplyConditionToNPC(roomID, inst.ID, "frightened", 1, -1)
	require.NoError(t, err)

	assert.True(t, cbt.HasCondition(inst.ID, "frightened"),
		"frightened must be active in combat Conditions map after ApplyConditionToNPC")
}

// TestApplyConditionToNPC_MirroredToInstanceConditions verifies that applying a
// condition in combat also mirrors it to the npc.Instance.Conditions ActiveSet.
//
// Precondition: Active combat in roomID; frightened is a registered condition.
// Postcondition: inst.Conditions.Has("frightened") is true after application.
func TestApplyConditionToNPC_MirroredToInstanceConditions(t *testing.T) {
	t.Parallel()
	const roomID = "room-mirror-cond"
	h, npcMgr, sessMgr := newConditionTargetingCombatHandler(t)
	inst, _ := startCombatForConditionTest(t, h, npcMgr, sessMgr, roomID, "u-mirror-cond")

	err := h.ApplyConditionToNPC(roomID, inst.ID, "frightened", 1, -1)
	require.NoError(t, err)

	require.NotNil(t, inst.Conditions, "Instance Conditions must be non-nil")
	assert.True(t, inst.Conditions.Has("frightened"),
		"frightened must be mirrored to Instance.Conditions after ApplyConditionToNPC")
}

// TestApplyConditionToNPC_AttackModReducedInRound verifies that an NPC with the
// frightened condition has its AttackMod reduced by the frightened attack penalty
// after a round starts.
//
// Precondition: Active combat; frightened applied to NPC (AttackPenalty=1 per stack).
// Postcondition: After StartRound, the NPC combatant's AttackMod equals -1.
func TestApplyConditionToNPC_AttackModReducedInRound(t *testing.T) {
	t.Parallel()
	const roomID = "room-atk-mod"
	h, npcMgr, sessMgr := newConditionTargetingCombatHandler(t)
	inst, cbt := startCombatForConditionTest(t, h, npcMgr, sessMgr, roomID, "u-atk-mod")

	err := h.ApplyConditionToNPC(roomID, inst.ID, "frightened", 1, 3)
	require.NoError(t, err)

	// Apply condition bonuses to combatant modifiers (mimics round resolution).
	npcCbt := cbt.GetCombatant(inst.ID)
	require.NotNil(t, npcCbt)
	condSet := cbt.Conditions[inst.ID]
	require.NotNil(t, condSet)

	// Isolate frightened: drop combat-start flat_footed (sucker_punch window)
	// so this assertion reflects only frightened's contribution.
	condSet.Remove(inst.ID, "flat_footed")

	npcCbt.AttackMod = condition.AttackBonus(condSet)
	npcCbt.ACMod = condition.ACBonus(condSet)

	// frightened with AttackPenalty=1 and ACPenalty=1 should reduce both by 1.
	assert.Equal(t, -1, npcCbt.AttackMod, "AttackMod must equal frightened attack penalty (-1)")
	assert.Equal(t, -1, npcCbt.ACMod, "ACMod must equal frightened AC penalty (-1)")
}

// TestApplyConditionToNPC_NoCombatReturnsError verifies that applying a condition
// when no combat is active returns a descriptive error.
//
// Precondition: No active combat in the given room.
// Postcondition: Returns a non-nil error.
func TestApplyConditionToNPC_NoCombatReturnsError(t *testing.T) {
	t.Parallel()
	h, _, _ := newConditionTargetingCombatHandler(t)

	err := h.ApplyConditionToNPC("no-combat-room", "fake-npc-id", "frightened", 1, -1)
	assert.Error(t, err, "must return error when no combat is active")
}

// TestProperty_ApplyConditionToNPC_AlwaysStoresCondition verifies that whenever
// a condition is successfully applied to an NPC in combat, the combat Conditions
// map reflects the application.
//
// Precondition: Active combat; frightened registered.
// Postcondition: HasCondition(npcID, "frightened") == true after successful apply.
func TestProperty_ApplyConditionToNPC_AlwaysStoresCondition(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roomID := "room-prop-" + rapid.StringMatching(`[a-z]{4}`).Draw(rt, "suffix")
		playerUID := "u-prop-" + rapid.StringMatching(`[a-z]{4}`).Draw(rt, "uid")

		h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
		npcMgr := h.npcMgr
		sessMgr := h.sessions

		inst, cbt := startCombatForConditionTest(t, h, npcMgr, sessMgr, roomID, playerUID)

		err := h.ApplyConditionToNPC(roomID, inst.ID, "frightened", 1, -1)
		require.NoError(rt, err)

		assert.True(rt, cbt.HasCondition(inst.ID, "frightened"),
			"frightened must be present in combat Conditions map after successful apply")
		assert.True(rt, inst.Conditions.Has("frightened"),
			"frightened must be mirrored to Instance.Conditions after successful apply")
	})
}
