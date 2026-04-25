package gameserver

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// REQ-RXN10: ReactionsRemaining resets to 1 after resolveAndAdvanceLocked.
func TestReactionsRemaining_ResetsToOneAfterRound(t *testing.T) {
	_, sessMgr, combatHandler := newAutoCombatSvc(t)

	player, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "rxn-reset-player",
		Username:  "Rxn",
		CharName:  "Rxn",
		RoomID:    "room_rxn",
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	require.NotNil(t, player)

	// Manually drain the reaction counter as if a reaction was spent this round.
	player.ReactionsRemaining = 0

	npcInst := &npc.Instance{
		TemplateID: "rxn-npc",
		RoomID:     "room_rxn",
		MaxHP:      10,
		CurrentHP:  10,
		AC:         12,
		Level:      1,
	}

	combatHandler.combatMu.Lock()
	cbt, _, err := combatHandler.startCombatLocked(player, npcInst)
	combatHandler.combatMu.Unlock()
	require.NoError(t, err)
	require.NotNil(t, cbt)

	// Drain again — round has not resolved yet.
	player.ReactionsRemaining = 0

	combatHandler.combatMu.Lock()
	combatHandler.resolveAndAdvanceLocked("room_rxn", cbt)
	combatHandler.combatMu.Unlock()

	assert.Equal(t, 1, player.ReactionsRemaining, "ReactionsRemaining must be reset to 1 after each round")
}

// REQ-RXN10: ReactionCallback skips when ReactionsRemaining is 0.
func TestReactionCallback_SkipsWhenNoReactionsRemaining(t *testing.T) {
	sess := &session.PlayerSession{
		UID:                "rxn-skip",
		ReactionsRemaining: 0,
		Reactions:          reaction.NewReactionRegistry(),
	}
	def := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnDamageTaken},
		Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage},
	}
	sess.Reactions.Register("rxn-skip", "test_feat", "Test Feat", def)

	called := false
	var cb reaction.ReactionCallback = func(_ context.Context, uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext, _ []reaction.PlayerReaction) (bool, *reaction.PlayerReaction, error) {
		if sess.ReactionsRemaining <= 0 {
			return false, nil, nil
		}
		called = true
		return true, nil, nil
	}
	sess.ReactionFn = cb

	ctx := reaction.ReactionContext{TriggerUID: "rxn-skip", DamagePending: new(5)}
	spent, _, err := sess.ReactionFn(context.Background(), "rxn-skip", reaction.TriggerOnDamageTaken, ctx, nil)
	require.NoError(t, err)
	assert.False(t, spent, "reaction must not be spent when ReactionsRemaining is 0")
	assert.False(t, called)
}

// REQ-RXN10: Second trigger is skipped after reaction was spent in same round.
func TestReactionCallback_SecondTriggerSkipped_WhenReactionSpent(t *testing.T) {
	sess := &session.PlayerSession{
		UID:                "rxn-spent",
		ReactionsRemaining: 1,
		Reactions:          reaction.NewReactionRegistry(),
	}
	def := reaction.ReactionDef{
		Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnDamageTaken},
		Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage},
	}
	sess.Reactions.Register("rxn-spent", "test_feat", "Test Feat", def)

	callCount := 0
	var cb reaction.ReactionCallback = func(_ context.Context, uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext, _ []reaction.PlayerReaction) (bool, *reaction.PlayerReaction, error) {
		if sess.ReactionsRemaining <= 0 {
			return false, nil, nil
		}
		callCount++
		sess.ReactionsRemaining--
		return true, nil, nil
	}
	sess.ReactionFn = cb

	ctx1 := reaction.ReactionContext{TriggerUID: "rxn-spent", DamagePending: new(5)}
	spent1, _, err := sess.ReactionFn(context.Background(), "rxn-spent", reaction.TriggerOnDamageTaken, ctx1, nil)
	require.NoError(t, err)
	assert.True(t, spent1)

	ctx2 := reaction.ReactionContext{TriggerUID: "rxn-spent", DamagePending: new(3)}
	spent2, _, err := sess.ReactionFn(context.Background(), "rxn-spent", reaction.TriggerOnDamageTaken, ctx2, nil)
	require.NoError(t, err)
	assert.False(t, spent2, "second trigger must be skipped after reaction is spent")
	assert.Equal(t, 1, callCount, "callback logic must only fire once per round")
}

// REQ-RXN22: Decline preserves the original save outcome.
func TestReactionCallback_Decline_OriginalOutcomePreserved(t *testing.T) {
	original := 2 // Failure
	ctx := reaction.ReactionContext{SaveOutcome: &original}

	// Simulate a decline: callback returns false without modifying ctx.
	var cb reaction.ReactionCallback = func(_ context.Context, uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext, _ []reaction.PlayerReaction) (bool, *reaction.PlayerReaction, error) {
		return false, nil, nil
	}
	spent, _, err := cb(context.Background(), "p1", reaction.TriggerOnSaveFail, ctx, nil)
	require.NoError(t, err)
	assert.False(t, spent)
	assert.Equal(t, 2, original, "original save outcome must be preserved when reaction is declined")
}

// REQ-RXN22: ApplyReactionEffect with RerollSave never worsens outcome (property test).
func TestReactionCallback_AcceptRerollSave_OutcomeImproved(t *testing.T) {
	for i := 0; i < 100; i++ {
		original := 3 // CritFailure — worst possible; can only stay same or improve
		ctx := reaction.ReactionContext{SaveOutcome: &original}
		effect := reaction.ReactionEffect{Type: reaction.ReactionEffectRerollSave, Keep: "better"}
		sess := &session.PlayerSession{}
		ApplyReactionEffect(sess, effect, &ctx)
		assert.LessOrEqual(t, *ctx.SaveOutcome, 3, "reroll must not worsen outcome")
		assert.GreaterOrEqual(t, *ctx.SaveOutcome, 0, "reroll must remain within valid range")
	}
}

// REQ-RXN16: nil ReactionFn on a session must not cause a panic in the combat dispatch wrapper.
func TestCombatHandler_NilReactionFn_NoSessionPanic(t *testing.T) {
	_, sessMgr, combatHandler := newAutoCombatSvc(t)

	player, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:       "rxn-nil-fn",
		Username:  "NilFn",
		CharName:  "NilFn",
		RoomID:    "room_nil",
		CurrentHP: 20,
		MaxHP:     20,
		Role:      "player",
	})
	require.NoError(t, err)
	require.NotNil(t, player)
	// Ensure ReactionFn is nil — default from AddPlayer is nil.
	player.ReactionFn = nil

	npcInst := &npc.Instance{
		TemplateID: "nil-npc",
		RoomID:     "room_nil",
		MaxHP:      10,
		CurrentHP:  10,
		AC:         12,
		Level:      1,
	}

	combatHandler.combatMu.Lock()
	cbt, _, err := combatHandler.startCombatLocked(player, npcInst)
	combatHandler.combatMu.Unlock()
	require.NoError(t, err)
	require.NotNil(t, cbt)

	assert.NotPanics(t, func() {
		combatHandler.combatMu.Lock()
		defer combatHandler.combatMu.Unlock()
		combatHandler.resolveAndAdvanceLocked("room_nil", cbt)
	})
}
