package effect

import (
	"sort"
	"time"
)

// effectKey uniquely identifies a single effect on a bearer.
// Per DEDUP-1: (bearer_uid, source_id, caster_uid).
// The bearer_uid is implicit (the EffectSet is per-bearer).
type effectKey struct {
	sourceID  string
	casterUID string
}

// Effect is a named bundle of bonuses from one source.
type Effect struct {
	EffectID       string
	SourceID       string
	CasterUID      string
	Bonuses        []Bonus
	DurKind        DurationKind
	DurRemain      int        // meaningful only for DurationRounds
	ExpiresAt      *time.Time // meaningful only for DurationCalendar
	Annotation     string
	LinkedToCaster bool // if true: removed when caster exits (DEDUP-12)
}

// EffectSet is a per-bearer collection of effects with dedup and version counter.
// A nil *EffectSet is safe: all methods are no-ops / zero-value returns.
type EffectSet struct {
	effects map[effectKey]Effect
	version uint64
}

// NewEffectSet returns an empty, initialized EffectSet.
func NewEffectSet() *EffectSet {
	return &EffectSet{effects: make(map[effectKey]Effect)}
}

func (s *EffectSet) key(e Effect) effectKey {
	return effectKey{sourceID: e.SourceID, casterUID: e.CasterUID}
}

// Apply inserts or overwrites the effect identified by (SourceID, CasterUID). (DEDUP-1)
func (s *EffectSet) Apply(e Effect) {
	if s == nil {
		return
	}
	s.effects[s.key(e)] = e
	s.version++
}

// Remove deletes the effect with the given (sourceID, casterUID) key.
func (s *EffectSet) Remove(sourceID, casterUID string) {
	if s == nil {
		return
	}
	k := effectKey{sourceID: sourceID, casterUID: casterUID}
	if _, ok := s.effects[k]; ok {
		delete(s.effects, k)
		s.version++
	}
}

// RemoveBySource removes all effects whose SourceID matches. (DEDUP-1)
func (s *EffectSet) RemoveBySource(sourceID string) {
	if s == nil {
		return
	}
	changed := false
	for k := range s.effects {
		if k.sourceID == sourceID {
			delete(s.effects, k)
			changed = true
		}
	}
	if changed {
		s.version++
	}
}

// RemoveByCaster removes all LinkedToCaster effects whose CasterUID matches. (DEDUP-12)
func (s *EffectSet) RemoveByCaster(casterUID string) {
	if s == nil {
		return
	}
	changed := false
	for k, e := range s.effects {
		if e.LinkedToCaster && k.casterUID == casterUID {
			delete(s.effects, k)
			changed = true
		}
	}
	if changed {
		s.version++
	}
}

// Tick decrements DurationRounds effects. An effect whose DurRemain is already
// 0 at the start of a Tick is expired and removed; otherwise DurRemain is
// decremented. This means an effect applied with DurRemain=1 survives one Tick
// (decrementing to 0) and expires on the next Tick.
// Returns the EffectIDs of removed effects.
func (s *EffectSet) Tick() []string {
	if s == nil {
		return nil
	}
	var expired []string
	for k, e := range s.effects {
		if e.DurKind != DurationRounds {
			continue
		}
		if e.DurRemain <= 0 {
			expired = append(expired, e.EffectID)
			delete(s.effects, k)
			continue
		}
		e.DurRemain--
		s.effects[k] = e
	}
	if len(expired) > 0 {
		s.version++
	}
	return expired
}

// TickCalendar removes effects whose ExpiresAt is before or equal to now.
func (s *EffectSet) TickCalendar(now time.Time) []string {
	if s == nil {
		return nil
	}
	var expired []string
	for k, e := range s.effects {
		if e.DurKind != DurationCalendar || e.ExpiresAt == nil {
			continue
		}
		if !now.Before(*e.ExpiresAt) {
			expired = append(expired, e.EffectID)
			delete(s.effects, k)
		}
	}
	if len(expired) > 0 {
		s.version++
	}
	return expired
}

// ClearEncounter removes all DurationEncounter effects.
func (s *EffectSet) ClearEncounter() {
	if s == nil {
		return
	}
	changed := false
	for k, e := range s.effects {
		if e.DurKind == DurationEncounter {
			delete(s.effects, k)
			changed = true
		}
	}
	if changed {
		s.version++
	}
}

// All returns a stable-sorted snapshot of all active effects.
func (s *EffectSet) All() []Effect {
	if s == nil {
		return nil
	}
	out := make([]Effect, 0, len(s.effects))
	for _, e := range s.effects {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SourceID != out[j].SourceID {
			return out[i].SourceID < out[j].SourceID
		}
		return out[i].CasterUID < out[j].CasterUID
	})
	return out
}

// Version returns the monotonic mutation counter. (DEDUP-8)
func (s *EffectSet) Version() uint64 {
	if s == nil {
		return 0
	}
	return s.version
}
