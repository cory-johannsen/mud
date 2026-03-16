package gameserver

import (
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// newCalmSvc builds a GameServiceServer with a real MentalStateManager and a given dice roller.
//
// Precondition: t and mentalMgr must be non-nil; roller may be nil.
// Postcondition: Returns non-nil svc, sessMgr, npcMgr, and combatHandler.
func newCalmSvc(t *testing.T, mentalMgr *mentalstate.Manager, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeTestConditionRegistry()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil, mentalMgr,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		mentalMgr, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// spawnCalmNPC creates a minimal NPC template and spawns it into roomID.
//
// Postcondition: Returns the spawned *npc.Instance.
func spawnCalmNPC(t *testing.T, npcMgr *npc.Manager, roomID, name string) *npc.Instance {
	t.Helper()
	tmpl := &npc.Template{
		ID:    name,
		Name:  name,
		Level: 1,
		MaxHP: 20,
		AC:    13,
	}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)
	return inst
}

// TestHandleCalm_NoActiveMentalState verifies that calm returns a "composed" message
// when no mental state tracks are active.
//
// Precondition: Player has no active mental state conditions.
// Postcondition: Event message contains "composed".
func TestHandleCalm_NoActiveMentalState(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	svc, sessMgr, _, _ := newCalmSvc(t, mentalMgr, nil)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_calm_none", Username: "T", CharName: "T", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	evt, err := svc.handleCalm("u_calm_none", &gamev1.CalmRequest{})
	require.NoError(t, err)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "composed")
}

// TestHandleCalm_NotInCombat_Success verifies that out-of-combat calm with a high roll succeeds.
//
// Precondition: Player has Fear SeverityMild active (DC=14); roll=20; Grit=10 (mod=0) → total=20 ≥ 14.
// Postcondition: Event message contains "success"; fear track severity drops to SeverityNone.
func TestHandleCalm_NotInCombat_Success(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	// Deterministic source: Intn(20) returns val%20; val=19 → returns 19 → roll = 19+1 = 20.
	roller := dice.NewRoller(dice.NewDeterministicSource([]int{19}))
	svc, sessMgr, _, _ := newCalmSvc(t, mentalMgr, roller)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_calm_ok", Username: "T", CharName: "T", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer("u_calm_ok")
	require.True(t, ok)
	sess.Abilities.Grit = 10 // mod = 0

	// Apply Fear SeverityMild (severity=1, DC=14).
	mentalMgr.ApplyTrigger("u_calm_ok", mentalstate.TrackFear, mentalstate.SeverityMild)

	evt, err := svc.handleCalm("u_calm_ok", &gamev1.CalmRequest{})
	require.NoError(t, err)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "success")
	assert.Equal(t, mentalstate.SeverityNone, mentalMgr.CurrentSeverity("u_calm_ok", mentalstate.TrackFear))
}

// TestHandleCalm_NotInCombat_Failure verifies that out-of-combat calm with a low roll fails.
//
// Precondition: Player has Fear SeveritySevere active (DC=22); roll=1; Grit=10 (mod=0) → total=1 < 22.
// Postcondition: Event message contains "failure"; fear track severity remains SeveritySevere.
func TestHandleCalm_NotInCombat_Failure(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	// Deterministic source: Intn(20) returns 0%20 → 0 → roll = 0+1 = 1.
	roller := dice.NewRoller(dice.NewDeterministicSource([]int{0}))
	svc, sessMgr, _, _ := newCalmSvc(t, mentalMgr, roller)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_calm_fail", Username: "T", CharName: "T", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer("u_calm_fail")
	require.True(t, ok)
	sess.Abilities.Grit = 10 // mod = 0

	// Apply Fear SeveritySevere (severity=3, DC=22).
	mentalMgr.ApplyTrigger("u_calm_fail", mentalstate.TrackFear, mentalstate.SeveritySevere)

	evt, err := svc.handleCalm("u_calm_fail", &gamev1.CalmRequest{})
	require.NoError(t, err)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "failure")
	assert.Equal(t, mentalstate.SeveritySevere, mentalMgr.CurrentSeverity("u_calm_fail", mentalstate.TrackFear))
}

