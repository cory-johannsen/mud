# Combat Stage 1: Dice & Roll Engine Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a standalone `internal/game/dice` package that parses and evaluates dice expressions (e.g. `2d6+3`, `d20`, `4d6kh3`) using a cryptographically secure RNG, returning structured results that include every individual die value and the final total.

**Architecture:** Pure Go library package with no external dependencies beyond `crypto/rand`. The parser converts an expression string into a structured `Expression` value; the roller evaluates it against a `Source` interface backed by `crypto/rand`. All rolls return a `RollResult` carrying the full audit trail (expression, per-die values, modifier, total). The package is used by the combat engine in Stage 3 and Lua bindings in Stage 6.

**Tech Stack:** Go 1.26, `crypto/rand`, `pgregory.net/rapid` (property tests), `github.com/stretchr/testify` (assertions). No new dependencies required — all are already in `go.mod`.

---

### Task 1: RollResult type + Source interface

**Files:**
- Create: `internal/game/dice/dice.go`
- Create: `internal/game/dice/dice_test.go`

**Step 1: Write the failing test**

```go
// internal/game/dice/dice_test.go
package dice_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/stretchr/testify/assert"
)

func TestRollResult_Total(t *testing.T) {
	r := dice.RollResult{
		Expression: "2d6+3",
		Dice:       []int{4, 5},
		Modifier:   3,
	}
	assert.Equal(t, 12, r.Total())
}

func TestRollResult_String(t *testing.T) {
	r := dice.RollResult{
		Expression: "2d6+3",
		Dice:       []int{4, 5},
		Modifier:   3,
	}
	s := r.String()
	assert.Contains(t, s, "2d6+3")
	assert.Contains(t, s, "12")
	assert.Contains(t, s, "[4 5]")
}
```

**Step 2: Run to confirm it fails**

```bash
go test ./internal/game/dice/... 2>&1
```
Expected: `cannot find package`

**Step 3: Write the implementation**

```go
// internal/game/dice/dice.go
package dice

import "fmt"

// RollResult holds the full audit trail for a single dice roll evaluation.
//
// Postcondition: Total() == sum(Dice) + Modifier.
type RollResult struct {
	Expression string // original expression string, e.g. "2d6+3"
	Dice       []int  // individual die results before modifier
	Modifier   int    // flat modifier (may be negative)
}

// Total returns the sum of all die results plus the modifier.
func (r RollResult) Total() int {
	sum := 0
	for _, d := range r.Dice {
		sum += d
	}
	return sum + r.Modifier
}

// String returns a human-readable audit string: "2d6+3 → [4 5] +3 = 12"
func (r RollResult) String() string {
	return fmt.Sprintf("%s → %v +%d = %d", r.Expression, r.Dice, r.Modifier, r.Total())
}

// Source is the randomness provider for dice rolls.
// The default implementation uses crypto/rand.
type Source interface {
	// Intn returns a non-negative random int in [0, n). n must be > 0.
	Intn(n int) int
}
```

**Step 4: Run to confirm it passes**

```bash
go test ./internal/game/dice/... -v 2>&1
```
Expected: `PASS`

**Step 5: Commit**

```bash
git add internal/game/dice/dice.go internal/game/dice/dice_test.go
git commit -m "feat(dice): RollResult type and Source interface"
```

---

### Task 2: Cryptographically secure Source implementation

**Files:**
- Create: `internal/game/dice/source.go`
- Modify: `internal/game/dice/dice_test.go`

**Step 1: Write the failing test**

Add to `dice_test.go`:

```go
func TestCryptoSource_Intn_InRange(t *testing.T) {
	src := dice.NewCryptoSource()
	for i := 0; i < 1000; i++ {
		v := src.Intn(6)
		assert.GreaterOrEqual(t, v, 0)
		assert.Less(t, v, 6)
	}
}

func TestCryptoSource_Intn_PanicsOnZero(t *testing.T) {
	src := dice.NewCryptoSource()
	assert.Panics(t, func() { src.Intn(0) })
}
```

