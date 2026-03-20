package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalAmpedTech returns a valid TechnologyDef with AmpedLevel and AmpedEffects set.
//
// Precondition: nativeLevel and ampedLevel must satisfy 1 <= nativeLevel < ampedLevel <= 10.
// Postcondition: Returns a non-nil TechnologyDef that passes Validate().
func minimalAmpedTech(id string, nativeLevel, ampedLevel int) *technology.TechnologyDef {
	baseEffect := technology.TechEffect{
		Type:   technology.EffectHeal,
		Amount: 1,
	}
	ampedEffect := technology.TechEffect{
		Type:   technology.EffectHeal,
		Amount: 5,
	}
	return &technology.TechnologyDef{
		ID:        id,
		Name:      id + " Name",
		Tradition: technology.TraditionBioSynthetic,
		Level:     nativeLevel,
		UsageType: technology.UsageSpontaneous,
		Range:     technology.RangeSelf,
		Targets:   technology.TargetsSingle,
		Duration:  "instant",
		Resolution: "none",
		Effects:     technology.TieredEffects{OnApply: []technology.TechEffect{baseEffect}},
		AmpedLevel:  ampedLevel,
		AmpedEffects: technology.TieredEffects{OnApply: []technology.TechEffect{ampedEffect}},
	}
}

// newAmpedSvc creates a GameServiceServer with a populated TechRegistry for amped-use tests.
//
// Precondition: t must be non-nil; defs must be non-nil.
// Postcondition: Returns svc with TechRegistry containing all defs; sessMgr is usable.
func newAmpedSvc(t *testing.T, defs []*technology.TechnologyDef) (*GameServiceServer, *session.Manager) {
	t.Helper()
	repo := newFakeSpontaneousUsePoolRepo(map[int]session.UsePool{})
	svc, sessMgr := newSpontaneousSvc(t, repo)

	reg := technology.NewRegistry()
	for _, d := range defs {
		reg.Register(d)
	}
	svc.SetTechRegistry(reg)
	return svc, sessMgr
}

// TestHandleAmpedUse_ExplicitLevelAtAmpedThreshold_REQ_AMP1 verifies that requesting
// a slot level >= AmpedLevel activates amped effects and decrements pool[slotLevel].
//
// Precondition: tech native level 1, amped level 3; pool[3].Remaining == 2.
// Postcondition: response contains "amped effect"; pool[3].Remaining == 1.
func TestHandleAmpedUse_ExplicitLevelAtAmpedThreshold_REQ_AMP1(t *testing.T) {
	tech := minimalAmpedTech("arc_spark", 1, 3)
	svc, sessMgr := newAmpedSvc(t, []*technology.TechnologyDef{tech})

	sess := addSpontaneousPlayer(t, sessMgr, "u_amp1",
		map[int][]string{1: {"arc_spark"}},
		map[int]session.UsePool{
			1: {Remaining: 3, Max: 3},
			3: {Remaining: 2, Max: 3},
		},
	)

	req := &gamev1.UseRequest{FeatId: "arc_spark", Target: "3"}
	stream := &fakeSessionStream{}
	evt, err := svc.handleAmpedUse("u_amp1", req, stream)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "arc_spark Name activated. Level-3 uses remaining: 1", "expected activation line, got: %q", msg)
	assert.Equal(t, 1, sess.SpontaneousUsePools[3].Remaining, "expected pool[3] decremented to 1")
	assert.Equal(t, 3, sess.SpontaneousUsePools[1].Remaining, "pool[1] must not change")
}

// TestHandleAmpedUse_ExplicitLevelBelowAmpedThreshold_REQ_AMP2 verifies that requesting
// a slot level below AmpedLevel activates base effects and decrements pool[slotLevel].
//
// Precondition: tech native level 1, amped level 3; pool[2].Remaining == 2.
// Postcondition: response contains "base effect"; pool[2].Remaining == 1.
func TestHandleAmpedUse_ExplicitLevelBelowAmpedThreshold_REQ_AMP2(t *testing.T) {
	tech := minimalAmpedTech("arc_spark", 1, 3)
	svc, sessMgr := newAmpedSvc(t, []*technology.TechnologyDef{tech})

	sess := addSpontaneousPlayer(t, sessMgr, "u_amp2",
		map[int][]string{1: {"arc_spark"}},
		map[int]session.UsePool{
			1: {Remaining: 3, Max: 3},
			2: {Remaining: 2, Max: 3},
		},
	)

	req := &gamev1.UseRequest{FeatId: "arc_spark", Target: "2"}
	stream := &fakeSessionStream{}
	evt, err := svc.handleAmpedUse("u_amp2", req, stream)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "arc_spark Name activated. Level-2 uses remaining: 1", "expected activation line, got: %q", msg)
	assert.Equal(t, 1, sess.SpontaneousUsePools[2].Remaining, "expected pool[2] decremented to 1")
}

