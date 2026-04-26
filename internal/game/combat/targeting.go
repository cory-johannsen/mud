package combat

import "fmt"

// TargetCategory enumerates the kinds of targeting an action may require.
//
// The seven categories cover the minimum-viable-scope of issue #249. AoE
// shape categories (burst/cone/line) are present here so callers can branch
// on category, but AoE-cell validation is deferred (see #250 for shape cell
// generation).
type TargetCategory int

const (
	// TargetUnknown is the zero value; intentionally invalid.
	TargetUnknown TargetCategory = iota
	// TargetSelf — the actor targets themselves; no UID required.
	TargetSelf
	// TargetSingleAlly — exactly one living ally.
	TargetSingleAlly
	// TargetSingleEnemy — exactly one living enemy.
	TargetSingleEnemy
	// TargetSingleAny — any single living combatant.
	TargetSingleAny
	// TargetAoEBurst — a burst centered on a grid cell.
	TargetAoEBurst
	// TargetAoECone — a cone originating from the actor.
	TargetAoECone
	// TargetAoELine — a line originating from the actor.
	TargetAoELine
)

// String returns the canonical category name used in logs and tests.
func (t TargetCategory) String() string {
	switch t {
	case TargetSelf:
		return "self"
	case TargetSingleAlly:
		return "single_ally"
	case TargetSingleEnemy:
		return "single_enemy"
	case TargetSingleAny:
		return "single_any"
	case TargetAoEBurst:
		return "aoe_burst"
	case TargetAoECone:
		return "aoe_cone"
	case TargetAoELine:
		return "aoe_line"
	default:
		return "unknown"
	}
}

// IsAoE reports whether the category names an area-of-effect template.
func (t TargetCategory) IsAoE() bool {
	switch t {
	case TargetAoEBurst, TargetAoECone, TargetAoELine:
		return true
	default:
		return false
	}
}

// IsSingle reports whether the category names a single-target selection that
// requires a target UID.
func (t TargetCategory) IsSingle() bool {
	switch t {
	case TargetSingleAlly, TargetSingleEnemy, TargetSingleAny:
		return true
	default:
		return false
	}
}

// TargetingError enumerates the discrete failure modes for target validation.
//
// TargetOK is the zero value and indicates no error. All other values are
// terminal errors; the caller MUST refuse to enqueue the action.
type TargetingError int

const (
	// TargetOK indicates no validation error.
	TargetOK TargetingError = iota
	// ErrTargetMissing — no target was supplied where one was required.
	ErrTargetMissing
	// ErrTargetNotInCombat — the supplied UID is not present in the combat.
	ErrTargetNotInCombat
	// ErrTargetDead — the target combatant is dead.
	ErrTargetDead
	// ErrOutOfRange — the target is beyond the action's max range.
	ErrOutOfRange
	// ErrLineOfFireBlocked — line of fire is obstructed (deferred — currently
	// always returns TargetOK from the LOF stage; reserved for future use).
	ErrLineOfFireBlocked
	// ErrWrongCategory — the target's allegiance does not match the
	// requested category (e.g. ally requested but an enemy supplied).
	ErrWrongCategory
	// ErrAoEShapeInvalid — the AoE template produced no valid cells (reserved
	// for ValidateAoE; see #250).
	ErrAoEShapeInvalid
)

// String returns the canonical error code used in logs and tests.
func (e TargetingError) String() string {
	switch e {
	case TargetOK:
		return "ok"
	case ErrTargetMissing:
		return "target_missing"
	case ErrTargetNotInCombat:
		return "target_not_in_combat"
	case ErrTargetDead:
		return "target_dead"
	case ErrOutOfRange:
		return "out_of_range"
	case ErrLineOfFireBlocked:
		return "line_of_fire_blocked"
	case ErrWrongCategory:
		return "wrong_category"
	case ErrAoEShapeInvalid:
		return "aoe_shape_invalid"
	default:
		return "unknown"
	}
}

// OK reports whether the error code is the success sentinel.
func (e TargetingError) OK() bool { return e == TargetOK }

