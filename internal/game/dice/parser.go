package dice

import (
	"fmt"
	"strconv"
	"strings"
)

// Expression represents a parsed dice expression ready to be rolled.
// Precondition: Count >= 1, Sides >= 2 after successful Parse.
type Expression struct {
	Raw         string // original input string
	Count       int    // number of dice
	Sides       int    // faces per die
	Modifier    int    // flat modifier (may be negative)
	KeepHighest int    // if > 0, keep only the N highest dice (e.g. 4d6kh3)
}

// Parse parses a dice expression string into an Expression.
// Supported forms: "d20", "2d6", "2d6+3", "4d8-2", "4d6kh3"
// Precondition: expr must be a non-empty string.
// Postcondition: Returns a non-nil Expression or a descriptive error.
func Parse(expr string) (Expression, error) {
	if expr == "" {
		return Expression{}, fmt.Errorf("dice: empty expression")
	}

	raw := expr
	s := strings.ToLower(expr)

	dIdx := strings.Index(s, "d")
	if dIdx < 0 {
		return Expression{}, fmt.Errorf("dice: missing 'd' in expression %q", raw)
	}

	// Parse count (the part before 'd'); defaults to 1 when omitted.
	var count int
	countStr := s[:dIdx]
	if countStr == "" {
		count = 1
	} else {
		var err error
		count, err = strconv.Atoi(countStr)
		if err != nil {
			return Expression{}, fmt.Errorf("dice: invalid die count in %q: %w", raw, err)
		}
		if count <= 0 {
			return Expression{}, fmt.Errorf("dice: invalid die count in %q: must be >= 1", raw)
		}
	}

	// Everything after 'd'.
	rest := s[dIdx+1:]

	// Extract KeepHighest suffix ("kh<N>") before any modifier.
	keepHighest := 0
	if khIdx := strings.Index(rest, "kh"); khIdx >= 0 {
		khPart := rest[khIdx+2:]
		rest = rest[:khIdx]

		// khPart may still have a modifier suffix; handle that.
		// Find the first '+' or '-' in khPart (not at position 0).
		modOffset := -1
		for i := 1; i < len(khPart); i++ {
			if khPart[i] == '+' || khPart[i] == '-' {
				modOffset = i
				break
			}
		}

		var khStr string
		if modOffset >= 0 {
			// Modifier is after the kh number; re-attach it to rest for later parsing.
			khStr = khPart[:modOffset]
			rest = rest + khPart[modOffset:]
		} else {
			khStr = khPart
		}

		kh, err := strconv.Atoi(khStr)
		if err != nil {
			return Expression{}, fmt.Errorf("dice: invalid kh value in %q: %w", raw, err)
		}
		if kh <= 0 || kh >= count {
			return Expression{}, fmt.Errorf("dice: kh value %d must be > 0 and < count %d in %q", kh, count, raw)
		}
		keepHighest = kh
	}

	// Parse sides and optional modifier from rest.
	// Find the first '+' or '-' that is not at position 0 (to skip leading sign).
	modOffset := -1
	for i := 1; i < len(rest); i++ {
		if rest[i] == '+' || rest[i] == '-' {
			modOffset = i
			break
		}
	}

	var sidesStr, modStr string
	if modOffset >= 0 {
		sidesStr = rest[:modOffset]
		modStr = rest[modOffset:]
	} else {
		sidesStr = rest
		modStr = ""
	}

	sides, err := strconv.Atoi(sidesStr)
	if err != nil {
		return Expression{}, fmt.Errorf("dice: invalid die sides in %q: %w", raw, err)
	}
	if sides < 2 {
		return Expression{}, fmt.Errorf("dice: invalid die sides in %q: must be >= 2", raw)
	}

	modifier := 0
	if modStr != "" {
		modifier, err = strconv.Atoi(modStr)
		if err != nil {
			return Expression{}, fmt.Errorf("dice: invalid modifier in %q: %w", raw, err)
		}
	}

	return Expression{
		Raw:         raw,
		Count:       count,
		Sides:       sides,
		Modifier:    modifier,
		KeepHighest: keepHighest,
	}, nil
}