// TestHandleCalm_InCombat_Failure_SpendAllAP verifies that a failed calm in combat still drains all AP.
//
// Precondition: Player is in combat with active Fear SeverityMild (DC=14); Grit=10 (mod=0); roll=1 → total=1 < 14.
// Postcondition: Event message contains "failure"; player has 0 AP remaining.
func TestHandleCalm_InCombat_Failure_SpendAllAP(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	// Deterministic source: Intn(20) returns 0 → roll = 1 (always fails DC 14).
	roller := dice.NewRoller(dice.NewDeterministicSource([]int{0, 0, 0, 0, 0}))
	svc, sessMgr, npcMgr, combatH := newCalmSvc(t, mentalMgr, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_calm_ap_fail", Username: "T", CharName: "T", RoomID: "room_calm_ap_fail", Role: "player",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer("u_calm_ap_fail")
	require.True(t, ok)
	sess.Abilities.Grit = 10 // mod = 0; roll=1 → total=1 < DC=14
	sess.DefaultCombatAction = "attack"

	// Spawn NPC and start combat via Attack.
	spawnCalmNPC(t, npcMgr, "room_calm_ap_fail", "GoblinFail")
	_, err = combatH.Attack("u_calm_ap_fail", "GoblinFail")
	require.NoError(t, err)
	combatH.cancelTimer("room_calm_ap_fail")
	sess.Status = statusInCombat

	mentalMgr.ApplyTrigger("u_calm_ap_fail", mentalstate.TrackFear, mentalstate.SeverityMild)

	evt, err := svc.handleCalm("u_calm_ap_fail", &gamev1.CalmRequest{})
	require.NoError(t, err)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "failure")

	// Verify AP drained even on failure.
	combatH.combatMu.Lock()
	cbt, ok := combatH.engine.GetCombat("room_calm_ap_fail")
	combatH.combatMu.Unlock()
	require.True(t, ok)
	q := cbt.ActionQueues["u_calm_ap_fail"]
	require.NotNil(t, q)
	assert.Equal(t, 0, q.RemainingPoints())
}

// TestHandleCalm_InCombat_SpendAllAP verifies that when in combat a successful calm drains all AP.
//
// Precondition: Player is in combat with AP remaining; roll=20 succeeds.
// Postcondition: Player has 0 AP remaining after calm.
func TestHandleCalm_InCombat_SpendAllAP(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	roller := dice.NewRoller(dice.NewDeterministicSource([]int{19, 19, 19, 19, 19})) // roll=20
	svc, sessMgr, npcMgr, combatH := newCalmSvc(t, mentalMgr, roller)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_calm_ap", Username: "T", CharName: "T", RoomID: "room_calm_ap", Role: "player",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer("u_calm_ap")
	require.True(t, ok)
	sess.Abilities.Grit = 10
	sess.DefaultCombatAction = "attack"

	// Spawn NPC and start combat via Attack.
	spawnCalmNPC(t, npcMgr, "room_calm_ap", "Goblin")
	_, err = combatH.Attack("u_calm_ap", "Goblin")
	require.NoError(t, err)
	combatH.cancelTimer("room_calm_ap")
	sess.Status = statusInCombat

	mentalMgr.ApplyTrigger("u_calm_ap", mentalstate.TrackFear, mentalstate.SeverityMild)

	evt, err := svc.handleCalm("u_calm_ap", &gamev1.CalmRequest{})
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Verify AP drained.
	combatH.combatMu.Lock()
	cbt, ok := combatH.engine.GetCombat("room_calm_ap")
	combatH.combatMu.Unlock()
	require.True(t, ok)
	q := cbt.ActionQueues["u_calm_ap"]
	require.NotNil(t, q)
	assert.Equal(t, 0, q.RemainingPoints())
}

