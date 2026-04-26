package combat

// MoveCause classifies why a combatant changed position during a round. It is
// threaded through reaction-trigger checks (e.g. CheckReactiveStrikesCtx) so
// move-trait granted movement (and any future suppressed-reaction movement)
// can be distinguished from regular Stride / MoveTo / forced movement.
//
// WMOVE-12: when Cause == MoveCauseMoveTrait the reactive-strike check
// early-returns without firing any reactions.
type MoveCause int

const (
	// MoveCauseStride is a movement triggered by a normal ActionStride.
	MoveCauseStride MoveCause = iota
	// MoveCauseMoveTo is a movement triggered by an out-of-combat MoveTo
	// request (or any other non-stride deliberate movement).
	MoveCauseMoveTo
	// MoveCauseMoveTrait is a movement granted by the Mobile / Move weapon
	// trait. Reactive strikes are suppressed for this cause.
	MoveCauseMoveTrait
	// MoveCauseForced is a movement caused by an outside force (push, pull,
	// teleport, knockback). Reaction handling is left to the caller; the
	// default reactive-strike check still applies unless the caller decides
	// otherwise.
	MoveCauseForced
)

// String returns a stable human-readable name for the MoveCause.
func (c MoveCause) String() string {
	switch c {
	case MoveCauseStride:
		return "stride"
	case MoveCauseMoveTo:
		return "move_to"
	case MoveCauseMoveTrait:
		return "move_trait"
	case MoveCauseForced:
		return "forced"
	default:
		return "unknown"
	}
}

// ReactionMoveContext describes a single position change for the purposes of
// reaction-trigger evaluation.
type ReactionMoveContext struct {
	// MoverID is the UID of the combatant that moved.
	MoverID string
	// FromX/FromY is the grid position the mover occupied before the move.
	FromX int
	FromY int
	// Cause is the reason the move happened. Reaction checks use this to
	// suppress / route per-cause behaviour.
	Cause MoveCause
}
