package dice_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// TestRollResult_Total verifies the postcondition: Total() == sum(Dice) + Modifier.
func TestRollResult_Total(t *testing.T) {
	r := dice.RollResult{
		Expression: "2d6+3",
		Dice:       []int{4, 5},
		Modifier:   3,
	}
	assert.Equal(t, 12, r.Total(), "Total() must equal sum(Dice)+Modifier")
}

// TestRollResult_String verifies the audit string contains expression, dice, and total.
func TestRollResult_String(t *testing.T) {
	r := dice.RollResult{
		Expression: "2d6+3",
		Dice:       []int{4, 5},
		Modifier:   3,
	}
	s := r.String()
	require.Contains(t, s, "2d6+3", "String() must contain the expression")
	require.Contains(t, s, "[4 5]", "String() must contain the dice results")
	require.Contains(t, s, "12", "String() must contain the total")
	assert.Equal(t, "2d6+3 \u2192 [4 5] +3 = 12", s, "String() must match exact format")
}

// TestRollResult_Total_Property uses property-based testing to verify the
// postcondition Total() == sum(Dice) + Modifier for arbitrary inputs.
func TestRollResult_Total_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dice_ := rapid.SliceOf(rapid.IntRange(1, 20)).Draw(rt, "dice")
		modifier := rapid.Int().Draw(rt, "modifier")

		r := dice.RollResult{
			Expression: "Nd6+M",
			Dice:       dice_,
			Modifier:   modifier,
		}

		expected := modifier
		for _, d := range dice_ {
			expected += d
		}

		assert.Equal(rt, expected, r.Total(),
			"Total() postcondition: must equal sum(Dice)+Modifier")
	})
}

// TestRollResult_String_Property verifies String() always contains the expression
// and the total for arbitrary RollResult values.
func TestRollResult_String_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		expr := rapid.StringMatching(`[0-9]+d[0-9]+[+-][0-9]+`).Draw(rt, "expression")
		dice_ := rapid.SliceOfN(rapid.IntRange(1, 20), 1, 10).Draw(rt, "dice")
		modifier := rapid.IntRange(-100, 100).Draw(rt, "modifier")

		r := dice.RollResult{
			Expression: expr,
			Dice:       dice_,
			Modifier:   modifier,
		}

		s := r.String()
		assert.True(rt, strings.Contains(s, expr),
			"String() must contain the expression %q", expr)
		assert.True(rt, strings.Contains(s, "\u2192"),
			"String() must contain the unicode arrow \u2192")
		assert.Contains(rt, s, fmt.Sprintf("%d", r.Total()),
			"String() must contain the computed total")
	})
}

// TestRollResult_String_PanicsOnEmptyExpression verifies that String() enforces
// its precondition and panics when Expression is empty.
func TestRollResult_String_PanicsOnEmptyExpression(t *testing.T) {
	r := dice.RollResult{Dice: []int{4}, Modifier: 0}
	assert.Panics(t, func() { _ = r.String() })
}

// TestCryptoSource_Intn_InRange verifies the postcondition:
// every value returned by Intn(6) is in [0, 6).
func TestCryptoSource_Intn_InRange(t *testing.T) {
	src := dice.NewCryptoSource()
	for i := 0; i < 1000; i++ {
		v := src.Intn(6)
		assert.GreaterOrEqual(t, v, 0)
		assert.Less(t, v, 6)
	}
}

// TestCryptoSource_Intn_PanicsOnZero verifies the precondition:
// Intn panics when called with n <= 0.
func TestCryptoSource_Intn_PanicsOnZero(t *testing.T) {
	src := dice.NewCryptoSource()
	assert.Panics(t, func() { src.Intn(0) })
	assert.Panics(t, func() { src.Intn(-1) })
}

// TestCryptoSource_Intn_InRange_Property uses property-based testing to verify
// the postcondition: Intn(n) returns a value in [0, n) for arbitrary n >= 1.
func TestCryptoSource_Intn_InRange_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 1<<20).Draw(rt, "n")
		src := dice.NewCryptoSource()
		v := src.Intn(n)
		assert.GreaterOrEqual(rt, v, 0,
			"Intn postcondition: result must be >= 0")
		assert.Less(rt, v, n,
			"Intn postcondition: result must be < n")
	})
}

