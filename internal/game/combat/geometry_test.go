package combat_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"pgregory.net/rapid"
)

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TestProperty_BurstCellsBoundedChebyshev asserts that BurstCells emits
// exactly (2r+1)^2 cells and that every cell lies within Chebyshev radius r
// of the center, where r = radiusFt/5.
func TestProperty_BurstCellsBoundedChebyshev(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		cx := rapid.IntRange(-50, 50).Draw(t, "cx")
		cy := rapid.IntRange(-50, 50).Draw(t, "cy")
		radiusFt := rapid.IntRange(5, 60).Draw(t, "radiusFt")
		cells := combat.BurstCells(combat.Cell{X: cx, Y: cy}, radiusFt)
		rad := radiusFt / 5
		for _, c := range cells {
			dx := absInt(c.X - cx)
			dy := absInt(c.Y - cy)
			if maxInt(dx, dy) > rad {
				t.Fatalf("cell %+v outside Chebyshev radius %d of (%d,%d)", c, rad, cx, cy)
			}
		}
		if want := (2*rad + 1) * (2*rad + 1); len(cells) != want {
			t.Fatalf("burst count: want %d got %d", want, len(cells))
		}
	})
}

// TestProperty_ConeCellsAlignedToFacing asserts that ConeCells excludes the
// apex (AOE-10) and that every emitted cell lies within the cone's Chebyshev
// length from the apex.
func TestProperty_ConeCellsAlignedToFacing(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		dir := combat.Direction(rapid.IntRange(0, 7).Draw(t, "dir"))
		lengthFt := rapid.IntRange(5, 60).Draw(t, "lenFt")
		apex := combat.Cell{X: 0, Y: 0}
		cells := combat.ConeCells(apex, dir, lengthFt)
		for _, c := range cells {
			if c == apex {
				t.Fatalf("apex must be excluded (AOE-10); got %+v", c)
			}
			d := maxInt(absInt(c.X), absInt(c.Y))
			if d > lengthFt/5 {
				t.Fatalf("cone cell %+v exceeds cone length %d (cells)", c, lengthFt/5)
			}
		}
	})
}

// TestProperty_LineCellsThicknessAndLength asserts that LineCells respects
// the depth*width budget. The +1 slack handles odd-width centering.
func TestProperty_LineCellsThicknessAndLength(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		dir := combat.Direction(rapid.IntRange(0, 7).Draw(t, "dir"))
		lengthFt := rapid.IntRange(5, 50).Draw(t, "lenFt")
		widthFt := rapid.IntRange(5, 25).Draw(t, "widthFt")
		cells := combat.LineCells(combat.Cell{X: 0, Y: 0}, dir, lengthFt, widthFt)
		wantLen := (lengthFt / 5) * (widthFt / 5)
		if len(cells) > wantLen+1 {
			t.Fatalf("line cells exceeded length*width budget: got %d, max %d",
				len(cells), wantLen+1)
		}
		// AOE-10: origin is never in the result.
		for _, c := range cells {
			if c.X == 0 && c.Y == 0 {
				t.Fatalf("origin must be excluded (AOE-10); got %+v", c)
			}
		}
	})
}

// TestBurstCells_ZeroRadius_ReturnsCenterOnly verifies the degenerate case.
func TestBurstCells_ZeroRadius_ReturnsCenterOnly(t *testing.T) {
	cells := combat.BurstCells(combat.Cell{X: 3, Y: 4}, 0)
	if len(cells) != 1 || cells[0] != (combat.Cell{X: 3, Y: 4}) {
		t.Fatalf("zero-radius burst: want [{3,4}] got %+v", cells)
	}
}

