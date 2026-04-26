package combat

import (
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/reaction"
)

// ActionType identifies what a combatant intends to do on their turn.
// The zero value (ActionUnknown) is intentionally invalid.
type ActionType int

const (
	ActionUnknown       ActionType = iota // zero value; intentionally invalid
	ActionAttack                          // costs 1 AP
	ActionStrike                          // costs 2 AP; two attacks with MAP
	ActionPass                            // costs 0 AP; forfeits remaining actions
	ActionReload                          // costs 1 AP; reload equipped firearm
	ActionFireBurst                       // costs 2 AP; burst fire
	ActionFireAutomatic                   // costs 3 AP; full-auto suppressive fire
	ActionThrow                           // costs 1 AP; throw explosive
	ActionUseAbility                      // costs AbilityCost AP; activate a class ability
	ActionStride                          // costs 1 AP; move 25ft toward or away from target
	ActionCoverHit                        // informational: attack absorbed by cover
	ActionCoverDestroy                    // informational: cover object destroyed
	ActionAid                             // costs 2 AP; aid an ally
	ActionUseTech                         // costs AbilityCost AP; activate a technology during round resolution
	ActionReady                           // costs 2 AP; prepares one 1-AP action bound to a trigger
	ActionHazardDamage                    // informational: terrain hazard fires damage on entry or round start
)

// Cost returns the action point cost for the ActionType.
// Precondition: a is a valid ActionType.
// Postcondition: returns the correct AP cost for each ActionType; returns 0 for
// ActionUnknown and unrecognized values.
func (a ActionType) Cost() int {
	switch a {
	case ActionAttack:
		return 1
	case ActionStrike:
		return 2
	case ActionPass:
		return 0
	case ActionReload:
		return 1
	case ActionFireBurst:
		return 2
	case ActionFireAutomatic:
		return 3
	case ActionThrow:
		return 1
	case ActionUseAbility:
		return 0 // cost comes from QueuedAction.AbilityCost
	case ActionStride:
		return 1
	case ActionAid:
		return 2
	case ActionUseTech:
		return 0 // cost comes from QueuedAction.AbilityCost
	case ActionReady:
		return 2
	default:
		// ActionUnknown and any unrecognized values have cost 0.
		return 0
	}
}

// String returns the human-readable name of the ActionType.
// Postcondition: returns "attack", "strike", "pass", "reload", "burst",
// "automatic", "throw", "use_ability", "stride", "aid", or "unknown".
func (a ActionType) String() string {
	switch a {
	case ActionAttack:
		return "attack"
	case ActionStrike:
		return "strike"
	case ActionPass:
		return "pass"
	case ActionReload:
		return "reload"
	case ActionFireBurst:
		return "burst"
	case ActionFireAutomatic:
		return "automatic"
	case ActionThrow:
		return "throw"
	case ActionUseAbility:
		return "use_ability"
	case ActionStride:
		return "stride"
	case ActionAid:
		return "aid"
	case ActionUseTech:
		return "use_tech"
	case ActionReady:
		return "ready"
	case ActionHazardDamage:
		return "hazard_damage"
	default:
		return "unknown"
	}
}

// QueuedAction represents one action a combatant intends to take this round.
type QueuedAction struct {
	Type        ActionType
	Target      string // NPC name for attack/strike; empty for pass
	Direction   string // used by ActionStride: "toward" or "away"
	WeaponID    string // for firearm actions; empty = unarmed
	ExplosiveID string // for ActionThrow
	AbilityID   string // for ActionUseAbility and ActionUseTech; the ClassFeature or Technology ID
	AbilityCost int    // for ActionUseAbility and ActionUseTech; AP cost
	TargetX     int32  // for ActionUseTech AoE burst center; -1 means unset
	TargetY     int32  // for ActionUseTech AoE burst center; -1 means unset
	// TargetUID is the canonical combatant UID for the action's primary target.
	// Populated by the targeting pipeline (see combat.ValidateSingleTarget and
	// gameserver.ResolveAndValidate, #249). May be empty for self/AoE-only
	// actions or for legacy code paths that still resolve targets via Target
	// (NPC display name). New code MUST prefer TargetUID over Target.
	TargetUID string
	// Ready fields — only meaningful when Type == ActionReady.
	ReadyTrigger    reaction.ReactionTriggerType // trigger that fires the prepared action
	ReadyAction     *QueuedAction                // the 1-AP action to execute on trigger
	ReadyTriggerTgt string                       // optional: restrict to a specific source UID
}

