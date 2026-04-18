package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// REQ-TSN-6: handleUse resolves a short name to the canonical tech ID.
// Scenario: player knows "tech_alpha" as spontaneous; pool for level 1 has 0 remaining.
// "use ta" (short name) must resolve to "tech_alpha" and return "No level 1 uses remaining."
// If resolution failed, the message would be "You don't know ta."
func TestHandleUse_ShortName_ResolvesToCanonicalID(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)

	techReg := technology.NewRegistry()
	techReg.Register(&technology.TechnologyDef{
		ID:        "tech_alpha",
		ShortName: "ta",
		Name:      "Alpha Technology",
		Tradition: technology.TraditionTechnical,
		Level:     1,
		UsageType: technology.UsageHardwired,
		Range:     technology.RangeSelf,
		Targets:   technology.TargetsSingle,
		Duration:  "instant",
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{{Type: technology.EffectUtility, UtilityType: "unlock"}},
		},
	})

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, nil, techReg, nil, nil,
		nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
		nil,
		nil,
		nil, nil,
	)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID:         "u_tsn",
		Username:    "TSNPlayer",
		CharName:    "TSNPlayer",
		CharacterID: 0,
		RoomID:      "room_a",
		Abilities:   character.AbilityScores{},
		Role:        "player",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer("u_tsn")
	require.True(t, ok)
	sess.KnownTechs = map[int][]string{1: {"tech_alpha"}}
	sess.SpontaneousUsePools = map[int]session.UsePool{1: {Remaining: 0, Max: 2}}

	event, err := svc.handleUse("u_tsn", "ta", "", 0, 0)
	require.NoError(t, err)
	require.NotNil(t, event)

	msgEvt := event.GetPayload().(*gamev1.ServerEvent_Message)
	require.NotNil(t, msgEvt)
	assert.Equal(t, "No level 1 uses remaining.", msgEvt.Message.Content)
}