**Step 2: Run to confirm it fails**

```bash
go test ./internal/game/dice/... 2>&1
```
Expected: `undefined: dice.NewCryptoSource`

**Step 3: Write the implementation**

```go
// internal/game/dice/source.go
package dice

import (
	"crypto/rand"
	"math/big"
)

// cryptoSource implements Source using crypto/rand.
type cryptoSource struct{}

// NewCryptoSource returns a Source backed by crypto/rand.
//
// Postcondition: Every value returned by Intn is in [0, n).
func NewCryptoSource() Source {
	return &cryptoSource{}
}

// Intn returns a cryptographically secure random int in [0, n).
//
// Precondition: n > 0. Panics if n <= 0.
func (c *cryptoSource) Intn(n int) int {
	if n <= 0 {
		panic("dice: Intn called with n <= 0")
	}
	val, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		// crypto/rand failure is unrecoverable in a security context.
		panic("dice: crypto/rand failure: " + err.Error())
	}
	return int(val.Int64())
}
```

**Step 4: Run to confirm it passes**

```bash
go test ./internal/game/dice/... -v 2>&1
```
Expected: `PASS`

**Step 5: Commit**

```bash
git add internal/game/dice/source.go internal/game/dice/dice_test.go
git commit -m "feat(dice): crypto/rand Source implementation"
```

---

### Task 3: Expression parser — basic NdX and NdX+M forms

**Files:**
- Create: `internal/game/dice/parser.go`
- Modify: `internal/game/dice/dice_test.go`

**Step 1: Write the failing tests**

Add to `dice_test.go`:

```go
func TestParse_BasicForms(t *testing.T) {
	tests := []struct {
		expr     string
		wantN    int
		wantSides int
		wantMod  int
		wantErr  bool
	}{
		{"d20", 1, 20, 0, false},
		{"2d6", 2, 6, 0, false},
		{"2d6+3", 2, 6, 3, false},
		{"4d8-2", 4, 8, -2, false},
		{"1d4+0", 1, 4, 0, false},
		{"d100", 1, 100, 0, false},
		{"", 0, 0, 0, true},
		{"abc", 0, 0, 0, true},
		{"2d0", 0, 0, 0, true},
		{"0d6", 0, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			expr, err := dice.Parse(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantN, expr.Count)
			assert.Equal(t, tt.wantSides, expr.Sides)
			assert.Equal(t, tt.wantMod, expr.Modifier)
		})
	}
}
```

Add `"github.com/stretchr/testify/require"` to imports in `dice_test.go`.

**Step 2: Run to confirm it fails**

```bash
go test ./internal/game/dice/... 2>&1
```
Expected: `undefined: dice.Parse`

**Step 3: Write the implementation**

