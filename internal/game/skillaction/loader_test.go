package skillaction_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/skillaction"
)

const demoralizeYAML = `
id: demoralize
display_name: Demoralize
description: test
ap_cost: 1
skill: smooth_talk
dc: { kind: target_will }
range: { kind: ranged, feet: 30 }
target_kinds: [npc]
outcomes:
  crit_success:
    effects:
      - apply_condition: { id: frightened, stacks: 2, duration_rounds: -1 }
      - narrative: { text: "{actor} crushes {target}." }
  success:
    effects:
      - apply_condition: { id: frightened, stacks: 1, duration_rounds: -1 }
  failure:
    effects:
      - narrative: { text: "{actor} is unconvincing." }
`

// frightenedRegistry returns a Registry containing only the canonical
// frightened condition, sufficient for loader validation tests.
func frightenedRegistry(t *testing.T) *condition.Registry {
	t.Helper()
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "frightened", Name: "Frightened", MaxStacks: 4})
	return reg
}

func TestLoad_AllFieldsParse(t *testing.T) {
	def, err := skillaction.Load([]byte(demoralizeYAML), frightenedRegistry(t))
	require.NoError(t, err)
	require.Equal(t, "demoralize", def.ID)
	require.Equal(t, 1, def.APCost)
	require.Equal(t, skillaction.DCKindTargetWill, def.DC.Kind)
	require.Equal(t, skillaction.RangeRanged, def.Range.Kind)
	require.Equal(t, 30, def.Range.Feet)
	require.Equal(t, []skillaction.TargetKind{skillaction.TargetKindNPC}, def.TargetKinds)

	cs := def.Outcomes[skillaction.CritSuccess]
	require.NotNil(t, cs)
	require.Len(t, cs.Effects, 2)
	ac, ok := cs.Effects[0].(skillaction.ApplyCondition)
	require.True(t, ok)
	require.Equal(t, "frightened", ac.ID)
	require.Equal(t, 2, ac.Stacks)
}

func TestLoad_RejectsUnknownCondition(t *testing.T) {
	bad := `
id: bogus
ap_cost: 1
skill: smooth_talk
dc: { kind: target_will }
range: { kind: melee_reach }
target_kinds: [npc]
outcomes:
  success:
    effects:
      - apply_condition: { id: not_a_condition, duration_rounds: 1 }
`
	_, err := skillaction.Load([]byte(bad), frightenedRegistry(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "not_a_condition")
}

func TestLoad_RejectsNegativeAPCost(t *testing.T) {
	bad := `
id: bogus
ap_cost: -1
skill: smooth_talk
dc: { kind: target_will }
range: { kind: melee_reach }
target_kinds: [npc]
outcomes: {}
`
	_, err := skillaction.Load([]byte(bad), frightenedRegistry(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "ap_cost")
}

func TestLoad_RejectsMultiKindEffect(t *testing.T) {
	bad := `
id: bogus
ap_cost: 1
skill: smooth_talk
dc: { kind: target_will }
range: { kind: melee_reach }
target_kinds: [npc]
outcomes:
  success:
    effects:
      - apply_condition: { id: frightened, stacks: 1 }
        narrative: { text: "x" }
`
	_, err := skillaction.Load([]byte(bad), frightenedRegistry(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "kinds")
}

func TestLoadDirectory_LoadsDemoralizeYAML(t *testing.T) {
	// Write the canonical demoralize.yaml into a temp dir so the test is
	// hermetic and does not depend on cwd.
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "content", "skill_actions", "demoralize.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "demoralize.yaml"), src, 0o644))

	defs, err := skillaction.LoadDirectory(dir, frightenedRegistry(t))
	require.NoError(t, err)
	require.Contains(t, defs, "demoralize")
}
