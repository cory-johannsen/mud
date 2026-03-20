package gameserver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/technology"
)

// innateRepoForGrpcTest implements InnateTechRepo for innate tech grpc tests.
// Named distinctly from fakeInnateRepo in technology_assignment_test.go (different package).
type innateRepoForGrpcTest struct {
	slots            map[string]*session.InnateSlot
	decremented      []string
	restoreAllCalled int
}

func (r *innateRepoForGrpcTest) GetAll(_ context.Context, _ int64) (map[string]*session.InnateSlot, error) {
	out := make(map[string]*session.InnateSlot)
	for k, v := range r.slots {
		cp := *v
		out[k] = &cp
	}
	return out, nil
}
func (r *innateRepoForGrpcTest) Set(_ context.Context, _ int64, techID string, maxUses int) error {
	if r.slots == nil {
		r.slots = make(map[string]*session.InnateSlot)
	}
	r.slots[techID] = &session.InnateSlot{MaxUses: maxUses, UsesRemaining: maxUses}
	return nil
}
func (r *innateRepoForGrpcTest) DeleteAll(_ context.Context, _ int64) error {
	r.slots = nil
	return nil
}
func (r *innateRepoForGrpcTest) Decrement(_ context.Context, _ int64, techID string) error {
	r.decremented = append(r.decremented, techID)
	if r.slots != nil {
		if s, ok := r.slots[techID]; ok && s.UsesRemaining > 0 {
			s.UsesRemaining--
		}
	}
	return nil
}
func (r *innateRepoForGrpcTest) RestoreAll(_ context.Context, _ int64) error {
	r.restoreAllCalled++
	for _, s := range r.slots {
		s.UsesRemaining = s.MaxUses
	}
	return nil
}

// innateTestService sets up a minimal service with innateTechRepo wired.
// handleUse does not use a stream — it returns (*gamev1.ServerEvent, error).
func innateTestService(t *testing.T, innateTechRepo *innateRepoForGrpcTest) (*GameServiceServer, string) {
	t.Helper()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	svc.SetInnateTechRepo(innateTechRepo)

	uid := "player-innate-test"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	return svc, uid
}

// REQ-INN1: use <tech> with uses remaining → activation message; DB decremented.
func TestHandleUse_InnateActivation_DecrementsCalled(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 1},
	}}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 1},
	}

	evt, err := svc.handleUse(uid, "acid_spit", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.True(t, strings.Contains(msg, "acid_spit"), "expected activation message containing acid_spit, got: %q", msg)
	assert.Contains(t, repo.decremented, "acid_spit", "expected Decrement called for acid_spit")
	assert.Equal(t, 0, sess.InnateTechs["acid_spit"].UsesRemaining)
}

// REQ-INN2: use <tech> with 0 uses → "No uses of <tech> remaining."
func TestHandleUse_InnateExhausted_ReturnsNoUsesMessage(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 0},
	}}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 0},
	}

	evt, err := svc.handleUse(uid, "acid_spit", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "No uses of acid_spit remaining", "expected exhausted message")
	assert.Empty(t, repo.decremented, "Decrement must not be called when exhausted")
}

// REQ-INN3: use <tech> not in innate techs → "You don't have innate tech <tech>."
func TestHandleUse_InnateNotKnown_ReturnsNotKnownMessage(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{}}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{}

	evt, err := svc.handleUse(uid, "acid_spit", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "don't have innate tech acid_spit")
}

// REQ-INN4: use <tech> unlimited (MaxUses=0) → activation message; Decrement NOT called.
func TestHandleUse_InnateUnlimited_NoDecrement(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"blackout_pulse": {MaxUses: 0, UsesRemaining: 0},
	}}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"blackout_pulse": {MaxUses: 0, UsesRemaining: 0},
	}

	evt, err := svc.handleUse(uid, "blackout_pulse", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.True(t, strings.Contains(msg, "blackout_pulse"), "expected activation message for blackout_pulse, got: %q", msg)
	assert.Empty(t, repo.decremented, "Decrement must NOT be called for unlimited tech")
}