// TestConeCells_GoldenVectors_Cardinal hand-verifies a 10-ft cone (depth=2)
// for every cardinal facing. The PF2E cardinal cone forms a square fan that
// is 2d+1 cells wide at depth d, yielding 3+5 = 8 cells at length 10ft.
func TestConeCells_GoldenVectors_Cardinal(t *testing.T) {
	apex := combat.Cell{X: 0, Y: 0}
	tests := []struct {
		name string
		dir  combat.Direction
		want map[combat.Cell]bool
	}{
		{
			name: "north",
			dir:  combat.DirN,
			want: map[combat.Cell]bool{
				{X: -1, Y: -1}: true, {X: 0, Y: -1}: true, {X: 1, Y: -1}: true,
				{X: -2, Y: -2}: true, {X: -1, Y: -2}: true, {X: 0, Y: -2}: true,
				{X: 1, Y: -2}: true, {X: 2, Y: -2}: true,
			},
		},
		{
			name: "east",
			dir:  combat.DirE,
			want: map[combat.Cell]bool{
				{X: 1, Y: -1}: true, {X: 1, Y: 0}: true, {X: 1, Y: 1}: true,
				{X: 2, Y: -2}: true, {X: 2, Y: -1}: true, {X: 2, Y: 0}: true,
				{X: 2, Y: 1}: true, {X: 2, Y: 2}: true,
			},
		},
		{
			name: "south",
			dir:  combat.DirS,
			want: map[combat.Cell]bool{
				{X: -1, Y: 1}: true, {X: 0, Y: 1}: true, {X: 1, Y: 1}: true,
				{X: -2, Y: 2}: true, {X: -1, Y: 2}: true, {X: 0, Y: 2}: true,
				{X: 1, Y: 2}: true, {X: 2, Y: 2}: true,
			},
		},
		{
			name: "west",
			dir:  combat.DirW,
			want: map[combat.Cell]bool{
				{X: -1, Y: -1}: true, {X: -1, Y: 0}: true, {X: -1, Y: 1}: true,
				{X: -2, Y: -2}: true, {X: -2, Y: -1}: true, {X: -2, Y: 0}: true,
				{X: -2, Y: 1}: true, {X: -2, Y: 2}: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := combat.ConeCells(apex, tc.dir, 10)
			gotSet := make(map[combat.Cell]bool, len(got))
			for _, c := range got {
				gotSet[c] = true
			}
			if len(gotSet) != len(tc.want) {
				t.Fatalf("%s: cell count: want %d got %d (cells=%+v)",
					tc.name, len(tc.want), len(gotSet), got)
			}
			for c := range tc.want {
				if !gotSet[c] {
					t.Errorf("%s: missing expected cell %+v", tc.name, c)
				}
			}
			for c := range gotSet {
				if !tc.want[c] {
					t.Errorf("%s: unexpected cell %+v", tc.name, c)
				}
			}
		})
	}
}

// TestConeCells_GoldenVectors_Diagonal hand-verifies the four diagonal cones
// at 10ft (depth=2). Each diagonal cone spans the matching quadrant with the
// cardinal axes excluded, yielding the 2x2 quadrant interior (4 cells).
func TestConeCells_GoldenVectors_Diagonal(t *testing.T) {
	apex := combat.Cell{X: 0, Y: 0}
	tests := []struct {
		name string
		dir  combat.Direction
		want map[combat.Cell]bool
	}{
		{
			name: "northeast",
			dir:  combat.DirNE,
			want: map[combat.Cell]bool{
				{X: 1, Y: -1}: true,
				{X: 1, Y: -2}: true,
				{X: 2, Y: -1}: true,
				{X: 2, Y: -2}: true,
			},
		},
		{
			name: "southeast",
			dir:  combat.DirSE,
			want: map[combat.Cell]bool{
				{X: 1, Y: 1}: true,
				{X: 1, Y: 2}: true,
				{X: 2, Y: 1}: true,
				{X: 2, Y: 2}: true,
			},
		},
		{
			name: "southwest",
			dir:  combat.DirSW,
			want: map[combat.Cell]bool{
				{X: -1, Y: 1}: true,
				{X: -1, Y: 2}: true,
				{X: -2, Y: 1}: true,
				{X: -2, Y: 2}: true,
			},
		},
		{
			name: "northwest",
			dir:  combat.DirNW,
			want: map[combat.Cell]bool{
				{X: -1, Y: -1}: true,
				{X: -1, Y: -2}: true,
				{X: -2, Y: -1}: true,
				{X: -2, Y: -2}: true,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := combat.ConeCells(apex, tc.dir, 10)
			gotSet := make(map[combat.Cell]bool, len(got))
			for _, c := range got {
				gotSet[c] = true
			}
			if len(gotSet) != len(tc.want) {
				t.Fatalf("%s: cell count: want %d got %d (cells=%+v)",
					tc.name, len(tc.want), len(gotSet), got)
			}
			for c := range tc.want {
				if !gotSet[c] {
					t.Errorf("%s: missing expected cell %+v", tc.name, c)
				}
			}
			for c := range gotSet {
				if !tc.want[c] {
					t.Errorf("%s: unexpected cell %+v", tc.name, c)
				}
			}
		})
	}
}