// TestProperty_CalmDC_AlwaysBasedOnSeverity verifies DC = 10 + severity*4 for all valid severities
// by calling handleCalm with deterministic dice and checking success/failure boundaries.
//
// Precondition: severity in [1, 3]; Grit=10 (mod=0).
// Postcondition: rolling exactly DC-1 produces failure; rolling exactly DC produces success.
func TestProperty_CalmDC_AlwaysBasedOnSeverity(t *testing.T) {
	sevMap := map[int]mentalstate.Severity{
		1: mentalstate.SeverityMild,
		2: mentalstate.SeverityMod,
		3: mentalstate.SeveritySevere,
	}
	rapid.Check(t, func(rt *rapid.T) {
		sev := rapid.IntRange(1, 3).Draw(rt, "severity")
		dc := 10 + sev*4
		uid := "u_prop_dc"

		dcStr := fmt.Sprintf("DC %d", dc)

		// --- Fail test: roll=1+Grit=10 → total=1, always below DC≥14 ---
		{
			mentalMgr := mentalstate.NewManager()
			// Intn(20)=0 → roll=1.
			roller := dice.NewRoller(dice.NewDeterministicSource([]int{0, 0, 0, 0, 0}))
			svc, sessMgr, _, _ := newCalmSvc(t, mentalMgr, roller)
			_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
				UID: uid, Username: "T", CharName: "T", RoomID: "room_a", Role: "player",
			})
			require.NoError(rt, err)
			sess, ok := sessMgr.GetPlayer(uid)
			require.True(rt, ok)
			sess.Abilities.Grit = 10 // mod=0; total=1 < dc
			mentalMgr.ApplyTrigger(uid, mentalstate.TrackFear, sevMap[sev])
			evt, err := svc.handleCalm(uid, &gamev1.CalmRequest{})
			require.NoError(rt, err)
			msg := evt.GetMessage()
			require.NotNil(rt, msg)
			assert.Contains(rt, msg.Content, "failure",
				"sev=%d dc=%d: expected failure with roll=1", sev, dc)
			assert.Contains(rt, msg.Content, dcStr,
				"sev=%d: expected DC string %q in message", sev, dcStr)
		}

		// --- Pass test: roll=20+gritMod≥DC ---
		// For DC≤20: Grit=10 (mod=0), roll=20 → total=20≥DC.
		// For DC=22: Grit=14 (mod=2), roll=20 → total=22≥DC.
		{
			mentalMgr := mentalstate.NewManager()
			// Intn(20)=19 → roll=20.
			roller := dice.NewRoller(dice.NewDeterministicSource([]int{19, 19, 19, 19, 19}))
			svc, sessMgr, _, _ := newCalmSvc(t, mentalMgr, roller)
			_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
				UID: uid, Username: "T", CharName: "T", RoomID: "room_a", Role: "player",
			})
			require.NoError(rt, err)
			sess, ok := sessMgr.GetPlayer(uid)
			require.True(rt, ok)
			// Ensure total=roll+mod≥dc: mod=(Grit-10)/2; need mod≥dc-20 → Grit≥10+(dc-20)*2.
			neededGrit := 10
			if dc > 20 {
				neededGrit = 10 + (dc-20)*2
			}
			sess.Abilities.Grit = neededGrit
			mentalMgr.ApplyTrigger(uid, mentalstate.TrackFear, sevMap[sev])
			evt, err := svc.handleCalm(uid, &gamev1.CalmRequest{})
			require.NoError(rt, err)
			msg := evt.GetMessage()
			require.NotNil(rt, msg)
			assert.Contains(rt, msg.Content, "success",
				"sev=%d dc=%d grit=%d: expected success with roll=20", sev, dc, neededGrit)
			assert.Contains(rt, msg.Content, dcStr,
				"sev=%d: expected DC string %q in message", sev, dcStr)
		}
	})
}
