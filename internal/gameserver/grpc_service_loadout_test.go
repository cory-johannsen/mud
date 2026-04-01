package gameserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newLoadoutSvcWithCombat(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)
	return svc, sessMgr, npcMgr, combatHandler
}

// setupLoadoutPlayer spawns an NPC, adds a player, marks them in-combat, and
// initialises combat via Attack. The player session has LoadoutSet with 2 presets
// and Active=0.
//
// Precondition: npcMgr and sessMgr are non-nil.
// Postcondition: PlayerSession exists with statusInCombat and LoadoutSet != nil.
func setupLoadoutPlayer(t testing.TB, uid, roomID, npcName string, sessMgr *session.Manager, npcMgr *npc.Manager, combatHandler *CombatHandler) *session.PlayerSession {
	t.Helper()
	_, err := npcMgr.Spawn(&npc.Template{
		ID: uid + "-guard", Name: npcName, Level: 1, MaxHP: 20, AC: 13, Awareness: 5,
	}, roomID)
	require.NoError(t, err)
	sess, addErr := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, Role: "player",
		RoomID: roomID, CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, addErr)
	sess.Conditions = condition.NewActiveSet()
	sess.Status = statusInCombat
	_, attackErr := combatHandler.Attack(uid, npcName)
	require.NoError(t, attackErr)
	combatHandler.cancelTimer(roomID)
	return sess
}

// TestHandleLoadout_InCombat_NoAP verifies that swapping a loadout preset while in combat
// with 0 AP remaining returns the "Not enough AP" message and does not change the active preset.
//
// Precondition: Player "lo_noap" in combat; all AP exhausted before swap attempt.
// Postcondition: Event narrative contains "Not enough AP to swap loadouts."; Active preset unchanged.
func TestHandleLoadout_InCombat_NoAP(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr, npcMgr, combatHandler := newLoadoutSvcWithCombat(t, roller)
	sess := setupLoadoutPlayer(t, "lo_noap", "room_lo_noap", "Guard", sessMgr, npcMgr, combatHandler)

	// Exhaust all AP so the swap should be denied.
	remaining := combatHandler.RemainingAP("lo_noap")
	if remaining > 0 {
		err := combatHandler.SpendAP("lo_noap", remaining)
		require.NoError(t, err)
	}

	initialActive := sess.LoadoutSet.Active

	ev, err := svc.handleLoadout("lo_noap", &gamev1.LoadoutRequest{Arg: "2"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "Not enough AP to swap loadouts.")
	assert.Equal(t, initialActive, sess.LoadoutSet.Active, "active preset must not change when AP denied")
}

// TestHandleLoadout_InCombat_WithAP verifies that swapping a loadout preset while in combat
// with sufficient AP deducts exactly 1 AP and changes the active preset.
//
// Precondition: Player "lo_ap" in combat with ≥1 AP remaining; Active=0.
// Postcondition: AP reduced by 1; Active preset changed to 1 (preset "2").
func TestHandleLoadout_InCombat_WithAP(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr, npcMgr, combatHandler := newLoadoutSvcWithCombat(t, roller)
	sess := setupLoadoutPlayer(t, "lo_ap", "room_lo_ap", "Bandit", sessMgr, npcMgr, combatHandler)

	remaining := combatHandler.RemainingAP("lo_ap")
	require.GreaterOrEqual(t, remaining, 1, "expected at least 1 AP after combat start")

	initialActive := sess.LoadoutSet.Active
	require.Equal(t, 0, initialActive)

	ev, err := svc.handleLoadout("lo_ap", &gamev1.LoadoutRequest{Arg: "2"})
	require.NoError(t, err)
	assert.Contains(t, ev.GetMessage().GetContent(), "Switched to preset 2.")
	assert.NotEqual(t, initialActive, sess.LoadoutSet.Active, "active preset must change after swap")
	assert.Equal(t, remaining-1, combatHandler.RemainingAP("lo_ap"), "exactly 1 AP must be deducted")
}

// TestHandleLoadout_InCombat_EmptyArg verifies that calling handleLoadout in combat with
// an empty arg (display only) does NOT deduct any AP and returns a LoadoutView.
//
// Precondition: Player "lo_empty" in combat with ≥1 AP remaining; Arg is "".
// Postcondition: AP is unchanged; event carries a LoadoutView (not a swap message).
func TestHandleLoadout_InCombat_EmptyArg(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr, npcMgr, combatHandler := newLoadoutSvcWithCombat(t, roller)
	_ = setupLoadoutPlayer(t, "lo_empty", "room_lo_empty", "Thug", sessMgr, npcMgr, combatHandler)

	apBefore := combatHandler.RemainingAP("lo_empty")
	require.GreaterOrEqual(t, apBefore, 1, "expected at least 1 AP")

	ev, err := svc.handleLoadout("lo_empty", &gamev1.LoadoutRequest{Arg: ""})
	require.NoError(t, err)
	require.NotNil(t, ev)
	// display-only call must return a sentinel-encoded MessageEvent with LoadoutView JSON.
	const sentinel = "\x00loadout\x00"
	content := ev.GetMessage().GetContent()
	require.True(t, strings.HasPrefix(content, sentinel), "display-only call must return sentinel-encoded MessageEvent")
	jsonStr := content[len(sentinel):]
	var lv gamev1.LoadoutView
	require.NoError(t, json.Unmarshal([]byte(jsonStr), &lv), "sentinel payload must be valid LoadoutView JSON")
	assert.NotEmpty(t, lv.Presets, "LoadoutView must contain presets")
	assert.Equal(t, apBefore, combatHandler.RemainingAP("lo_empty"), "AP must not be deducted for empty-arg (display) call")
}

// TestHandleLoadout_OutOfCombat_NoAPCheck verifies that swapping a loadout preset outside
// of combat succeeds without any AP check, even when combatH would report 0 AP.
//
// Precondition: Player "lo_ooc" not in combat; Active=0.
// Postcondition: Active preset changes to 1; no error; no AP deducted.
func TestHandleLoadout_OutOfCombat_NoAPCheck(t *testing.T) {
	logger := zaptest.NewLogger(t)
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	svc, sessMgr, _, _ := newLoadoutSvcWithCombat(t, roller)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "lo_ooc", Username: "lo_ooc", CharName: "lo_ooc", Role: "player",
		RoomID: "room_lo_ooc", CurrentHP: 10, MaxHP: 10,
	})
	require.NoError(t, err)
	sess.Conditions = condition.NewActiveSet()
	// Do NOT set statusInCombat — player is out of combat.

	require.Equal(t, 0, sess.LoadoutSet.Active)

	ev, swapErr := svc.handleLoadout("lo_ooc", &gamev1.LoadoutRequest{Arg: "2"})
	require.NoError(t, swapErr)
	assert.Contains(t, ev.GetMessage().GetContent(), "Switched to preset 2.")
	assert.Equal(t, 1, sess.LoadoutSet.Active, "active preset must change out of combat without AP check")
}
