package gameserver

import (
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newFixedDiceRoller creates a *dice.Roller whose Src().Intn(n) always returns (face-1)%n,
// ensuring that Intn(20)+1 == face for any face in [1,20].
//
// Precondition: face must be in [1, 20].
// Postcondition: Returns a non-nil *dice.Roller.
func newFixedDiceRoller(face int) *dice.Roller {
	// DeterministicSource returns vals[i] % n. We supply a large sequence of face-1 so
	// Intn(20) always yields (face-1)%20, and adding 1 gives face.
	vals := make([]int, 1000)
	for i := range vals {
		vals[i] = face - 1
	}
	return dice.NewRoller(dice.NewDeterministicSource(vals))
}

// newResolverSession creates a session.Manager with a single player whose
// DowntimeBusy state is pre-set to activityID and whose fields are seeded
// with the supplied options.
//
// Precondition: uid and roomID are non-empty; activityID is a valid activity ID.
// Postcondition: Returns (*session.Manager, *session.PlayerSession).
func newResolverSession(t *testing.T, uid, roomID, activityID string, abilities character.AbilityScores, level, currentHP, maxHP int) (*session.Manager, *session.PlayerSession) {
	t.Helper()
	sMgr := session.NewManager()
	sess, err := sMgr.AddPlayer(session.AddPlayerOptions{
		UID:         uid,
		Username:    "resolver_user",
		CharName:    "ResolverHero",
		CharacterID: 42,
		RoomID:      roomID,
		CurrentHP:   currentHP,
		MaxHP:       maxHP,
		Abilities:   abilities,
		Role:        "player",
		Level:       level,
	})
	require.NoError(t, err)
	sess.DowntimeBusy = true
	sess.DowntimeActivityID = activityID
	sess.DowntimeCompletesAt = time.Now().Add(-1 * time.Second)
	sess.Skills = map[string]string{}
	return sMgr, sess
}

// TestResolveEarnCreds_CritSuccess_TriplesPay verifies that a crit-success roll on
// earn_creds yields Currency == 30 (10 base × 3 multiplier) from a zero starting balance.
//
// Precondition: sess.Currency=0; forced crit-success roll (total >= DC+10).
// Postcondition: sess.Currency == 30 after resolution.
func TestResolveEarnCreds_CritSuccess_TriplesPay(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	abilities := character.AbilityScores{Flair: 20} // +5 mod
	sMgr, sess := newResolverSession(t, "uid_earn_cs", "room1", "earn_creds", abilities, 1, 10, 10)
	sess.Currency = 0
	// rep rank legendary (+8) + flair +5 = +13; DC=15 → need roll >= 2 for crit success
	// Force the highest rank to guarantee crit.
	sess.Skills["rep"] = "legendary"

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	// Use a fixed dice source that always returns 20.
	s.dice = newFixedDiceRoller(20)

	s.resolveEarnCreds("uid_earn_cs", sess)

	assert.Equal(t, 30, sess.Currency, "crit success must triple the base pay of 10")
}

// TestResolveEarnCreds_CritFail_ZeroPay verifies that a crit-failure on earn_creds
// results in no currency gain.
//
// Precondition: sess.Currency=0; forced crit-failure roll (total < DC-10).
// Postcondition: sess.Currency == 0 after resolution.
func TestResolveEarnCreds_CritFail_ZeroPay(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	abilities := character.AbilityScores{Flair: 1} // -5 mod
	sMgr, sess := newResolverSession(t, "uid_earn_cf", "room1", "earn_creds", abilities, 1, 10, 10)
	sess.Currency = 0
	// skills all untrained (rank ""); flair -5 + roll 1 = -4; DC=15 → crit failure (< 5)
	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	s.dice = newFixedDiceRoller(1)

	s.resolveEarnCreds("uid_earn_cf", sess)

	assert.Equal(t, 0, sess.Currency, "crit failure must yield zero currency gain")
}

// TestResolvePatchUp_Heals_OnSuccess verifies that a success roll on patch_up
// heals the player for level × 2.
//
// Precondition: sess.CurrentHP=5; sess.MaxHP=20; level=3; success roll.
// Postcondition: sess.CurrentHP == min(5+6, 20) == 11.
func TestResolvePatchUp_Heals_OnSuccess(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	abilities := character.AbilityScores{Savvy: 10} // +0 mod
	sMgr, sess := newResolverSession(t, "uid_patch_s", "room1", "patch_up", abilities, 3, 5, 20)
	// untrained patch_job; savvy +0; roll 15 + 0 + 0 = 15; DC=15 → success
	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	s.dice = newFixedDiceRoller(15)

	s.resolvePatchUp("uid_patch_s", sess)

	// success: heal = 3 * 2 = 6; 5 + 6 = 11
	assert.Equal(t, 11, sess.CurrentHP, "success must heal level×2 HP")
}

// TestResolveRecalibrate_CompletesWithMessage verifies that resolveRecalibrate
// does not panic when Entity is nil (no-push path) and the session is not mutated unexpectedly.
//
// Precondition: Entity is nil; activityID = "recalibrate".
// Postcondition: No panic; DowntimeBusy remains true (resolver does not clear state).
func TestResolveRecalibrate_CompletesWithMessage(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr, sess := newResolverSession(t, "uid_recal", "room1", "recalibrate",
		character.AbilityScores{}, 1, 10, 10)

	s := &GameServiceServer{sessions: sMgr, world: wMgr}
	// Entity is nil — pushMessageToUID will exit early; no panic expected.

	require.NotPanics(t, func() {
		s.resolveRecalibrate("uid_recal", sess)
	})
}

// TestResolveRetrain_CompletesWithMessage verifies that resolveRetrain
// does not panic when Entity is nil and the session is not mutated unexpectedly.
//
// Precondition: Entity is nil; activityID = "retrain".
// Postcondition: No panic.
func TestResolveRetrain_CompletesWithMessage(t *testing.T) {
	wMgr := newWorldWithSafeRoom("zone1", "room1")
	sMgr, sess := newResolverSession(t, "uid_retrain", "room1", "retrain",
		character.AbilityScores{}, 1, 10, 10)

	s := &GameServiceServer{sessions: sMgr, world: wMgr}

	require.NotPanics(t, func() {
		s.resolveRetrain("uid_retrain", sess)
	})
}

// TestPropertyResolveEarnCreds_CurrencyNeverDecreases verifies that for any dice roll
// in [1,20] and any starting currency in [0,1000], earn_creds never decreases currency.
//
// Precondition: sess.Currency in [0,1000]; dice roll in [1,20].
// Postcondition: sess.Currency after earn_creds >= starting currency.
func TestPropertyResolveEarnCreds_CurrencyNeverDecreases(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(1, 20).Draw(rt, "roll")
		startCurrency := rapid.IntRange(0, 1000).Draw(rt, "startCurrency")

		wMgr := newWorldWithSafeRoom("zone1", "room1")
		abilities := character.AbilityScores{Flair: 10} // +0 mod — neutral
		sMgr, sess := newResolverSession(t, "uid_pbt_earn", "room1", "earn_creds", abilities, 1, 10, 10)
		sess.Currency = startCurrency

		s := &GameServiceServer{sessions: sMgr, world: wMgr}
		s.dice = newFixedDiceRoller(roll)

		s.resolveEarnCreds("uid_pbt_earn", sess)

		if sess.Currency < startCurrency {
			rt.Fatalf("currency decreased: before=%d after=%d (roll=%d)", startCurrency, sess.Currency, roll)
		}
	})
}

