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
	// SlotHands is the hands armor slot (covers both hands).
	SlotHands ArmorSlot = "hands"
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
	// SlotLeftRing1 through SlotLeftRing5 are the left-hand ring slots.
	SlotLeftRing1 AccessorySlot = "left_ring_1"
	SlotLeftRing2 AccessorySlot = "left_ring_2"
	SlotLeftRing3 AccessorySlot = "left_ring_3"
	SlotLeftRing4 AccessorySlot = "left_ring_4"
	SlotLeftRing5 AccessorySlot = "left_ring_5"
	// SlotRightRing1 through SlotRightRing5 are the right-hand ring slots.
	SlotRightRing1 AccessorySlot = "right_ring_1"
	SlotRightRing2 AccessorySlot = "right_ring_2"
	SlotRightRing3 AccessorySlot = "right_ring_3"
	SlotRightRing4 AccessorySlot = "right_ring_4"
	SlotRightRing5 AccessorySlot = "right_ring_5"
)

// slotDisplayNames maps every slot identifier to its human-readable label.
var slotDisplayNames = map[string]string{
	"head":         "Head",
	"left_arm":     "Left Arm",
	"right_arm":    "Right Arm",
	"torso":        "Torso",
	"hands":        "Hands",
	"left_leg":     "Left Leg",
	"right_leg":    "Right Leg",
	"feet":         "Feet",
	"neck":         "Neck",
	"left_ring_1":  "Left Hand Ring 1",
	"left_ring_2":  "Left Hand Ring 2",
	"left_ring_3":  "Left Hand Ring 3",
	"left_ring_4":  "Left Hand Ring 4",
	"left_ring_5":  "Left Hand Ring 5",
	"right_ring_1": "Right Hand Ring 1",
	"right_ring_2": "Right Hand Ring 2",
	"right_ring_3": "Right Hand Ring 3",
	"right_ring_4": "Right Hand Ring 4",
	"right_ring_5": "Right Hand Ring 5",
	"main":         "Main Hand",
	"off":          "Off Hand",
}

// SlotDisplayName returns the human-readable label for a slot identifier.
//
// Precondition: slot is a non-empty string.
// Postcondition: returns the registered label, or slot itself if not found.
func SlotDisplayName(slot string) string {
	if label, ok := slotDisplayNames[slot]; ok {
		return label
	}
	return slot
}

// SlottedItem records an item occupying any equipment slot (armor or accessory).
type SlottedItem struct {
	// ItemDefID is the unique item definition identifier.
	ItemDefID string
	// Name is the display name shown to the player.
	Name string
}

// Equipment holds all armor and accessory slots for a character.
// These slots are shared across all weapon presets.
type Equipment struct {
	// Armor maps each ArmorSlot to the item equipped there, or nil when empty.
	Armor map[ArmorSlot]*SlottedItem
	// Accessories maps each AccessorySlot to the item equipped there, or nil when empty.
	Accessories map[AccessorySlot]*SlottedItem
}

// NewEquipment returns an empty Equipment with initialised maps.
//
// Postcondition: Armor and Accessories are non-nil, empty maps.
func NewEquipment() *Equipment {
	return &Equipment{
		Armor:       make(map[ArmorSlot]*SlottedItem),
		Accessories: make(map[AccessorySlot]*SlottedItem),
	}
}

// DefenseStats holds aggregated defensive statistics computed from all equipped armor slots.
type DefenseStats struct {
	ACBonus      int // sum of all equipped slot ac_bonus values
	EffectiveDex int // min(dexMod, strictest DexCap) across equipped slots
	CheckPenalty int // sum of all slot check_penalty values (non-positive)
	SpeedPenalty int // sum of speed_penalty values
	StrengthReq  int // max strength_req across all equipped slots
}

// ComputedDefenses aggregates PF2e-style defense stats from all currently equipped armor slots.
//
// Precondition: reg must be non-nil; dexMod may be any integer.
// Postcondition: ACBonus equals the sum of all equipped slot ac_bonus values;
// EffectiveDex <= dexMod and <= any single slot's DexCap.
func (e *Equipment) ComputedDefenses(reg *Registry, dexMod int) DefenseStats {
	stats := DefenseStats{EffectiveDex: dexMod}
	hasDexCap := false
	for _, slotted := range e.Armor {
		if slotted == nil {
			continue
		}
		def, ok := reg.Armor(slotted.ItemDefID)
		if !ok {
			continue
		}
		stats.ACBonus += def.ACBonus
		stats.CheckPenalty += def.CheckPenalty
		stats.SpeedPenalty += def.SpeedPenalty
		if def.StrengthReq > stats.StrengthReq {
			stats.StrengthReq = def.StrengthReq
		}
		if !hasDexCap || def.DexCap < stats.EffectiveDex {
			stats.EffectiveDex = def.DexCap
			hasDexCap = true
		}
	}
	if stats.EffectiveDex > dexMod {
		stats.EffectiveDex = dexMod
	}
	return stats
}