```go
// internal/game/dice/parser.go
package dice

import (
	"fmt"
	"strconv"
	"strings"
)

// Expression represents a parsed dice expression ready to be rolled.
//
// Precondition: Count >= 1, Sides >= 2 after successful Parse.
type Expression struct {
	Raw      string // original input string
	Count    int    // number of dice
	Sides    int    // faces per die
	Modifier int    // flat modifier (may be negative)
	KeepHighest int // if > 0, keep only the N highest dice (e.g. 4d6kh3)
}

// Parse parses a dice expression string into an Expression.
//
// Supported forms:
//   - "d20"       → 1d20+0
//   - "2d6"       → 2d6+0
//   - "2d6+3"     → 2d6+3
//   - "4d8-2"     → 4d8-2
//   - "4d6kh3"    → roll 4d6, keep highest 3
//
// Precondition: expr must be a non-empty string.
// Postcondition: Returns a non-nil Expression or a descriptive error.
func Parse(expr string) (Expression, error) {
	if expr == "" {
		return Expression{}, fmt.Errorf("dice: empty expression")
	}

	s := strings.ToLower(strings.TrimSpace(expr))

	// Extract keep-highest suffix: "4d6kh3"
	keepHighest := 0
	if idx := strings.Index(s, "kh"); idx != -1 {
		kh, err := strconv.Atoi(s[idx+2:])
		if err != nil || kh <= 0 {
			return Expression{}, fmt.Errorf("dice: invalid keep-highest in %q", expr)
		}
		keepHighest = kh
		s = s[:idx]
	}

	// Split on 'd'
	dIdx := strings.Index(s, "d")
	if dIdx == -1 {
		return Expression{}, fmt.Errorf("dice: missing 'd' in expression %q", expr)
	}

	countStr := s[:dIdx]
	rest := s[dIdx+1:]

	count := 1
	if countStr != "" {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil || count <= 0 {
			return Expression{}, fmt.Errorf("dice: invalid die count in %q", expr)
		}
	}

	// Split rest on + or -
	modifier := 0
	sidesStr := rest
	for i, ch := range rest {
		if i == 0 {
			continue // skip leading digit
		}
		if ch == '+' || ch == '-' {
			mod, err := strconv.Atoi(rest[i:])
			if err != nil {
				return Expression{}, fmt.Errorf("dice: invalid modifier in %q", expr)
			}
			modifier = mod
			sidesStr = rest[:i]
			break
		}
	}

	sides, err := strconv.Atoi(sidesStr)
	if err != nil || sides < 2 {
		return Expression{}, fmt.Errorf("dice: invalid die sides in %q", expr)
	}

	if keepHighest > 0 && keepHighest >= count {
		return Expression{}, fmt.Errorf("dice: keep-highest %d must be less than count %d in %q", keepHighest, count, expr)
	}

	return Expression{
		Raw:         expr,
		Count:       count,
		Sides:       sides,
		Modifier:    modifier,
		KeepHighest: keepHighest,
	}, nil
}
```

**Step 4: Run to confirm it passes**

```bash
go test ./internal/game/dice/... -v 2>&1
```
Expected: all `TestParse_*` tests `PASS`

**Step 5: Commit**

```bash
git add internal/game/dice/parser.go internal/game/dice/dice_test.go
git commit -m "feat(dice): expression parser for NdX, NdX+M, NdXkhK forms"
```

---

### Task 4: Roller — evaluate an Expression against a Source

**Files:**
- Create: `internal/game/dice/roller.go`
- Modify: `internal/game/dice/dice_test.go`

**Step 1: Write the failing tests**

Add to `dice_test.go`:

```go
// deterministicSource returns values from a fixed sequence, cycling.
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

func TestRoll_BasicResult(t *testing.T) {
	// Intn(6) will return 3 and 4 (values 3,4 mod 6)
	src := newDetSource(3, 4)
	expr, err := dice.Parse("2d6+3")
	require.NoError(t, err)

	result, err := dice.Roll(expr, src)
	require.NoError(t, err)

	// die values = Intn(6)+1 = 4, 5
	assert.Equal(t, []int{4, 5}, result.Dice)
	assert.Equal(t, 3, result.Modifier)
	assert.Equal(t, 12, result.Total()) // 4+5+3
	assert.Equal(t, "2d6+3", result.Expression)
}

func TestRoll_D20NoModifier(t *testing.T) {
	src := newDetSource(14) // Intn(20) = 14, die = 15
	expr, _ := dice.Parse("d20")
	result, err := dice.Roll(expr, src)
	require.NoError(t, err)
	assert.Equal(t, []int{15}, result.Dice)
	assert.Equal(t, 0, result.Modifier)
	assert.Equal(t, 15, result.Total())
}

func TestRoll_KeepHighest(t *testing.T) {
	// 4d6kh3: roll 4 dice, keep 3 highest
	// Intn(6) returns 0,1,2,3 → die values 1,2,3,4 → keep 3 highest: 2,3,4 → sum=9
	src := newDetSource(0, 1, 2, 3)
	expr, err := dice.Parse("4d6kh3")
	require.NoError(t, err)
	result, err := dice.Roll(expr, src)
	require.NoError(t, err)
	assert.Equal(t, 3, len(result.Dice))
	assert.Equal(t, 9, result.Total())
}

func TestRoll_NegativeModifier(t *testing.T) {
	src := newDetSource(5) // Intn(8)=5 → die=6
	expr, _ := dice.Parse("1d8-2")
	result, err := dice.Roll(expr, src)
	require.NoError(t, err)
	assert.Equal(t, 4, result.Total()) // 6-2
}
```

