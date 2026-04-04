package session

// HotbarSlot kind constants.
const (
	HotbarSlotKindCommand    = "command"
	HotbarSlotKindFeat       = "feat"
	HotbarSlotKindTechnology = "technology"
	HotbarSlotKindThrowable  = "throwable"
	HotbarSlotKindConsumable = "consumable"
)

// HotbarSlot is a typed hotbar entry. Kind identifies the slot type; Ref holds
// the command text (for "command") or the item/feat/tech ID (for all others).
//
// Invariant: IsEmpty() ⟺ Ref == "".
type HotbarSlot struct {
	Kind string // one of the HotbarSlotKind* constants
	Ref  string // command text or item/feat/tech ID
}

// ActivationCommand returns the game command executed when this slot fires.
// Returns "" when Ref is empty (slot is unassigned).
//
// Precondition: none.
// Postcondition: Returns a valid command string or "".
func (s HotbarSlot) ActivationCommand() string {
	if s.Ref == "" {
		return ""
	}
	switch s.Kind {
	case HotbarSlotKindFeat, HotbarSlotKindTechnology, HotbarSlotKindConsumable:
		return "use " + s.Ref
	case HotbarSlotKindThrowable:
		return "throw " + s.Ref
	default: // "command" or unset kind
		return s.Ref
	}
}

// IsEmpty returns true when the slot has no bound action.
func (s HotbarSlot) IsEmpty() bool {
	return s.Ref == ""
}

// CommandSlot creates a HotbarSlot of kind "command" with the given text.
func CommandSlot(text string) HotbarSlot {
	return HotbarSlot{Kind: HotbarSlotKindCommand, Ref: text}
}
