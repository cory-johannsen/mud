package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestBuildPlayerCombatant_LoadoutFallback_FromSession verifies that buildPlayerCombatant
// populates the combatant's Loadout from sess.LoadoutSet.ActivePreset() when no entry
// exists in h.loadouts (i.e., RegisterLoadout was never called for this UID).
//
// This is the post-login case: LoadoutSet is loaded from DB but h.loadouts is empty.
//
// Precondition: Session has LoadoutSet with a ranged weapon in the active preset.
//
//	h.loadouts does NOT contain an entry for the player's UID.
//
// Postcondition: After Attack (which calls startCombatLocked -> buildPlayerCombatant),
//
//	the player's combat.Combatant has Loadout != nil and
//	Loadout.MainHand.Def.ID == the equipped weapon's ID.
func TestBuildPlayerCombatant_LoadoutFallback_FromSession(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)

	_, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	engine := combat.NewEngine()
	combatHandler := NewCombatHandler(
		engine, npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil, nil,
	)

	uid := "lf_player"
	roomID := "room_lf"
	npcName := "Guard"

	// Spawn an NPC in the room.
	_, err := npcMgr.Spawn(&npc.Template{
		ID: uid + "-guard", Name: npcName, Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	// Add the player session.
	sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, Role: "player",
		RoomID: roomID, CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, addErr)
	sess.Conditions = condition.NewActiveSet()

	// Build a LoadoutSet with a ranged weapon in the active preset.
	pistolDef := &inventory.WeaponDef{
		ID: "pistol_lf", Name: "Pistol LF",
		DamageDice: "1d6", DamageType: "piercing",
		RangeIncrement: 30, ReloadActions: 1, MagazineCapacity: 15,
		FiringModes:         []inventory.FiringMode{inventory.FiringModeSingle},
		ProficiencyCategory: "simple_ranged",
		Rarity:              "salvage",
	}
	loadoutSet := inventory.NewLoadoutSet()
	require.NoError(t, loadoutSet.Presets[0].EquipMainHand(pistolDef))
	sess.LoadoutSet = loadoutSet

	// Crucially: do NOT call combatHandler.RegisterLoadout(uid, ...).
	// h.loadouts must be empty for this UID to exercise the fallback path.

	// Trigger combat start (calls startCombatLocked -> buildPlayerCombatant).
	_, attackErr := combatHandler.Attack(uid, npcName)
	require.NoError(t, attackErr)
	combatHandler.cancelTimer(roomID)

	// Retrieve the combat and the player's combatant to verify loadout wiring.
	cbt, ok := engine.GetCombat(roomID)
	require.True(t, ok, "combat must exist in room after Attack")

	playerCbt := cbt.GetCombatant(uid)
	require.NotNil(t, playerCbt, "player combatant must exist in combat")

	assert.NotNil(t, playerCbt.Loadout,
		"Loadout must not be nil when sess.LoadoutSet has an active preset")
	require.NotNil(t, playerCbt.Loadout.MainHand,
		"Loadout.MainHand must not be nil when active preset has a weapon equipped")
	require.NotNil(t, playerCbt.Loadout.MainHand.Def,
		"Loadout.MainHand.Def must not be nil")
	assert.Equal(t, "pistol_lf", playerCbt.Loadout.MainHand.Def.ID,
		"Loadout.MainHand.Def.ID must match the weapon equipped in the active preset")
}

// TestBuildPlayerCombatant_LoadoutFallback_NilLoadoutSet verifies that buildPlayerCombatant
// does not panic and leaves Loadout as nil when sess.LoadoutSet is nil.
//
// Precondition: sess.LoadoutSet is nil; h.loadouts has no entry for uid.
// Postcondition: playerCbt.Loadout is nil; no panic.
func TestBuildPlayerCombatant_LoadoutFallback_NilLoadoutSet(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)

	_, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	engine := combat.NewEngine()
	combatHandler := NewCombatHandler(
		engine, npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil, nil,
	)

	uid := "lf_nil_player"
	roomID := "room_lf_nil"
	npcName := "Thug"

	_, err := npcMgr.Spawn(&npc.Template{
		ID: uid + "-thug", Name: npcName, Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, Role: "player",
		RoomID: roomID, CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, addErr)
	sess.Conditions = condition.NewActiveSet()

	// Explicitly nil out the LoadoutSet to simulate a session with no loadout data.
	sess.LoadoutSet = nil

	// Must not panic.
	_, attackErr := combatHandler.Attack(uid, npcName)
	require.NoError(t, attackErr)
	combatHandler.cancelTimer(roomID)

	cbt, ok := engine.GetCombat(roomID)
	require.True(t, ok, "combat must exist in room after Attack")

	playerCbt := cbt.GetCombatant(uid)
	require.NotNil(t, playerCbt, "player combatant must exist in combat")

	assert.Nil(t, playerCbt.Loadout,
		"Loadout must remain nil when both h.loadouts and sess.LoadoutSet are nil")
}