**Step 2: Run to confirm it fails**

```bash
go test ./internal/game/dice/... 2>&1
```
Expected: `undefined: dice.Roll`

**Step 3: Write the implementation**

```go
// internal/game/dice/roller.go
package dice

import "sort"

// Roll evaluates an Expression using the given Source and returns a RollResult.
//
// Precondition: expr must come from Parse (Count >= 1, Sides >= 2); src must be non-nil.
// Postcondition: len(result.Dice) == expr.Count (or expr.KeepHighest if set).
// result.Total() == sum(result.Dice) + result.Modifier.
func Roll(expr Expression, src Source) (RollResult, error) {
	raw := make([]int, expr.Count)
	for i := range raw {
		raw[i] = src.Intn(expr.Sides) + 1 // convert [0,sides) → [1,sides]
	}

	kept := raw
	if expr.KeepHighest > 0 {
		sorted := make([]int, len(raw))
		copy(sorted, raw)
		sort.Sort(sort.Reverse(sort.IntSlice(sorted)))
		kept = sorted[:expr.KeepHighest]
	}

	return RollResult{
		Expression: expr.Raw,
		Dice:       kept,
		Modifier:   expr.Modifier,
	}, nil
}

// MustParse parses expr and panics on error. Useful for package-level constants.
//
// Precondition: expr must be a valid dice expression.
func MustParse(expr string) Expression {
	e, err := Parse(expr)
	if err != nil {
		panic("dice.MustParse: " + err.Error())
	}
	return e
}
```

**Step 4: Run to confirm it passes**

```bash
go test ./internal/game/dice/... -v 2>&1
```
Expected: all `TestRoll_*` tests `PASS`

**Step 5: Commit**

```bash
git add internal/game/dice/roller.go internal/game/dice/dice_test.go
git commit -m "feat(dice): Roll evaluator with keep-highest support"
```

---

### Task 5: Convenience roller — parse + roll in one call

**Files:**
- Create: `internal/game/dice/roller.go` (append to existing)
- Modify: `internal/game/dice/dice_test.go`

**Step 1: Write the failing test**

Add to `dice_test.go`:

```go
func TestRollExpr_Convenience(t *testing.T) {
	src := newDetSource(9) // Intn(20)=9 → die=10
	result, err := dice.RollExpr("d20", src)
	require.NoError(t, err)
	assert.Equal(t, 10, result.Total())
}

func TestRollExpr_InvalidExpr(t *testing.T) {
	src := newDetSource(0)
	_, err := dice.RollExpr("not-dice", src)
	assert.Error(t, err)
}
```

**Step 2: Run to confirm it fails**

```bash
go test ./internal/game/dice/... 2>&1
```
Expected: `undefined: dice.RollExpr`

**Step 3: Add RollExpr to roller.go**

Append to `internal/game/dice/roller.go`:

```go
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
```

**Step 4: Run to confirm it passes**

```bash
go test ./internal/game/dice/... -v 2>&1
```
Expected: `PASS`

**Step 5: Commit**

```bash
git add internal/game/dice/roller.go internal/game/dice/dice_test.go
git commit -m "feat(dice): RollExpr convenience function"
```

