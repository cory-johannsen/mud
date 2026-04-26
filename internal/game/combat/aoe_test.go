package combat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// REQ-AOE-HELPER-1: CombatantsInRadius returns nil when cbt has no combatants.
func TestCombatantsInRadius_EmptyCombat_ReturnsNil(t *testing.T) {
	cbt := &Combat{Combatants: []*Combatant{}}
	center := Combatant{GridX: 0, GridY: 0}
	result := CombatantsInRadius(cbt, center, 10)
	assert.Nil(t, result)
}

// REQ-AOE-HELPER-2: CombatantsInRadius excludes dead combatants.
func TestCombatantsInRadius_ExcludesDead(t *testing.T) {
	dead := &Combatant{ID: "dead", GridX: 0, GridY: 0, Dead: true, CurrentHP: 0}
	alive := &Combatant{ID: "alive", GridX: 0, GridY: 0, Dead: false, CurrentHP: 10}
	cbt := &Combat{Combatants: []*Combatant{dead, alive}}
	center := Combatant{GridX: 0, GridY: 0}

	result := CombatantsInRadius(cbt, center, 10)

	require.Len(t, result, 1)
	assert.Equal(t, "alive", result[0].ID)
}

// REQ-AOE-HELPER-3: CombatantsInRadius excludes combatants outside radius.
func TestCombatantsInRadius_ExcludesOutOfRange(t *testing.T) {
	near := &Combatant{ID: "near", GridX: 1, GridY: 0, Dead: false, CurrentHP: 10}
	far := &Combatant{ID: "far", GridX: 10, GridY: 0, Dead: false, CurrentHP: 10}
	cbt := &Combat{Combatants: []*Combatant{near, far}}
	center := Combatant{GridX: 0, GridY: 0}

	// radiusFt = 10 ft = 2 squares; near is 1 square = 5 ft away (in), far is 10 squares = 50 ft (out)
	result := CombatantsInRadius(cbt, center, 10)

	require.Len(t, result, 1)
	assert.Equal(t, "near", result[0].ID)
}

// REQ-AOE-HELPER-4: CombatantsInRadius includes combatants exactly at the radius boundary.
func TestCombatantsInRadius_IncludesAtBoundary(t *testing.T) {
	// 2 squares = 10 ft; radiusFt = 10 → should include
	boundary := &Combatant{ID: "boundary", GridX: 2, GridY: 0, Dead: false, CurrentHP: 10}
	cbt := &Combat{Combatants: []*Combatant{boundary}}
	center := Combatant{GridX: 0, GridY: 0}

	result := CombatantsInRadius(cbt, center, 10)
	require.Len(t, result, 1)
	assert.Equal(t, "boundary", result[0].ID)
}

// REQ-AOE-HELPER-5: CombatantsInRadius works when target is at grid origin (0,0).
func TestCombatantsInRadius_OriginCenter_IncludesNearby(t *testing.T) {
	c1 := &Combatant{ID: "c1", GridX: 0, GridY: 0, Dead: false, CurrentHP: 5}
	c2 := &Combatant{ID: "c2", GridX: 0, GridY: 1, Dead: false, CurrentHP: 5}
	c3 := &Combatant{ID: "c3", GridX: 5, GridY: 5, Dead: false, CurrentHP: 5}
	cbt := &Combat{Combatants: []*Combatant{c1, c2, c3}}
	center := Combatant{GridX: 0, GridY: 0}

	// radius 10 ft = 2 squares; c1 at distance 0, c2 at 5 ft, c3 at 25 ft
	result := CombatantsInRadius(cbt, center, 10)
	require.Len(t, result, 2)
	ids := make(map[string]bool)
	for _, r := range result {
		ids[r.ID] = true
	}
	assert.True(t, ids["c1"])
	assert.True(t, ids["c2"])
	assert.False(t, ids["c3"])
}

