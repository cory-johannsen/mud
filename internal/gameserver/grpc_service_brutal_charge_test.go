package gameserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

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

// newBrutalChargeSvc builds a GameServiceServer wired with a real CombatHandler
// so the queued attack path for Brutal Charge is exercised end-to-end.
func newBrutalChargeSvc(t *testing.T) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := condition.NewRegistry()
	feat := &ruleset.Feat{
		ID:             "brutal_charge",
		Name:           "Brutal Charge",
		Active:         true,
		ActionCost:     1,
		ActivateText:   "You close the distance before they can react.",
		RequiresCombat: true,
	}
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{feat})
	featsRepo := &stubFeatsRepo{data: map[int64][]string{0: {feat.ID}}}
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), zap.NewNop())
	invRegistry := inventory.NewRegistry()
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		200*time.Millisecond, condReg, worldMgr, nil, invRegistry, nil, nil, nil, nil, nil,
	)
	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, invRegistry, nil,
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

// TestHandleBrutalCharge_QueuesAttack verifies GH #233: after invoking the
// Brutal Charge feat via handleUse, the player's action queue contains an
// ActionAttack targeting the nearest opponent — i.e. an attack is "scheduled
// as part of the effect during combat resolution", with no additional manual
// command required.
func TestHandleBrutalCharge_QueuesAttack(t *testing.T) {
	const uid = "u_bc_queue"
	const roomID = "room_a"
	svc, sessMgr, npcMgr, combatH := newBrutalChargeSvc(t)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    uid,
		CharName:    uid,
		CharacterID: 0,
		RoomID:      roomID,
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

	tmpl := &npc.Template{ID: "bc-grunt", Name: "Grunt", Level: 1, MaxHP: 20, AC: 13}
	inst, err := npcMgr.Spawn(tmpl, roomID)
	require.NoError(t, err)

	_, err = combatH.Attack(uid, "Grunt")
	require.NoError(t, err)
	combatH.cancelTimer(roomID)
	sess.Status = statusInCombat

	// Clear the initial queued attack from combatH.Attack so only Brutal
	// Charge's effect remains.
	cbt, ok := combatH.engine.GetCombat(roomID)
	require.True(t, ok)
	q := cbt.ActionQueues[uid]
	require.NotNil(t, q)
	q.ClearActions()

	// Place the opponent within range so Brutal Charge's Attack path is taken
	// (rather than the "no targets to charge toward" early exit).
	npcCbt := cbt.GetCombatant(inst.ID)
	require.NotNil(t, npcCbt)
	// Ensure player and NPC are on the grid; default spawn usually does this.
	assert.GreaterOrEqual(t, cbt.GridWidth, 1)

	evt, err := svc.handleUse(uid, "brutal_charge", "Grunt", -1, -1)
	require.NoError(t, err)
	require.NotNil(t, evt, "handleUse must return an event")

	// After handleBrutalCharge completes, the player's queue must contain an
	// ActionAttack targeting the opponent.
	actions := q.QueuedActions()
	require.NotEmpty(t, actions, "Brutal Charge must schedule an attack (GH #233)")
	var found bool
	for _, a := range actions {
		if a.Type == combat.ActionAttack && a.Target == "Grunt" {
			found = true
			break
		}
	}
	assert.True(t, found,
		"Brutal Charge must queue an ActionAttack against the target; got actions=%v", actions)
}