---

### Task 6: Property-based tests

**Files:**
- Modify: `internal/game/dice/dice_test.go`

**Step 1: Write the property tests**

Add to `dice_test.go` (also add `"pgregory.net/rapid"` to imports):

```go
func TestProperty_TotalAlwaysInRange(t *testing.T) {
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

		min := count*1 + modifier
		max := count*sides + modifier
		total := result.Total()
		if total < min || total > max {
			rt.Fatalf("total %d outside [%d, %d] for %s", total, min, max, expr.Raw)
		}
	})
}

func TestProperty_DiceCountMatchesExpression(t *testing.T) {
	src := dice.NewCryptoSource()
	rapid.Check(t, func(rt *rapid.T) {
		count := rapid.IntRange(1, 20).Draw(rt, "count")
		sides := rapid.IntRange(2, 100).Draw(rt, "sides")

		expr := dice.Expression{
			Raw:   fmt.Sprintf("%dd%d", count, sides),
			Count: count,
			Sides: sides,
		}
		result, err := dice.Roll(expr, src)
		if err != nil {
			rt.Fatal(err)
		}
		if len(result.Dice) != count {
			rt.Fatalf("expected %d dice, got %d", count, len(result.Dice))
		}
	})
}

func TestProperty_EachDieInRange(t *testing.T) {
	src := dice.NewCryptoSource()
	rapid.Check(t, func(rt *rapid.T) {
		count := rapid.IntRange(1, 10).Draw(rt, "count")
		sides := rapid.IntRange(2, 20).Draw(rt, "sides")

		expr := dice.Expression{
			Raw:   fmt.Sprintf("%dd%d", count, sides),
			Count: count,
			Sides: sides,
		}
		result, err := dice.Roll(expr, src)
		if err != nil {
			rt.Fatal(err)
		}
		for _, d := range result.Dice {
			if d < 1 || d > sides {
				rt.Fatalf("die value %d outside [1, %d]", d, sides)
			}
		}
	})
}

func TestProperty_ParseRoundtrip(t *testing.T) {
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
			rt.Fatal(err)
		}
		if expr.Count != count {
			rt.Fatalf("Count: got %d want %d", expr.Count, count)
		}
		if expr.Sides != sides {
			rt.Fatalf("Sides: got %d want %d", expr.Sides, sides)
		}
		if expr.Modifier != modifier {
			rt.Fatalf("Modifier: got %d want %d", expr.Modifier, modifier)
		}
	})
}

func TestProperty_KeepHighestReducesCount(t *testing.T) {
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
		if len(result.Dice) != keep {
			rt.Fatalf("expected %d kept dice, got %d", keep, len(result.Dice))
		}
	})
}
```

Also add `"fmt"` to imports if not already present.

**Step 2: Run property tests**

```bash
go test ./internal/game/dice/... -v -count=1 -timeout=60s 2>&1
```
Expected: all property tests `PASS` with "OK, passed 100 tests"

**Step 3: Commit**

```bash
git add internal/game/dice/dice_test.go
git commit -m "test(dice): property-based tests for range, count, parse roundtrip, keep-highest"
```

---

### Task 7: Logger integration — all rolls logged at debug level

**Files:**
- Create: `internal/game/dice/logged_roller.go`
- Modify: `internal/game/dice/dice_test.go`

**Step 1: Write the failing test**

Add to `dice_test.go` (add `"go.uber.org/zap/zaptest"` to imports):

```go
func TestLoggedRoller_LogsResult(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := newDetSource(9)
	roller := dice.NewLoggedRoller(src, logger)

	result, err := roller.RollExpr("d20")
	require.NoError(t, err)
	assert.Equal(t, 10, result.Total())
	// zaptest captures logs; just verify no error returned and result is correct.
}

func TestLoggedRoller_InvalidExprReturnsError(t *testing.T) {
	logger := zaptest.NewLogger(t)
	src := newDetSource(0)
	roller := dice.NewLoggedRoller(src, logger)
	_, err := roller.RollExpr("bad")
	assert.Error(t, err)
}
```

