package detection

import "sync"

// Map is the per-combat (observer, target) → State table. Absent pairs default
// to Observed. Operations are goroutine-safe: combat resolution touches the
// map from action handlers and from the broadcast goroutine that builds
// per-recipient RoomViews.
//
// The on-disk shape is map[observerUID]map[targetUID]State; an empty map is
// equivalent to "every pair is Observed".
type Map struct {
	mu    sync.RWMutex
	pairs map[string]map[string]State
}

// NewMap constructs an empty Map. All pairs default to Observed until set.
func NewMap() *Map {
	return &Map{pairs: map[string]map[string]State{}}
}

// Get returns the state of target as observed by observer. Returns Observed
// for any unset pair, including the case where observer == target. Get is
// safe to call on a nil receiver and returns Observed.
func (m *Map) Get(observerUID, targetUID string) State {
	if m == nil {
		return Observed
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	row, ok := m.pairs[observerUID]
	if !ok {
		return Observed
	}
	st, ok := row[targetUID]
	if !ok {
		return Observed
	}
	return st
}

// Set assigns the (observer, target) pair to st. Setting a pair to Observed
// is equivalent to Clear: the row entry is removed so absent and explicitly-
// observed pairs are indistinguishable.
//
// Precondition: observerUID and targetUID must be non-empty and distinct.
func (m *Map) Set(observerUID, targetUID string, st State) {
	if m == nil || observerUID == "" || targetUID == "" || observerUID == targetUID {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if st == Observed {
		if row, ok := m.pairs[observerUID]; ok {
			delete(row, targetUID)
			if len(row) == 0 {
				delete(m.pairs, observerUID)
			}
		}
		return
	}
	row, ok := m.pairs[observerUID]
	if !ok {
		row = map[string]State{}
		m.pairs[observerUID] = row
	}
	row[targetUID] = st
}

// Clear removes the (observer, target) pair. After Clear, Get returns
// Observed for that pair.
func (m *Map) Clear(observerUID, targetUID string) {
	m.Set(observerUID, targetUID, Observed)
}

// ForObserver returns a snapshot map of every non-Observed target state for
// observerUID. The returned map is a fresh copy and may be mutated by the
// caller. Returns an empty map (never nil) if observerUID has no entries.
func (m *Map) ForObserver(observerUID string) map[string]State {
	out := map[string]State{}
	if m == nil {
		return out
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if row, ok := m.pairs[observerUID]; ok {
		for k, v := range row {
			out[k] = v
		}
	}
	return out
}

// AdvanceTowardObserved moves the (observer, target) pair one rung up the
// detection ladder toward Observed, modelling PF2E's "damage advances
// awareness" rule: Concealed → Observed, Hidden → Concealed, Undetected →
// Hidden, Unnoticed → Undetected. Invisible and Observed are no-ops in v1.
func AdvanceTowardObserved(m *Map, observerUID, targetUID string) {
	if m == nil {
		return
	}
	cur := m.Get(observerUID, targetUID)
	var next State
	switch cur {
	case Concealed:
		next = Observed
	case Hidden:
		next = Concealed
	case Undetected:
		next = Hidden
	case Unnoticed:
		next = Undetected
	default:
		// Observed and Invisible are no-ops.
		return
	}
	m.Set(observerUID, targetUID, next)
}
