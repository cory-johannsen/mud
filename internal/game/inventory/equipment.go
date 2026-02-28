package inventory

// ArmorSlot identifies a body-armor equipment slot.
type ArmorSlot string

const (
	// SlotHead is the head armor slot.
	SlotHead ArmorSlot = "head"
	// SlotLeftArm is the left-arm armor slot.
	SlotLeftArm ArmorSlot = "left_arm"
	// SlotRightArm is the right-arm armor slot.
	SlotRightArm ArmorSlot = "right_arm"
	// SlotTorso is the torso armor slot.
	SlotTorso ArmorSlot = "torso"
	// SlotLeftLeg is the left-leg armor slot.
	SlotLeftLeg ArmorSlot = "left_leg"
	// SlotRightLeg is the right-leg armor slot.
	SlotRightLeg ArmorSlot = "right_leg"
	// SlotFeet is the feet armor slot.
	SlotFeet ArmorSlot = "feet"
)

// AccessorySlot identifies an accessory equipment slot.
type AccessorySlot string

const (
	// SlotNeck is the neck accessory slot.
	SlotNeck AccessorySlot = "neck"
	// SlotRing1 through SlotRing10 are the ring accessory slots.
	SlotRing1  AccessorySlot = "ring_1"
	SlotRing2  AccessorySlot = "ring_2"
	SlotRing3  AccessorySlot = "ring_3"
	SlotRing4  AccessorySlot = "ring_4"
	SlotRing5  AccessorySlot = "ring_5"
	SlotRing6  AccessorySlot = "ring_6"
	SlotRing7  AccessorySlot = "ring_7"
	SlotRing8  AccessorySlot = "ring_8"
	SlotRing9  AccessorySlot = "ring_9"
	SlotRing10 AccessorySlot = "ring_10"
)

// EquippedArmorItem records an equipped armor or accessory item.
// Full item definitions will be populated in feature #4 (weapon and armor library).
// For now only the item definition ID and display name are stored.
type EquippedArmorItem struct {
	// ItemDefID is the unique item definition identifier.
	ItemDefID string
	// Name is the display name shown to the player.
	Name string
}

// Equipment holds all armor and accessory slots for a character.
// These slots are shared across all weapon presets.
type Equipment struct {
	// Armor maps each ArmorSlot to the item equipped there, or nil when empty.
	Armor map[ArmorSlot]*EquippedArmorItem
	// Accessories maps each AccessorySlot to the item equipped there, or nil when empty.
	Accessories map[AccessorySlot]*EquippedArmorItem
}

// NewEquipment returns an empty Equipment with initialised maps.
//
// Postcondition: Armor and Accessories are non-nil, empty maps.
func NewEquipment() *Equipment {
	return &Equipment{
		Armor:       make(map[ArmorSlot]*EquippedArmorItem),
		Accessories: make(map[AccessorySlot]*EquippedArmorItem),
	}
}
