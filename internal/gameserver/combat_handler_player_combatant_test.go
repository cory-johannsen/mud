package gameserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// makePlayerSessionWithLevel creates a PlayerSession with an explicit Level and Brutality score.
func makePlayerSessionWithLevel(t *testing.T, sessMgr *session.Manager, uid, roomID string, level, brutality int) *session.PlayerSession {
	t.Helper()
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "testuser",
		CharName:    "Hero",
		CharacterID: 1,
		RoomID:      roomID,
		CurrentHP:   50,
		MaxHP:       50,
		Abilities:   character.AbilityScores{Brutality: brutality},
		Role:        "player",
	})
	if err != nil {
		t.Fatalf("makePlayerSessionWithLevel AddPlayer: %v", err)
	}
	sess.Level = level
	return sess
}

// REQ-CBT-PC-1: buildPlayerCombatant MUST set Level equal to the session's Level.
func TestBuildPlayerCombatant_LevelMatchesSession(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	const roomID = "room-pc-level"
	spawnTestNPC(t, h.npcMgr, roomID)
	sess := makePlayerSessionWithLevel(t, h.sessions, "pc-level-1", roomID, 9, 16)

	cbt := buildPlayerCombatant(sess, h)
	assert.Equal(t, 9, cbt.Level,
		"buildPlayerCombatant must use sess.Level, not a hardcoded value")
}

// REQ-CBT-PC-2: buildPlayerCombatant MUST set StrMod equal to AbilityMod(sess.Abilities.Brutality).
func TestBuildPlayerCombatant_StrModMatchesBrutality(t *testing.T) {
	h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
	const roomID = "room-pc-strmod"
	spawnTestNPC(t, h.npcMgr, roomID)
	// Brutality 16 → modifier = floor((16-10)/2) = +3
	sess := makePlayerSessionWithLevel(t, h.sessions, "pc-strmod-1", roomID, 9, 16)

	cbt := buildPlayerCombatant(sess, h)
	want := combat.AbilityMod(16)
	assert.Equal(t, want, cbt.StrMod,
		"buildPlayerCombatant must use AbilityMod(Brutality), not a hardcoded value")
}

// REQ-CBT-PC-3: For any session Level and Brutality, buildPlayerCombatant Level and StrMod
// must match the session values exactly.
func TestProperty_BuildPlayerCombatant_LevelAndStrModAlwaysMatchSession(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		brutality := rapid.IntRange(1, 30).Draw(rt, "brutality")

		h := makeCombatHandler(t, func(_ string, _ []*gamev1.CombatEvent) {})
		const roomID = "room-pc-prop"
		spawnTestNPC(t, h.npcMgr, roomID)
		uid := rapid.StringMatching(`[a-z]{4,8}`).Draw(rt, "uid")
		sess := makePlayerSessionWithLevel(t, h.sessions, uid, roomID, level, brutality)

		cbt := buildPlayerCombatant(sess, h)

		assert.Equal(rt, level, cbt.Level,
			"Level must equal sess.Level for any level value")
		assert.Equal(rt, combat.AbilityMod(brutality), cbt.StrMod,
			"StrMod must equal AbilityMod(Brutality) for any brutality value")
	})
}
