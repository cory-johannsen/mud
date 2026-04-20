package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ai"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestStartPursuitCombatLocked_LoadoutFallback_FromSession verifies that
// startPursuitCombatLocked resolves the equipped weapon from
// sess.LoadoutSet.ActivePreset() when h.loadouts has no entry for the player
// (BUG-142: player attacks with fists despite having a melee weapon equipped).
//
// Precondition: Player has a melee weapon equipped via sess.LoadoutSet only;
//
//	h.loadouts does NOT contain an entry for the player UID.
//
// Postcondition: Player combatant in pursuit combat has WeaponName equal to
//
//	the equipped weapon's Name (not "fists").
func TestStartPursuitCombatLocked_LoadoutFallback_FromSession(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)

	roomA := &world.Room{
		ID: "pursuit-room-a", ZoneID: "zone-pursuit", Title: "Room A",
		Exits: []world.Exit{{Direction: "north", TargetRoom: "pursuit-room-b"}},
	}
	roomB := &world.Room{
		ID: "pursuit-room-b", ZoneID: "zone-pursuit", Title: "Room B",
		Exits: []world.Exit{{Direction: "south", TargetRoom: "pursuit-room-a"}},
	}
	zone := &world.Zone{
		ID: "zone-pursuit", StartRoom: "pursuit-room-a",
		Rooms: map[string]*world.Room{"pursuit-room-a": roomA, "pursuit-room-b": roomB},
	}
	wm, err := world.NewManager([]*world.Zone{zone})
	require.NoError(t, err)

	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	engine := combat.NewEngine()
	aiReg := ai.NewRegistry()

	h := NewCombatHandler(
		engine, npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), wm, nil, nil, aiReg,
		nil, nil, nil, nil,
	)

	uid := "pursuit-weapon-player"

	sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, Role: "player",
		RoomID: "pursuit-room-b", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, addErr)
	sess.Conditions = condition.NewActiveSet()

	// Equip a melee weapon via LoadoutSet only — do NOT call RegisterLoadout,
	// simulating the post-login case where h.loadouts is empty for this UID.
	weaponDef := &inventory.WeaponDef{
		ID: "rebar-club-test", Name: "Rebar Club",
		DamageDice: "1d8", DamageType: "bludgeoning",
		ProficiencyCategory: "simple_weapons",
		Rarity:              "salvage",
	}
	loadoutSet := inventory.NewLoadoutSet()
	require.NoError(t, loadoutSet.Presets[0].EquipMainHand(weaponDef))
	sess.LoadoutSet = loadoutSet

	// Spawn an NPC in the destination room (room-b) to be a pursuer.
	npcInst, spawnErr := npcMgr.Spawn(&npc.Template{
		ID: "pursuit-npc-142", Name: "Pursuer", Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, "pursuit-room-b")
	require.NoError(t, spawnErr)

	// Call startPursuitCombatLocked directly (requires combatMu to be held).
	h.combatMu.Lock()
	evts, pursuitErr := h.startPursuitCombatLocked(sess, []*npc.Instance{npcInst})
	h.combatMu.Unlock()
	require.NoError(t, pursuitErr)
	require.NotEmpty(t, evts)

	defer h.cancelTimer("pursuit-room-b")

	// Retrieve the combat and verify weapon resolution.
	cbt, ok := engine.GetCombat("pursuit-room-b")
	require.True(t, ok, "pursuit combat must exist in room-b")

	playerCbt := cbt.GetCombatant(uid)
	require.NotNil(t, playerCbt, "player must be a combatant in pursuit combat")

	assert.Equal(t, "Rebar Club", playerCbt.WeaponName,
		"WeaponName must be the equipped weapon name, not 'fists'")
	assert.Equal(t, "bludgeoning", playerCbt.WeaponDamageType,
		"WeaponDamageType must match the equipped weapon")
}
