// Package eventbus provides an in-process publish/subscribe bus for fan-out of
// server events to all active SSE subscribers.
package eventbus

import (
	"encoding/json"
	"sync"
	"time"
)

// Event is a JSON-serialisable envelope wrapping a single server event.
//
// Invariant: Type is a non-empty proto message name; Payload is valid JSON.
type Event struct {
	Type    string          // proto message name, e.g. "CombatEvent"
	Payload json.RawMessage // protojson-encoded ServerEvent payload
	Time    time.Time
}

// EventBus fans out published events to all current subscribers.
//
// Invariant: all operations are safe for concurrent use.
type EventBus struct {
	mu      sync.RWMutex
	subs    map[uint64]chan Event
	nextID  uint64
	bufSize int
}

// New returns a running EventBus. bufSize is the per-subscriber channel buffer.
//
// Precondition: bufSize must be > 0.
// Postcondition: Returns a ready-to-use EventBus.
func New(bufSize int) *EventBus {
	if bufSize <= 0 {
		bufSize = 64
	}
	return &EventBus{
		subs:    make(map[uint64]chan Event),
		bufSize: bufSize,
	}
}

// Subscribe returns a channel that receives published events and an unsubscribe
// function. The caller MUST call unsubscribe when done to avoid resource leaks.
//
// Postcondition: The returned channel is buffered with bufSize capacity.
func (b *EventBus) Subscribe() (<-chan Event, func()) {
	b.mu.Lock()
	id := b.nextID
	b.nextID++
	ch := make(chan Event, b.bufSize)
	b.subs[id] = ch
	b.mu.Unlock()

	unsub := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.dropLocked(id)
	}
	return ch, unsub
}

// Publish sends e to all current subscribers.
// Subscribers whose buffer is full are dropped non-blockingly.
//
// Postcondition: All subscribers with available buffer space receive e.
func (b *EventBus) Publish(e Event) {
	// Collect IDs of full subscribers while holding read lock.
	b.mu.RLock()
	type pending struct {
		id uint64
		ch chan Event
	}
	var toSend []pending
	for id, ch := range b.subs {
		toSend = append(toSend, pending{id: id, ch: ch})
	}
	b.mu.RUnlock()

	// Attempt non-blocking sends; accumulate those that failed (full buffer).
	var full []uint64
	for _, p := range toSend {
		select {
		case p.ch <- e:
		default:
			full = append(full, p.id)
		}
	}

	// Drop subscribers that had full buffers.
	if len(full) > 0 {
		b.mu.Lock()
		for _, id := range full {
			b.dropLocked(id)
		}
		b.mu.Unlock()
	}
}

// dropLocked removes subscriber id from the map and closes its channel.
// It is a no-op if id is not present (idempotent).
//
// Precondition: b.mu MUST be held for writing by the caller.
func (b *EventBus) dropLocked(id uint64) {
	ch, ok := b.subs[id]
	if !ok {
		return
	}
	delete(b.subs, id)
	close(ch)
}
