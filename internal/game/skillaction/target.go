package skillaction

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// PreconditionError is the structured error returned by ValidateTarget when a
// pre-resolution gate fails. The handler MUST surface it to the client without
// consuming AP (NCA-17). The Field/Detail pair is suitable for both telnet
// echo and structured RPC responses.
type PreconditionError struct {
	Field  string
	Detail string
}

func (e *PreconditionError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Detail)
}

// TargetCtx is the slim, side-effect-free view of the actor / target / combat
// trio that ValidateTarget needs. Constructing this in the handler keeps the
// skillaction package free of session/manager dependencies.
type TargetCtx struct {
	Combat *combat.Combat
	Actor  *combat.Combatant
	Target *combat.Combatant
}

// ValidateTarget runs the standard target gates for the given action. It
// composes:
//
//  1. Target-kind filter (NCA-12) — npc / player / any / self.
//  2. Range gate (NCA-16) — melee_reach / ranged_feet / self.
//  3. The shared combat.ValidateSingleTarget pipeline — presence, liveness,
//     allegiance, distance, and the line-of-fire seam (NCA-15, currently a
//     no-op until #267 lands).
//
// Returns a *PreconditionError on any failure; nil on success.
func ValidateTarget(tc TargetCtx, def *ActionDef) error {
	if def == nil {
		return &PreconditionError{Field: "def", Detail: "action def is nil"}
	}
	if tc.Actor == nil {
		return &PreconditionError{Field: "actor", Detail: "actor is nil"}
	}

	// Self range short-circuit — target may legitimately be nil/self.
	if def.Range.Kind == RangeSelf {
		if tc.Target != nil && tc.Target.ID != tc.Actor.ID {
			return &PreconditionError{Field: "range", Detail: "must target self"}
		}
		return nil
	}

	if tc.Target == nil {
		return &PreconditionError{Field: "target", Detail: "no target selected"}
	}

	// Stage 1: target-kind filter.
	if !targetKindAllowed(tc.Actor, tc.Target, def.TargetKinds) {
		return &PreconditionError{Field: "target_kind", Detail: targetKindMessage(tc.Target, def.TargetKinds)}
	}

	// Stage 2 + standard combat target pipeline (presence, liveness, allegiance, range, LoF seam).
	maxRangeFt := 0
	switch def.Range.Kind {
	case RangeMelee:
		maxRangeFt = 5 // adjacency in PF2E
	case RangeRanged:
		maxRangeFt = def.Range.Feet
	}
	res := combat.ValidateSingleTarget(tc.Combat, tc.Actor, tc.Target.ID, combat.TargetSingleEnemy, maxRangeFt, false)
	if !res.OK() {
		return &PreconditionError{Field: rangeField(res.Err), Detail: res.Detail}
	}
	return PostValidateTarget(tc, def)
}

// PostValidateTarget is the line-of-fire seam reserved for #267 (NCA-15).
// Currently a no-op; tests may stub via go-link tricks if needed.
var PostValidateTarget = func(tc TargetCtx, def *ActionDef) error {
	return nil
}

// targetKindAllowed reports whether the target's kind matches one of the
// def's allowed TargetKind values.
func targetKindAllowed(actor, target *combat.Combatant, allowed []TargetKind) bool {
	for _, k := range allowed {
		switch k {
		case TargetKindAny:
			return true
		case TargetKindNPC:
			if !target.IsPlayer() {
				return true
			}
		case TargetKindPlayer:
			if target.IsPlayer() {
				return true
			}
		case TargetKindSelf:
			if actor != nil && target != nil && actor.ID == target.ID {
				return true
			}
		}
	}
	return false
}

func targetKindMessage(target *combat.Combatant, allowed []TargetKind) string {
	if len(allowed) == 1 {
		switch allowed[0] {
		case TargetKindNPC:
			return fmt.Sprintf("%s is not an NPC", target.Name)
		case TargetKindPlayer:
			return fmt.Sprintf("%s is not a player", target.Name)
		case TargetKindSelf:
			return fmt.Sprintf("must target self, not %s", target.Name)
		}
	}
	return fmt.Sprintf("target kind %q is not allowed", target.Name)
}

func rangeField(e combat.TargetingError) string {
	switch e {
	case combat.ErrOutOfRange:
		return "range"
	case combat.ErrTargetNotInCombat, combat.ErrTargetMissing:
		return "target"
	case combat.ErrTargetDead:
		return "target_dead"
	case combat.ErrWrongCategory:
		return "target_kind"
	default:
		return "target"
	}
}
