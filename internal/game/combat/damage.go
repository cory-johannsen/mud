package combat

import (
	"fmt"
	"math"
	"strings"
)

// DamageStage identifies a stage in the damage resolution pipeline (MULT-1).
type DamageStage string

const (
	StageBase       DamageStage = "base"
	StageMultiplier DamageStage = "multiplier"
	StageHalver     DamageStage = "halver"
	StageWeakness   DamageStage = "weakness"
	StageResistance DamageStage = "resistance"
	StageFloor      DamageStage = "floor"
)

// DamageAdditive is a flat contributor summed into the base damage. Value may be negative
// (e.g. a condition penalty).
type DamageAdditive struct {
	Label  string
	Value  int
	Source string
}

// DamageMultiplier is a ×Factor source. Factor must be > 1.0 (MULT-8).
type DamageMultiplier struct {
	Label  string
	Factor float64
	Source string
}

// DamageHalver is a boolean flag; any present triggers one halving after multipliers (MULT-3).
type DamageHalver struct {
	Label  string
	Source string
}

// DamageInput holds everything ResolveDamage needs — no RNG, no state.
type DamageInput struct {
	Additives   []DamageAdditive
	Multipliers []DamageMultiplier
	Halvers     []DamageHalver
	DamageType  string
	Weakness    int
	Resistance  int
}

// DamageBreakdownStep records one stage's contribution to the final result.
type DamageBreakdownStep struct {
	Stage   DamageStage
	Before  int
	Delta   int
	After   int
	Detail  string
	Sources []string
}

// DamageResult is the output of ResolveDamage.
type DamageResult struct {
	Final     int
	Breakdown []DamageBreakdownStep
}

// ResolveDamage runs the six-stage damage pipeline and returns the final damage and an
// ordered breakdown.
//
// Precondition: every in.Multipliers[i].Factor > 1.0.
// Postcondition: Final >= 0; Breakdown[0].Stage == StageBase (MULT-13).
// Pure: no RNG, no state mutation, no I/O (MULT-11).
func ResolveDamage(in DamageInput) DamageResult {
	var steps []DamageBreakdownStep

	base := 0
	var addSources []string
	for _, a := range in.Additives {
		base += a.Value
		addSources = append(addSources, a.Source)
	}
	steps = append(steps, DamageBreakdownStep{
		Stage:   StageBase,
		Before:  0,
		Delta:   base,
		After:   base,
		Detail:  "flat additives",
		Sources: addSources,
	})
	cur := base

	if len(in.Multipliers) > 0 {
		sumExtra := 0.0
		var mSources []string
		for _, m := range in.Multipliers {
			sumExtra += m.Factor - 1.0
			mSources = append(mSources, m.Source)
		}
		effective := 1.0 + sumExtra
		afterMult := int(math.Floor(float64(cur) * effective))
		steps = append(steps, DamageBreakdownStep{
			Stage:   StageMultiplier,
			Before:  cur,
			Delta:   afterMult - cur,
			After:   afterMult,
			Detail:  fmt.Sprintf("×%.0f effective (from %d source(s))", effective, len(in.Multipliers)),
			Sources: mSources,
		})
		cur = afterMult
	}

	if len(in.Halvers) > 0 {
		afterHalve := int(math.Floor(float64(cur) / 2))
		var hSources []string
		for _, h := range in.Halvers {
			hSources = append(hSources, h.Source)
		}
		steps = append(steps, DamageBreakdownStep{
			Stage:   StageHalver,
			Before:  cur,
			Delta:   afterHalve - cur,
			After:   afterHalve,
			Detail:  in.Halvers[0].Label,
			Sources: hSources,
		})
		cur = afterHalve
	}

	if in.Weakness > 0 {
		afterWeak := cur + in.Weakness
		steps = append(steps, DamageBreakdownStep{
			Stage:   StageWeakness,
			Before:  cur,
			Delta:   in.Weakness,
			After:   afterWeak,
			Detail:  fmt.Sprintf("+%d (%s weakness)", in.Weakness, in.DamageType),
			Sources: []string{"target:weakness"},
		})
		cur = afterWeak
	}

	if in.Resistance > 0 {
		afterRes := cur - in.Resistance
		steps = append(steps, DamageBreakdownStep{
			Stage:   StageResistance,
			Before:  cur,
			Delta:   -in.Resistance,
			After:   afterRes,
			Detail:  fmt.Sprintf("-%d (%s resistance)", in.Resistance, in.DamageType),
			Sources: []string{"target:resistance"},
		})
		cur = afterRes
	}

	if cur < 0 {
		steps = append(steps, DamageBreakdownStep{
			Stage:  StageFloor,
			Before: cur,
			Delta:  -cur,
			After:  0,
			Detail: "floored at 0",
		})
		cur = 0
	}

	return DamageResult{Final: cur, Breakdown: steps}
}