// TargetingResult is the structured output of every validation call.
//
// When Err == TargetOK the action MAY proceed; Detail is empty.
// Otherwise the action MUST be refused and Detail SHOULD be surfaced to the
// caller for narration.
type TargetingResult struct {
	Err    TargetingError
	Detail string
}

// OK reports whether validation succeeded.
func (r TargetingResult) OK() bool { return r.Err == TargetOK }

// okResult is the canonical success result.
var okResult = TargetingResult{Err: TargetOK}

// fail builds a TargetingResult with the given error code and detail.
func fail(code TargetingError, format string, args ...any) TargetingResult {
	return TargetingResult{Err: code, Detail: fmt.Sprintf(format, args...)}
}

// areAllies reports whether two combatants are on the same side.
//
// Players are always allied with other players. NPCs with matching FactionID
// are allied; NPCs with empty FactionID are treated as a faction of one
// (allied only with themselves). Players and NPCs are never allied (the
// faction-based PvP/recruitment system is out of scope here).
func areAllies(a, b *Combatant) bool {
	if a == nil || b == nil {
		return false
	}
	if a.ID == b.ID {
		return true
	}
	if a.IsPlayer() && b.IsPlayer() {
		return true
	}
	if !a.IsPlayer() && !b.IsPlayer() {
		if a.FactionID == "" || b.FactionID == "" {
			return false
		}
		return a.FactionID == b.FactionID
	}
	return false
}

// ValidateSingleTarget runs the six-stage validation pipeline for a single
// target action.
//
// Stages, in order:
//  1. Target presence — reject empty UID for single-target categories.
//  2. Combatant lookup — reject UIDs not in cbt.Combatants.
//  3. Liveness — reject dead targets.
//  4. Category match — reject ally/enemy mismatches.
//  5. Range — reject targets beyond maxRangeFt (zero or negative means
//     unlimited range).
//  6. Line of fire — currently a no-op (returns TargetOK); reserved for the
//     LOF feature.
//
// Precondition: cbt and actor MUST be non-nil.
// Postcondition: returns okResult on success, or a structured failure with a
// descriptive Detail message.
func ValidateSingleTarget(cbt *Combat, actor *Combatant, targetID string, category TargetCategory, maxRangeFt int, requiresLOF bool) TargetingResult {
	if cbt == nil {
		return fail(ErrTargetNotInCombat, "no active combat")
	}
	if actor == nil {
		return fail(ErrTargetMissing, "actor is nil")
	}

	// Self category short-circuits — no UID required.
	if category == TargetSelf {
		return okResult
	}

	// Stage 1: presence.
	if targetID == "" {
		return fail(ErrTargetMissing, "no target selected")
	}

	// Stage 2: combatant lookup.
	target := cbt.GetCombatant(targetID)
	if target == nil {
		return fail(ErrTargetNotInCombat, "target %q is not in this combat", targetID)
	}

	// Stage 3: liveness.
	if target.IsDead() {
		return fail(ErrTargetDead, "%s is dead", target.Name)
	}

	// Stage 4: category match.
	switch category {
	case TargetSingleAlly:
		if !areAllies(actor, target) {
			return fail(ErrWrongCategory, "%s is not an ally", target.Name)
		}
	case TargetSingleEnemy:
		if areAllies(actor, target) {
			return fail(ErrWrongCategory, "%s is not an enemy", target.Name)
		}
	case TargetSingleAny:
		// any living combatant is acceptable.
	default:
		// Unknown / AoE categories are not handled by this function.
		return fail(ErrWrongCategory, "category %s is not a single-target category", category)
	}

	// Stage 5: range.
	if maxRangeFt > 0 {
		dist := CombatRange(*actor, *target)
		if dist > maxRangeFt {
			return fail(ErrOutOfRange, "%s is %d ft away (max %d ft)", target.Name, dist, maxRangeFt)
		}
	}

	// Stage 6: line of fire — deferred.
	_ = requiresLOF

	return okResult
}
