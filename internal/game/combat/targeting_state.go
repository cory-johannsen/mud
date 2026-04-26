package combat

import "sync"

// CombatTargeting holds the per-player sticky-target selection used by
// gameserver enqueue-time targeting (#249). The selection persists across
// turns within a single combat and is cleared on combat end / target death /
// explicit clear.
//
// CombatTargeting lives in the combat package (not gameserver) so that
// session.PlayerSession can hold it without inducing a session→gameserver
// import cycle. Higher layers wire its lifecycle into combat-start /
// target-death / combat-end events.
//
// CombatTargeting is concurrency-safe.
type CombatTargeting struct {
	mu       sync.Mutex
	targetID string
}

// NewCombatTargeting constructs an empty CombatTargeting.
func NewCombatTargeting() *CombatTargeting {
	return &CombatTargeting{}
}

// TargetID returns the currently selected target UID, or "" when none.
func (s *CombatTargeting) TargetID() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.targetID
}

// Set updates the sticky target to uid. Empty string is treated as Clear.
func (s *CombatTargeting) Set(uid string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.targetID = uid
	s.mu.Unlock()
}

// Clear drops the current sticky target.
func (s *CombatTargeting) Clear() {
	s.Set("")
}

// OnCombatStart re-evaluates the sticky target at the moment combat begins.
//
// If actor has exactly one living enemy in the combat, that enemy becomes the
// sticky target. Otherwise the existing selection is left untouched (callers
// may invoke Clear first if they want to reset).
//
// Precondition: cbt and actor MUST be non-nil.
func (s *CombatTargeting) OnCombatStart(cbt *Combat, actor *Combatant) {
	if s == nil || cbt == nil || actor == nil {
		return
	}
	s.autoSelect(cbt, actor)
}

// OnTargetDeath is invoked when any combatant dies. If deadID matches the
// current sticky target the selection is cleared and re-evaluated against
// the post-death combatant list.
func (s *CombatTargeting) OnTargetDeath(cbt *Combat, actor *Combatant, deadID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.targetID != deadID {
		s.mu.Unlock()
		return
	}
	s.targetID = ""
	s.mu.Unlock()
	if cbt == nil || actor == nil {
		return
	}
	s.autoSelect(cbt, actor)
}

// autoSelect applies the single-living-enemy auto-target rule.
//
// When exactly one living enemy remains in cbt, it becomes the sticky target.
// When the existing selection is already a living enemy, it is preserved.
// Otherwise the selection is left empty (caller must explicitly choose).
func (s *CombatTargeting) autoSelect(cbt *Combat, actor *Combatant) {
	enemies := livingEnemies(cbt, actor)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.targetID != "" {
		for _, e := range enemies {
			if e.ID == s.targetID {
				return
			}
		}
		s.targetID = ""
	}
	if len(enemies) == 1 {
		s.targetID = enemies[0].ID
	}
}

// ResolveAndValidate selects the target UID to use for an action and runs
// ValidateSingleTarget against it.
//
// Selection rule: when inlineUID is non-empty, it is used (the player typed
// an explicit override). Otherwise the sticky selection is used.
//
// Precondition: cbt and actor MUST be non-nil.
// Postcondition: returns (resolvedUID, validation result). On success the
// sticky selection is updated to the resolved UID so the next call without
// an inline override reuses it.
func (s *CombatTargeting) ResolveAndValidate(cbt *Combat, actor *Combatant, category TargetCategory, maxRangeFt int, inlineUID string) (string, TargetingResult) {
	uid := inlineUID
	if uid == "" {
		uid = s.TargetID()
	}
	res := ValidateSingleTarget(cbt, actor, uid, category, maxRangeFt, false)
	if res.OK() && uid != "" {
		s.Set(uid)
	}
	return uid, res
}

// livingEnemies returns the living combatants in cbt that are NOT allies of
// actor.
func livingEnemies(cbt *Combat, actor *Combatant) []*Combatant {
	var out []*Combatant
	if cbt == nil || actor == nil {
		return out
	}
	for _, c := range cbt.Combatants {
		if c == nil || c.ID == actor.ID || c.IsDead() {
			continue
		}
		if areAllies(actor, c) {
			continue
		}
		out = append(out, c)
	}
	return out
}
