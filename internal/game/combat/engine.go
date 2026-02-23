package combat

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/cory-johannsen/mud/internal/game/condition"
)

// Combat holds the live state of a single combat encounter in a room.
type Combat struct {
	// RoomID is the room where this combat takes place.
	RoomID string
	// Combatants is the initiative-ordered list of participants.
	Combatants []*Combatant
	// turnIndex is the index of the current actor.
	turnIndex int
	// Over is true when combat has been resolved.
	Over bool
	// Round is the current round number, starting at 0 and incrementing each StartRound call.
	Round int
	// ActionQueues maps combatant UID to their ActionQueue for the current round.
	ActionQueues map[string]*ActionQueue
	// Conditions maps combatant UID to their active condition set.
	Conditions map[string]*condition.ActiveSet
	// condRegistry is the condition registry for this combat.
	condRegistry *condition.Registry
}

// RoundConditionEvent records a condition applied or removed during round startup.
type RoundConditionEvent struct {
	UID         string
	Name        string
	ConditionID string
	CondName    string
	Stacks      int
	Applied     bool // true=applied/advanced, false=removed/expired
}

// StartRound increments Round, ticks conditions, resolves dying recovery checks,
// applies stunned AP reduction, and resets ActionQueues for all living combatants.
//
// Postcondition: Round incremented by 1; ActionQueues reset; returns condition events.
func (c *Combat) StartRound(actionsPerRound int) []RoundConditionEvent {
	return c.StartRoundWithSrc(actionsPerRound, realSrc{})
}

// StartRoundWithSrc is like StartRound but accepts an injectable Source for dice (testable).
//
// Precondition: src must not be nil; actionsPerRound >= 0.
func (c *Combat) StartRoundWithSrc(actionsPerRound int, src Source) []RoundConditionEvent {
	c.Round++
	var events []RoundConditionEvent

	for _, cbt := range c.Combatants {
		if cbt.IsDead() {
			continue
		}
		s := c.Conditions[cbt.ID]

		// Tick durations; collect expired conditions
		expired := s.Tick()
		for _, id := range expired {
			def, _ := c.condRegistry.Get(id)
			name := id
			if def != nil {
				name = def.Name
			}
			events = append(events, RoundConditionEvent{
				UID: cbt.ID, Name: cbt.Name,
				ConditionID: id, CondName: name,
				Applied: false,
			})
		}

		// Dying recovery check (DC 15 flat check)
		if s.Has("dying") {
			dyingStacks := s.Stacks("dying")
			roll := src.Intn(20) + 1
			switch {
			case roll >= 25: // crit success: remove dying entirely, restore to 1 HP
				s.Remove("dying")
				cbt.CurrentHP = 1
				events = append(events, RoundConditionEvent{
					UID: cbt.ID, Name: cbt.Name,
					ConditionID: "dying", CondName: "Dying",
					Applied: false,
				})
			case roll >= 15: // success: remove dying, apply wounded +1, restore to 1 HP
				s.Remove("dying")
				if woundedDef, ok := c.condRegistry.Get("wounded"); ok {
					_ = s.Apply(woundedDef, 1, -1)
				}
				cbt.CurrentHP = 1
				events = append(events, RoundConditionEvent{
					UID: cbt.ID, Name: cbt.Name,
					ConditionID: "dying", CondName: "Dying",
					Applied: false,
				})
				events = append(events, RoundConditionEvent{
					UID: cbt.ID, Name: cbt.Name,
					ConditionID: "wounded", CondName: "Wounded",
					Stacks: s.Stacks("wounded"),
					Applied: true,
				})
			default: // failure: advance dying stacks
				dyingStacks++
				if dyingStacks >= 4 {
					// Dying 4 = death
					cbt.CurrentHP = 0
					cbt.Dead = true
					s.Remove("dying")
					events = append(events, RoundConditionEvent{
						UID: cbt.ID, Name: cbt.Name,
						ConditionID: "dying", CondName: "Dying",
						Stacks: 4, Applied: false,
					})
				} else {
					if dyingDef, ok := c.condRegistry.Get("dying"); ok {
						s.Remove("dying")
						_ = s.Apply(dyingDef, dyingStacks, -1)
					}
					events = append(events, RoundConditionEvent{
						UID: cbt.ID, Name: cbt.Name,
						ConditionID: "dying", CondName: "Dying",
						Stacks: dyingStacks, Applied: true,
					})
				}
			}
		}
	}

	// Reset action queues with stunned AP reduction
	c.ActionQueues = make(map[string]*ActionQueue)
	for _, cbt := range c.Combatants {
		if cbt.IsDead() {
			continue
		}
		ap := actionsPerRound
		s := c.Conditions[cbt.ID]
		reduction := condition.StunnedAPReduction(s)
		ap -= reduction
		if ap < 0 {
			ap = 0
		}
		c.ActionQueues[cbt.ID] = NewActionQueue(cbt.ID, ap)
	}

	return events
}

