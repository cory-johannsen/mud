package ai

import "fmt"

// Registry indexes Planners by domain ID.
//
// Invariant: each domain ID is registered at most once.
type Registry struct {
	planners map[string]*Planner
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{planners: make(map[string]*Planner)}
}

// Register creates and stores a Planner for domain.
//
// Precondition: domain and caller must not be nil.
// Postcondition: returns error on domain ID collision.
func (r *Registry) Register(domain *Domain, caller ScriptCaller, zoneID string) error {
	if _, exists := r.planners[domain.ID]; exists {
		return fmt.Errorf("ai.Registry: domain %q already registered", domain.ID)
	}
	r.planners[domain.ID] = NewPlanner(domain, caller, zoneID)
	return nil
}

// PlannerFor returns the Planner for domainID, or false if not registered.
func (r *Registry) PlannerFor(domainID string) (*Planner, bool) {
	p, ok := r.planners[domainID]
	return p, ok
}
