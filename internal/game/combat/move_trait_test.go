package combat_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// moveTraitSrc is a deterministic combat.Source for tests.
type moveTraitSrc struct{ r *rand.Rand }

func (s moveTraitSrc) Intn(n int) int { return s.r.Intn(n) }

// TestMoveTrait_QueueFreeAction_NoAPSpent confirms WMOVE-7: a free Stride
// queued via QueueFreeAction does not consume AP and does not increment the
// per-round movement AP cap.
func TestMoveTrait_QueueFreeAction_NoAPSpent(t *testing.T) {
	q := combat.NewActionQueue("uid", 3)
	require.Equal(t, 3, q.RemainingPoints())
	err := q.QueueFreeAction(combat.QueuedAction{
		Type:         combat.ActionMoveTraitStride,
		StrikeAction: 1,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, q.RemainingPoints(), "free action must not deduct AP (WMOVE-7)")
	assert.Equal(t, 0, q.MovementAPSpent(), "free Stride must not increment movementAPSpent (WMOVE-7)")
	assert.Len(t, q.QueuedActions(), 1)
}

// TestMoveTrait_QueueFreeAction_RejectsNonAllowlisted confirms WMOVE-8: only
// the explicit allowlist of free-action types is accepted.
func TestMoveTrait_QueueFreeAction_RejectsNonAllowlisted(t *testing.T) {
	q := combat.NewActionQueue("uid", 3)
	err := q.QueueFreeAction(combat.QueuedAction{Type: combat.ActionStride})
	require.Error(t, err, "QueueFreeAction must reject types outside the allowlist (WMOVE-8)")
	err = q.QueueFreeAction(combat.QueuedAction{Type: combat.ActionStrike})
	require.Error(t, err)
	err = q.QueueFreeAction(combat.QueuedAction{Type: combat.ActionAttack})
	require.Error(t, err)
}

// TestMoveTrait_QueueFreeAction_OncePerStrike confirms WMOVE-10: at most one
// ActionMoveTraitStride may be queued for a given StrikeAction id.
func TestMoveTrait_QueueFreeAction_OncePerStrike(t *testing.T) {
	q := combat.NewActionQueue("uid", 3)
	require.NoError(t, q.QueueFreeAction(combat.QueuedAction{
		Type: combat.ActionMoveTraitStride, StrikeAction: 7,
	}))
	err := q.QueueFreeAction(combat.QueuedAction{
		Type: combat.ActionMoveTraitStride, StrikeAction: 7,
	})
	require.Error(t, err, "second free Stride for the same Strike must be rejected (WMOVE-10)")
}

// TestMoveTrait_QueueFreeAction_SeparateStrikesAllowed: different StrikeAction
// ids must be allowed to queue independent free Strides.
func TestMoveTrait_QueueFreeAction_SeparateStrikesAllowed(t *testing.T) {
	q := combat.NewActionQueue("uid", 6)
	require.NoError(t, q.QueueFreeAction(combat.QueuedAction{
		Type: combat.ActionMoveTraitStride, StrikeAction: 1,
	}))
	require.NoError(t, q.QueueFreeAction(combat.QueuedAction{
		Type: combat.ActionMoveTraitStride, StrikeAction: 2,
	}))
}

// TestMoveTrait_ActionType_CostIsZero confirms ActionMoveTraitStride.Cost() == 0.
func TestMoveTrait_ActionType_CostIsZero(t *testing.T) {
	assert.Equal(t, 0, combat.ActionMoveTraitStride.Cost())
}

// TestMoveTrait_ActionType_String returns a stable, descriptive name.
func TestMoveTrait_ActionType_String(t *testing.T) {
	assert.Equal(t, "move_trait_stride", combat.ActionMoveTraitStride.String())
}

// TestCheckReactiveStrikesCtx_SuppressedForMoveTrait confirms WMOVE-12:
// reactions are suppressed when MoveCause == MoveTrait.
func TestCheckReactiveStrikesCtx_SuppressedForMoveTrait(t *testing.T) {
	cbt := &combat.Combat{
		GridWidth:  10,
		GridHeight: 10,
		Combatants: []*combat.Combatant{
			{ID: "hero", Name: "Hero", Kind: combat.KindPlayer, GridX: 5, GridY: 5, MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1},
			{ID: "thug", Name: "Thug", Kind: combat.KindNPC, GridX: 5, GridY: 5, MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1},
		},
	}
	// Both at same cell — pre-stride distance = 0; reactive strike WOULD fire
	// for a normal cause if the mover stepped away.
	cbt.Combatants[0].GridX = 7
	cbt.Combatants[0].GridY = 5

	src := moveTraitSrc{r: rand.New(rand.NewSource(1))}
	events := combat.CheckReactiveStrikesCtx(cbt, combat.ReactionMoveContext{
		MoverID: "hero", FromX: 5, FromY: 5, Cause: combat.MoveCauseMoveTrait,
	}, src, nil)
	assert.Empty(t, events, "MoveTrait must suppress reactive strikes (WMOVE-12)")
}

// TestCheckReactiveStrikesCtx_FiresForNormalStride confirms the cause-aware
// path still fires reactions for a normal Stride.
func TestCheckReactiveStrikesCtx_FiresForNormalStride(t *testing.T) {
	cbt := &combat.Combat{
		GridWidth:  10,
		GridHeight: 10,
		Combatants: []*combat.Combatant{
			{ID: "hero", Name: "Hero", Kind: combat.KindPlayer, GridX: 7, GridY: 5, MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1},
			{ID: "thug", Name: "Thug", Kind: combat.KindNPC, GridX: 5, GridY: 5, MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1},
		},
	}
	src := moveTraitSrc{r: rand.New(rand.NewSource(1))}
	events := combat.CheckReactiveStrikesCtx(cbt, combat.ReactionMoveContext{
		MoverID: "hero", FromX: 5, FromY: 5, Cause: combat.MoveCauseStride,
	}, src, nil)
	// Hero was at (5,5) adjacent to thug, moved to (7,5); thug should make a
	// reactive strike (regardless of hit/miss outcome).
	require.Len(t, events, 1, "normal Stride away from adjacency must trigger reactive strike")
	assert.Equal(t, combat.ActionAttack, events[0].ActionType)
	assert.Equal(t, "thug", events[0].ActorID)
}

// TestProperty_QueueFreeAction_NeverConsumesAP is the rapid property version
// of TestMoveTrait_QueueFreeAction_NoAPSpent: across arbitrary AP totals and
// queue sequences, free Strides never deduct AP.
func TestProperty_QueueFreeAction_NeverConsumesAP(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ap := rapid.IntRange(0, 10).Draw(rt, "ap")
		n := rapid.IntRange(1, 5).Draw(rt, "n")
		q := combat.NewActionQueue("uid", ap)
		for i := 0; i < n; i++ {
			err := q.QueueFreeAction(combat.QueuedAction{
				Type:         combat.ActionMoveTraitStride,
				StrikeAction: int64(i + 1),
			})
			if err != nil {
				rt.Fatalf("QueueFreeAction failed: %v", err)
			}
		}
		if q.RemainingPoints() != ap {
			rt.Fatalf("AP changed after free actions: got %d, want %d", q.RemainingPoints(), ap)
		}
		if q.MovementAPSpent() != 0 {
			rt.Fatalf("movementAPSpent != 0: got %d", q.MovementAPSpent())
		}
	})
}