**Step 2: Run to confirm it fails**

```bash
go test ./internal/game/dice/... 2>&1
```
Expected: `undefined: dice.NewLoggedRoller`

**Step 3: Write the implementation**

```go
// internal/game/dice/logged_roller.go
package dice

import "go.uber.org/zap"

// Roller wraps a Source and logger to provide logged dice rolling.
// All rolls are logged at debug level with the expression, individual dice, modifier, and total.
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

// Roll evaluates expr and logs the result.
//
// Precondition: expr must come from Parse.
// Postcondition: result is logged at debug level; returns RollResult or error.
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
```

**Step 4: Run to confirm it passes**

```bash
go test ./internal/game/dice/... -v 2>&1
```
Expected: `PASS`

**Step 5: Commit**

```bash
git add internal/game/dice/logged_roller.go internal/game/dice/dice_test.go
git commit -m "feat(dice): Roller with structured debug logging per COMBAT-29"
```

---

### Task 8: Wire Roller into gameserver startup

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `cmd/gameserver/main.go`

**Context:** The gameserver currently has no dice dependency. Add a `*dice.Roller` field to `GameServiceServer` so Stage 3 (combat) can use it without a refactor. No behavior changes — just wiring.

**Step 1: Add dice.Roller field to GameServiceServer**

In `internal/gameserver/grpc_service.go`, add the field and update `NewGameServiceServer`:

```go
// At top of file, add import:
"github.com/cory-johannsen/mud/internal/game/dice"

// Add field to GameServiceServer struct:
type GameServiceServer struct {
    // ... existing fields ...
    dice *dice.Roller
}

// Update NewGameServiceServer signature:
func NewGameServiceServer(
    world *world.Manager,
    sessions *session.Manager,
    commands *command.Registry,
    charSaver CharacterSaver,
    dice *dice.Roller,
    logger *zap.Logger,
) *GameServiceServer {
    return &GameServiceServer{
        // ... existing fields ...
        dice: dice,
    }
}
```

**Step 2: Update cmd/gameserver/main.go**

```go
// Add import:
"github.com/cory-johannsen/mud/internal/game/dice"

// After logger is created, before NewGameServiceServer:
cryptoSrc := dice.NewCryptoSource()
diceRoller := dice.NewLoggedRoller(cryptoSrc, logger)

// Pass to NewGameServiceServer:
gameServer := gameserver.NewGameServiceServer(worldMgr, sessionMgr, cmdRegistry, charRepo, diceRoller, logger)
```

**Step 3: Build to confirm no compile errors**

```bash
go build ./... 2>&1
```
Expected: no errors

**Step 4: Run all tests**

```bash
go test -race -count=1 -timeout=600s ./... 2>&1 | tail -30
```
Expected: all packages pass

**Step 5: Commit**

```bash
git add internal/gameserver/grpc_service.go cmd/gameserver/main.go
git commit -m "feat(dice): wire Roller into gameserver startup"
```

---

## Verification Checklist

- [ ] `go test ./internal/game/dice/... -v` — all unit and property tests pass
- [ ] `go build ./...` — no compile errors
- [ ] `go test -race -count=1 -timeout=600s ./...` — full suite passes under race detector
- [ ] `internal/game/dice` has zero external dependencies (only stdlib + zap)
- [ ] `dice.Roller` is wired into `GameServiceServer` ready for Stage 3

## Requirements Covered

| Req | Description |
|-----|-------------|
| COMBAT-27 | Dice subsystem: d4, d6, d8, d10, d12, d20, d100 |
| COMBAT-28 | Uses crypto/rand for all randomness |
| COMBAT-29 | All rolls logged with expression, dice values, modifier, total |
