package combat

import "fmt"

// ActionType identifies what a combatant intends to do on their turn.
type ActionType int

const (
	ActionAttack ActionType = iota + 1 // costs 1 AP
	ActionStrike                        // costs 2 AP; two attacks with MAP
	ActionPass                          // costs 0 AP; forfeits remaining actions
)

// Cost returns the action point cost for the ActionType.
// Precondition: a is a valid ActionType (ActionAttack, ActionStrike, or ActionPass).
// Postcondition: returns 1 for ActionAttack, 2 for ActionStrike, 0 for ActionPass.
func (a ActionType) Cost() int {
	switch a {
	case ActionAttack:
		return 1
	case ActionStrike:
		return 2
	case ActionPass:
		return 0
	default:
		return 0
	}
}

// String returns the human-readable name of the ActionType.
// Postcondition: returns "attack", "strike", or "pass".
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
// Invariant: Remaining >= 0 at all times.
type ActionQueue struct {
	UID       string
	MaxPoints int
	Remaining int
	Actions   []QueuedAction
}

// NewActionQueue creates a new ActionQueue for the given combatant UID with the
// specified number of action points per round.
// Precondition: actionsPerRound >= 0.
// Postcondition: Remaining == actionsPerRound, Actions is empty.
func NewActionQueue(uid string, actionsPerRound int) *ActionQueue {
	return &ActionQueue{
		UID:       uid,
		MaxPoints: actionsPerRound,
		Remaining: actionsPerRound,
		Actions:   []QueuedAction{},
	}
}

// Enqueue adds a QueuedAction to the queue if sufficient AP are available.
// For ActionPass, Remaining is set to 0 regardless of current value.
// Precondition: q is non-nil.
// Postcondition: on success, action is appended and Remaining is decremented by cost;
// on error, queue state is unchanged and Remaining >= 0.
func (q *ActionQueue) Enqueue(a QueuedAction) error {
	cost := a.Type.Cost()
	if a.Type == ActionPass {
		q.Actions = append(q.Actions, a)
		q.Remaining = 0
		return nil
	}
	if cost > q.Remaining {
		return fmt.Errorf("insufficient AP: need %d, have %d", cost, q.Remaining)
	}
	q.Actions = append(q.Actions, a)
	q.Remaining -= cost
	return nil
}

// HasPoints reports whether the combatant has remaining action points and has not
// yet submitted their turn (via ActionPass or exhausting all AP).
// Postcondition: returns true iff Remaining > 0 and IsSubmitted() is false.
func (q *ActionQueue) HasPoints() bool {
	return q.Remaining > 0 && !q.IsSubmitted()
}

// IsSubmitted reports whether the combatant has committed all their actions for
// this round, either by spending all AP or by queuing a ActionPass.
// Postcondition: returns true iff Remaining == 0 or any queued action is ActionPass.
func (q *ActionQueue) IsSubmitted() bool {
	if q.Remaining == 0 {
		return true
	}
	for _, a := range q.Actions {
		if a.Type == ActionPass {
			return true
		}
	}
	return false
}
