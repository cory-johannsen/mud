// Package detection implements the PF2E detection states ladder — the
// per-pair (observer, target) → State machine that drives flat-check gating,
// off-guard application, square-guess targeting, and outbound RoomView
// filtering.
//
// See docs/superpowers/specs/2026-04-24-detection-states.md and
// docs/superpowers/plans/2026-04-25-detection-states.md for the full
// requirements catalogue.
package detection

// State is the PF2E detection state of a target relative to a specific
// observer. Pair states are asymmetric: A's state to B is independent of B's
// state to A.
type State int

const (
	// Observed is the default: the observer can see the target clearly. No
	// flat-check or off-guard is applied; attacks proceed normally.
	Observed State = iota
	// Concealed: the observer can pinpoint the target's square but the target
	// has visual concealment. Attacks must succeed at a DC 5 flat check.
	Concealed
	// Hidden: the observer knows the target's square but cannot see them
	// precisely. Attacks must succeed at a DC 11 flat check; on success the
	// target is off-guard against this attack.
	Hidden
	// Undetected: the observer does not know the target's location. Attacks
	// require a square-guess and a DC 11 flat check; the target is off-guard
	// regardless.
	Undetected
	// Unnoticed: like Undetected but the observer does not even know the
	// target exists. RoomView strips Unnoticed targets from the recipient's
	// view entirely.
	Unnoticed
	// Invisible: the observer cannot see the target at all. With an auditory
	// cue this round the pair behaves like Hidden; otherwise like Undetected.
	Invisible
)

// String returns the canonical lowercase identifier for this State, matching
// the YAML condition IDs (observed, concealed, hidden, undetected, unnoticed,
// invisible).
func (s State) String() string {
	switch s {
	case Observed:
		return "observed"
	case Concealed:
		return "concealed"
	case Hidden:
		return "hidden"
	case Undetected:
		return "undetected"
	case Unnoticed:
		return "unnoticed"
	case Invisible:
		return "invisible"
	}
	return "unknown"
}

// MissChancePercent is the approximate chance an attack from this state is
// converted to an automatic miss by the flat check. Used for diagnostic /
// telemetry purposes; the authoritative gating logic lives in GateAttack.
//
// PF2E: Concealed = DC 5 flat check ≈ 20% miss; Hidden / Undetected = DC 11
// flat check ≈ 50% miss. Unnoticed and Invisible-without-sound also include
// a wrong-square auto-miss, modelled as 100% here as a conservative
// placeholder; the real probability depends on grid size and is computed by
// GateAttack.
func (s State) MissChancePercent() int {
	switch s {
	case Observed:
		return 0
	case Concealed:
		return 20
	case Hidden, Undetected, Invisible:
		return 50
	case Unnoticed:
		return 100
	}
	return 0
}

// ParseState resolves a canonical lowercase identifier to its State. Returns
// (Observed, false) for unknown identifiers.
func ParseState(s string) (State, bool) {
	switch s {
	case "observed":
		return Observed, true
	case "concealed":
		return Concealed, true
	case "hidden":
		return Hidden, true
	case "undetected":
		return Undetected, true
	case "unnoticed":
		return Unnoticed, true
	case "invisible":
		return Invisible, true
	}
	return Observed, false
}
