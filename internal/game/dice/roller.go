package dice

import "sort"

// Roll evaluates an Expression using the given Source and returns a RollResult.
//
// Precondition: expr must come from Parse (Count >= 1, Sides >= 2); src must be non-nil.
// Postcondition: len(result.Dice) == expr.Count when KeepHighest == 0, or
//
//	len(result.Dice) == expr.KeepHighest when KeepHighest > 0.
//	result.Total() == sum(result.Dice) + result.Modifier.
func Roll(expr Expression, src Source) (RollResult, error) {
	rolled := make([]int, expr.Count)
	for i := range rolled {
		rolled[i] = src.Intn(expr.Sides) + 1
	}

	kept := rolled
	if expr.KeepHighest > 0 {
		sorted := make([]int, len(rolled))
		copy(sorted, rolled)
		sort.Sort(sort.Reverse(sort.IntSlice(sorted)))
		kept = sorted[:expr.KeepHighest]
	}

	return RollResult{
		Expression: expr.Raw,
		Dice:       kept,
		Modifier:   expr.Modifier,
	}, nil
}

// RollExpr parses expr and rolls it using src in a single call.
//
// Precondition: expr must be a valid dice expression string; src must be non-nil.
// Postcondition: Returns a RollResult or a parse/roll error.
func RollExpr(expr string, src Source) (RollResult, error) {
	e, err := Parse(expr)
	if err != nil {
		return RollResult{}, err
	}
	return Roll(e, src)
}

// MustParse parses expr and panics on error. Useful for package-level constants.
//
// Precondition: expr must be a valid dice expression.
func MustParse(expr string) Expression {
	e, err := Parse(expr)
	if err != nil {
		panic("dice: MustParse failed for expression " + expr + ": " + err.Error())
	}
	return e
}
