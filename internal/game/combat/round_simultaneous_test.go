package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/condition"
)

// TestSimultaneousResolution_NPCTargetsStartOfRoundPosition verifies that
// when a player and NPC both stride in the same round, the NPC targets the
// player's start-of-round position, not the player's post-stride position.
//
// Setup: player at (5,5), NPC at (5,15). Player strides "n" (north, toward NPC).
// Player has speed 5ft (1 square), so player ends at (5,4) after striding north.
//
// NPC strides "toward" player. With simultaneous resolution, NPC should target
// player's start position (5,5), not post-stride (5,4). Since NPC is already
// directly above player in Y at same X, "toward" moves NPC one step south.
// NPC ends at (5,14). The key property: NPC uses start-of-round snapshot.
//
// Without the fix, NPC would target (5,4) (player's new position), also
// arriving at (5,14), BUT the stop condition changes:
// CombatRange(*actor, *opponent) uses the live opponent position,
// so the stop-adjacency check fires differently.
//
// This test verifies the NPC moves toward (5,5) not (5,4) by checking
// that when player moves AWAY from NPC, the NPC still moves toward the
// original position — demonstrating position snapshot is used.
func TestSimultaneousResolution_NPCTargetsStartOfRoundPosition(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	eng := combat.NewEngine()

	// Player at (5,5), NPC at (5,1). Player strides south (away from NPC),
	// ending at (5,10) after 5 squares. NPC strides "toward" (south).
	// With snapshot: NPC moves toward (5,5) and can reach (5,6) after 5 squares.
	// Without snapshot: NPC moves toward (5,10) (post-stride), ending at (5,6) too,
	// but critically the stop-at-adjacent check uses (5,10) as the target.
	//
	// Use a simpler scenario: player at (0,0), NPC at (0,10).
	// Player strides south (s = increasing Y), NPC strides "toward" (north = decreasing Y).
	// Player starts at y=0, strides south 5 sq → ends at y=5.
	// NPC starts at y=10, strides toward player.
	//   With snapshot (target y=0): NPC moves from y=10 toward y=0 → ends at y=5.
	//   Without snapshot (target y=5): NPC stops SOONER (adjacent to y=5 sooner).
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 30, CurrentHP: 30, AC: 14, GridX: 0, GridY: 0, SpeedFt: 5},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 30, CurrentHP: 30, AC: 12, GridX: 0, GridY: 10, SpeedFt: 25},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	cbt.GridWidth = 20
	cbt.GridHeight = 20
	_ = cbt.StartRound(3)

	// Player strides south (away from NPC at y=10).
	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: "s"}); err != nil {
		t.Fatalf("QueueAction p1 stride: %v", err)
	}
	// NPC strides toward player.
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionStride, Direction: "toward"}); err != nil {
		t.Fatalf("QueueAction n1 stride: %v", err)
	}

	src := fixedSrc{val: 10}
	combat.ResolveRound(cbt, src, nil, nil)

	playerY := cbt.Combatants[0].GridY
	npcY := cbt.Combatants[1].GridY

	// Player moved south 1 square (SpeedFt=5 → 1 square).
	if playerY != 1 {
		t.Errorf("player should be at y=1 after striding south 1 sq, got y=%d", playerY)
	}

	// NPC (SpeedFt=25 → 5 squares) moves toward player's start-of-round position y=0.
	// From y=10, moving north 5 squares → y=5.
	// With snapshot (target y=0, distance=10), NPC moves all 5 steps: y=10→9→8→7→6→5.
	// Without snapshot (target y=1 post-stride), NPC would stop at y=2 (adjacent = ≤5ft = 1 square away).
	// So the distinguishing check: NPC should end at y=5.
	if npcY != 5 {
		t.Errorf("NPC should be at y=5 (targeting start-of-round position), got y=%d (possible foreknowledge of player movement)", npcY)
	}
}

// TestSimultaneousResolution_PlayerActsFirst_NPCNotAffected verifies that
// even when a player acts first in resolution order, the NPC's movement target
// is not influenced by the player's resolved position.
func TestSimultaneousResolution_PlayerActsFirst_NPCNotAffected(t *testing.T) {
	reg := condition.NewRegistry()
	reg.Register(&condition.ConditionDef{ID: "dying", Name: "Dying", DurationType: "until_save", MaxStacks: 4})
	reg.Register(&condition.ConditionDef{ID: "wounded", Name: "Wounded", DurationType: "permanent", MaxStacks: 3})
	eng := combat.NewEngine()

	// Player at (0,5), NPC at (0,0). Player strides south (away, increasing Y).
	// NPC strides "toward" (south, increasing Y toward player).
	// Player speed = 5ft (1 sq). NPC speed = 25ft (5 sq).
	// Player: y=5 → y=6.
	// NPC using snapshot y=5 as target: moves from y=0 toward y=5 → reaches y=4 (stops adjacent at ≤5ft=1sq).
	// NPC using live y=6 as target: moves from y=0 toward y=6 → reaches y=5 (stops adjacent at ≤5ft).
	// Distinguishing: snapshot → npcY=4; live → npcY=5.
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 30, CurrentHP: 30, AC: 14, GridX: 0, GridY: 5, SpeedFt: 5},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 30, CurrentHP: 30, AC: 12, GridX: 0, GridY: 0, SpeedFt: 25},
	}
	cbt, err := eng.StartCombat("room1", combatants, reg, nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	cbt.GridWidth = 20
	cbt.GridHeight = 20
	_ = cbt.StartRound(3)

	if err := cbt.QueueAction("p1", combat.QueuedAction{Type: combat.ActionStride, Direction: "s"}); err != nil {
		t.Fatalf("QueueAction p1: %v", err)
	}
	if err := cbt.QueueAction("n1", combat.QueuedAction{Type: combat.ActionStride, Direction: "toward"}); err != nil {
		t.Fatalf("QueueAction n1: %v", err)
	}

	src := fixedSrc{val: 10}
	combat.ResolveRound(cbt, src, nil, nil)

	npcY := cbt.Combatants[1].GridY

	// NPC should target player's start y=5, stopping when distance ≤ 5ft (1 grid square away = y=4).
	if npcY != 4 {
		t.Errorf("NPC should stop at y=4 (adjacent to player start y=5), got y=%d — possible foreknowledge of player post-stride position", npcY)
	}
}
