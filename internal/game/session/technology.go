package session

// PreparedSlot holds one prepared technology slot.
type PreparedSlot struct {
	TechID   string
	Expended bool
}

// InnateSlot tracks an innate technology granted by a region or archetype.
// MaxUses == 0 means unlimited.
type InnateSlot struct {
	MaxUses       int
	UsesRemaining int
}

// UsePool tracks remaining and maximum daily uses for a spontaneous tech level.
type UsePool struct {
	Remaining int
	Max       int
}
