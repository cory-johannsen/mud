package condition

// ActiveCondition tracks one applied condition on an entity.
type ActiveCondition struct {
	Def               *ConditionDef
	Stacks            int
	DurationRemaining int // -1 = permanent or until_save
}

// ActiveSet tracks all conditions currently applied to one combatant.
// It is not safe for concurrent use; the caller must serialise access.
type ActiveSet struct {
	conditions map[string]*ActiveCondition
}

// NewActiveSet creates an empty ActiveSet.
func NewActiveSet() *ActiveSet {
	return &ActiveSet{conditions: make(map[string]*ActiveCondition)}
}

// Apply adds or updates a condition on this entity.
// If the condition is already present, stacks are incremented (capped at MaxStacks).
// If MaxStacks == 0 (unstackable), stacks is always stored as 1.
// duration is rounds remaining; use -1 for permanent or until_save.
//
// Postcondition: Has(def.ID) is true after a successful Apply.
func (s *ActiveSet) Apply(def *ConditionDef, stacks, duration int) error {
	// Determine effective stacks
	effectiveStacks := stacks
	if def.MaxStacks == 0 {
		effectiveStacks = 1
	}

	if existing, ok := s.conditions[def.ID]; ok {
		// Increment existing stacks
		newStacks := existing.Stacks + effectiveStacks
		if def.MaxStacks > 0 && newStacks > def.MaxStacks {
			newStacks = def.MaxStacks
		}
		if def.MaxStacks == 0 {
			newStacks = 1
		}
		existing.Stacks = newStacks
		if duration > existing.DurationRemaining {
			existing.DurationRemaining = duration
		}
		return nil
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
	return nil
}

// Remove deletes the condition with the given ID from the set.
// If the condition is not present, Remove is a no-op.
//
// Postcondition: Has(id) is false.
func (s *ActiveSet) Remove(id string) {
	delete(s.conditions, id)
}

// Tick decrements the DurationRemaining of all "rounds"-type conditions by 1.
// Conditions that reach 0 are removed. "permanent" and "until_save" conditions
// (DurationRemaining == -1) are not affected.
//
// Postcondition: Returns the IDs of conditions that expired this tick.
func (s *ActiveSet) Tick() []string {
	var expired []string
	for id, ac := range s.conditions {
		if ac.Def.DurationType != "rounds" || ac.DurationRemaining < 0 {
			continue
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

// All returns a snapshot slice of all active conditions.
func (s *ActiveSet) All() []*ActiveCondition {
	out := make([]*ActiveCondition, 0, len(s.conditions))
	for _, ac := range s.conditions {
		out = append(out, ac)
	}
	return out
}
