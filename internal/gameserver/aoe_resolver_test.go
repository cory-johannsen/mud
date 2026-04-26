package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// makeCombatWithCombatants returns a Combat populated with NPC combatants at
// the supplied (x,y) cells. Identifiers are assigned positionally.
func makeCombatWithCombatants(coords [][2]int) *combat.Combat {
	cbt := &combat.Combat{}
	for i, xy := range coords {
		c := &combat.Combatant{
			ID:    string(rune('a' + i)),
			Name:  "npc" + string(rune('a'+i)),
			Kind:  combat.KindNPC,
			GridX: xy[0],
			GridY: xy[1],
			CurrentHP: 10,
			MaxHP:     10,
		}
		cbt.Combatants = append(cbt.Combatants, c)
	}
	return cbt
}

func TestCellsForTemplate_Burst(t *testing.T) {
	tmpl := &gamev1.AoeTemplate{
		Shape:   gamev1.AoeTemplate_SHAPE_BURST,
		AnchorX: 5,
		AnchorY: 5,
	}
	cells := CellsForTemplate(tmpl, 10, 0, 0) // radius 10ft = 2 cells.
	// Expect (2*2+1)^2 = 25 cells.
	if got := len(cells); got != 25 {
		t.Fatalf("burst cell count: want 25, got %d", got)
	}
	// Verify the centre is included.
	found := false
	for _, c := range cells {
		if c.X == 5 && c.Y == 5 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("burst cells did not contain anchor (5,5)")
	}
}

func TestCellsForTemplate_Cone(t *testing.T) {
	tmpl := &gamev1.AoeTemplate{
		Shape:   gamev1.AoeTemplate_SHAPE_CONE,
		AnchorX: 5,
		AnchorY: 5,
		Facing:  gamev1.AoeTemplate_DIR_E,
	}
	cells := CellsForTemplate(tmpl, 0, 15, 0) // length 15ft = 3 cells.
	if len(cells) == 0 {
		t.Fatalf("cone cells: expected non-empty")
	}
	// Apex must be excluded (AOE-10).
	for _, c := range cells {
		if c.X == 5 && c.Y == 5 {
			t.Fatalf("cone cells must not contain apex")
		}
	}
	// All cone cells facing east must satisfy x > apex.X.
	for _, c := range cells {
		if c.X <= 5 {
			t.Fatalf("east-facing cone has cell at x=%d (apex x=5)", c.X)
		}
	}
}

func TestCellsForTemplate_Line(t *testing.T) {
	tmpl := &gamev1.AoeTemplate{
		Shape:   gamev1.AoeTemplate_SHAPE_LINE,
		AnchorX: 0,
		AnchorY: 0,
		Facing:  gamev1.AoeTemplate_DIR_E,
	}
	cells := CellsForTemplate(tmpl, 0, 25, 5) // 5 cells long, 1 cell wide.
	if got := len(cells); got != 5 {
		t.Fatalf("line cells: want 5, got %d (cells=%v)", got, cells)
	}
	for _, c := range cells {
		if c.Y != 0 {
			t.Fatalf("east line should have y=0, got %v", c)
		}
		if c.X == 0 {
			t.Fatalf("line origin (0,0) must be excluded per AOE-10")
		}
	}
}

func TestDirFromProto_AllDirections(t *testing.T) {
	cases := []struct {
		in   gamev1.AoeTemplate_Direction
		want combat.Direction
	}{
		{gamev1.AoeTemplate_DIR_N, combat.DirN},
		{gamev1.AoeTemplate_DIR_NE, combat.DirNE},
		{gamev1.AoeTemplate_DIR_E, combat.DirE},
		{gamev1.AoeTemplate_DIR_SE, combat.DirSE},
		{gamev1.AoeTemplate_DIR_S, combat.DirS},
		{gamev1.AoeTemplate_DIR_SW, combat.DirSW},
		{gamev1.AoeTemplate_DIR_W, combat.DirW},
		{gamev1.AoeTemplate_DIR_NW, combat.DirNW},
		{gamev1.AoeTemplate_DIR_UNSPECIFIED, combat.DirN}, // stable default.
	}
	for _, tc := range cases {
		if got := dirFromProto(tc.in); got != tc.want {
			t.Errorf("dirFromProto(%v): want %v, got %v", tc.in, tc.want, got)
		}
	}
}

func TestPostFilterAffectedCells_IdentityV1(t *testing.T) {
	in := []combat.Cell{{X: 1, Y: 1}, {X: 2, Y: 2}}
	out := PostFilterAffectedCells(in, ResolveContext{})
	if len(out) != len(in) {
		t.Fatalf("identity filter: len changed (in=%d out=%d)", len(in), len(out))
	}
	for i := range in {
		if in[i] != out[i] {
			t.Fatalf("identity filter: index %d changed (in=%v out=%v)", i, in[i], out[i])
		}
	}
}