// REQ-INN5: use (no-arg) lists innate techs; unlimited shown as (unlimited); exhausted omitted.
func TestHandleUse_NoArg_ListsInnateTechs(t *testing.T) {
	repo := &innateRepoForGrpcTest{}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"blackout_pulse": {MaxUses: 0, UsesRemaining: 0}, // unlimited — include in choices
		"acid_spit":      {MaxUses: 1, UsesRemaining: 1}, // 1 remaining — include
		"pressure_burst": {MaxUses: 1, UsesRemaining: 0}, // exhausted — omit
	}

	evt, err := svc.handleUse(uid, "", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	choices := evt.GetUseResponse().GetChoices()
	var descriptions []string
	for _, c := range choices {
		descriptions = append(descriptions, c.GetDescription())
	}
	joined := strings.Join(descriptions, "\n")

	assert.Contains(t, joined, "blackout_pulse")
	assert.Contains(t, joined, "unlimited")
	assert.Contains(t, joined, "acid_spit")
	assert.NotContains(t, joined, "pressure_burst", "exhausted innate tech must not appear in list")
}

// Property: for MaxUses >= 1 and UsesRemaining in [1, MaxUses], activation
// always decrements UsesRemaining by exactly 1 and calls Decrement exactly once.
func TestProperty_InnateActivation_DecrementsExactlyOnce(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxUses := rapid.IntRange(1, 10).Draw(rt, "maxUses")
		usesRemaining := rapid.IntRange(1, maxUses).Draw(rt, "usesRemaining")

		repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
			"test_tech": {MaxUses: maxUses, UsesRemaining: usesRemaining},
		}}
		svc, uid := innateTestService(t, repo)
		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Skip()
		}
		sess.InnateTechs = map[string]*session.InnateSlot{
			"test_tech": {MaxUses: maxUses, UsesRemaining: usesRemaining},
		}

		evt, err := svc.handleUse(uid, "test_tech", "")
		if err != nil {
			rt.Fatalf("handleUse failed: %v", err)
		}
		if evt == nil {
			rt.Fatal("handleUse returned nil event")
		}

		// Must call Decrement exactly once
		if len(repo.decremented) != 1 || repo.decremented[0] != "test_tech" {
			rt.Errorf("expected Decrement called once for test_tech, got: %v", repo.decremented)
		}
		// Session UsesRemaining must decrease by 1
		if sess.InnateTechs["test_tech"].UsesRemaining != usesRemaining-1 {
			rt.Errorf("expected UsesRemaining=%d, got=%d", usesRemaining-1, sess.InnateTechs["test_tech"].UsesRemaining)
		}
	})
}

// Property: for MaxUses >= 1 and UsesRemaining == 0, exhausted message returned
// and Decrement never called.
func TestProperty_InnateExhausted_NoDecrement(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		maxUses := rapid.IntRange(1, 10).Draw(rt, "maxUses")

		repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
			"test_tech": {MaxUses: maxUses, UsesRemaining: 0},
		}}
		svc, uid := innateTestService(t, repo)
		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Skip()
		}
		sess.InnateTechs = map[string]*session.InnateSlot{
			"test_tech": {MaxUses: maxUses, UsesRemaining: 0},
		}

		evt, err := svc.handleUse(uid, "test_tech", "")
		if err != nil {
			rt.Fatalf("handleUse failed: %v", err)
		}
		if evt == nil {
			rt.Fatal("handleUse returned nil event")
		}

		msg := evt.GetMessage().GetContent()
		if !strings.Contains(msg, "No uses of test_tech remaining") {
			rt.Errorf("expected exhausted message, got: %q", msg)
		}
		if len(repo.decremented) != 0 {
			rt.Errorf("Decrement must not be called on exhausted tech, got: %v", repo.decremented)
		}
	})
}

