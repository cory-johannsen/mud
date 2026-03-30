package npc_test

import (
	"strconv"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// baseRovingYAML is a valid boss-tier NPC with a 2-room roving route.
const baseRovingYAML = `
id: test_rover
name: Test Rover
description: A test roving NPC.
level: 5
max_hp: 50
ac: 14
tier: boss
npc_type: combat
roving:
  route:
    - room_a
    - room_b
  travel_interval: "3m"
  explore_probability: 0.1
`

func TestRovingConfig_ValidTemplate_Loads(t *testing.T) {
	tmpl, err := npc.LoadTemplateFromBytes([]byte(baseRovingYAML))
	require.NoError(t, err)
	require.NotNil(t, tmpl.Roving)
	assert.Equal(t, []string{"room_a", "room_b"}, tmpl.Roving.Route)
	assert.Equal(t, "3m", tmpl.Roving.TravelInterval)
	assert.InDelta(t, 0.1, tmpl.Roving.ExploreProbability, 1e-9)
}

func TestProperty_RovingConfig_Validate_RouteRequired(t *testing.T) {
	yaml := `
id: test_rover
name: Test Rover
description: A test.
level: 5
max_hp: 50
ac: 14
tier: boss
npc_type: combat
roving:
  route: []
  travel_interval: "5m"
  explore_probability: 0.0
`
	_, err := npc.LoadTemplateFromBytes([]byte(yaml))
	assert.ErrorContains(t, err, "roving.route must not be empty")
}

func TestProperty_RovingConfig_Validate_TravelIntervalInvalid(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Draw an invalid duration string (not parseable by time.ParseDuration).
		// We use strings that don't match Go duration syntax.
		invalid := rapid.SampledFrom([]string{"3mins", "five", "1day", "abc", "1h30"}).Draw(rt, "invalid_interval")
		yaml := `
id: test_rover
name: Test Rover
description: A test.
level: 5
max_hp: 50
ac: 14
tier: boss
npc_type: combat
roving:
  route:
    - room_a
    - room_b
  travel_interval: "` + invalid + `"
  explore_probability: 0.0
`
		_, err := npc.LoadTemplateFromBytes([]byte(yaml))
		assert.ErrorContains(t, err, "roving.travel_interval")
	})
}

func TestProperty_RovingConfig_Validate_CombatMustBeBoss(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tier := rapid.SampledFrom([]string{"minion", "standard", "elite", "champion"}).Draw(rt, "tier")
		yaml := `
id: test_rover
name: Test Rover
description: A test.
level: 5
max_hp: 50
ac: 14
tier: ` + tier + `
npc_type: combat
roving:
  route:
    - room_a
    - room_b
  travel_interval: "5m"
  explore_probability: 0.0
`
		_, err := npc.LoadTemplateFromBytes([]byte(yaml))
		assert.ErrorContains(t, err, "roving combat NPCs must have tier: boss")
	})
}

func TestProperty_RovingConfig_Validate_ProbabilityRange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Out-of-range probability: generate values outside [0,1].
		sign := rapid.SampledFrom([]float64{-1.0, 1.0}).Draw(rt, "sign")
		magnitude := rapid.Float64Range(0.001, 100.0).Draw(rt, "mag")
		prob := sign * magnitude
		if prob >= 0 && prob <= 1 {
			return // skip valid values
		}
		// Format the float to a string for embedding in YAML.
		yaml := `
id: test_rover
name: Test Rover
description: A test.
level: 5
max_hp: 50
ac: 14
tier: boss
npc_type: combat
roving:
  route:
    - room_a
    - room_b
  travel_interval: "5m"
  explore_probability: ` + formatFloat(prob) + `
`
		_, err := npc.LoadTemplateFromBytes([]byte(yaml))
		assert.ErrorContains(t, err, "roving.explore_probability must be in [0,1]")
	})
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', 6, 64)
}

func TestNewInstance_RovingState_InitializedWhenRovingSet(t *testing.T) {
	tmpl, err := npc.LoadTemplateFromBytes([]byte(baseRovingYAML))
	require.NoError(t, err)
	inst := npc.NewInstance("test-1", tmpl, "room_a")
	assert.Equal(t, 0, inst.RovingRouteIndex)
	assert.Equal(t, 1, inst.RovingRouteDir)
	assert.False(t, inst.RovingNextMoveAt.IsZero())
	assert.True(t, inst.RovingPausedUntil.IsZero())
}

func TestNewInstance_RovingState_NotSetWhenRovingNil(t *testing.T) {
	tmpl, err := npc.LoadTemplateFromBytes([]byte(`
id: plain_npc
name: Plain NPC
description: No roving.
level: 1
max_hp: 20
ac: 12
`))
	require.NoError(t, err)
	inst := npc.NewInstance("test-2", tmpl, "room_a")
	assert.Equal(t, 0, inst.RovingRouteIndex)
	assert.Equal(t, 0, inst.RovingRouteDir)
	assert.True(t, inst.RovingNextMoveAt.IsZero())
	assert.True(t, inst.RovingPausedUntil.IsZero())
}