func TestResolveAoeCells_BurstAffectsAndPopulatesCells(t *testing.T) {
	cbt := makeCombatWithCombatants([][2]int{
		{5, 5}, // inside a 10ft burst centered at (5,5): yes
		{6, 6}, // inside: yes
		{8, 8}, // outside: no
	})
	tmpl := &gamev1.AoeTemplate{
		Shape:   gamev1.AoeTemplate_SHAPE_BURST,
		AnchorX: 5,
		AnchorY: 5,
	}
	affected, cells := resolveAoeCells(tmpl, 10, 0, 0, 5, 5, cbt)
	if len(affected) != 2 {
		t.Fatalf("burst affected: want 2, got %d", len(affected))
	}
	if len(cells) != 25 {
		t.Fatalf("burst cells: want 25, got %d", len(cells))
	}
	// AOE-12: outbound template.Cells must be populated.
	if got := len(tmpl.GetCells()); got != 25 {
		t.Fatalf("template.Cells populated: want 25, got %d", got)
	}
}

func TestResolveAoeCells_ConeShape(t *testing.T) {
	cbt := makeCombatWithCombatants([][2]int{
		{6, 5}, // east of (5,5), in cone: yes
		{4, 5}, // west of (5,5), not in east cone: no
	})
	tmpl := &gamev1.AoeTemplate{
		Shape:   gamev1.AoeTemplate_SHAPE_CONE,
		AnchorX: 5,
		AnchorY: 5,
		Facing:  gamev1.AoeTemplate_DIR_E,
	}
	affected, _ := resolveAoeCells(tmpl, 0, 15, 0, 5, 5, cbt)
	if len(affected) != 1 {
		t.Fatalf("cone affected: want 1, got %d", len(affected))
	}
	if affected[0].GridX != 6 {
		t.Fatalf("cone affected wrong cell: %+v", affected[0])
	}
}

func TestResolveAoeCells_LineShape(t *testing.T) {
	cbt := makeCombatWithCombatants([][2]int{
		{1, 0}, // east of (0,0), on line: yes
		{2, 0}, // also on line: yes
		{0, 1}, // perpendicular: no
	})
	tmpl := &gamev1.AoeTemplate{
		Shape:   gamev1.AoeTemplate_SHAPE_LINE,
		AnchorX: 0,
		AnchorY: 0,
		Facing:  gamev1.AoeTemplate_DIR_E,
	}
	affected, _ := resolveAoeCells(tmpl, 0, 25, 5, 0, 0, cbt)
	if len(affected) != 2 {
		t.Fatalf("line affected: want 2, got %d (got=%+v)", len(affected), affected)
	}
}

func TestResolveAoeCells_NilTemplateSynthesisesBurst(t *testing.T) {
	// AOE-16: a nil template plus legacy targetX/targetY for an AoeRadius>0
	// content must synthesise a burst template at runtime.
	cbt := makeCombatWithCombatants([][2]int{
		{3, 3}, // centre of a 5ft burst: yes
		{4, 3}, // inside 5ft burst: yes
		{6, 6}, // outside: no
	})
	affected, cells := resolveAoeCells(nil, 5, 0, 0, 3, 3, cbt)
	if len(affected) != 2 {
		t.Fatalf("synthesised burst affected: want 2, got %d", len(affected))
	}
	// Burst radius 5ft = 1 cell -> (2*1+1)^2 = 9.
	if len(cells) != 9 {
		t.Fatalf("synthesised burst cells: want 9, got %d", len(cells))
	}
}

func TestResolveAoeCells_EmptyIntersection(t *testing.T) {
	// AOE-20: when the placement intersects no living combatants the resolver
	// returns an empty slice (the AP-consume / narrative path is the caller's).
	cbt := makeCombatWithCombatants([][2]int{
		{0, 0}, // far from anchor
	})
	tmpl := &gamev1.AoeTemplate{
		Shape:   gamev1.AoeTemplate_SHAPE_BURST,
		AnchorX: 9,
		AnchorY: 9,
	}
	affected, cells := resolveAoeCells(tmpl, 5, 0, 0, 9, 9, cbt)
	if len(affected) != 0 {
		t.Fatalf("empty intersection: want 0 affected, got %d", len(affected))
	}
	if len(cells) == 0 {
		t.Fatalf("empty intersection: cells should still be populated for client preview")
	}
}
