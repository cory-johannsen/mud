package combat

import (
	"math"
)

// MoveWeights weights the four NPC tactical-movement goals. Zero in any field
// means "use the default value" via WithDefaults.
//
// The defaults are tuned so range dominates (NPCs primarily move toward / away
// from the threat), terrain meaningfully discounts difficult footing, cover is
// pursued when available, and spread keeps allies from clumping.
type MoveWeights struct {
	Range   float64
	Cover   float64
	Spread  float64
	Terrain float64
}

// DefaultMoveWeights are the baseline tactical weights applied to NPCs that do
// not declare per-template overrides. These match the spec's MOVE-16 defaults.
var DefaultMoveWeights = MoveWeights{
	Range:   1.0,
	Cover:   0.5,
	Spread:  0.3,
	Terrain: 0.4,
}

// WithDefaults fills any zero-valued field on w from DefaultMoveWeights.
func (w MoveWeights) WithDefaults() MoveWeights {
	if w.Range == 0 {
		w.Range = DefaultMoveWeights.Range
	}
	if w.Cover == 0 {
		w.Cover = DefaultMoveWeights.Cover
	}
	if w.Spread == 0 {
		w.Spread = DefaultMoveWeights.Spread
	}
	if w.Terrain == 0 {
		w.Terrain = DefaultMoveWeights.Terrain
	}
	return w
}

// MoveContext captures the per-NPC inputs the goal functions need that are
// not derivable from the Combatant struct alone (range increment in feet,
// cover availability, faction allies). Consumers in the gameserver package
// build this from npc instance / inventory registry data and hand it to
// ChooseMoveDestination.
type MoveContext struct {
	// RangeIncrementFt is the equipped weapon's range increment in feet.
	// 0 = melee.
	RangeIncrementFt int
	// UseCover is true when the NPC's combat strategy permits hugging cover.
	UseCover bool
	// CoverTierAt returns the cover tier ("", "lesser", "standard", "greater")
	// the NPC would receive at the candidate cell relative to the target. nil
	// means "no cover model wired" — coverGoal returns 0 in that case.
	CoverTierAt func(cell GridCell) string
	// Allies is the list of living faction-mates the NPC should avoid clumping
	// with. Always excludes the NPC itself.
	Allies []*Combatant
	// Weights override DefaultMoveWeights when non-zero per field.
	Weights MoveWeights
}

// chebyshev returns the Chebyshev distance between two grid cells.
func chebyshev(a, b GridCell) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

// CandidateCells enumerates every grid cell reachable from the actor's current
// position within its remaining stride budget for the round. Each cell on the
// returned slice is in-bounds, passable (not greater_difficult terrain), and
// not currently occupied by another living combatant or cover. The actor's
// current cell is always included, even when no other moves are available
// (MOVE-9: staying put is always a candidate).
//
// Approximation: we bound the search by a Chebyshev radius equal to the
// per-stride SpeedBudget × MaxMovementAP. Difficult terrain (cost 2) means
// some candidates within the radius would not actually be reachable in two
// strides, but the scoring layer will simply prefer reachable cells; the
// stride loop itself enforces budget at execution time.
//
// Precondition: cbt and actor must not be nil; actor must be on the grid.
// Postcondition: returned slice contains actor's current cell and only cells
// satisfying the above filters.
func CandidateCells(cbt *Combat, actor *Combatant) []GridCell {
	if cbt == nil || actor == nil {
		return nil
	}
	width := cbt.GridWidth
	if width <= 0 {
		width = 10
	}
	height := cbt.GridHeight
	if height <= 0 {
		height = 10
	}
	speed := actor.SpeedBudget()
	if speed < 1 {
		speed = 1
	}
	radius := speed * MaxMovementAP

	here := GridCell{X: actor.GridX, Y: actor.GridY}
	out := make([]GridCell, 0, (2*radius+1)*(2*radius+1))
	out = append(out, here)
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			x := actor.GridX + dx
			y := actor.GridY + dy
			if x < 0 || y < 0 || x >= width || y >= height {
				continue
			}
			if CellBlocked(cbt, actor.ID, x, y) {
				continue
			}
			// TerrainGreaterDifficult reports passable=false; exclude.
			if _, passable := cbt.EntryCost(x, y); !passable {
				continue
			}
			out = append(out, GridCell{X: x, Y: y})
		}
	}
	return out
}