// REQ-AOE-HELPER-6 (property): CombatantsInRadius never returns dead combatants.
func TestProperty_CombatantsInRadius_NeverReturnsDead(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 10).Draw(rt, "n")
		combatants := make([]*Combatant, n)
		for i := range combatants {
			dead := rapid.Bool().Draw(rt, "dead")
			combatants[i] = &Combatant{
				ID:        rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "id"),
				GridX:     rapid.IntRange(0, 9).Draw(rt, "gx"),
				GridY:     rapid.IntRange(0, 9).Draw(rt, "gy"),
				Dead:      dead,
				CurrentHP: rapid.IntRange(0, 100).Draw(rt, "hp"),
			}
		}
		cbt := &Combat{Combatants: combatants}
		center := Combatant{
			GridX: rapid.IntRange(0, 9).Draw(rt, "cx"),
			GridY: rapid.IntRange(0, 9).Draw(rt, "cy"),
		}
		radius := rapid.IntRange(0, 100).Draw(rt, "radius")

		result := CombatantsInRadius(cbt, center, radius)
		for _, c := range result {
			assert.False(rt, c.IsDead(), "dead combatant must not appear in result")
		}
	})
}

// REQ-AOE-HELPER-8: CombatantsInCells returns only living combatants whose cell
// is in the input set; dead combatants and off-set combatants are excluded.
func TestCombatantsInCells_FiltersDeadAndOffSet(t *testing.T) {
	a := &Combatant{ID: "a", GridX: 5, GridY: 5, Dead: false, CurrentHP: 10}
	b := &Combatant{ID: "b", GridX: 6, GridY: 6, Dead: true, CurrentHP: 0}
	c := &Combatant{ID: "c", GridX: 9, GridY: 9, Dead: false, CurrentHP: 10}
	cbt := &Combat{Combatants: []*Combatant{a, b, c}}

	got := CombatantsInCells(cbt, []Cell{{X: 5, Y: 5}, {X: 6, Y: 6}})

	require.Len(t, got, 1)
	assert.Same(t, a, got[0])
}

// REQ-AOE-HELPER-9: CombatantsInCells returns nil for empty cell set.
func TestCombatantsInCells_EmptyCells_ReturnsNil(t *testing.T) {
	cbt := &Combat{Combatants: []*Combatant{
		{ID: "a", GridX: 0, GridY: 0, CurrentHP: 10},
	}}
	assert.Nil(t, CombatantsInCells(cbt, nil))
	assert.Nil(t, CombatantsInCells(cbt, []Cell{}))
}

// REQ-AOE-HELPER-10: CombatantsInRadius regression — behaviour after refactor
// to delegate through CombatantsInCells/BurstCells matches Chebyshev distance.
func TestCombatantsInRadius_RegressionAfterRefactor(t *testing.T) {
	a := &Combatant{ID: "a", GridX: 5, GridY: 5, Dead: false, CurrentHP: 10}
	b := &Combatant{ID: "b", GridX: 5, GridY: 6, Dead: false, CurrentHP: 10}
	cbt := &Combat{Combatants: []*Combatant{a, b}}
	center := Combatant{GridX: 5, GridY: 5}

	got := CombatantsInRadius(cbt, center, 10)
	require.Len(t, got, 2)
}

// REQ-AOE-HELPER-7 (property): All returned combatants are within radiusFt of center.
func TestProperty_CombatantsInRadius_AllWithinRadius(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		combatants := make([]*Combatant, n)
		for i := range combatants {
			combatants[i] = &Combatant{
				ID:        rapid.StringMatching(`[a-z]{3,8}`).Draw(rt, "id"),
				GridX:     rapid.IntRange(0, 9).Draw(rt, "gx"),
				GridY:     rapid.IntRange(0, 9).Draw(rt, "gy"),
				Dead:      false,
				CurrentHP: rapid.IntRange(1, 100).Draw(rt, "hp"),
			}
		}
		cbt := &Combat{Combatants: combatants}
		center := Combatant{
			GridX: rapid.IntRange(0, 9).Draw(rt, "cx"),
			GridY: rapid.IntRange(0, 9).Draw(rt, "cy"),
		}
		radius := rapid.IntRange(0, 100).Draw(rt, "radius")

		result := CombatantsInRadius(cbt, center, radius)
		for _, c := range result {
			dist := CombatRange(*c, center)
			assert.LessOrEqual(rt, dist, radius, "combatant at dist %d must be within radius %d", dist, radius)
		}
	})
}