// TestProperty_QueueFreeAction_OncePerStrikeStable: regardless of the order in
// which strike ids are queued, a duplicate id is always rejected.
func TestProperty_QueueFreeAction_OncePerStrikeStable(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		ids := rapid.SliceOfN(rapid.Int64Range(1, 5), 1, 10).Draw(rt, "ids")
		q := combat.NewActionQueue("uid", 3)
		seen := map[int64]bool{}
		for _, id := range ids {
			err := q.QueueFreeAction(combat.QueuedAction{
				Type: combat.ActionMoveTraitStride, StrikeAction: id,
			})
			if seen[id] {
				if err == nil {
					rt.Fatalf("duplicate StrikeAction %d not rejected", id)
				}
			} else {
				if err != nil {
					rt.Fatalf("first QueueFreeAction for %d failed: %v", id, err)
				}
				seen[id] = true
			}
		}
	})
}

// TestProperty_QueueFreeAction_RejectsNonAllowlisted: across arbitrary action
// types, only ActionMoveTraitStride is accepted.
func TestProperty_QueueFreeAction_RejectsNonAllowlisted(t *testing.T) {
	candidates := []combat.ActionType{
		combat.ActionUnknown,
		combat.ActionAttack,
		combat.ActionStrike,
		combat.ActionPass,
		combat.ActionReload,
		combat.ActionStride,
		combat.ActionAid,
		combat.ActionThrow,
		combat.ActionFireBurst,
		combat.ActionFireAutomatic,
		combat.ActionUseAbility,
		combat.ActionUseTech,
		combat.ActionReady,
		combat.ActionMoveTraitStride,
	}
	rapid.Check(t, func(rt *rapid.T) {
		at := rapid.SampledFrom(candidates).Draw(rt, "type")
		q := combat.NewActionQueue("uid", 3)
		err := q.QueueFreeAction(combat.QueuedAction{Type: at, StrikeAction: 1})
		if at == combat.ActionMoveTraitStride {
			if err != nil {
				rt.Fatalf("ActionMoveTraitStride must be accepted, got %v", err)
			}
		} else {
			if err == nil {
				rt.Fatalf("non-allowlisted type %v must be rejected", at)
			}
		}
	})
}
