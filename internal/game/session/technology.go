package session

// PreparedSlot holds one prepared technology slot.
type PreparedSlot struct {
	TechID   string
	Expended bool
}

// InnateSlot tracks an innate technology granted by an archetype.
// MaxUses == 0 means unlimited.
type InnateSlot struct {
	MaxUses int
}
