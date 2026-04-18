package gameserver

// REQ-60-1: Activating Overpower in combat MUST deduct 2 Action Points from the player's queue.
// REQ-60-2: Activating Overpower MUST be rejected with an error message when an item is
//
//	equipped in the off-hand slot.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

const (
	overpowerCondID   = "brutal_surge_active"
	overpowerFeatID   = "overpower"
	overpowerFeatName = "Overpower"
	overpowerRoom     = "room_a"
)

// overpowerFeatDef returns the canonical Overpower feat definition used in all #60 tests.
func overpowerFeatDef() *ruleset.Feat {
	return &ruleset.Feat{
		ID:             overpowerFeatID,
		Name:           overpowerFeatName,
		Active:         true,
		ActivateText:   "You put everything into it.",
		ConditionID:    overpowerCondID,
		RequiresCombat: true,
	}
}

// overpowerCondReg returns a condition registry containing brutal_surge_active.
func overpowerCondReg() *condition.Registry {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{
		ID:           overpowerCondID,
		Name:         "Brutal Surge Active",
		DurationType: "encounter",
	})
	return reg
}

// newOverpowerSvc builds a GameServiceServer wired with a real CombatHandler
// so SpendAP calls work correctly.
//
// Precondition: t must be non-nil.
// Postcondition: Returns non-nil svc, sessMgr, npcMgr, and combatHandler sharing the same sessions.
func newOverpowerSvc(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := overpowerCondReg()
	feat := overpowerFeatDef()
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{feat})
	featsRepo := &stubFeatsRepo{data: map[int64][]string{0: {feat.ID}}}
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), zap.NewNop())
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		200*time.Millisecond, condReg, nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		[]*ruleset.Feat{feat}, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// spawnOverpowerNPC creates a minimal NPC in roomID for combat initiation.
func spawnOverpowerNPC(t *testing.T, npcMgr *npc.Manager, roomID, name string) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{ID: name, Name: name, Level: 1, MaxHP: 20, AC: 13}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)
	return inst
}

// oneHandedKnifeForOffHand returns a valid minimal one-handed weapon suitable for equipping
// in the off-hand slot.
func oneHandedKnifeForOffHand() *inventory.WeaponDef {
	return &inventory.WeaponDef{
		ID:                  "test_knife",
		Name:                "Test Knife",
		DamageDice:          "1d4",
		DamageType:          "piercing",
		Kind:                inventory.WeaponKindOneHanded,
		ProficiencyCategory: "simple_weapons",
		Rarity:              "salvage",
		RarityStatMultiplier: 1.0,
	}
}

// addOverpowerPlayer adds a player to sessMgr in overpowerRoom with a fresh LoadoutSet.
//
// Postcondition: Returns *session.PlayerSession with Conditions and LoadoutSet initialized.
func addOverpowerPlayer(t *testing.T, sessMgr *session.Manager, uid string) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 0,
		RoomID:      overpowerRoom,
		CurrentHP:   10,
		MaxHP:       10,
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	sess.DefaultCombatAction = "attack"
	if sess.LoadoutSet == nil {
		sess.LoadoutSet = inventory.NewLoadoutSet()
	}
	return sess
}

// apRemainingForPlayer reads the RemainingPoints from the player's combat action queue.
//
// Precondition: player must be in active combat; combatMu must be unlocked.
// Postcondition: Returns remaining AP, or -1 if player/combat not found.
func apRemainingForPlayer(combatH *CombatHandler, uid, roomID string) int {
	combatH.combatMu.Lock()
	defer combatH.combatMu.Unlock()
	cbt, ok := combatH.engine.GetCombat(roomID)
	if !ok {
		return -1
	}
	q, ok := cbt.ActionQueues[uid]
	if !ok {
		return -1
	}
	return q.RemainingPoints()
}

