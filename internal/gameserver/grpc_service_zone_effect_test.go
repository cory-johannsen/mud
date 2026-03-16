package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"pgregory.net/rapid"
)

// newZoneEffectCombatHandler creates a CombatHandler with room_a having the given zone effect.
func newZoneEffectCombatHandler(t *testing.T, diceVal int, effect world.RoomEffect) (*CombatHandler, *session.Manager, *npc.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	src := &fixedDiceSource{val: diceVal}
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	npcMgr := npc.NewManager()

	// Inject zone effect into room_a.
	if r, ok := worldMgr.GetRoom("room_a"); ok {
		r.Effects = []world.RoomEffect{effect}
	}

	mentalMgr := mentalstate.NewManager()

	ch := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), worldMgr, nil, nil, nil, nil, nil, mentalMgr,
	)
	return ch, sessMgr, npcMgr
}

// TestZoneEffect_Combat_FailedSave_AppliesTrigger verifies that a player who fails the
// zone effect save has the corresponding mental state applied.
func TestZoneEffect_Combat_FailedSave_AppliesTrigger(t *testing.T) {
	// diceVal = 0 → roll = 1, GritMod = 0 → total = 1 < BaseDC 12 → fail.
	effect := world.RoomEffect{
		Track: "despair", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}
	ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 0, effect)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)
	require.NotNil(t, cbt)

	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()

	// Cooldown must NOT be set on a failed save.
	if sess.ZoneEffectCooldowns != nil {
		assert.Zero(t, sess.ZoneEffectCooldowns["room_a:despair"],
			"failed save must not set cooldown")
	}
	// Mental state manager should show despair applied.
	assert.Equal(t, mentalstate.SeverityMild, ch.mentalStateMgr.CurrentSeverity("p1", mentalstate.TrackDespair),
		"ApplyTrigger must have been called: despair should be mild after failed save")
}

// TestZoneEffect_Combat_SuccessfulSave_SetsCooldown verifies that a player who succeeds
// the zone effect save is granted cooldown immunity.
func TestZoneEffect_Combat_SuccessfulSave_SetsCooldown(t *testing.T) {
	// diceVal = 19 → roll = 20, GritMod = 0 → total = 20 >= BaseDC 12 → success.
	effect := world.RoomEffect{
		Track: "despair", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}
	ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 19, effect)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)

	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()

	require.NotNil(t, sess.ZoneEffectCooldowns)
	assert.Equal(t, int64(3), sess.ZoneEffectCooldowns["room_a:despair"],
		"successful save must set cooldown to CooldownRounds")
}

// TestZoneEffect_Combat_WithCooldown_Skipped verifies that a player who is still on
// cooldown immunity does not have a zone effect applied.
func TestZoneEffect_Combat_WithCooldown_Skipped(t *testing.T) {
	effect := world.RoomEffect{
		Track: "despair", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}
	ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 0, effect)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	// Pre-set cooldown.
	sess.ZoneEffectCooldowns = map[string]int64{"room_a:despair": 3}

	ch.combatMu.Lock()
	cbt, _, combatErr := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, combatErr)

	condsBefore := 0
	if sess.Conditions != nil {
		condsBefore = len(sess.Conditions.All())
	}

	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()

	condsAfter := 0
	if sess.Conditions != nil {
		condsAfter = len(sess.Conditions.All())
	}
	assert.Equal(t, condsBefore, condsAfter, "no new conditions when effect is on cooldown")
}

// TestZoneEffect_Combat_NPCSkipped verifies that NPC combatants are not affected by zone effects.
func TestZoneEffect_Combat_NPCSkipped(t *testing.T) {
	effect := world.RoomEffect{
		Track: "rage", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}
	ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 0, effect)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		ch.combatMu.Lock()
		ch.autoQueueNPCsLocked(cbt)
		ch.combatMu.Unlock()
	})
	assert.Equal(t, 10, inst.CurrentHP)
}

// TestZoneEffect_Combat_MentalStateMgrNil_NoPanic verifies that a nil mentalStateMgr
// does not cause a panic when zone effects are present.
func TestZoneEffect_Combat_MentalStateMgrNil_NoPanic(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	src := &fixedDiceSource{val: 0}
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	npcMgr := npc.NewManager()

	if r, ok := worldMgr.GetRoom("room_a"); ok {
		r.Effects = []world.RoomEffect{{Track: "despair", Severity: "mild", BaseDC: 12, CooldownRounds: 3}}
	}

	// nil mentalStateMgr (param 14)
	ch := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), worldMgr, nil, nil, nil, nil, nil, nil,
	)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		ch.combatMu.Lock()
		ch.autoQueueNPCsLocked(cbt)
		ch.combatMu.Unlock()
	}, "nil mentalStateMgr must not panic")
}

