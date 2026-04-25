// internal/game/reaction/ready.go
package reaction

import "sync"

// AllowedReadyTriggers is the fixed menu of triggers a player may bind a Ready action to.
// Per REACTION-15 any other trigger is rejected at enqueue time.
var AllowedReadyTriggers = map[ReactionTriggerType]bool{
	TriggerOnEnemyEntersRoom:   true,
	TriggerOnEnemyMoveAdjacent: true,
	TriggerOnAllyDamaged:       true,
}

// AllowedReadyActionTypes is the whitelist of action types a player may prepare with Ready.
// Per REACTION-16 any other action type is rejected at enqueue time.
var AllowedReadyActionTypes = map[string]bool{
	"attack":      true,
	"stride":      true,
	"throw":       true,
	"reload":      true,
	"use_ability": true,
	"use_tech":    true,
}

// ReadyActionDesc is a minimal serializable description of the action to execute
// when a Ready entry fires. Uses string types to avoid an import cycle between
// the combat and reaction packages (combat already imports reaction).
type ReadyActionDesc struct {
	Type        string // one of AllowedReadyActionTypes keys
	Target      string
	Direction   string
	WeaponID    string
	ExplosiveID string
	AbilityID   string
	AbilityCost int // must equal 1 per REACTION-16 for use_ability/use_tech
}

// ReadyEntry represents one pending Ready action registered for a round.
type ReadyEntry struct {
	UID        string
	Trigger    ReactionTriggerType
	TriggerTgt string // optional: restrict to a specific sourceUID; empty means any source
	Action     ReadyActionDesc
	RoundSet   int // the round in which this entry was registered
}

// ReadyRegistry tracks pending Ready entries for the current round.
// All methods are safe for concurrent use.
type ReadyRegistry struct {
	mu      sync.Mutex
	entries []ReadyEntry
}

// NewReadyRegistry creates an empty ReadyRegistry.
func NewReadyRegistry() *ReadyRegistry {
	return &ReadyRegistry{}
}

// Add registers a ReadyEntry.
func (r *ReadyRegistry) Add(e ReadyEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
}

// Consume atomically finds, removes, and returns the first ReadyEntry matching
// uid+trigger (and TriggerTgt when non-empty). Returns nil when none match.
func (r *ReadyRegistry) Consume(uid string, trigger ReactionTriggerType, sourceUID string) *ReadyEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, e := range r.entries {
		if e.UID != uid || e.Trigger != trigger {
			continue
		}
		if e.TriggerTgt != "" && e.TriggerTgt != sourceUID {
			continue
		}
		found := e
		r.entries = append(r.entries[:i], r.entries[i+1:]...)
		return &found
	}
	return nil
}

// ExpireRound removes all entries whose RoundSet equals round.
func (r *ReadyRegistry) ExpireRound(round int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.entries[:0]
	for _, e := range r.entries {
		if e.RoundSet != round {
			kept = append(kept, e)
		}
	}
	r.entries = kept
}

// Cancel removes all entries for the given UID. Called when a player clears
// their action queue before ResolveRound starts.
func (r *ReadyRegistry) Cancel(uid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.entries[:0]
	for _, e := range r.entries {
		if e.UID != uid {
			kept = append(kept, e)
		}
	}
	r.entries = kept
}
