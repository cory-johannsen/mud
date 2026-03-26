package quest

import (
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// NewServiceProvider creates a Service for wire injection.
//
// Precondition: registry and repo must be non-nil.
// Postcondition: returns a non-nil *Service.
func NewServiceProvider(
	registry QuestRegistry,
	repo QuestRepository,
	xpSvc XPAwarder,
	invRegistry *inventory.Registry,
	charSaver InventorySaver,
) *Service {
	return NewService(registry, repo, xpSvc, invRegistry, charSaver)
}