// TestZoneEffect_Combat_CooldownDecrement_ReachesZero verifies that a zone effect cooldown
// decrements each round and fires again once it reaches zero.
func TestZoneEffect_Combat_CooldownDecrement_ReachesZero(t *testing.T) {
	effect := world.RoomEffect{
		Track: "despair", Severity: "mild",
		BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
	}

	worldMgr, sessMgr := testWorldAndSession(t)
	if r, ok := worldMgr.GetRoom("room_a"); ok {
		r.Effects = []world.RoomEffect{effect}
	}
	npcMgr := npc.NewManager()
	mentalMgr := mentalstate.NewManager()
	// seqSource: rolls 1-2 consumed by RollInitiative (2 combatants),
	// roll 3 = 19 → zone effect save success → cooldown=3,
	// subsequent rolls = 0 → zone effect save fail.
	src := newSeqSource(0, 0, 19, 0, 0, 0, 0)
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	ch := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), worldMgr, nil, nil, nil, nil, nil, mentalMgr,
	)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)

	// Round 1: success → cooldown = 3.
	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()
	require.NotNil(t, sess.ZoneEffectCooldowns)
	assert.Equal(t, int64(3), sess.ZoneEffectCooldowns["room_a:despair"])

	// Rounds 2-4: decrement 3→2→1→0.
	for i := 0; i < 3; i++ {
		ch.combatMu.Lock()
		ch.autoQueueNPCsLocked(cbt)
		ch.combatMu.Unlock()
	}
	assert.Equal(t, int64(0), sess.ZoneEffectCooldowns["room_a:despair"],
		"after 3 decrements, cooldown should reach 0")

	// Round 5: roll=0 → fail → effect fires again.
	severityBefore := ch.mentalStateMgr.CurrentSeverity("p1", mentalstate.TrackDespair)
	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()
	severityAfter := ch.mentalStateMgr.CurrentSeverity("p1", mentalstate.TrackDespair)
	assert.GreaterOrEqual(t, int(severityAfter), int(severityBefore),
		"after cooldown reaches 0, effect must fire again on the next round with failing save")
}

// TestZoneEffect_Combat_CrossRoom_CooldownDecrement verifies REQ-T13:
// cooldowns from previously-visited rooms are decremented regardless of current room effects.
func TestZoneEffect_Combat_CrossRoom_CooldownDecrement(t *testing.T) {
	// Current room has NO effects — but old_room:despair cooldown should still decrement.
	worldMgr, sessMgr := testWorldAndSession(t)
	// room_a has NO effects (default)
	src := &fixedDiceSource{val: 19}
	roller := dice.NewLoggedRoller(src, zap.NewNop())
	npcMgr := npc.NewManager()
	mentalMgr := mentalstate.NewManager()
	ch := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), worldMgr, nil, nil, nil, nil, nil, mentalMgr,
	)

	tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "p1", Username: "p1", CharName: "Alice",
		RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
	})
	require.NoError(t, err)

	// Pre-inject a cooldown from a previously-visited room (room_a has no effects itself).
	sess.ZoneEffectCooldowns = map[string]int64{"old_room:despair": 2}

	ch.combatMu.Lock()
	cbt, _, err := ch.startCombatLocked(sess, inst)
	ch.combatMu.Unlock()
	require.NoError(t, err)

	// One round — old_room:despair should decrement to 1 even though room_a has no effects.
	ch.combatMu.Lock()
	ch.autoQueueNPCsLocked(cbt)
	ch.combatMu.Unlock()

	assert.Equal(t, int64(1), sess.ZoneEffectCooldowns["old_room:despair"],
		"cross-room cooldown must decrement even when current room has no zone effects")
}

// TestProperty_ZoneEffect_Combat_AnyTrack verifies that zone effects fire correctly for
// all supported mental state tracks when the player fails the save.
func TestProperty_ZoneEffect_Combat_AnyTrack(t *testing.T) {
	trackNames := []string{"rage", "despair", "delirium", "fear"}
	rapid.Check(t, func(rt *rapid.T) {
		idx := rapid.IntRange(0, 3).Draw(rt, "track_idx")
		trackName := trackNames[idx]

		effect := world.RoomEffect{
			Track: trackName, Severity: "mild",
			BaseDC: 12, CooldownRounds: 3, CooldownMinutes: 5,
		}
		// diceVal=0 → always fails.
		ch, sessMgr, npcMgr := newZoneEffectCombatHandler(t, 0, effect)

		tmpl := &npc.Template{ID: "rat", Name: "Rat", MaxHP: 10, AC: 10, Level: 1}
		inst, err := npcMgr.Spawn(tmpl, "room_a")
		require.NoError(rt, err)
		uid := "prop_" + rt.Name()
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: uid, CharName: "X",
			RoomID: "room_a", CurrentHP: 10, MaxHP: 10, Role: "player",
		})
		require.NoError(rt, err)

		ch.combatMu.Lock()
		cbt, _, err := ch.startCombatLocked(sess, inst)
		ch.combatMu.Unlock()
		require.NoError(rt, err)

		assert.NotPanics(rt, func() {
			ch.combatMu.Lock()
			ch.autoQueueNPCsLocked(cbt)
			ch.combatMu.Unlock()
		})
		// Failed save must NOT set cooldown.
		if sess.ZoneEffectCooldowns != nil {
			assert.Zero(rt, sess.ZoneEffectCooldowns["room_a:"+trackName],
				"failed save must not set cooldown for track %s", trackName)
		}
		// Mental state should be non-zero after failed save.
		trackConst := map[string]mentalstate.Track{
			"rage":     mentalstate.TrackRage,
			"despair":  mentalstate.TrackDespair,
			"delirium": mentalstate.TrackDelirium,
			"fear":     mentalstate.TrackFear,
		}[trackName]
		assert.NotEqual(rt, mentalstate.SeverityNone, ch.mentalStateMgr.CurrentSeverity(uid, trackConst),
			"ApplyTrigger must set non-zero severity for track %s after failed save", trackName)
	})
}
