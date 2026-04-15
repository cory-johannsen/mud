package condition

import (
	"fmt"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/cory-johannsen/mud/internal/scripting"
)

// ActiveCondition tracks one applied condition on an entity.
type ActiveCondition struct {
	Def               *ConditionDef
	Stacks            int
	DurationRemaining int        // -1 = permanent or until_save
	Source            string     // e.g. "drawback:goon"; empty for non-tagged conditions
	ExpiresAt         *time.Time // non-nil for real-time-expiring conditions
}

// ActiveSet tracks all conditions currently applied to one combatant.
// It is not safe for concurrent use; the caller must serialise access.
type ActiveSet struct {
	conditions map[string]*ActiveCondition
	scriptMgr  *scripting.Manager
	zoneID     string
}

// NewActiveSet creates an empty ActiveSet.
func NewActiveSet() *ActiveSet {
	return &ActiveSet{conditions: make(map[string]*ActiveCondition)}
}

// SetScripting attaches a scripting.Manager and zoneID to this ActiveSet.
// Subsequent Apply/Remove/Tick calls will fire Lua hooks via mgr.
//
// Precondition: mgr may be nil; passing nil disables Lua hooks.
// Postcondition: Lua hooks are enabled for this set when mgr is non-nil.
func (s *ActiveSet) SetScripting(mgr *scripting.Manager, zoneID string) {
	s.scriptMgr = mgr
	s.zoneID = zoneID
}

// Apply adds or updates a condition on this entity.
// If the condition is already present, stacks are incremented (capped at MaxStacks).
// If MaxStacks == 0 (unstackable), stacks is always stored as 1.
// duration is rounds remaining; use -1 for permanent or until_save.
//
// Precondition: def must not be nil.
// Postcondition: Has(def.ID) is true; stacks are incremented on re-apply (capped at MaxStacks);
// DurationRemaining is updated to max(existing, duration) on re-apply.
func (s *ActiveSet) Apply(uid string, def *ConditionDef, stacks, duration int) error {
	if def == nil {
		return fmt.Errorf("Apply: def must not be nil")
	}

	if existing, ok := s.conditions[def.ID]; ok {
		if def.MaxStacks == 0 {
			// unstackable: stacks stays at 1; extend duration if longer
			if duration > existing.DurationRemaining {
				existing.DurationRemaining = duration
			}
			return nil
		}
		newStacks := existing.Stacks + stacks
		if newStacks > def.MaxStacks {
			newStacks = def.MaxStacks
		}
		existing.Stacks = newStacks
		if duration > existing.DurationRemaining {
			existing.DurationRemaining = duration
		}
		return nil
	}

	// Determine effective stacks for new condition
	effectiveStacks := stacks
	if def.MaxStacks == 0 {
		effectiveStacks = 1
	}
	capped := effectiveStacks
	if def.MaxStacks > 0 && capped > def.MaxStacks {
		capped = def.MaxStacks
	}
	s.conditions[def.ID] = &ActiveCondition{
		Def:               def,
		Stacks:            capped,
		DurationRemaining: duration,
	}

	if s.scriptMgr != nil && def.LuaOnApply != "" {
		stks := s.conditions[def.ID].Stacks
		s.scriptMgr.CallHook(s.zoneID, def.LuaOnApply, //nolint:errcheck
			lua.LString(uid), lua.LString(def.ID),
			lua.LNumber(stks), lua.LNumber(float64(duration)),
		)
	}

	return nil
}

// Remove deletes the condition with the given ID from the set.
// If the condition is not present, Remove is a no-op.
//
// Postcondition: Has(id) is false.
func (s *ActiveSet) Remove(uid, id string) {
	removed := s.conditions[id]
	delete(s.conditions, id)
	if s.scriptMgr != nil && removed != nil && removed.Def.LuaOnRemove != "" {
		s.scriptMgr.CallHook(s.zoneID, removed.Def.LuaOnRemove, //nolint:errcheck
			lua.LString(uid), lua.LString(id),
		)
	}
}

// Tick decrements the DurationRemaining of all "rounds"-type conditions by 1.
// Conditions that reach 0 are removed. "permanent" and "until_save" conditions
// (DurationRemaining == -1) are not affected.
//
// LuaOnTick receives DurationRemaining before the decrement — the pre-tick remaining value.
//
// Postcondition: For every id in the returned slice, Has(id) is false.
// Conditions with DurationType != "rounds" or DurationRemaining == -1 are not affected.
func (s *ActiveSet) Tick(uid string) []string {
	var expired []string
	// Deleting map entries during range iteration is safe per the Go specification.
	for id, ac := range s.conditions {
		if ac.Def.DurationType != "rounds" || ac.DurationRemaining < 0 {
			continue // non-rounds conditions: no tick hook
		}
		if s.scriptMgr != nil && ac.Def.LuaOnTick != "" {
			s.scriptMgr.CallHook(s.zoneID, ac.Def.LuaOnTick, //nolint:errcheck
				lua.LString(uid), lua.LString(id),
				lua.LNumber(float64(ac.Stacks)), lua.LNumber(float64(ac.DurationRemaining)),
			)
		}
		ac.DurationRemaining--
		if ac.DurationRemaining <= 0 {
			expired = append(expired, id)
			delete(s.conditions, id)
		}
	}
	return expired
}

