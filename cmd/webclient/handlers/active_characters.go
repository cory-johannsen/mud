// Package handlers provides HTTP handlers for the webclient API.
package handlers

import "sync"

// ActiveCharacterRegistry tracks which character IDs currently have an active WebSocket session.
//
// Invariant: ids is non-nil after NewActiveCharacterRegistry returns.
type ActiveCharacterRegistry struct {
	mu  sync.RWMutex
	ids map[int64]bool
}

// NewActiveCharacterRegistry creates a new, empty registry.
//
// Postcondition: Returned registry is ready for concurrent use.
func NewActiveCharacterRegistry() *ActiveCharacterRegistry {
	return &ActiveCharacterRegistry{ids: make(map[int64]bool)}
}

// Register marks charID as having an active session.
//
// Precondition: charID > 0.
// Postcondition: IsActive(charID) returns true.
func (r *ActiveCharacterRegistry) Register(charID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids[charID] = true
}

// Deregister removes charID from the active set.
//
// Precondition: charID > 0.
// Postcondition: IsActive(charID) returns false.
func (r *ActiveCharacterRegistry) Deregister(charID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.ids, charID)
}

// IsActive reports whether charID has an active WebSocket session.
//
// Precondition: charID > 0.
// Postcondition: Returns true iff Register was called without a subsequent Deregister.
func (r *ActiveCharacterRegistry) IsActive(charID int64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ids[charID]
}
