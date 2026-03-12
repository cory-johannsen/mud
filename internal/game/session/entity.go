// Package session provides player session tracking and room presence management
// for the game backend.
package session

import (
	"fmt"
	"sync"
	"time"
)

// BridgeEntity routes push calls to a Go channel, bridging
// the session system to the gRPC streaming layer.
type BridgeEntity struct {
	uid    string
	events chan []byte
	mu     sync.Mutex
	closed bool
}

// NewBridgeEntity creates a BridgeEntity for the given player UID.
//
// Precondition: uid must be non-empty.
// Postcondition: Returns a BridgeEntity with an open events channel.
func NewBridgeEntity(uid string, bufferSize int) *BridgeEntity {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &BridgeEntity{
		uid:    uid,
		events: make(chan []byte, bufferSize),
	}
}

// UID returns the player's unique identifier.
func (e *BridgeEntity) UID() string {
	return e.uid
}

// Push sends data to the events channel.
//
// Precondition: data must be a non-nil byte slice.
// Postcondition: Data is enqueued to the events channel, or an error if the entity is closed or full.
func (e *BridgeEntity) Push(data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return fmt.Errorf("entity %s is closed", e.uid)
	}
	select {
	case e.events <- data:
		return nil
	default:
		return fmt.Errorf("entity %s event buffer full", e.uid)
	}
}

// PushBlocking sends data to the events channel, blocking until the send succeeds
// or the timeout elapses. Returns an error if the entity is closed or the timeout is exceeded.
//
// Precondition: data must be a non-nil byte slice; timeout must be > 0.
// Postcondition: Data is enqueued, or an error is returned without dropping the event silently.
func (e *BridgeEntity) PushBlocking(data []byte, timeout time.Duration) error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return fmt.Errorf("entity %s is closed", e.uid)
	}
	ch := e.events
	e.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case ch <- data:
		return nil
	case <-timer.C:
		return fmt.Errorf("entity %s push timed out after %v", e.uid, timeout)
	}
}

// Events returns the read-only events channel.
// The gRPC stream goroutine reads from this channel to send ServerEvents.
func (e *BridgeEntity) Events() <-chan []byte {
	return e.events
}

// Close marks the entity as closed and closes the events channel.
//
// Postcondition: The events channel is closed. Further Push calls return an error.
func (e *BridgeEntity) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.closed {
		e.closed = true
		close(e.events)
	}
	return nil
}

// IsClosed reports whether the entity has been closed.
func (e *BridgeEntity) IsClosed() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.closed
}
