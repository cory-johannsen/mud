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
}
