package gameserver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// useTechRegistry builds a minimal technology.Registry with a single def registered.
func useTechRegistry(def *technology.TechnologyDef) *technology.Registry {
	r := technology.NewRegistry()
	r.Register(def)
	return r
}

// REQ-TER15: When out of combat and tech requires targeting (resolution != none/empty and targets != self),
// and no targetID is given, the response must contain a "target" prompt message.
func TestHandleUse_REQ_TER15_OutOfCombatNoTarget_PromptForTarget(t *testing.T) {
	techDef := &technology.TechnologyDef{
		ID:         "neural_spike",
		Targets:    technology.TargetsSingle,
		Resolution: "attack",
	}
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	svc.SetTechRegistry(useTechRegistry(techDef))

	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"neural_spike": {MaxUses: 3, UsesRemaining: 3},
	}}
	svc.SetInnateTechRepo(repo)
	uid := "p-ter15"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"neural_spike": {MaxUses: 3, UsesRemaining: 3},
	}

	// No targetID — svc.combatH is nil so player is "not in combat".
	evt, err := svc.handleUse(uid, "neural_spike", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected a message event")
	assert.True(t, strings.Contains(strings.ToLower(msg.Content), "target"),
		"expected 'target' prompt, got: %q", msg.Content)
}

// REQ-TER16: When out of combat and a targetID is given for a targeting tech,
// the response must contain "not in combat".
func TestHandleUse_REQ_TER16_OutOfCombatWithTarget_NotInCombatMessage(t *testing.T) {
	techDef := &technology.TechnologyDef{
		ID:         "neural_spike",
		Targets:    technology.TargetsSingle,
		Resolution: "attack",
	}
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	svc.SetTechRegistry(useTechRegistry(techDef))

	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"neural_spike": {MaxUses: 3, UsesRemaining: 3},
	}}
	svc.SetInnateTechRepo(repo)
	uid := "p-ter16"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"neural_spike": {MaxUses: 3, UsesRemaining: 3},
	}

	evt, err := svc.handleUse(uid, "neural_spike", "ganger", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage()
	require.NotNil(t, msg, "expected a message event")
	assert.True(t, strings.Contains(strings.ToLower(msg.Content), "not in combat"),
		"expected 'not in combat' message, got: %q", msg.Content)
}

// REQ-TER17: When tech is self-targeting (resolution: none, targets: self) and out of combat,
// it resolves successfully; a heal effect increases CurrentHP.
//
// Property: for any initial HP in [1,90] and any heal amount in [1,10], after activation
// CurrentHP == min(initialHP+healAmt, maxHP).
func TestHandleUse_REQ_TER17_SelfTargetingHeal_OutOfCombat(t *testing.T) {
	const maxHP = 100
	rapid.Check(t, func(rt *rapid.T) {
		healAmt := rapid.IntRange(1, 10).Draw(rt, "heal_amt")
		initialHP := rapid.IntRange(1, 90).Draw(rt, "initial_hp")

		sessMgr := session.NewManager()
		svc := testMinimalService(t, sessMgr)

		techDef := &technology.TechnologyDef{
			ID:         "nano_repair",
			Targets:    "self",
			Resolution: "none",
			Effects: technology.TieredEffects{
				OnApply: []technology.TechEffect{
					{
						Type:   technology.EffectHeal,
						Amount: healAmt,
					},
				},
			},
		}
		svc.SetTechRegistry(useTechRegistry(techDef))

		repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
			"nano_repair": {MaxUses: 5, UsesRemaining: 5},
		}}
		svc.SetInnateTechRepo(repo)
		uid := "p-ter17"
		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:      uid,
			Username: uid,
			CharName: uid,
			RoomID:   "room_a",
			Role:     "player",
			MaxHP:    maxHP,
		})
		if err != nil {
			rt.Skip()
		}
		sess, ok := sessMgr.GetPlayer(uid)
		if !ok {
			rt.Skip()
		}
		sess.CurrentHP = initialHP
		sess.MaxHP = maxHP
		sess.InnateTechs = map[string]*session.InnateSlot{
			"nano_repair": {MaxUses: 5, UsesRemaining: 5},
		}

		evt, err := svc.handleUse(uid, "nano_repair", "", 0, 0)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if evt == nil {
			rt.Fatalf("expected non-nil event")
		}
		msg := evt.GetMessage()
		if msg == nil {
			rt.Fatalf("expected a message event")
		}
		// Must not return error messages about targeting or combat.
		lower := strings.ToLower(msg.Content)
		if strings.Contains(lower, "not in combat") {
			rt.Fatalf("unexpected 'not in combat' for self-tech: %q", msg.Content)
		}
		if strings.HasPrefix(lower, "specify a target") {
			rt.Fatalf("unexpected target prompt for self-tech: %q", msg.Content)
		}
		// HP must have increased (capped at maxHP).
		expected := initialHP + healAmt
		if expected > maxHP {
			expected = maxHP
		}
		if sess.CurrentHP != expected {
			rt.Fatalf("expected HP %d, got %d (initial=%d heal=%d)", expected, sess.CurrentHP, initialHP, healAmt)
		}
	})
}