// realSrc wraps math/rand for production dice rolls.
type realSrc struct{}

func (realSrc) Intn(n int) int { return rand.Intn(n) }

// ApplyCondition applies condition condID to combatant uid.
// Returns error if uid is not a combatant or condID is unknown.
//
// Precondition: uid must be a valid combatant ID; condID must be registered.
// Postcondition: The condition is active on the combatant.
func (c *Combat) ApplyCondition(uid, condID string, stacks, duration int) error {
	def, ok := c.condRegistry.Get(condID)
	if !ok {
		return fmt.Errorf("unknown condition %q", condID)
	}
	s, ok := c.Conditions[uid]
	if !ok {
		return fmt.Errorf("combatant %q not found", uid)
	}
	return s.Apply(def, stacks, duration)
}

// RemoveCondition removes condID from combatant uid. No-op if not present.
func (c *Combat) RemoveCondition(uid, condID string) {
	if s, ok := c.Conditions[uid]; ok {
		s.Remove(condID)
	}
}

// GetConditions returns a snapshot of active conditions for uid.
// Returns nil if uid has no combatant entry.
func (c *Combat) GetConditions(uid string) []*condition.ActiveCondition {
	if s, ok := c.Conditions[uid]; ok {
		return s.All()
	}
	return nil
}

// HasCondition reports whether uid currently has condition condID active.
func (c *Combat) HasCondition(uid, condID string) bool {
	if s, ok := c.Conditions[uid]; ok {
		return s.Has(condID)
	}
	return false
}

// DyingStacks returns the current dying stack count for uid, or 0.
func (c *Combat) DyingStacks(uid string) int {
	if s, ok := c.Conditions[uid]; ok {
		return s.Stacks("dying")
	}
	return 0
}

// QueueAction enqueues an action for the combatant with the given UID.
//
// Precondition: uid must be a living combatant in this combat with an active queue.
// Postcondition: Returns error if uid not found or AP insufficient; otherwise action is appended.
func (c *Combat) QueueAction(uid string, a QueuedAction) error {
	q, ok := c.ActionQueues[uid]
	if !ok {
		return fmt.Errorf("combatant %q not found or has no active queue", uid)
	}
	return q.Enqueue(a)
}

// AllActionsSubmitted reports whether every living combatant's queue IsSubmitted.
//
// Postcondition: Returns true iff all living combatants have no remaining AP or have passed.
func (c *Combat) AllActionsSubmitted() bool {
	for _, cbt := range c.Combatants {
		if cbt.IsDead() {
			continue
		}
		q, ok := c.ActionQueues[cbt.ID]
		if !ok {
			return false
		}
		if !q.IsSubmitted() {
			return false
		}
	}
	return true
}

