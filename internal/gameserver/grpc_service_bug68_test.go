package gameserver

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// stubInnateTechRepo is a no-op implementation for tests that don't use innate tech persistence.
type stubInnateTechRepo68 struct{}

func (r *stubInnateTechRepo68) GetAll(_ context.Context, _ int64) (map[string]int, error) {
	return nil, nil
}

// REQ-BUG68-1: FeatEntry.IsReaction must be true when the feat's Reaction field is non-nil.
func TestHandleChar_ReactionFeat_HasIsReactionTrue(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	reactionFeat := &ruleset.Feat{
		ID:          "shield-block",
		Name:        "Shield Block",
		Active:      true,
		Description: "Block with your shield.",
		Reaction: &reaction.ReactionDef{
			Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnDamageTaken},
			Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage},
		},
	}
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{reactionFeat})
	featsRepo := &stubFeatsRepo{
		data: map[int64][]string{
			0: {"shield-block"},
		},
	}

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		[]*ruleset.Feat{reactionFeat}, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_rxn_feat",
		Username:    "Blocker",
		CharName:    "Blocker",
		CharacterID: 0,
		RoomID:      "room_a",
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)

	event, err := svc.handleChar("u_rxn_feat")
	require.NoError(t, err)
	require.NotNil(t, event)

	cs, ok := event.Payload.(*gamev1.ServerEvent_CharacterSheet)
	require.True(t, ok, "expected CharacterSheet payload")
	require.Len(t, cs.CharacterSheet.Feats, 1)
	assert.True(t, cs.CharacterSheet.Feats[0].IsReaction, "FeatEntry.IsReaction must be true for a reaction feat")
}

// REQ-BUG68-2: FeatEntry.IsReaction must be false when the feat's Reaction field is nil.
func TestHandleChar_ActiveFeat_HasIsReactionFalse(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	activeFeat := &ruleset.Feat{
		ID:           "quick-draw",
		Name:         "Quick Draw",
		Active:       true,
		Description:  "Draw fast.",
		ActivateText: "Draw!",
		Reaction:     nil,
	}
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{activeFeat})
	featsRepo := &stubFeatsRepo{
		data: map[int64][]string{
			0: {"quick-draw"},
		},
	}

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		[]*ruleset.Feat{activeFeat}, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_active_feat",
		Username:    "Gunner",
		CharName:    "Gunner",
		CharacterID: 0,
		RoomID:      "room_a",
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)

	event, err := svc.handleChar("u_active_feat")
	require.NoError(t, err)
	require.NotNil(t, event)

	cs, ok := event.Payload.(*gamev1.ServerEvent_CharacterSheet)
	require.True(t, ok, "expected CharacterSheet payload")
	require.Len(t, cs.CharacterSheet.Feats, 1)
	assert.False(t, cs.CharacterSheet.Feats[0].IsReaction, "FeatEntry.IsReaction must be false for a non-reaction feat")
}

// REQ-BUG68-3: InnateSlotView.IsReaction must be true when the tech's Reaction field is non-nil.
func TestHandleChar_InnateReactionTech_HasIsReactionTrue(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	reactionTech := &technology.TechnologyDef{
		ID:          "chrome-reflex",
		Name:        "Chrome Reflex",
		UsageType:   technology.UsageInnate,
		Description: "Reflexive chrome augmentation.",
		Tradition:   technology.TraditionBioSynthetic,
		Range:       technology.RangeSelf,
		Level:       1,
		Reaction: &reaction.ReactionDef{
			Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnDamageTaken},
			Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage},
		},
	}
	techReg := technology.NewRegistry()
	techReg.Register(reactionTech)

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, techReg, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_rxn_tech",
		Username:    "Chrome",
		CharName:    "Chrome",
		CharacterID: 0,
		RoomID:      "room_a",
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"chrome-reflex": {UsesRemaining: 1, MaxUses: 1},
	}

	event, err := svc.handleChar("u_rxn_tech")
	require.NoError(t, err)
	require.NotNil(t, event)

	cs, ok := event.Payload.(*gamev1.ServerEvent_CharacterSheet)
	require.True(t, ok, "expected CharacterSheet payload")
	require.Len(t, cs.CharacterSheet.InnateSlots, 1)
	assert.True(t, cs.CharacterSheet.InnateSlots[0].IsReaction, "InnateSlotView.IsReaction must be true for a reaction tech")
}

// REQ-BUG68-4: Property test — FeatEntry.IsReaction always equals (Feat.Reaction != nil).
func TestProperty_FeatEntry_IsReactionMatchesFeatReactionField(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		hasReaction := rapid.Bool().Draw(rt, "hasReaction")
		isActive := rapid.Bool().Draw(rt, "isActive")

		var rxnDef *reaction.ReactionDef
		if hasReaction {
			rxnDef = &reaction.ReactionDef{
				Triggers: []reaction.ReactionTriggerType{reaction.TriggerOnDamageTaken},
				Effect:   reaction.ReactionEffect{Type: reaction.ReactionEffectReduceDamage},
			}
		}

		feat := &ruleset.Feat{
			ID:       "prop-feat",
			Name:     "Prop Feat",
			Active:   isActive,
			Reaction: rxnDef,
		}
		featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{feat})
		featsRepo := &stubFeatsRepo{
			data: map[int64][]string{0: {"prop-feat"}},
		}

		worldMgr, sessMgr := testWorldAndSession(t)
		logger := zaptest.NewLogger(t)

		svc := newTestGameServiceServer(
			worldMgr, sessMgr,
			command.DefaultRegistry(),
			NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
			NewChatHandler(sessMgr),
			logger,
			nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil, "",
			nil, nil, nil,
			[]*ruleset.Feat{feat}, featRegistry, featsRepo,
			nil, nil, nil, nil, nil, nil, nil,
			nil, nil,
			nil,
			nil,
			nil, nil,
		)

		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID:         "u_prop",
			Username:    "Prop",
			CharName:    "Prop",
			CharacterID: 0,
			RoomID:      "room_a",
			Abilities:   character.AbilityScores{},
			Role:        "player",
		})
		require.NoError(t, err)

		event, err := svc.handleChar("u_prop")
		require.NoError(t, err)
		require.NotNil(t, event)

		cs, ok := event.Payload.(*gamev1.ServerEvent_CharacterSheet)
		require.True(t, ok)
		require.Len(t, cs.CharacterSheet.Feats, 1)

		entry := cs.CharacterSheet.Feats[0]
		assert.Equal(t, hasReaction, entry.IsReaction,
			"FeatEntry.IsReaction must equal (Feat.Reaction != nil)")
	})
}