func TestParse_BasicForms(t *testing.T) {
	tests := []struct {
		expr      string
		wantN     int
		wantSides int
		wantMod   int
		wantKH    int
		wantErr   bool
	}{
		{"d20", 1, 20, 0, 0, false},
		{"2d6", 2, 6, 0, 0, false},
		{"2d6+3", 2, 6, 3, 0, false},
		{"4d8-2", 4, 8, -2, 0, false},
		{"1d4+0", 1, 4, 0, 0, false},
		{"d100", 1, 100, 0, 0, false},
		{"4d6kh3", 4, 6, 0, 3, false}, // keep-highest success
		{"", 0, 0, 0, 0, true},
		{"abc", 0, 0, 0, 0, true},
		{"2d0", 0, 0, 0, 0, true},   // sides < 2
		{"0d6", 0, 0, 0, 0, true},   // count = 0
		{"3d6kh3", 0, 0, 0, 0, true}, // kh == count
		{"4d6kh0", 0, 0, 0, 0, true}, // kh == 0
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			expr, err := dice.Parse(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expr, expr.Raw)
			assert.Equal(t, tt.wantN, expr.Count)
			assert.Equal(t, tt.wantSides, expr.Sides)
			assert.Equal(t, tt.wantMod, expr.Modifier)
			assert.Equal(t, tt.wantKH, expr.KeepHighest)
		})
	}
}

// deterministicSource cycles through a fixed sequence of values for deterministic testing.
// Not goroutine-safe; for sequential test use only.
type deterministicSource struct {
	values []int
	idx    int
}

func (d *deterministicSource) Intn(n int) int {
	v := d.values[d.idx%len(d.values)] % n
	d.idx++
	return v
}

func newDetSource(vals ...int) dice.Source {
	return &deterministicSource{values: vals}
}

// TestRoll_BasicResult verifies Roll produces correct dice, modifier, total, and expression
// for a basic NdX+M expression using a deterministic source.
func TestRoll_BasicResult(t *testing.T) {
	// Intn(6) returns 3,4 → die values 4,5
	src := newDetSource(3, 4)
	expr, err := dice.Parse("2d6+3")
	require.NoError(t, err)
	result, err := dice.Roll(expr, src)
	require.NoError(t, err)
	assert.Equal(t, []int{4, 5}, result.Dice)
	assert.Equal(t, 3, result.Modifier)
	assert.Equal(t, 12, result.Total()) // 4+5+3
	assert.Equal(t, "2d6+3", result.Expression)
}

// TestRoll_D20NoModifier verifies Roll with a single d20 and no modifier.
func TestRoll_D20NoModifier(t *testing.T) {
	src := newDetSource(14) // Intn(20)=14 → die=15
	expr, _ := dice.Parse("d20")
	result, err := dice.Roll(expr, src)
	require.NoError(t, err)
	assert.Equal(t, []int{15}, result.Dice)
	assert.Equal(t, 0, result.Modifier)
	assert.Equal(t, 15, result.Total())
}

// TestRoll_KeepHighest verifies that Roll with kh suffix keeps only the N highest dice.
func TestRoll_KeepHighest(t *testing.T) {
	// 4d6kh3: Intn(6) returns 0,1,2,3 → die values 1,2,3,4 → keep 3 highest: [4,3,2] → sum=9
	src := newDetSource(0, 1, 2, 3)
	expr, err := dice.Parse("4d6kh3")
	require.NoError(t, err)
	result, err := dice.Roll(expr, src)
	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Dice))
	assert.Equal(t, 9, result.Total())
}

// TestRoll_NegativeModifier verifies Roll applies a negative modifier correctly.
func TestRoll_NegativeModifier(t *testing.T) {
	src := newDetSource(5) // Intn(8)=5 → die=6
	expr, _ := dice.Parse("1d8-2")
	result, err := dice.Roll(expr, src)
	require.NoError(t, err)
	assert.Equal(t, 4, result.Total()) // 6-2
}

// TestMustParse_ValidExpr verifies MustParse returns a correct Expression for valid input.
func TestMustParse_ValidExpr(t *testing.T) {
	expr := dice.MustParse("2d6+3")
	assert.Equal(t, 2, expr.Count)
	assert.Equal(t, 6, expr.Sides)
	assert.Equal(t, 3, expr.Modifier)
}