// TestLineCells_GoldenVector_East hand-verifies a 15-ft long, 15-ft wide line
// pointing east from the origin: depth=3, width=3, total=9 cells.
func TestLineCells_GoldenVector_East(t *testing.T) {
	got := combat.LineCells(combat.Cell{X: 0, Y: 0}, combat.DirE, 15, 15)
	want := map[combat.Cell]bool{
		{X: 1, Y: -1}: true, {X: 1, Y: 0}: true, {X: 1, Y: 1}: true,
		{X: 2, Y: -1}: true, {X: 2, Y: 0}: true, {X: 2, Y: 1}: true,
		{X: 3, Y: -1}: true, {X: 3, Y: 0}: true, {X: 3, Y: 1}: true,
	}
	gotSet := make(map[combat.Cell]bool, len(got))
	for _, c := range got {
		gotSet[c] = true
	}
	if len(gotSet) != len(want) {
		t.Fatalf("east line: count: want %d got %d (cells=%+v)", len(want), len(gotSet), got)
	}
	for c := range want {
		if !gotSet[c] {
			t.Errorf("east line: missing %+v", c)
		}
	}
}

// TestLineCells_GoldenVector_NorthWidth5 verifies a 10-ft long, 5-ft wide
// line pointing north: depth=2, width=1, total=2 cells, exactly along the
// facing axis.
func TestLineCells_GoldenVector_NorthWidth5(t *testing.T) {
	got := combat.LineCells(combat.Cell{X: 0, Y: 0}, combat.DirN, 10, 5)
	want := []combat.Cell{{X: 0, Y: -1}, {X: 0, Y: -2}}
	if len(got) != len(want) {
		t.Fatalf("north line width=5: count: want %d got %d (%+v)", len(want), len(got), got)
	}
	gotSet := map[combat.Cell]bool{}
	for _, c := range got {
		gotSet[c] = true
	}
	for _, c := range want {
		if !gotSet[c] {
			t.Errorf("north line width=5: missing %+v", c)
		}
	}
}

// TestLineCells_GoldenVector_SoutheastDiagonal verifies a diagonal line: 10ft
// long, 5ft wide pointing southeast yields cells stepping +1,+1 each square.
func TestLineCells_GoldenVector_SoutheastDiagonal(t *testing.T) {
	got := combat.LineCells(combat.Cell{X: 0, Y: 0}, combat.DirSE, 10, 5)
	want := []combat.Cell{{X: 1, Y: 1}, {X: 2, Y: 2}}
	if len(got) != len(want) {
		t.Fatalf("SE line: count: want %d got %d (%+v)", len(want), len(got), got)
	}
	gotSet := map[combat.Cell]bool{}
	for _, c := range got {
		gotSet[c] = true
	}
	for _, c := range want {
		if !gotSet[c] {
			t.Errorf("SE line: missing %+v", c)
		}
	}
}

// TestFacingFrom_CardinalAxes verifies pure-axial vectors yield their cardinal.
func TestFacingFrom_CardinalAxes(t *testing.T) {
	src := combat.Cell{X: 5, Y: 5}
	tests := []struct {
		dst  combat.Cell
		want combat.Direction
	}{
		{combat.Cell{X: 5, Y: 0}, combat.DirN},
		{combat.Cell{X: 10, Y: 5}, combat.DirE},
		{combat.Cell{X: 5, Y: 10}, combat.DirS},
		{combat.Cell{X: 0, Y: 5}, combat.DirW},
	}
	for _, tc := range tests {
		got := combat.FacingFrom(src, tc.dst)
		if got != tc.want {
			t.Errorf("FacingFrom(%+v -> %+v): want %d got %d", src, tc.dst, tc.want, got)
		}
	}
}

// TestFacingFrom_Diagonals verifies equal-magnitude offsets yield diagonals.
func TestFacingFrom_Diagonals(t *testing.T) {
	src := combat.Cell{X: 0, Y: 0}
	tests := []struct {
		dst  combat.Cell
		want combat.Direction
	}{
		{combat.Cell{X: 3, Y: -3}, combat.DirNE},
		{combat.Cell{X: 3, Y: 3}, combat.DirSE},
		{combat.Cell{X: -3, Y: 3}, combat.DirSW},
		{combat.Cell{X: -3, Y: -3}, combat.DirNW},
	}
	for _, tc := range tests {
		got := combat.FacingFrom(src, tc.dst)
		if got != tc.want {
			t.Errorf("FacingFrom(%+v -> %+v): want %d got %d", src, tc.dst, tc.want, got)
		}
	}
}

// TestFacingFrom_Identity returns DirN for src == dst.
func TestFacingFrom_Identity(t *testing.T) {
	src := combat.Cell{X: 7, Y: 7}
	if got := combat.FacingFrom(src, src); got != combat.DirN {
		t.Errorf("FacingFrom(identity): want DirN got %d", got)
	}
}
