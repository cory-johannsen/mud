package combat_test

// Tests for Task 4 of #248 — terrain-type penalties in the stride loop.
//
// REQ-TERRAIN-6: Stride MUST consume EntryCost per cell from SpeedBudget.
// REQ-TERRAIN-7: Stride MUST stop when remaining budget < EntryCost of the next cell.
// REQ-TERRAIN-8: Stride MUST stop before a TerrainGreaterDifficult (impassable) cell.
// REQ-TERRAIN-17: Greater-difficult barriers MUST emit an informational narrative.
// REQ-TERRAIN-18: Zero-budget strides MUST emit an informational narrative.

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// runStride queues a single stride action for actor with the given direction
// and resolves one round. Returns the events emitted during the round.
func runStride(cbt *combat.Combat, actor *combat.Combatant, dir string, src combat.Source) []combat.RoundEvent {
	if cbt.ActionQueues == nil {
		cbt.ActionQueues = map[string]*combat.ActionQueue{}
	}
	q := combat.NewActionQueue(actor.ID, 3)
	if err := q.Enqueue(combat.QueuedAction{Type: combat.ActionStride, Direction: dir}); err != nil {
		panic(err)
	}
	cbt.ActionQueues[actor.ID] = q
	return combat.ResolveRound(cbt, src, func(string, int) {}, nil, 0)
}

// newStrideActor returns a player combatant placed at (gx, gy) with the given
// movement speed (in feet).
func newStrideActor(id string, gx, gy, speedFt int) *combat.Combatant {
	return &combat.Combatant{
		ID: id, Name: id, Kind: combat.KindPlayer,
		MaxHP: 20, CurrentHP: 20, AC: 12,
		GridX: gx, GridY: gy, SpeedFt: speedFt,
	}
}

// TestStride_AllNormal_MovesFullBudget verifies that a stride across normal
// terrain moves exactly SpeedBudget() squares.
func TestStride_AllNormal_MovesFullBudget(t *testing.T) {
	actor := newStrideActor("actor", 0, 5, 25) // SpeedBudget == 5
	// Place an opponent far to the right so "toward" produces a +x delta and
	// the actor never reaches melee range mid-stride.
	opponent := &combat.Combatant{
		ID: "enemy", Name: "Enemy", Kind: combat.KindNPC,
		MaxHP: 30, CurrentHP: 30, AC: 12,
		GridX: 19, GridY: 5,
	}
	cbt := &combat.Combat{
		GridWidth:    20,
		GridHeight:   20,
		Combatants:   []*combat.Combatant{actor, opponent},
		ActionQueues: map[string]*combat.ActionQueue{},
	}
	budget := actor.SpeedBudget()
	startX := actor.GridX
	runStride(cbt, actor, "toward", fixedSrc{val: 0})
	moved := actor.GridX - startX
	if moved != budget {
		t.Fatalf("all-normal stride: expected to move %d squares, moved %d (GridX=%d)",
			budget, moved, actor.GridX)
	}
}

// TestStride_DifficultHalvesDistance verifies that difficult terrain doubles
// cost, halving the distance traversed by a full-budget stride.
func TestStride_DifficultHalvesDistance(t *testing.T) {
	actor := newStrideActor("actor", 0, 5, 25) // SpeedBudget == 5
	opponent := &combat.Combatant{
		ID: "enemy", Name: "Enemy", Kind: combat.KindNPC,
		MaxHP: 30, CurrentHP: 30, AC: 12,
		GridX: 19, GridY: 5,
	}
	terrain := map[combat.GridCell]*combat.TerrainCell{}
	for x := 1; x <= 19; x++ {
		terrain[combat.GridCell{X: x, Y: 5}] = &combat.TerrainCell{X: x, Y: 5, Type: combat.TerrainDifficult}
	}
	cbt := &combat.Combat{
		GridWidth:    20,
		GridHeight:   20,
		Combatants:   []*combat.Combatant{actor, opponent},
		Terrain:      terrain,
		ActionQueues: map[string]*combat.ActionQueue{},
	}
	startX := actor.GridX
	runStride(cbt, actor, "toward", fixedSrc{val: 0})
	moved := actor.GridX - startX
	// budget=5, each difficult cell costs 2 → floor(5/2)=2 cells.
	if moved != 2 {
		t.Fatalf("difficult terrain: expected to move 2 squares, moved %d (GridX=%d)",
			moved, actor.GridX)
	}
}

// TestStride_GreaterDifficultBlocks verifies stride stops before a
// greater_difficult cell.
func TestStride_GreaterDifficultBlocks(t *testing.T) {
	actor := newStrideActor("actor", 0, 5, 25) // SpeedBudget == 5
	opponent := &combat.Combatant{
		ID: "enemy", Name: "Enemy", Kind: combat.KindNPC,
		MaxHP: 30, CurrentHP: 30, AC: 12,
		GridX: 19, GridY: 5,
	}
	terrain := map[combat.GridCell]*combat.TerrainCell{
		{X: 1, Y: 5}: {X: 1, Y: 5, Type: combat.TerrainGreaterDifficult},
	}
	cbt := &combat.Combat{
		GridWidth:    20,
		GridHeight:   20,
		Combatants:   []*combat.Combatant{actor, opponent},
		Terrain:      terrain,
		ActionQueues: map[string]*combat.ActionQueue{},
	}
	events := runStride(cbt, actor, "toward", fixedSrc{val: 0})
	if actor.GridX != 0 {
		t.Fatalf("greater_difficult: actor must not advance past the barrier, got GridX=%d", actor.GridX)
	}
	// REQ-TERRAIN-17: must emit an informational narrative for the stride.
	found := false
	for _, e := range events {
		if e.ActionType == combat.ActionStride && e.ActorID == "actor" {
			found = true
			if e.Narrative == "" {
				t.Error("greater_difficult: stride event must have a non-empty narrative")
			}
		}
	}
	if !found {
		t.Error("greater_difficult: must emit a stride event")
	}
}

// TestStride_ZeroBudgetNoMove verifies that a combatant whose first step is
// unaffordable emits an informational narrative and does not move.
func TestStride_ZeroBudgetNoMove(t *testing.T) {
	actor := newStrideActor("actor", 0, 5, 5) // SpeedBudget == 1
	opponent := &combat.Combatant{
		ID: "enemy", Name: "Enemy", Kind: combat.KindNPC,
		MaxHP: 30, CurrentHP: 30, AC: 12,
		GridX: 19, GridY: 5,
	}
	terrain := map[combat.GridCell]*combat.TerrainCell{
		// First step is difficult (cost 2) — exceeds budget 1.
		{X: 1, Y: 5}: {X: 1, Y: 5, Type: combat.TerrainDifficult},
	}
	cbt := &combat.Combat{
		GridWidth:    20,
		GridHeight:   20,
		Combatants:   []*combat.Combatant{actor, opponent},
		Terrain:      terrain,
		ActionQueues: map[string]*combat.ActionQueue{},
	}
	events := runStride(cbt, actor, "toward", fixedSrc{val: 0})
	if actor.GridX != 0 {
		t.Fatalf("zero-budget: actor must not move, got GridX=%d", actor.GridX)
	}
	// REQ-TERRAIN-18: must emit an informational stride event.
	found := false
	for _, e := range events {
		if e.ActionType == combat.ActionStride && e.ActorID == "actor" {
			found = true
			if e.Narrative == "" {
				t.Error("zero-budget: stride event must have a non-empty narrative")
			}
		}
	}
	if !found {
		t.Error("zero-budget: must emit a stride event")
	}
}