// MaxMovementAP is the maximum action points a combatant may spend on movement
// (Stride or Step) in a single round per PF2e rules.
const MaxMovementAP = 2

// ActionQueue tracks a combatant's remaining action points and queued actions.
// Invariant: remaining >= 0 at all times.
// Invariant: movementAPSpent <= MaxMovementAP at all times.
type ActionQueue struct {
	UID             string
	MaxPoints       int
	remaining       int
	movementAPSpent int
	actions         []QueuedAction
}

// RemainingPoints returns the number of action points still available this round.
func (q *ActionQueue) RemainingPoints() int { return q.remaining }

// DeductAP reduces remaining action points by cost without queuing an action.
// Use for out-of-queue combat actions (raise_shield, take_cover, feint, demoralize, first_aid).
//
// Precondition: cost > 0.
// Postcondition: remaining decremented by cost on success; unchanged on error.
func (q *ActionQueue) DeductAP(cost int) error {
	if cost <= 0 {
		return fmt.Errorf("DeductAP: cost must be positive, got %d", cost)
	}
	if q.remaining < cost {
		return fmt.Errorf("not enough AP: have %d, need %d", q.remaining, cost)
	}
	q.remaining -= cost
	return nil
}

// DeductMovementAP reduces remaining action points by cost for a movement action
// (Stride or Step), enforcing the per-round movement AP cap (MaxMovementAP).
//
// Precondition: cost > 0.
// Postcondition: on success, remaining and movementAPSpent both decremented/incremented
// by cost; on error, queue state is unchanged.
func (q *ActionQueue) DeductMovementAP(cost int) error {
	if cost <= 0 {
		return fmt.Errorf("DeductMovementAP: cost must be positive, got %d", cost)
	}
	if q.movementAPSpent+cost > MaxMovementAP {
		return fmt.Errorf("movement limit reached: may spend at most %d AP on movement per round (already spent %d)", MaxMovementAP, q.movementAPSpent)
	}
	if q.remaining < cost {
		return fmt.Errorf("not enough AP: have %d, need %d", q.remaining, cost)
	}
	q.remaining -= cost
	q.movementAPSpent += cost
	return nil
}

// MovementAPSpent returns how many action points have been spent on movement this round.
func (q *ActionQueue) MovementAPSpent() int { return q.movementAPSpent }

// AddAP adds n action points to remaining.
//
// Precondition: n >= 0.
// Postcondition: remaining increases by n.
func (q *ActionQueue) AddAP(n int) {
	if n <= 0 {
		return
	}
	q.remaining += n
}

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
	if a.Type == ActionReady {
		if a.ReadyAction == nil {
			return fmt.Errorf("ActionReady requires a non-nil ReadyAction")
		}
		if !reaction.AllowedReadyTriggers[a.ReadyTrigger] {
			return fmt.Errorf("ActionReady: trigger %q is not in the allowed trigger menu", a.ReadyTrigger)
		}
		preparedTypeStr := a.ReadyAction.Type.String()
		if !reaction.AllowedReadyActionTypes[preparedTypeStr] {
			return fmt.Errorf("ActionReady: prepared action type %q is not in the allowed whitelist", preparedTypeStr)
		}
		if (a.ReadyAction.Type == ActionUseAbility || a.ReadyAction.Type == ActionUseTech) &&
			a.ReadyAction.AbilityCost != 1 {
			return fmt.Errorf("ActionReady: prepared %s must have AbilityCost == 1, got %d",
				preparedTypeStr, a.ReadyAction.AbilityCost)
		}
		if 2 > q.remaining {
			return fmt.Errorf("insufficient AP: need 2, have %d", q.remaining)
		}
		q.actions = append(q.actions, a)
		q.remaining -= 2
		return nil
	}
	cost := a.Type.Cost()
	if a.Type == ActionUseAbility || a.Type == ActionUseTech {
		cost = a.AbilityCost
	}
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

// ClearActions drains all queued actions, restores remaining AP to MaxPoints,
// and marks the queue as unsubmitted (IsSubmitted() returns false after this call).
//
// Postcondition: len(QueuedActions()) == 0; RemainingPoints() == MaxPoints; IsSubmitted() == false.
func (q *ActionQueue) ClearActions() {
	q.actions = q.actions[:0]
	q.remaining = q.MaxPoints
	q.movementAPSpent = 0
}