// Property: no-arg list includes exactly techs where MaxUses==0 OR UsesRemaining>0.
func TestProperty_InnateNoArgList_ExcludesExhausted(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate N distinct tech IDs with random slot states
		n := rapid.IntRange(1, 5).Draw(rt, "n")

		slots := make(map[string]*session.InnateSlot)
		expectedIDs := make(map[string]bool)
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("tech_%d", i)
			maxUses := rapid.IntRange(0, 3).Draw(rt, fmt.Sprintf("maxUses_%d", i))
			var remaining int
			if maxUses == 0 {
				remaining = 0 // unlimited
				expectedIDs[id] = true
			} else {
				remaining = rapid.IntRange(0, maxUses).Draw(rt, fmt.Sprintf("remaining_%d", i))
				if remaining > 0 {
					expectedIDs[id] = true
				}
			}
			slots[id] = &session.InnateSlot{MaxUses: maxUses, UsesRemaining: remaining}
		}

		repo := &innateRepoForGrpcTest{}
		svc, uid := innateTestService(t, repo)
		sess, ok := svc.sessions.GetPlayer(uid)
		if !ok {
			rt.Skip()
		}
		sess.InnateTechs = slots

		evt, err := svc.handleUse(uid, "", "")
		if err != nil {
			rt.Fatalf("handleUse failed: %v", err)
		}
		if evt == nil {
			rt.Fatal("handleUse returned nil event")
		}

		choices := evt.GetUseResponse().GetChoices()
		listedIDs := make(map[string]bool)
		for _, c := range choices {
			// Only count innate_tech category entries
			if c.GetCategory() == "innate_tech" {
				listedIDs[c.GetFeatId()] = true
			}
		}

		for id := range expectedIDs {
			if !listedIDs[id] {
				rt.Errorf("expected %q in list (MaxUses=%d, Remaining=%d), but not found",
					id, slots[id].MaxUses, slots[id].UsesRemaining)
			}
		}
		for id := range listedIDs {
			if !expectedIDs[id] {
				rt.Errorf("unexpected %q in list (MaxUses=%d, Remaining=%d)",
					id, slots[id].MaxUses, slots[id].UsesRemaining)
			}
		}
	})
}

// REQ-INN6: After rest, limited innate slots restored to max (session + DB RestoreAll called).
func TestHandleRest_RestoresInnateSlots(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 0},
	}}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 0},
	}

	stream := &fakeSessionStream{}
	err := svc.handleRest(uid, "req1", stream)
	require.NoError(t, err)

	assert.Equal(t, 1, repo.restoreAllCalled, "RestoreAll must be called once on rest")
	assert.Equal(t, 1, sess.InnateTechs["acid_spit"].UsesRemaining, "session slot must be restored")
}

// REQ-CRX6: use <tech> is blocked for reaction-bearing techs.
func TestHandleUse_ReactionTech_BlocksActivation(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"chrome_reflex": {MaxUses: 1, UsesRemaining: 1},
	}}
	svc, uid := innateTestService(t, repo)

	// Build a registry with chrome_reflex having a Reaction.
	reg := technology.NewRegistry()
	reg.Register(&technology.TechnologyDef{
		ID:        "chrome_reflex",
		Name:      "Chrome Reflex",
		Tradition: technology.TraditionNeural,
		Level:     1,
		UsageType: technology.UsageInnate,
		Range:     technology.RangeSelf,
		Targets:   technology.TargetsSingle,
		Duration:  "instant",
		Resolution: "none",
		Reaction: &reaction.ReactionDef{
			Triggers: []reaction.ReactionTriggerType{
				reaction.TriggerOnSaveFail,
				reaction.TriggerOnSaveCritFail,
			},
			Effect: reaction.ReactionEffect{
				Type: reaction.ReactionEffectRerollSave,
				Keep: "better",
			},
		},
		Effects: technology.TieredEffects{
			OnApply: []technology.TechEffect{{Type: technology.EffectUtility, UtilityType: "reaction_placeholder"}},
		},
	})
	svc.SetTechRegistry(reg)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"chrome_reflex": {MaxUses: 1, UsesRemaining: 1},
	}

	evt, err := svc.handleUse(uid, "chrome_reflex", "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "fires automatically as a reaction", "expected reaction block message")
	assert.Empty(t, repo.decremented, "Decrement must NOT be called for reaction tech")
	assert.Equal(t, 1, sess.InnateTechs["chrome_reflex"].UsesRemaining, "uses must not be decremented")
}