// TestMustParse_InvalidPanics verifies MustParse panics on invalid input.
func TestMustParse_InvalidPanics(t *testing.T) {
	assert.Panics(t, func() { dice.MustParse("bad") })
}

func TestRoll_Property_Postconditions(t *testing.T) {
	src := dice.NewCryptoSource()
	rapid.Check(t, func(rt *rapid.T) {
		count := rapid.IntRange(1, 10).Draw(rt, "count")
		sides := rapid.IntRange(2, 20).Draw(rt, "sides")
		modifier := rapid.IntRange(-10, 10).Draw(rt, "modifier")

		expr := dice.Expression{
			Raw:      fmt.Sprintf("%dd%d", count, sides),
			Count:    count,
			Sides:    sides,
			Modifier: modifier,
		}

		result, err := dice.Roll(expr, src)
		if err != nil {
			rt.Fatal(err)
		}

		// Postcondition: len(Dice) == Count when no KeepHighest
		if len(result.Dice) != count {
			rt.Fatalf("expected %d dice, got %d", count, len(result.Dice))
		}

		// Postcondition: each die in [1, sides]
		for _, d := range result.Dice {
			if d < 1 || d > sides {
				rt.Fatalf("die value %d outside [1, %d]", d, sides)
			}
		}

		// Postcondition: Total() == sum(Dice) + Modifier
		sum := 0
		for _, d := range result.Dice {
			sum += d
		}
		if result.Total() != sum+modifier {
			rt.Fatalf("Total() %d != sum(%d) + modifier(%d)", result.Total(), sum, modifier)
		}
	})
}

func TestRoll_Property_KeepHighest_Postconditions(t *testing.T) {
	src := dice.NewCryptoSource()
	rapid.Check(t, func(rt *rapid.T) {
		count := rapid.IntRange(2, 10).Draw(rt, "count")
		keep := rapid.IntRange(1, count-1).Draw(rt, "keep")
		sides := rapid.IntRange(2, 20).Draw(rt, "sides")

		expr := dice.Expression{
			Raw:         fmt.Sprintf("%dd%dkh%d", count, sides, keep),
			Count:       count,
			Sides:       sides,
			KeepHighest: keep,
		}

		result, err := dice.Roll(expr, src)
		if err != nil {
			rt.Fatal(err)
		}

		// Postcondition: len(Dice) == KeepHighest
		if len(result.Dice) != keep {
			rt.Fatalf("expected %d kept dice, got %d", keep, len(result.Dice))
		}

		// Postcondition: each die in [1, sides]
		for _, d := range result.Dice {
			if d < 1 || d > sides {
				rt.Fatalf("die value %d outside [1, %d]", d, sides)
			}
		}
	})
}

// TestParse_Property_ValidExpressionsHaveCorrectFields verifies that for any
// valid NdX+M expression, Parse produces Count=N, Sides=X, Modifier=M.
func TestParse_Property_ValidExpressionsHaveCorrectFields(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		count := rapid.IntRange(1, 20).Draw(rt, "count")
		sides := rapid.IntRange(2, 100).Draw(rt, "sides")
		modifier := rapid.IntRange(-20, 20).Draw(rt, "modifier")

		var exprStr string
		if modifier >= 0 {
			exprStr = fmt.Sprintf("%dd%d+%d", count, sides, modifier)
		} else {
			exprStr = fmt.Sprintf("%dd%d%d", count, sides, modifier)
		}

		expr, err := dice.Parse(exprStr)
		if err != nil {
			rt.Fatalf("unexpected error for %q: %v", exprStr, err)
		}
		if expr.Count != count {
			rt.Fatalf("Count: got %d want %d for %q", expr.Count, count, exprStr)
		}
		if expr.Sides != sides {
			rt.Fatalf("Sides: got %d want %d for %q", expr.Sides, sides, exprStr)
		}
		if expr.Modifier != modifier {
			rt.Fatalf("Modifier: got %d want %d for %q", expr.Modifier, modifier, exprStr)
		}
		if expr.Raw != exprStr {
			rt.Fatalf("Raw: got %q want %q", expr.Raw, exprStr)
		}
	})
}