// Has reports whether the condition with id is currently active.
func (s *ActiveSet) Has(id string) bool {
	_, ok := s.conditions[id]
	return ok
}

// Stacks returns the current stack count for condition id, or 0 if not present.
func (s *ActiveSet) Stacks(id string) int {
	if ac, ok := s.conditions[id]; ok {
		return ac.Stacks
	}
	return 0
}

// ClearEncounter removes all conditions with DurationType == "encounter".
// Called at the end of combat to clear temporary combat-only conditions.
// Precondition: s must not be nil.
// Postcondition: all encounter-duration conditions are removed; other conditions unchanged.
func (s *ActiveSet) ClearEncounter() {
	for id, ac := range s.conditions {
		if ac.Def.DurationType == "encounter" {
			delete(s.conditions, id)
		}
	}
}

// ClearAll removes every active condition regardless of duration type.
// Called on respawn to fully reset the condition state of a character.
//
// Precondition: s must not be nil.
// Postcondition: len(s.All()) == 0.
func (s *ActiveSet) ClearAll() {
	for id := range s.conditions {
		delete(s.conditions, id)
	}
}

// ApplyTagged is like Apply but attaches a source tag to the condition.
//
// Precondition: def must not be nil; source may be empty.
// Postcondition: Has(def.ID) is true; condition Source is set to source.
func (s *ActiveSet) ApplyTagged(uid string, def *ConditionDef, stacks, duration int, source string) error {
	if err := s.Apply(uid, def, stacks, duration); err != nil {
		return err
	}
	if ac, ok := s.conditions[def.ID]; ok {
		ac.Source = source
	}
	return nil
}

// ApplyTaggedWithExpiry applies a condition with a source tag and a real-time expiry.
// The condition is removed by TickCalendar when now >= expiresAt.
//
// Precondition: def must not be nil.
func (s *ActiveSet) ApplyTaggedWithExpiry(uid string, def *ConditionDef, stacks int, source string, expiresAt time.Time) error {
	if err := s.Apply(uid, def, stacks, -1); err != nil {
		return err
	}
	if ac, ok := s.conditions[def.ID]; ok {
		ac.Source = source
		ac.ExpiresAt = new(expiresAt)
	}
	return nil
}

// SourceOf returns the Source tag of the active condition with id, or "" if not present.
func (s *ActiveSet) SourceOf(id string) string {
	if ac, ok := s.conditions[id]; ok {
		return ac.Source
	}
	return ""
}

// RemoveBySource removes all conditions whose Source equals source.
//
// Postcondition: Has(id) is false for all conditions with matching source.
func (s *ActiveSet) RemoveBySource(uid, source string) {
	for id, ac := range s.conditions {
		if ac.Source == source {
			s.Remove(uid, id)
		}
	}
}

// TickCalendar removes all conditions with a non-nil ExpiresAt that is before or equal to now.
// Returns the IDs of removed conditions.
//
// Postcondition: all expired real-time conditions are removed.
func (s *ActiveSet) TickCalendar(uid string, now time.Time) []string {
	var expired []string
	for id, ac := range s.conditions {
		if ac.ExpiresAt != nil && !now.Before(*ac.ExpiresAt) {
			expired = append(expired, id)
			s.Remove(uid, id)
		}
	}
	return expired
}

// Active returns a snapshot of all active conditions.
//
// Postcondition: returned slice is a copy; mutations do not affect the set.
func (s *ActiveSet) Active(uid string) []ActiveCondition {
	result := make([]ActiveCondition, 0, len(s.conditions))
	for _, ac := range s.conditions {
		result = append(result, *ac)
	}
	return result
}

// CopyTo copies all conditions from s into dst, preserving Stacks, DurationRemaining,
// Source, and ExpiresAt. Existing conditions in dst with the same ID are overwritten.
//
// Precondition: s may be nil (no-op); dst may be nil (no-op); uid is the entity owning dst.
// Postcondition: dst contains all conditions from s with identical state.
func (s *ActiveSet) CopyTo(dst *ActiveSet, uid string) {
	if s == nil || dst == nil {
		return
	}
	for _, ac := range s.conditions {
		_ = dst.Apply(uid, ac.Def, ac.Stacks, ac.DurationRemaining)
		if entry, ok := dst.conditions[ac.Def.ID]; ok {
			entry.Source = ac.Source
			if ac.ExpiresAt != nil {
				t := *ac.ExpiresAt
				entry.ExpiresAt = &t
			}
		}
	}
}

// All returns a slice of pointers to the active conditions.
// The slice itself is a new allocation (mutating the slice does not affect the set),
// but the pointed-to ActiveCondition values are shared — callers must not modify them.
func (s *ActiveSet) All() []*ActiveCondition {
	out := make([]*ActiveCondition, 0, len(s.conditions))
	for _, ac := range s.conditions {
		out = append(out, ac)
	}
	return out
}