// FormatBreakdownInline renders the breakdown as a single indented line appended
// to the combat narrative. Returns "" when breakdown has only StageBase (trivial). (MULT-14)
//
// Postcondition: returns "" iff len(steps) <= 1.
func FormatBreakdownInline(steps []DamageBreakdownStep) string {
	if len(steps) <= 1 {
		return ""
	}
	var parts []string
	for _, step := range steps {
		switch step.Stage {
		case StageBase:
			parts = append(parts, fmt.Sprintf("base %d", step.After))
		case StageMultiplier:
			// Recover effective multiplier from delta/before; protect against zero base.
			eff := 1.0
			if step.Before != 0 {
				eff = 1.0 + float64(step.Delta)/float64(step.Before)
			}
			parts = append(parts, fmt.Sprintf("×%.0f [%s] = %d", eff, step.Detail, step.After))
		case StageHalver:
			parts = append(parts, fmt.Sprintf("halved [%s] = %d", step.Detail, step.After))
		case StageWeakness:
			parts = append(parts, fmt.Sprintf("weakness +%d = %d", step.Delta, step.After))
		case StageResistance:
			parts = append(parts, fmt.Sprintf("resistance %d = %d", step.Delta, step.After))
		case StageFloor:
			parts = append(parts, "floored = 0")
		}
	}
	line := "  (" + strings.Join(parts, " → ") + ")"
	return wrapAtArrow(line, 80)
}

// FormatBreakdownVerbose renders the full multi-line damage breakdown block (MULT-15).
// Returns "" when no steps. Intended for delivery to observers who have the
// ShowDamageBreakdown preference enabled.
//
// Postcondition: returns "" iff len(steps) == 0.
func FormatBreakdownVerbose(steps []DamageBreakdownStep) string {
	if len(steps) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, step := range steps {
		switch step.Stage {
		case StageBase:
			sb.WriteString(fmt.Sprintf("  base:       %+d\n", step.After))
		case StageMultiplier:
			sb.WriteString(fmt.Sprintf("  multiplier: %s\n", step.Detail))
			sb.WriteString(fmt.Sprintf("  after:      %d\n", step.After))
		case StageHalver:
			sb.WriteString(fmt.Sprintf("  halver:     %s = %d\n", step.Detail, step.After))
		case StageWeakness:
			sb.WriteString(fmt.Sprintf("  weakness:   %s\n", step.Detail))
		case StageResistance:
			sb.WriteString(fmt.Sprintf("  resistance: %s\n", step.Detail))
		case StageFloor:
			sb.WriteString("  final:      0 (floored)\n")
		}
	}
	if steps[len(steps)-1].Stage != StageFloor {
		last := steps[len(steps)-1]
		sb.WriteString(fmt.Sprintf("  final:      %d\n", last.After))
	}
	return sb.String()
}

// wrapAtArrow wraps a breakdown line at " → " boundaries so no segment exceeds maxWidth.
func wrapAtArrow(line string, maxWidth int) string {
	if len(line) <= maxWidth {
		return line
	}
	segments := strings.Split(line, " → ")
	var sb strings.Builder
	cur := ""
	for i, seg := range segments {
		candidate := cur
		if i > 0 {
			candidate += " → "
		}
		candidate += seg
		if len(candidate) > maxWidth && cur != "" {
			sb.WriteString(cur + "\n")
			cur = "    → " + seg
		} else {
			cur = candidate
		}
	}
	sb.WriteString(cur)
	return sb.String()
}