// RangeGoal scores how desirable a cell is for the actor's preferred engagement
// distance. Melee NPCs (RangeIncrementFt == 0) prefer adjacency (Chebyshev 1);
// ranged NPCs prefer their first range increment (range/5 cells). The score is
// 1.0 at the optimal distance and falls linearly to 0.0 at the worst case
// (current grid diagonal). Ranged NPCs explicitly score point-blank cells
// (distance <= 1) at 0.0 — they should never prefer to be in melee.
//
// Precondition: target may be nil; in that case the goal returns 0.
// Postcondition: result is in [0, 1].
func RangeGoal(ctx MoveContext, target *Combatant, cell GridCell, gridDiagonal int) float64 {
	if target == nil {
		return 0
	}
	dist := chebyshev(cell, GridCell{X: target.GridX, Y: target.GridY})
	if ctx.RangeIncrementFt > 0 && dist <= 1 {
		return 0
	}
	preferred := 1
	if ctx.RangeIncrementFt > 0 {
		preferred = ctx.RangeIncrementFt / 5
		if preferred < 1 {
			preferred = 1
		}
	}
	worst := gridDiagonal
	if worst < 1 {
		worst = 1
	}
	delta := dist - preferred
	if delta < 0 {
		delta = -delta
	}
	score := 1.0 - float64(delta)/float64(worst)
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}

// CoverGoal scores how desirable a cell is for cover relative to the current
// threat target. Returns 0 when the NPC's strategy disables cover or no cover
// model is wired. Otherwise lesser/standard/greater map to 0.33/0.66/1.0.
//
// Postcondition: result is in [0, 1].
func CoverGoal(ctx MoveContext, cell GridCell) float64 {
	if !ctx.UseCover || ctx.CoverTierAt == nil {
		return 0
	}
	switch ctx.CoverTierAt(cell) {
	case CoverTierGreater:
		return 1.0
	case CoverTierStandard:
		return 0.66
	case CoverTierLesser:
		return 0.33
	default:
		return 0
	}
}

// SpreadGoal scores how desirable a cell is for keeping the NPC away from
// faction-allied combatants. Score scales linearly with the Chebyshev distance
// to the nearest living ally, saturating at the ranged NPC's first increment
// (or 6 cells / 30 ft for melee NPCs). NPCs with no allies always score 1.0.
//
// Postcondition: result is in [0, 1].
func SpreadGoal(ctx MoveContext, actor *Combatant, cell GridCell) float64 {
	nearest := math.MaxInt32
	for _, a := range ctx.Allies {
		if a == nil || a == actor || a.IsDead() {
			continue
		}
		if a.FactionID != actor.FactionID {
			continue
		}
		d := chebyshev(cell, GridCell{X: a.GridX, Y: a.GridY})
		if d < nearest {
			nearest = d
		}
	}
	if nearest == math.MaxInt32 {
		return 1.0
	}
	cap := 6
	if ctx.RangeIncrementFt > 0 {
		ri := ctx.RangeIncrementFt / 5
		if ri > cap {
			cap = ri
		}
	}
	if cap < 1 {
		cap = 1
	}
	score := float64(nearest) / float64(cap)
	if score > 1 {
		score = 1
	}
	if score < 0 {
		score = 0
	}
	return score
}

