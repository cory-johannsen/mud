package gameserver

import (
	"sync"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// targetingRegistry is a per-player CombatTargeting registry keyed by player
// UID. The state is held in gameserver (rather than session.PlayerSession)
// because combat already imports session — a *combat.CombatTargeting field
// on PlayerSession would close the import cycle session→combat→session.
//
// This registry is the canonical home for sticky-target state at enqueue
// time. The handler wiring (telnet attack/strike/use) is deferred to a
// follow-up of #249; this scaffolding gives the wiring layer a stable
// callable surface.
//
// targetingRegistry is concurrency-safe.
type targetingRegistry struct {
	mu      sync.RWMutex
	byOwner map[string]*combat.CombatTargeting
}

// newTargetingRegistry constructs an empty registry.
func newTargetingRegistry() *targetingRegistry {
	return &targetingRegistry{byOwner: map[string]*combat.CombatTargeting{}}
}

// For returns the CombatTargeting bound to ownerUID, lazily creating one on
// first access. The returned value is safe to retain across calls.
func (r *targetingRegistry) For(ownerUID string) *combat.CombatTargeting {
	if r == nil || ownerUID == "" {
		return nil
	}
	r.mu.RLock()
	if s, ok := r.byOwner[ownerUID]; ok {
		r.mu.RUnlock()
		return s
	}
	r.mu.RUnlock()
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.byOwner[ownerUID]; ok {
		return s
	}
	s := combat.NewCombatTargeting()
	r.byOwner[ownerUID] = s
	return s
}

// Drop removes the targeting state for ownerUID — invoked when the player
// leaves combat or disconnects.
func (r *targetingRegistry) Drop(ownerUID string) {
	if r == nil || ownerUID == "" {
		return
	}
	r.mu.Lock()
	delete(r.byOwner, ownerUID)
	r.mu.Unlock()
}
