package combat

import "fmt"

// ActionType identifies what a combatant intends to do on their turn.
// The zero value (ActionUnknown) is intentionally invalid.
type ActionType int

const (
	ActionUnknown ActionType = iota // zero value; intentionally invalid
	ActionAttack                    // costs 1 AP
	ActionStrike                    // costs 2 AP; two attacks with MAP
	ActionPass                      // costs 0 AP; forfeits remaining actions
)

// Cost returns the action point cost for the ActionType.
// Precondition: a is a valid ActionType (ActionAttack, ActionStrike, or ActionPass).
// Postcondition: returns 1 for ActionAttack, 2 for ActionStrike, 0 for ActionPass,
// and 0 for ActionUnknown (the zero value is intentionally invalid but has cost 0).
func (a ActionType) Cost() int {
	switch a {
	case ActionAttack:
		return 1
	case ActionStrike:
		return 2
	case ActionPass:
		return 0
	default:
		// ActionUnknown and any unrecognized values have cost 0.
		return 0
	}
}

// String returns the human-readable name of the ActionType.
// Postcondition: returns "attack", "strike", "pass", or "unknown".
func (a ActionType) String() string {
	switch a {
	case ActionAttack:
		return "attack"
	case ActionStrike:
		return "strike"
	case ActionPass:
		return "pass"
	default:
		return "unknown"
	}
}

// QueuedAction represents one action a combatant intends to take this round.
type QueuedAction struct {
	Type   ActionType
	Target string // NPC name for attack/strike; empty for pass
}

// ActionQueue tracks a combatant's remaining action points and queued actions.
// Invariant: remaining >= 0 at all times.
type ActionQueue struct {
	UID       string
	MaxPoints int
	remaining int
	actions   []QueuedAction
}

// RemainingPoints returns the number of action points still available this round.
func (q *ActionQueue) RemainingPoints() int { return q.remaining }

// QueuedActions returns a copy of the slice of queued actions.
func (q *ActionQueue) QueuedActions() []QueuedAction {
	cp := make([]QueuedAction, len(q.actions))
	copy(cp, q.actions)
	return cp
}

// NewActionQueue creates a new ActionQueue for the given combatant UID with the
// specified number of action points per round.
// Precondition: actionsPerRound >= 0.
// Postcondition: RemainingPoints() == actionsPerRound, QueuedActions() is empty.
func NewActionQueue(uid string, actionsPerRound int) *ActionQueue {
	return &ActionQueue{
		UID:       uid,
		MaxPoints: actionsPerRound,
		remaining: actionsPerRound,
		actions:   []QueuedAction{},
	}
}

// Enqueue adds a QueuedAction to the queue if sufficient AP are available.
// For ActionPass, remaining is set to 0 regardless of current value.
// Precondition: q is non-nil; a.Type must not be ActionUnknown.
// Postcondition: on success, action is appended and remaining is decremented by cost;
// on error, queue state is unchanged and remaining >= 0.
func (q *ActionQueue) Enqueue(a QueuedAction) error {
	if a.Type == ActionUnknown {
		return fmt.Errorf("invalid action type: ActionUnknown is not a valid action")
	}
	cost := a.Type.Cost()
	if a.Type == ActionPass {
		q.actions = append(q.actions, a)
		q.remaining = 0
		return nil
	}
	if cost > q.remaining {
		return fmt.Errorf("insufficient AP: need %d, have %d", cost, q.remaining)
	}
	q.actions = append(q.actions, a)
	q.remaining -= cost
	return nil
}

// HasPoints reports whether the combatant has remaining action points and has not
// yet submitted their turn (via ActionPass or exhausting all AP).
// Postcondition: returns true iff remaining > 0 and IsSubmitted() is false.
func (q *ActionQueue) HasPoints() bool {
	return q.remaining > 0 && !q.IsSubmitted()
}

// IsSubmitted reports whether the combatant has committed all their actions for
// this round, either by spending all AP or by queuing a ActionPass.
// Postcondition: returns true iff remaining == 0 or any queued action is ActionPass.
func (q *ActionQueue) IsSubmitted() bool {
	if q.remaining == 0 {
		return true
	}
	// Belt-and-suspenders guard: Enqueue already sets remaining=0 on ActionPass,
	// but we scan the queue to handle any future code paths that bypass Enqueue.
	for _, a := range q.actions {
		if a.Type == ActionPass {
			return true
		}
	}
	return false
}
