package gameserver_test

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gameserver "github.com/cory-johannsen/mud/internal/gameserver"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// makePositionBroadcastHandler builds a CombatHandler suitable for position-broadcast tests.
//
// Precondition: broadcastFn must be non-nil.
// Postcondition: Returns a non-nil CombatHandler backed by its own npc.Manager and session.Manager.
func makePositionBroadcastHandler(
	t *testing.T,
	broadcastFn func(string, []*gamev1.CombatEvent),
) (*gameserver.CombatHandler, *npc.Manager, *session.Manager) {
	t.Helper()
	npcMgr := npc.NewManager()
	sessMgr := session.NewManager()
	logger := zap.NewNop()
	src := dice.NewCryptoSource()
	roller := dice.NewLoggedRoller(src, logger)
	engine := combat.NewEngine()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	reg.Register(&condition.ConditionDef{ID: "flat_footed", Name: "Flat-Footed", DurationType: "rounds", MaxStacks: 0, ACPenalty: 2})
	h := gameserver.NewCombatHandler(
		engine, npcMgr, sessMgr, roller, broadcastFn,
		200*time.Millisecond,
		reg,
		nil, nil, nil, nil, nil, nil, nil, nil,
	)
	return h, npcMgr, sessMgr
}

// waitForPositionBroadcast polls broadcast events until a COMBAT_EVENT_TYPE_POSITION
// event is observed in a non-END round broadcast, or the timeout elapses.
// Returns true if a POSITION event was seen.
//
// Precondition: getEvents must be non-nil.
// Postcondition: Returns true iff at least one POSITION event was observed within timeout.
func waitForPositionBroadcast(t *testing.T, getEvents func() [][]*gamev1.CombatEvent, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, batch := range getEvents() {
			for _, e := range batch {
				if e.Type == gamev1.CombatEventType_COMBAT_EVENT_TYPE_POSITION {
					return true
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// TestPositionEventsIncludedInRoundBroadcast verifies that after a timer-fired
// round resolves (non-terminal: NPC survives), the broadcast includes at least
// one COMBAT_EVENT_TYPE_POSITION event for each combatant.
//
// Precondition: NPC has high HP (survives the round); player has a pistol.
// Postcondition: POSITION events are broadcast within one round duration.
func TestPositionEventsIncludedInRoundBroadcast(t *testing.T) {
	broadcastFn, getEvents := makeBroadcastCapture()
	h, npcMgr, sessMgr := makePositionBroadcastHandler(t, broadcastFn)

	const roomID = "room-pos-bcast"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-pos", Name: "Guard", Level: 1, MaxHP: 1000, AC: 30, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "player-pos",
		Username:  "Hero",
		CharName:  "Hero",
		RoomID:    roomID,
		CurrentHP: 1000,
		MaxHP:     1000,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	// Equip pistol so player can attack at combat range.
	equipTestPistol(t, h, "player-pos")

	_, err = h.Attack("player-pos", "Guard")
	require.NoError(t, err)

	// Wait up to 2 seconds for the first round timer to fire and broadcast positions.
	// testRoundDuration is 200ms, so this is 10× the round duration.
	got := waitForPositionBroadcast(t, getEvents, 2*time.Second)
	assert.True(t, got, "expected COMBAT_EVENT_TYPE_POSITION event after round resolution")

	// Verify at least 2 POSITION events (one per combatant: player + NPC).
	var posCount int
	for _, batch := range getEvents() {
		for _, e := range batch {
			if e.Type == gamev1.CombatEventType_COMBAT_EVENT_TYPE_POSITION {
				posCount++
			}
		}
	}
	assert.GreaterOrEqual(t, posCount, 2, "expected at least 2 POSITION events (player + NPC)")
}

// TestPositionEventsCarryCorrectCoordinates verifies that POSITION events in a
// non-terminal round broadcast contain valid grid coordinates (each within [0, 19]).
//
// Precondition: NPC survives; player has a pistol.
// Postcondition: All POSITION event coordinates are within [0, 19].
func TestPositionEventsCarryCorrectCoordinates(t *testing.T) {
	broadcastFn, getEvents := makeBroadcastCapture()
	h, npcMgr, sessMgr := makePositionBroadcastHandler(t, broadcastFn)

	const roomID = "room-pos-coords"
	_, err := npcMgr.Spawn(&npc.Template{
		ID: "guard-coords", Name: "GuardCoords", Level: 1, MaxHP: 1000, AC: 30, Awareness: 5,
	}, roomID)
	require.NoError(t, err)

	_, err = sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "player-coords",
		Username:  "Hero",
		CharName:  "Hero",
		RoomID:    roomID,
		CurrentHP: 1000,
		MaxHP:     1000,
		Abilities: character.AbilityScores{},
		Role:      "player",
	})
	require.NoError(t, err)

	equipTestPistol(t, h, "player-coords")

	_, err = h.Attack("player-coords", "GuardCoords")
	require.NoError(t, err)

	_ = waitForPositionBroadcast(t, getEvents, 2*time.Second)

	for _, batch := range getEvents() {
		for _, e := range batch {
			if e.Type != gamev1.CombatEventType_COMBAT_EVENT_TYPE_POSITION {
				continue
			}
			assert.GreaterOrEqual(t, int(e.AttackerX), 0, "AttackerX out of bounds for %s", e.Attacker)
			assert.LessOrEqual(t, int(e.AttackerX), 19, "AttackerX out of bounds for %s", e.Attacker)
			assert.GreaterOrEqual(t, int(e.AttackerY), 0, "AttackerY out of bounds for %s", e.Attacker)
			assert.LessOrEqual(t, int(e.AttackerY), 19, "AttackerY out of bounds for %s", e.Attacker)
		}
	}
}