// CurrentTurn returns the combatant whose turn it currently is, skipping dead ones.
//
// Postcondition: Returns a non-nil living combatant, or nil if all are dead.
func (c *Combat) CurrentTurn() *Combatant {
	for range c.Combatants {
		cbt := c.Combatants[c.turnIndex]
		if !cbt.IsDead() {
			return cbt
		}
		c.turnIndex = (c.turnIndex + 1) % len(c.Combatants)
	}
	return nil
}

// AdvanceTurn moves to the next combatant in initiative order.
//
// Postcondition: turnIndex is incremented modulo len(Combatants).
func (c *Combat) AdvanceTurn() {
	c.turnIndex = (c.turnIndex + 1) % len(c.Combatants)
}

// LivingCombatants returns a snapshot of combatants with CurrentHP > 0.
//
// Postcondition: All returned combatants have CurrentHP > 0.
func (c *Combat) LivingCombatants() []*Combatant {
	var alive []*Combatant
	for _, cbt := range c.Combatants {
		if !cbt.IsDead() {
			alive = append(alive, cbt)
		}
	}
	return alive
}

// HasLivingNPCs reports whether any NPC combatant is still alive.
//
// Postcondition: Returns true iff at least one KindNPC combatant has CurrentHP > 0.
func (c *Combat) HasLivingNPCs() bool {
	for _, cbt := range c.Combatants {
		if cbt.Kind == KindNPC && !cbt.IsDead() {
			return true
		}
	}
	return false
}

// HasLivingPlayers reports whether any player combatant is still alive.
//
// Postcondition: Returns true iff at least one KindPlayer combatant has CurrentHP > 0.
func (c *Combat) HasLivingPlayers() bool {
	for _, cbt := range c.Combatants {
		if cbt.Kind == KindPlayer && !cbt.IsDead() {
			return true
		}
	}
	return false
}

// Engine manages all active Combat encounters, keyed by room ID.
// All methods are safe for concurrent use.
type Engine struct {
	mu      sync.RWMutex
	combats map[string]*Combat
}

// NewEngine creates an empty combat Engine.
//
// Postcondition: Returns a non-nil Engine ready for use.
func NewEngine() *Engine {
	return &Engine{combats: make(map[string]*Combat)}
}

// StartCombat begins a new combat in roomID with the given combatants.
// Combatants are sorted by Initiative descending before storing.
//
// Precondition: roomID must be non-empty; combatants must have at least 2 entries.
// Postcondition: Returns the new Combat or an error if combat is already active in roomID.
func (e *Engine) StartCombat(roomID string, combatants []*Combatant, condRegistry *condition.Registry) (*Combat, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.combats[roomID]; exists {
		return nil, fmt.Errorf("combat already active in room %q", roomID)
	}

	sorted := make([]*Combatant, len(combatants))
	copy(sorted, combatants)
	sortByInitiativeDesc(sorted)

	cbt := &Combat{
		RoomID:       roomID,
		Combatants:   sorted,
		ActionQueues: make(map[string]*ActionQueue),
		Conditions:   make(map[string]*condition.ActiveSet),
		condRegistry: condRegistry,
	}
	for _, c := range sorted {
		cbt.Conditions[c.ID] = condition.NewActiveSet()
	}
	e.combats[roomID] = cbt
	return cbt, nil
}

// GetCombat returns the active combat in roomID.
//
// Postcondition: Returns (combat, true) if found, or (nil, false) otherwise.
func (e *Engine) GetCombat(roomID string) (*Combat, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cbt, ok := e.combats[roomID]
	return cbt, ok
}

// EndCombat removes the combat record for roomID.
//
// Precondition: roomID must be non-empty.
func (e *Engine) EndCombat(roomID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.combats, roomID)
}

// sortByInitiativeDesc sorts combatants in place, highest initiative first.
func sortByInitiativeDesc(combatants []*Combatant) {
	n := len(combatants)
	for i := 1; i < n; i++ {
		for j := i; j > 0 && combatants[j].Initiative > combatants[j-1].Initiative; j-- {
			combatants[j], combatants[j-1] = combatants[j-1], combatants[j]
		}
	}
}
