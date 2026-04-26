package skillaction_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/skillaction"
)

func TestResolve_AppliesEffectsInOrder(t *testing.T) {
	def, err := skillaction.Load([]byte(demoralizeYAML), frightenedRegistry(t))
	require.NoError(t, err)

	var seen []skillaction.Effect
	ctx := skillaction.ResolveContext{
		ActorName:  "Riker",
		TargetName: "Thug",
		Apply:      func(e skillaction.Effect) { seen = append(seen, e) },
	}
	// roll 18 + bonus 5 vs DC 10 → 23 vs 10 = +13 → crit success.
	out, err := skillaction.Resolve(ctx, def, 18, 5, 10)
	require.NoError(t, err)
	require.Equal(t, skillaction.CritSuccess, out.DoS)
	require.Len(t, seen, 2)

	ac, ok := seen[0].(skillaction.ApplyCondition)
	require.True(t, ok)
	require.Equal(t, "frightened", ac.ID)
	require.Equal(t, 2, ac.Stacks)

	require.Contains(t, out.Narrative, "Riker")
	require.Contains(t, out.Narrative, "Thug")
}

func TestResolve_NarrativeFallback_WhenOutcomeMissing(t *testing.T) {
	// Define an action with only crit_success defined; resolve a roll that
	// triggers Failure → no outcome → fallback narrative.
	yaml := `
id: stub
ap_cost: 1
skill: smooth_talk
dc: { kind: target_will }
range: { kind: melee_reach }
target_kinds: [npc]
outcomes:
  crit_success:
    effects:
      - narrative: { text: "perfect" }
`
	def, err := skillaction.Load([]byte(yaml), frightenedRegistry(t))
	require.NoError(t, err)
	out, err := skillaction.Resolve(skillaction.ResolveContext{ActorName: "A", TargetName: "B"}, def, 12, 0, 15)
	require.NoError(t, err)
	require.Equal(t, skillaction.Failure, out.DoS)
	require.Contains(t, out.Narrative, "fails")
}

func TestResolve_NoApplyCallback_StillSetsEffectsOnOutcome(t *testing.T) {
	def, err := skillaction.Load([]byte(demoralizeYAML), frightenedRegistry(t))
	require.NoError(t, err)
	out, err := skillaction.Resolve(skillaction.ResolveContext{}, def, 18, 5, 10)
	require.NoError(t, err)
	require.Len(t, out.Effects, 2, "Resolve must record effects on the Outcome even without an Apply callback")
}
