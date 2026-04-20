package ai

import "fmt"

// ItemDomainRegistry maintains a mapping of domain ID → ItemPlanner for AI item domains.
// This registry is separate from the NPC AI Registry to keep item and NPC domains isolated.
// Satisfies REQ-AIE-7.
type ItemDomainRegistry struct {
	planners map[string]*ItemPlanner
}

// NewItemDomainRegistry creates an empty ItemDomainRegistry.
//
// Postcondition: PlannerFor returns (nil, false) for any domain ID.
func NewItemDomainRegistry() *ItemDomainRegistry {
	return &ItemDomainRegistry{planners: make(map[string]*ItemPlanner)}
}

// Register adds a domain to the registry, creating an ItemPlanner for it.
//
// Precondition: domain must not be nil and must have a unique non-empty ID.
// Postcondition: PlannerFor(domain.ID) returns the new planner.
func (r *ItemDomainRegistry) Register(domain *Domain) error {
	if domain == nil {
		return fmt.Errorf("ai.ItemDomainRegistry.Register: domain must not be nil")
	}
	if _, exists := r.planners[domain.ID]; exists {
		return fmt.Errorf("ai.ItemDomainRegistry.Register: domain %q already registered", domain.ID)
	}
	r.planners[domain.ID] = NewItemPlanner(domain)
	return nil
}

// PlannerFor retrieves the ItemPlanner for the given domain ID.
//
// Postcondition: Returns (planner, true) if registered, (nil, false) otherwise.
func (r *ItemDomainRegistry) PlannerFor(domainID string) (*ItemPlanner, bool) {
	p, ok := r.planners[domainID]
	return p, ok
}
