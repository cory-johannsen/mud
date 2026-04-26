package combat

// Cell is an integer (x,y) coordinate on the combat grid. Cells correspond to
// 5-foot squares. Cell{0,0} is the apex/origin used by AOE templates.
type Cell struct {
	X int
	Y int
}

// Direction is one of eight compass octants used to orient cone and line AOEs.
type Direction int

// Direction octants. The numeric values follow a clockwise sweep starting
// from north; tests in this package depend on the iota order being stable.
const (
	DirN Direction = iota
	DirNE
	DirE
	DirSE
	DirS
	DirSW
	DirW
	DirNW
)

// facingDelta returns the unit (dx, dy) vector for a facing direction.
//
// On the combat grid, +X is east and +Y is south (screen-style coordinates).
// Diagonal facings return both components ±1; the helpers that consume the
// delta interpret distance in Chebyshev squares so the unit vector need not
// be normalised.
func facingDelta(d Direction) (int, int) {
	switch d {
	case DirN:
		return 0, -1
	case DirNE:
		return 1, -1
	case DirE:
		return 1, 0
	case DirSE:
		return 1, 1
	case DirS:
		return 0, 1
	case DirSW:
		return -1, 1
	case DirW:
		return -1, 0
	case DirNW:
		return -1, -1
	default:
		return 0, 0
	}
}

// isDiagonal reports whether the facing is one of the four diagonal octants.
func isDiagonal(d Direction) bool {
	fdx, fdy := facingDelta(d)
	return fdx != 0 && fdy != 0
}

// BurstCells returns every cell within Chebyshev radius radiusFt/5 of center,
// inclusive of the center itself. The result has exactly (2r+1)^2 entries
// where r = radiusFt/5.
//
// Precondition: radiusFt must be >= 0.
// Postcondition: the returned slice contains center; no duplicate cells; every
// cell c satisfies max(|c.X-center.X|, |c.Y-center.Y|) <= radiusFt/5.
func BurstCells(center Cell, radiusFt int) []Cell {
	rad := radiusFt / 5
	if rad < 0 {
		rad = 0
	}
	out := make([]Cell, 0, (2*rad+1)*(2*rad+1))
	for dy := -rad; dy <= rad; dy++ {
		for dx := -rad; dx <= rad; dx++ {
			out = append(out, Cell{X: center.X + dx, Y: center.Y + dy})
		}
	}
	return out
}

// ConeCells returns every cell inside the PF2E cone template anchored at apex
// and oriented toward facing, of length lengthFt feet. Per AOE-10 the apex is
// excluded from the result.
//
// The implementation forms a Chebyshev half-square (cardinal facings) or
// quadrant (diagonal facings) of depth lengthFt/5, omitting any cell whose
// offset from the apex has a non-positive dot product with the facing vector.
//
// Precondition: lengthFt must be >= 0.
// Postcondition: every returned cell c satisfies
//
//	max(|c.X-apex.X|, |c.Y-apex.Y|) <= lengthFt/5
//
// and c != apex.
func ConeCells(apex Cell, facing Direction, lengthFt int) []Cell {
	depth := lengthFt / 5
	if depth <= 0 {
		return nil
	}
	fdx, fdy := facingDelta(facing)
	diag := isDiagonal(facing)

	out := make([]Cell, 0, (2*depth+1)*(2*depth+1))
	for dy := -depth; dy <= depth; dy++ {
		for dx := -depth; dx <= depth; dx++ {
			if dx == 0 && dy == 0 {
				continue // AOE-10: exclude apex
			}
			if diag {
				// Diagonal cone: include only the quadrant matching the facing.
				if fdx != 0 && dx*fdx < 0 {
					continue
				}
				if fdy != 0 && dy*fdy < 0 {
					continue
				}
				if dx == 0 || dy == 0 {
					// Cells on the cardinal axes are not part of the diagonal cone.
					continue
				}
			} else {
				// Cardinal cone: include the half-plane facing direction, with
				// |perpendicular offset| <= |facing offset| forming a triangle.
				if fdx != 0 {
					if dx*fdx <= 0 {
						continue
					}
					if abs(dy) > abs(dx) {
						continue
					}
				}
				if fdy != 0 {
					if dy*fdy <= 0 {
						continue
					}
					if abs(dx) > abs(dy) {
						continue
					}
				}
			}
			out = append(out, Cell{X: apex.X + dx, Y: apex.Y + dy})
		}
	}
	return out
}

// LineCells returns every cell inside the PF2E line template originating at
// origin, oriented toward facing, of length lengthFt feet and width widthFt
// feet (centered on the facing axis). Per AOE-10 the origin cell is excluded.
//
// Length is measured in 5-ft cells (lengthFt/5 cells deep). Width is measured
// in 5-ft cells (widthFt/5 cells across). When width is even the line is
// centered with a one-cell offset toward the negative perpendicular axis;
// when odd the line is exactly centered.
//
// Precondition: lengthFt must be >= 0; widthFt must be >= 5 to produce any
// cells (a line with width < 5 ft has no thickness).
// Postcondition: returned slice has at most (lengthFt/5)*(widthFt/5) cells;
// origin is not included.
func LineCells(origin Cell, facing Direction, lengthFt, widthFt int) []Cell {
	depth := lengthFt / 5
	width := widthFt / 5
	if depth <= 0 || width <= 0 {
		return nil
	}
	fdx, fdy := facingDelta(facing)
	// Perpendicular axis: rotate facing 90° clockwise.
	pdx, pdy := -fdy, fdx

	// Centered range of width cells: w0..w1 inclusive.
	w0 := -((width - 1) / 2)
	w1 := width / 2

	out := make([]Cell, 0, depth*width)
	for d := 1; d <= depth; d++ {
		for w := w0; w <= w1; w++ {
			cx := origin.X + d*fdx + w*pdx
			cy := origin.Y + d*fdy + w*pdy
			out = append(out, Cell{X: cx, Y: cy})
		}
	}
	return out
}

// FacingFrom returns the cardinal or diagonal Direction that best approximates
// the vector from src to dst. Axial directions win ties (i.e. when
// |dx| == |dy| the diagonal is selected, but pure-axial vectors with the
// other component equal to zero always select the matching cardinal).
//
// When src == dst, FacingFrom returns DirN as a stable default.
//
// Precondition: none.
// Postcondition: returned Direction is in [DirN, DirNW].
func FacingFrom(src, dst Cell) Direction {
	dx := dst.X - src.X
	dy := dst.Y - src.Y
	if dx == 0 && dy == 0 {
		return DirN
	}
	return facingFromDelta(dx, dy)
}

// facingFromDelta classifies an integer offset into one of eight octants.
// The classifier uses the ratio of |dx| and |dy|: components within a factor
// of two of each other produce a diagonal; otherwise the larger axis wins.
func facingFromDelta(dx, dy int) Direction {
	adx := abs(dx)
	ady := abs(dy)
	// Cardinal: one component dominates by 2× or more.
	if adx >= 2*ady {
		if dx > 0 {
			return DirE
		}
		return DirW
	}
	if ady >= 2*adx {
		if dy > 0 {
			return DirS
		}
		return DirN
	}
	// Diagonal quadrant.
	switch {
	case dx > 0 && dy < 0:
		return DirNE
	case dx > 0 && dy > 0:
		return DirSE
	case dx < 0 && dy > 0:
		return DirSW
	case dx < 0 && dy < 0:
		return DirNW
	}
	// Should be unreachable given the cardinal short-circuits above.
	return DirN
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
