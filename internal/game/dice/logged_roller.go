package dice

import "go.uber.org/zap"

// Roller wraps a Source and logger to provide logged dice rolling.
// All rolls are logged at debug level with expression, dice values, modifier, and total.
//
// Satisfies COMBAT-29: all dice rolls logged with expression, result, and modifiers.
type Roller struct {
	src    Source
	logger *zap.Logger
}

// NewLoggedRoller creates a Roller that rolls with src and logs each roll to logger.
//
// Precondition: src and logger must be non-nil.
func NewLoggedRoller(src Source, logger *zap.Logger) *Roller {
	return &Roller{src: src, logger: logger}
}

// Roll evaluates expr and logs the result at debug level.
//
// Precondition: expr must come from Parse.
// Postcondition: result logged; returns RollResult or error.
func (r *Roller) Roll(expr Expression) (RollResult, error) {
	result, err := Roll(expr, r.src)
	if err != nil {
		return RollResult{}, err
	}
	r.logger.Debug("dice roll",
		zap.String("expression", result.Expression),
		zap.Ints("dice", result.Dice),
		zap.Int("modifier", result.Modifier),
		zap.Int("total", result.Total()),
	)
	return result, nil
}

// RollExpr parses expr and rolls it, logging the result.
//
// Precondition: expr must be a valid dice expression string.
// Postcondition: Returns a RollResult or a parse/roll error.
func (r *Roller) RollExpr(expr string) (RollResult, error) {
	e, err := Parse(expr)
	if err != nil {
		return RollResult{}, err
	}
	return r.Roll(e)
}
