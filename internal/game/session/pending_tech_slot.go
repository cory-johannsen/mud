package session

// PendingTechSlot represents one trainer-required pending technology slot.
// It corresponds to a row in character_pending_tech_slots.
type PendingTechSlot struct {
	CharLevel int
	TechLevel int
	Tradition string
	UsageType string // "prepared" | "spontaneous"
	Remaining int
}