// TestHandleAmpedUse_DepletedSlot_REQ_AMP3 verifies that a depleted pool at the requested
// level returns an appropriate error message.
//
// Precondition: pool[3].Remaining == 0.
// Postcondition: message == "No level-3 uses remaining."
func TestHandleAmpedUse_DepletedSlot_REQ_AMP3(t *testing.T) {
	tech := minimalAmpedTech("arc_spark", 1, 3)
	svc, sessMgr := newAmpedSvc(t, []*technology.TechnologyDef{tech})

	addSpontaneousPlayer(t, sessMgr, "u_amp3",
		map[int][]string{1: {"arc_spark"}},
		map[int]session.UsePool{
			3: {Remaining: 0, Max: 3},
		},
	)

	req := &gamev1.UseRequest{FeatId: "arc_spark", Target: "3"}
	stream := &fakeSessionStream{}
	evt, err := svc.handleAmpedUse("u_amp3", req, stream)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Equal(t, "No level-3 uses remaining.", msg)
}

// TestHandleAmpedUse_InvalidLevelToken_REQ_AMP4 verifies that a non-numeric level token
// returns an appropriate error message.
//
// Precondition: target arg is "enemy 5x".
// Postcondition: message == "Invalid level: 5x."
func TestHandleAmpedUse_InvalidLevelToken_REQ_AMP4(t *testing.T) {
	tech := minimalAmpedTech("arc_spark", 1, 3)
	svc, sessMgr := newAmpedSvc(t, []*technology.TechnologyDef{tech})

	addSpontaneousPlayer(t, sessMgr, "u_amp4",
		map[int][]string{1: {"arc_spark"}},
		map[int]session.UsePool{1: {Remaining: 3, Max: 3}},
	)

	req := &gamev1.UseRequest{FeatId: "arc_spark", Target: "enemy 5x"}
	stream := &fakeSessionStream{}
	evt, err := svc.handleAmpedUse("u_amp4", req, stream)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Equal(t, "Invalid level: 5x.", msg)
}

// TestHandleAmpedUse_LevelBelowNative_REQ_AMP5 verifies that requesting a slot level
// below the tech's native level returns an error.
//
// Precondition: tech native level 2; requested level 1.
// Postcondition: message == "Cannot use arc_spark below its native level (2)."
func TestHandleAmpedUse_LevelBelowNative_REQ_AMP5(t *testing.T) {
	tech := minimalAmpedTech("arc_spark", 2, 4)
	svc, sessMgr := newAmpedSvc(t, []*technology.TechnologyDef{tech})

	addSpontaneousPlayer(t, sessMgr, "u_amp5",
		map[int][]string{2: {"arc_spark"}},
		map[int]session.UsePool{
			1: {Remaining: 3, Max: 3},
			2: {Remaining: 3, Max: 3},
		},
	)

	req := &gamev1.UseRequest{FeatId: "arc_spark", Target: "1"}
	stream := &fakeSessionStream{}
	evt, err := svc.handleAmpedUse("u_amp5", req, stream)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Equal(t, "Cannot use arc_spark below its native level (2).", msg)
}

// TestHandleAmpedUse_NoValidLevels_REQ_AMP6 verifies that when all pools at or above
// the tech's native level are depleted, an appropriate error is returned.
//
// Precondition: all pools >= native level have Remaining == 0.
// Postcondition: message == "No uses remaining at any level."
func TestHandleAmpedUse_NoValidLevels_REQ_AMP6(t *testing.T) {
	tech := minimalAmpedTech("arc_spark", 1, 3)
	svc, sessMgr := newAmpedSvc(t, []*technology.TechnologyDef{tech})

	addSpontaneousPlayer(t, sessMgr, "u_amp6",
		map[int][]string{1: {"arc_spark"}},
		map[int]session.UsePool{
			1: {Remaining: 0, Max: 3},
			2: {Remaining: 0, Max: 3},
			3: {Remaining: 0, Max: 3},
		},
	)

	req := &gamev1.UseRequest{FeatId: "arc_spark", Target: ""}
	stream := &fakeSessionStream{}
	evt, err := svc.handleAmpedUse("u_amp6", req, stream)
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Equal(t, "No uses remaining at any level.", msg)
}
