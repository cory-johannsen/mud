package npc_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/world"
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

// stubWorldReader implements npc.WorldRoomReader for tests.
type stubWorldReader struct {
	rooms map[string]*world.Room
}

func (s *stubWorldReader) GetRoom(id string) (*world.Room, bool) {
	r, ok := s.rooms[id]
	return r, ok
}

func makeExit(dir, target string) world.Exit {
	return world.Exit{Direction: world.Direction(dir), TargetRoom: target}
}

// buildRovingTemplate creates a roving boss NPC template using LoadTemplateFromBytes.
// route must have >= 2 entries. explorePct is set after load.
func buildRovingTemplate(t testing.TB, route []string, explorePct float64) *npc.Template {
	t.Helper()
	routeYAML := ""
	for _, r := range route {
		routeYAML += "\n    - " + r
	}
	yaml := `
id: test_rover
name: Test Rover
description: Roving test NPC.
level: 5
max_hp: 50
ac: 14
tier: boss
npc_type: combat
roving:
  route:` + routeYAML + `
  travel_interval: "1ms"
  explore_probability: 0.0
`
	tmpl, err := npc.LoadTemplateFromBytes([]byte(yaml))
	require.NoError(t, err)
	tmpl.Roving.ExploreProbability = explorePct
	return tmpl
}

func TestProperty_RovingManager_RouteFollowing(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		routeLen := rapid.IntRange(3, 6).Draw(rt, "route_len")
		route := make([]string, routeLen)
		for i := range route {
			route[i] = fmt.Sprintf("room_%d", i)
		}
		mgr := npc.NewManager()
		tmpl := buildRovingTemplate(t, route, 0.0)
		inst, err := mgr.Spawn(tmpl, route[0])
		require.NoError(t, err)

		rooms := make(map[string]*world.Room, routeLen)
		for _, id := range route {
			rooms[id] = &world.Room{ID: id}
		}
		sw := &stubWorldReader{rooms: rooms}

		var moves []string
		rm := npc.NewRovingManager(mgr, sw, func(_, _, to string) { moves = append(moves, to) })
		rm.Register(inst, tmpl)

		// Walk forward to end of route then back to start: 2*(routeLen-1) steps total.
		// Sleep 2ms between ticks to ensure the 1ms travel_interval has elapsed.
		totalSteps := 2 * (routeLen - 1)
		for i := 0; i < totalSteps; i++ {
			time.Sleep(2 * time.Millisecond)
			rm.Tick(time.Now())
		}

		// After a full forward+backward traversal the NPC must be back at route[0].
		assert.Equal(t, route[0], inst.RoomID, "NPC must return to start after full ping-pong")
		assert.Equal(t, totalSteps, len(moves), "every step must produce a move event")
	})
}

func TestProperty_RovingManager_PauseRespected(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		pauseMinutes := rapid.IntRange(1, 120).Draw(rt, "pause_minutes")
		pauseDur := time.Duration(pauseMinutes) * time.Minute

		mgr := npc.NewManager()
		route := []string{"room_a", "room_b"}
		tmpl := buildRovingTemplate(t, route, 0.0)
		inst, err := mgr.Spawn(tmpl, "room_a")
		require.NoError(t, err)

		sw := &stubWorldReader{rooms: map[string]*world.Room{
			"room_a": {ID: "room_a"},
			"room_b": {ID: "room_b"},
		}}
		moved := false
		rm := npc.NewRovingManager(mgr, sw, func(_, _, _ string) { moved = true })
		rm.Register(inst, tmpl)

		rm.PauseFor(inst.ID, pauseDur)
		time.Sleep(2 * time.Millisecond)
		rm.Tick(time.Now())
		assert.False(t, moved, "NPC must not move while paused (pause=%v)", pauseDur)
		assert.Equal(t, "room_a", inst.RoomID)

		// Clear pause by setting it to a past time.
		rm.PauseFor(inst.ID, -time.Second)
		time.Sleep(2 * time.Millisecond)
		rm.Tick(time.Now())
		assert.True(t, moved, "NPC must move after pause clears")
	})
}

func TestProperty_RovingManager_ExploreDeviation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		exitCount := rapid.IntRange(1, 5).Draw(rt, "exit_count")
		exits := make([]world.Exit, exitCount)
		exitRooms := make(map[string]bool, exitCount)
		allRooms := map[string]*world.Room{}
		for i := range exits {
			id := fmt.Sprintf("exit_room_%d", i)
			exits[i] = makeExit("north", id)
			exitRooms[id] = true
			allRooms[id] = &world.Room{ID: id}
		}
		allRooms["room_a"] = &world.Room{ID: "room_a", Exits: exits}
		allRooms["room_b"] = &world.Room{ID: "room_b"}

		mgr := npc.NewManager()
		tmpl := buildRovingTemplate(t, []string{"room_a", "room_b"}, 1.0) // always deviate
		inst, err := mgr.Spawn(tmpl, "room_a")
		require.NoError(t, err)

		sw := &stubWorldReader{rooms: allRooms}
		var dest string
		rm := npc.NewRovingManager(mgr, sw, func(_, _, to string) { dest = to })
		rm.Register(inst, tmpl)
		time.Sleep(2 * time.Millisecond)
		rm.Tick(time.Now())

		assert.True(t, exitRooms[dest], "explore deviation must pick one of the %d adjacent rooms, got %q", exitCount, dest)
	})
}

func TestProperty_RovingManager_UnregisterOnDeath(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		_ = rapid.IntRange(1, 100).Draw(rt, "initial_hp") // vary starting HP for coverage

		mgr := npc.NewManager()
		route := []string{"room_a", "room_b"}
		tmpl := buildRovingTemplate(t, route, 0.0)
		inst, err := mgr.Spawn(tmpl, "room_a")
		require.NoError(t, err)

		sw := &stubWorldReader{rooms: map[string]*world.Room{
			"room_a": {ID: "room_a"},
			"room_b": {ID: "room_b"},
		}}
		moved := false
		rm := npc.NewRovingManager(mgr, sw, func(_, _, _ string) { moved = true })
		rm.Register(inst, tmpl)

		inst.CurrentHP = 0 // kill the NPC
		time.Sleep(2 * time.Millisecond)
		rm.Tick(time.Now())
		assert.False(t, moved, "dead NPC must not move")
		assert.Equal(t, "room_a", inst.RoomID)

		// Second tick: NPC already unregistered, no panic, no move.
		rm.Tick(time.Now())
		assert.False(t, moved)
	})
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
