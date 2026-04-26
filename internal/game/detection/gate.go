package detection

// GateOutcome is the disposition GateAttack reaches for the candidate attack.
type GateOutcome int

const (
	// GateProceed means the attack roll proceeds to normal AC resolution.
	// OffGuard may still be true on the result.
	GateProceed GateOutcome = iota
	// GateAutoMiss means the attack is converted to a miss before AC
	// resolution — typically by a failed flat check or a wrong-square guess.
	GateAutoMiss
)

// GateResult is the disposition of an attempted attack from observer→target,
// after consulting the pair's detection state.
type GateResult struct {
	// Outcome is the gate disposition: Proceed or AutoMiss.
	Outcome GateOutcome
	// OffGuard, when true, means the target should be treated as off-guard
	// against this attack (typically via the existing condition pipeline).
	// Off-guard is conferred by Hidden / Undetected / Unnoticed / Invisible
	// pair-states regardless of whether the gate proceeded or auto-missed.
	OffGuard bool
	// FlatRoll is the d20 roll consulted for the flat check, or 0 if no
	// check was rolled. Diagnostic only.
	FlatRoll int
	// FlatDC is the DC of the flat check that gated this attack, or 0 if no
	// check was rolled. Diagnostic only.
	FlatDC int
}

// IntnSource is the minimal RNG interface GateAttack needs. It matches the
// combat package's Source interface so the gate can share the same RNG.
type IntnSource interface {
	Intn(n int) int
}

// Cell is a grid coordinate used for square-guess targeting against an
// Undetected / Unnoticed / Invisible target. Cells compare with == and may
// be used as map keys.
type Cell struct {
	X int
	Y int
}

type gateOpts struct {
	hasGuess bool
	guess    Cell
	// targetCell is the actual cell the target occupies, used to compare
	// against the guess. Zero-valued by default; callers wishing to gate on
	// square-guess accuracy MUST pass WithTargetCell.
	hasTargetCell bool
	targetCell    Cell
	// soundCue indicates the target made an auditory action this round
	// (PF2E "auditory" trait). Required for Invisible to fall through to
	// Hidden semantics rather than Undetected.
	soundCue bool
}

// Option mutates a GateAttack invocation.
type Option func(*gateOpts)

// WithSquareGuess attaches a guessed cell from the attacker. Required for
// Undetected / Unnoticed / Invisible-without-sound targets to have any chance
// of resolving as a hit; the gate auto-misses without a guess for those
// states.
func WithSquareGuess(c Cell) Option {
	return func(o *gateOpts) {
		o.hasGuess = true
		o.guess = c
	}
}

// WithTargetCell supplies the target's actual cell so the gate can compare
// the guess for square-guess accuracy.
func WithTargetCell(c Cell) Option {
	return func(o *gateOpts) {
		o.hasTargetCell = true
		o.targetCell = c
	}
}

// WithSoundCue marks that the target made a sound this round. Required for
// Invisible to fall through to Hidden semantics rather than Undetected.
func WithSoundCue(made bool) Option {
	return func(o *gateOpts) {
		o.soundCue = made
	}
}

// GateAttack is the uniform per-state gate that runs before normal AC
// resolution. It returns the gate disposition (Proceed or AutoMiss) and the
// off-guard flag the caller should apply (typically by attaching a transient
// off_guard condition via the existing pipeline).
//
// The DC matrix per state is:
//
//	Observed                : no check, OffGuard=false, Proceed
//	Concealed               : DC 5 flat check; fail → AutoMiss; pass → Proceed
//	Hidden                  : DC 11 flat check; fail → AutoMiss; pass → Proceed, OffGuard=true
//	Undetected / Unnoticed  : Wrong-square guess → AutoMiss, OffGuard=true
//	                          Right-square guess → DC 11 flat check; fail → AutoMiss; pass → Proceed
//	                          Both branches always set OffGuard=true.
//	Invisible               : if soundCue → behave as Hidden; else as Undetected.
//
// Precondition: src must be non-nil for any state that rolls a flat check
// (i.e., everything except Observed).
func GateAttack(st State, src IntnSource, opts ...Option) GateResult {
	o := gateOpts{}
	for _, opt := range opts {
		opt(&o)
	}
	switch st {
	case Observed:
		return GateResult{Outcome: GateProceed}
	case Concealed:
		roll := src.Intn(20) + 1
		if roll < 5 {
			return GateResult{Outcome: GateAutoMiss, FlatRoll: roll, FlatDC: 5}
		}
		return GateResult{Outcome: GateProceed, FlatRoll: roll, FlatDC: 5}
	case Hidden:
		roll := src.Intn(20) + 1
		if roll < 11 {
			return GateResult{Outcome: GateAutoMiss, FlatRoll: roll, FlatDC: 11}
		}
		return GateResult{Outcome: GateProceed, OffGuard: true, FlatRoll: roll, FlatDC: 11}
	case Undetected, Unnoticed:
		// Wrong-square guess (or no guess at all) is an automatic miss.
		// The target is off-guard regardless.
		if !o.hasGuess || (o.hasTargetCell && o.guess != o.targetCell) {
			return GateResult{Outcome: GateAutoMiss, OffGuard: true}
		}
		roll := src.Intn(20) + 1
		if roll < 11 {
			return GateResult{Outcome: GateAutoMiss, OffGuard: true, FlatRoll: roll, FlatDC: 11}
		}
		return GateResult{Outcome: GateProceed, OffGuard: true, FlatRoll: roll, FlatDC: 11}
	case Invisible:
		if o.soundCue {
			return GateAttack(Hidden, src, opts...)
		}
		return GateAttack(Undetected, src, opts...)
	}
	return GateResult{Outcome: GateProceed}
}