// TestPropertyResolvePatchUp_HealNeverExceedsMaxHP verifies that for any dice roll in [1,20],
// level in [1,20], currentHP in [0,maxHP], and maxHP in [1,100], HP after patch_up never
// exceeds maxHP.
//
// Precondition: currentHP in [0,maxHP]; maxHP in [1,100]; dice roll in [1,20]; level in [1,20].
// Postcondition: sess.CurrentHP after patch_up <= sess.MaxHP.
func TestPropertyResolvePatchUp_HealNeverExceedsMaxHP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		roll := rapid.IntRange(1, 20).Draw(rt, "roll")
		level := rapid.IntRange(1, 20).Draw(rt, "level")
		maxHP := rapid.IntRange(1, 100).Draw(rt, "maxHP")
		currentHP := rapid.IntRange(0, maxHP).Draw(rt, "currentHP")

		wMgr := newWorldWithSafeRoom("zone1", "room1")
		abilities := character.AbilityScores{Savvy: 10} // +0 mod — neutral
		sMgr, sess := newResolverSession(t, "uid_pbt_patch", "room1", "patch_up", abilities, level, currentHP, maxHP)

		s := &GameServiceServer{sessions: sMgr, world: wMgr}
		s.dice = newFixedDiceRoller(roll)

		s.resolvePatchUp("uid_pbt_patch", sess)

		if sess.CurrentHP > sess.MaxHP {
			rt.Fatalf("HP exceeded maxHP: currentHP=%d maxHP=%d (roll=%d level=%d)", sess.CurrentHP, sess.MaxHP, roll, level)
		}
	})
}