// TestHandleUse_Overpower_InCombat_ConsumesOneAP verifies that activating Overpower
// while in active combat deducts exactly 1 AP from the player's action queue.
//
// REQ-60-1: Overpower MUST consume 2 AP when activated during combat.
//
// Precondition: Player is in combat with 3 AP; off-hand is empty.
// Postcondition: Player has 1 AP remaining after Overpower activation.
func TestHandleUse_Overpower_InCombat_ConsumesTwoAP(t *testing.T) {
	const uid = "u_op_ap"
	svc, sessMgr, npcMgr, combatH := newOverpowerSvc(t)

	sess := addOverpowerPlayer(t, sessMgr, uid)
	spawnOverpowerNPC(t, npcMgr, overpowerRoom, "GruntAP")

	_, err := combatH.Attack(uid, "GruntAP")
	require.NoError(t, err)
	combatH.cancelTimer(overpowerRoom)
	sess.Status = statusInCombat

	// combatH.Attack queues an attack costing 1 AP; player starts at 3 and is now at 2.
	apBefore := apRemainingForPlayer(combatH, uid, overpowerRoom)
	require.Equal(t, 2, apBefore, "expected 2 AP after queuing the initial attack (3 - 1)")

	evt, err := svc.handleUse(uid, overpowerFeatID, "", -1, -1)
	require.NoError(t, err)
	require.NotNil(t, evt, "expected non-nil event")

	apAfter := apRemainingForPlayer(combatH, uid, overpowerRoom)
	assert.Equal(t, 0, apAfter, "Overpower must deduct exactly 2 AP (REQ-60-1)")
}

// TestHandleUse_Overpower_OffHandOccupied_IsRejected verifies that Overpower is refused
// when the player has any item equipped in the off-hand slot.
//
// REQ-60-2: Overpower MUST be rejected with an error message when off-hand is occupied.
//
// Precondition: Player is in combat; active loadout preset has a one-handed weapon in OffHand.
// Postcondition: Event is a Message (not UseResponse); message contains "off-hand"; AP unchanged.
func TestHandleUse_Overpower_OffHandOccupied_IsRejected(t *testing.T) {
	const uid = "u_op_offhand"
	svc, sessMgr, npcMgr, combatH := newOverpowerSvc(t)

	sess := addOverpowerPlayer(t, sessMgr, uid)
	spawnOverpowerNPC(t, npcMgr, overpowerRoom, "GruntOH")

	_, err := combatH.Attack(uid, "GruntOH")
	require.NoError(t, err)
	combatH.cancelTimer(overpowerRoom)
	sess.Status = statusInCombat

	// Equip an item in the off-hand.
	preset := sess.LoadoutSet.ActivePreset()
	require.NotNil(t, preset)
	require.NoError(t, preset.EquipOffHand(oneHandedKnifeForOffHand()))

	apBefore := apRemainingForPlayer(combatH, uid, overpowerRoom)

	evt, err := svc.handleUse(uid, overpowerFeatID, "", -1, -1)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected Message event, not UseResponse, when off-hand is occupied")
	assert.Contains(t, msg.Content, "off-hand", "rejection message must mention off-hand (REQ-60-2)")

	apAfter := apRemainingForPlayer(combatH, uid, overpowerRoom)
	assert.Equal(t, apBefore, apAfter, "AP must not be spent when Overpower is rejected")
}

// TestProperty_Overpower_EmptyOffHand_AlwaysConsumesAP is a property-based companion to
// TestHandleUse_Overpower_InCombat_ConsumesOneAP.
//
// Property: regardless of starting HP or ability scores, an empty off-hand always results
// in exactly 1 AP spent when Overpower is activated in combat.
//
// REQ-60-1 (property): 2 AP deduction is invariant over player stats.
func TestProperty_Overpower_EmptyOffHand_AlwaysConsumesAP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		hp := rapid.IntRange(1, 50).Draw(rt, "hp")
		uid := "u_op_prop"

		svc, sessMgr, npcMgr, combatH := newOverpowerSvc(t)
		sess := addOverpowerPlayer(t, sessMgr, uid)
		sess.CurrentHP = hp
		sess.MaxHP = hp
		spawnOverpowerNPC(t, npcMgr, overpowerRoom, "GruntProp")

		_, err := combatH.Attack(uid, "GruntProp")
		require.NoError(rt, err)
		combatH.cancelTimer(overpowerRoom)
		sess.Status = statusInCombat

		apBefore := apRemainingForPlayer(combatH, uid, overpowerRoom)

		_, err = svc.handleUse(uid, overpowerFeatID, "", -1, -1)
		require.NoError(rt, err)

		apAfter := apRemainingForPlayer(combatH, uid, overpowerRoom)
		assert.Equal(rt, apBefore-2, apAfter,
			"Overpower must deduct exactly 2 AP (hp=%d)", hp)
	})
}
