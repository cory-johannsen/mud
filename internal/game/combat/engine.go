package combat

import (
	"fmt"
	"sync"
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
func (e *Engine) StartCombat(roomID string, combatants []*Combatant) (*Combat, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.combats[roomID]; exists {
		return nil, fmt.Errorf("combat already active in room %q", roomID)
	}

	sorted := make([]*Combatant, len(combatants))
	copy(sorted, combatants)
	sortByInitiativeDesc(sorted)

	cbt := &Combat{
		RoomID:     roomID,
		Combatants: sorted,
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