// TerrainGoal scores how desirable a cell is on terrain grounds. Normal cells
// score 1.0; difficult cells score 0.5; hazardous cells score 0.0. Greater-
// difficult cells are filtered out at candidate enumeration so they never
// reach this scorer.
//
// Postcondition: result is in [0, 1].
func TerrainGoal(cbt *Combat, cell GridCell) float64 {
	if cbt == nil {
		return 1.0
	}
	tc := cbt.TerrainAt(cell.X, cell.Y)
	switch tc.Type {
	case TerrainHazardous:
		return 0.0
	case TerrainDifficult:
		return 0.5
	default:
		return 1.0
	}
}

// scoreCell computes the weighted sum of all four goals for a candidate.
func scoreCell(cbt *Combat, ctx MoveContext, actor, target *Combatant, cell GridCell, gridDiagonal int, flipRange bool) float64 {
	w := ctx.Weights.WithDefaults()
	rg := RangeGoal(ctx, target, cell, gridDiagonal)
	if flipRange {
		rg = 1.0 - rg
	}
	return w.Range*rg +
		w.Cover*CoverGoal(ctx, cell) +
		w.Spread*SpreadGoal(ctx, actor, cell) +
		w.Terrain*TerrainGoal(cbt, cell)
}

// ChooseMoveDestination returns the cell the NPC should attempt to occupy this
// round, or nil to indicate "stay put". The chooser enumerates candidates,
// scores each by the weighted goal sum, and picks the highest-scoring cell.
// Ties break by lower Y, then lower X (deterministic).
//
// When the highest-scoring cell is the actor's current position (or scores
// within an epsilon of staying put), the function returns nil so the caller
// can skip queuing a stride entirely (MOVE-20).
//
// flipRange may be set by callers when the NPC is fleeing (HP below the flee
// threshold) or routed (combat threat above the courage threshold) — the
// rangeGoal is mirrored so the NPC actively seeks distance from the target.
//
// Precondition: cbt and actor must not be nil; target may be nil (in which
// case no movement is chosen).
// Postcondition: returned pointer, when non-nil, refers to a cell present in
// CandidateCells(cbt, actor).
func ChooseMoveDestination(cbt *Combat, actor, target *Combatant, ctx MoveContext, flipRange bool) *GridCell {
	if cbt == nil || actor == nil || target == nil {
		return nil
	}
	candidates := CandidateCells(cbt, actor)
	if len(candidates) <= 1 {
		return nil
	}
	gridDiagonal := cbt.GridWidth
	if cbt.GridHeight > gridDiagonal {
		gridDiagonal = cbt.GridHeight
	}
	if gridDiagonal <= 0 {
		gridDiagonal = 10
	}

	here := GridCell{X: actor.GridX, Y: actor.GridY}
	hereScore := scoreCell(cbt, ctx, actor, target, here, gridDiagonal, flipRange)

	bestScore := math.Inf(-1)
	var best GridCell
	for _, cell := range candidates {
		s := scoreCell(cbt, ctx, actor, target, cell, gridDiagonal, flipRange)
		if s > bestScore {
			bestScore = s
			best = cell
			continue
		}
		if s == bestScore {
			// Deterministic tie-break: lower Y wins, then lower X.
			if cell.Y < best.Y || (cell.Y == best.Y && cell.X < best.X) {
				best = cell
			}
		}
	}

	const epsilon = 0.01
	if best == here || (bestScore-hereScore) < epsilon {
		return nil
	}
	return &best
}

// StepToward returns the unit (dx, dy) step that moves one Chebyshev cell from
// 'from' toward 'to'. dx, dy ∈ {-1, 0, 1}; (0,0) means already coincident.
func StepToward(from, to GridCell) (int, int) {
	dx := 0
	if to.X > from.X {
		dx = 1
	} else if to.X < from.X {
		dx = -1
	}
	dy := 0
	if to.Y > from.Y {
		dy = 1
	} else if to.Y < from.Y {
		dy = -1
	}
	return dx, dy
}
