package skillaction

import (
	"fmt"
	"strings"
)

// Outcome is the structured result of a single Resolve call.
type Outcome struct {
	DoS       DegreeOfSuccess
	Roll      int
	Bonus     int
	DC        int
	Effects   []Effect // outcome's effect list (may be nil)
	Narrative string   // chosen narrative line (placeholders already substituted)
	APRefund  bool     // NCA-33: when true the wrapper SHOULD refund APCost
}

// ResolveContext carries the per-call inputs to Resolve. The Apply callback is
// the side-effect dispatch hook — Resolve itself never mutates external state
// (NCA-11).
type ResolveContext struct {
	ActorName  string
	TargetName string
	Apply      func(eff Effect) // optional; when nil, effects are still recorded on the Outcome
}

// Resolve runs the DoS computation and projects the matching OutcomeDef into
// an Outcome. When ctx.Apply is non-nil, every effect is dispatched in YAML
// order. Narrative substitution honours {actor} and {target} placeholders.
//
// Precondition: def != nil; def.Outcomes is populated (may omit any band — the
// returned Outcome.Effects will be nil for missing bands, and the Narrative
// will fall back to a generic template).
func Resolve(ctx ResolveContext, def *ActionDef, roll, bonus, dc int) (Outcome, error) {
	if def == nil {
		return Outcome{}, fmt.Errorf("Resolve: def is nil")
	}
	dos := DoS(roll, bonus, dc)
	out := Outcome{DoS: dos, Roll: roll, Bonus: bonus, DC: dc}
	od := def.Outcomes[dos]
	if od != nil {
		out.Effects = od.Effects
		out.APRefund = od.APRefund
		if ctx.Apply != nil {
			for _, eff := range od.Effects {
				ctx.Apply(eff)
			}
		}
		out.Narrative = renderNarrative(ctx, def, dos, od)
	} else {
		out.Narrative = fallbackNarrative(ctx, def, dos)
	}
	return out, nil
}

// renderNarrative picks the first Narrative effect's text in the outcome's
// effect list, falls back to OutcomeDef.Narrative, and finally to the generic
// template. {actor} / {target} placeholders are substituted.
func renderNarrative(ctx ResolveContext, def *ActionDef, dos DegreeOfSuccess, od *OutcomeDef) string {
	text := od.Narrative
	if text == "" {
		return fallbackNarrative(ctx, def, dos)
	}
	return substitute(ctx, text)
}

func fallbackNarrative(ctx ResolveContext, def *ActionDef, dos DegreeOfSuccess) string {
	verb := "succeeds"
	switch dos {
	case CritSuccess:
		verb = "succeeds critically"
	case Success:
		verb = "succeeds"
	case Failure:
		verb = "fails"
	case CritFailure:
		verb = "fails critically"
	}
	display := def.DisplayName
	if display == "" {
		display = def.ID
	}
	if ctx.TargetName != "" {
		return fmt.Sprintf("%s %s %s against %s.", nameOr(ctx.ActorName, "Someone"), verb, display, ctx.TargetName)
	}
	return fmt.Sprintf("%s %s %s.", nameOr(ctx.ActorName, "Someone"), verb, display)
}

func substitute(ctx ResolveContext, s string) string {
	s = strings.ReplaceAll(s, "{actor}", nameOr(ctx.ActorName, "the actor"))
	s = strings.ReplaceAll(s, "{target}", nameOr(ctx.TargetName, "the target"))
	return s
}

func nameOr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
