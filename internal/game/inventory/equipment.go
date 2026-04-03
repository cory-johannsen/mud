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
	// InstanceID links this slot to the corresponding ItemInstance (UUID).
	InstanceID string
	// Durability is a cached copy; the ItemInstance is the source of truth.
	Durability int
	// Modifier is the item modifier: "" | "tuned" | "defective" | "cursed".
	Modifier string
	// CurseRevealed is true once the cursed item has been equipped.
	CurseRevealed bool
	// Rarity is the item rarity tier: "salvage" | "street" | "mil_spec" | "black_market" | "ghost".
	// Set at wear time from the ArmorDef.Rarity for color display (REQ-EM-4).
	Rarity string
	// AffixedMaterials is a cached copy from DB; set by armor wear path.
	AffixedMaterials          []string // each entry: "<material_id>:<grade_id>"
	// MaterialMaxDurabilityBonus is a cached copy; effective max = base MaxDurability + this
	MaterialMaxDurabilityBonus int
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
	// Resistances maps damage type → effective flat reduction (highest single-source value per type).
	Resistances map[string]int
	// Weaknesses maps damage type → total flat addition (sum across all sources).
	Weaknesses map[string]int
}

// ComputedDefenses aggregates PF2e-style defense stats from all currently equipped armor slots.
//
// Precondition: reg must be non-nil; dexMod may be any integer.
// Postcondition: ACBonus equals the sum of all equipped slot ac_bonus values;
// EffectiveDex <= dexMod and <= any single slot's DexCap.
func (e *Equipment) ComputedDefenses(reg *Registry, dexMod int) DefenseStats {
	stats := DefenseStats{
		EffectiveDex: dexMod,
		Resistances:  make(map[string]int),
		Weaknesses:   make(map[string]int),
	}
	hasDexCap := false
	for _, slotted := range e.Armor {
		if slotted == nil {
			continue
		}
		// REQ-EM-7/8: skip broken slots (durability==0 when durability is tracked via InstanceID).
		if slotted.InstanceID != "" && slotted.Durability == 0 {
			continue
		}
		def, ok := reg.Armor(slotted.ItemDefID)
		if !ok {
			continue
		}
		slotAC := def.ACBonus
		// REQ-EM-22: apply Modifier AC adjustment.
		switch slotted.Modifier {
		case "tuned":
			slotAC++
		case "defective":
			slotAC--
		case "cursed":
			slotAC -= 2
		}
		stats.ACBonus += slotAC
		stats.CheckPenalty += def.CheckPenalty
		stats.SpeedPenalty += def.SpeedPenalty
		if def.StrengthReq > stats.StrengthReq {
			stats.StrengthReq = def.StrengthReq
		}
		if !hasDexCap || def.DexCap < stats.EffectiveDex {
			stats.EffectiveDex = def.DexCap
			hasDexCap = true
		}
		for dmgType, val := range def.Resistances {
			if val > stats.Resistances[dmgType] {
				stats.Resistances[dmgType] = val
			}
		}
		for dmgType, val := range def.Weaknesses {
			stats.Weaknesses[dmgType] += val
		}
	}
	if stats.EffectiveDex > dexMod {
		stats.EffectiveDex = dexMod
	}
	return stats
}

// ComputedDefensesWithSetBonuses computes defense stats and applies the active
// set bonus ACBonus on top (REQ-EM-35).
//
// Precondition: reg must be non-nil; dexMod may be any integer.
// Postcondition: ACBonus includes both equipped-armor contribution and sb.ACBonus.
func (e *Equipment) ComputedDefensesWithSetBonuses(reg *Registry, dexMod int, sb SetBonusSummary) DefenseStats {
	stats := e.ComputedDefenses(reg, dexMod)
	stats.ACBonus += sb.ACBonus
	return stats
}

// armorProfBonus returns the PF2E proficiency bonus for the given rank at the given level.
func armorProfBonus(level int, rank string) int {
	switch rank {
	case "trained":
		return level + 2
	case "expert":
		return level + 4
	case "master":
		return level + 6
	case "legendary":
		return level + 8
	}
	return 0
}

// ComputedDefensesWithProficiencies aggregates defense stats, applying armor proficiency rules:
// - For armor in a category the player is trained in: add proficiency bonus to ACBonus; skip check/speed penalties.
// - For armor in an untrained category: apply check/speed penalties; no proficiency bonus.
//
// Precondition: reg must be non-nil; profs may be nil (treated as all untrained).
// Postcondition: ACBonus includes per-slot proficiency bonus for trained categories.
func (e *Equipment) ComputedDefensesWithProficiencies(reg *Registry, dexMod int, profs map[string]string, level int) DefenseStats {
	stats := DefenseStats{
		EffectiveDex: dexMod,
		Resistances:  make(map[string]int),
		Weaknesses:   make(map[string]int),
	}
	hasDexCap := false
	for _, slotted := range e.Armor {
		if slotted == nil {
			continue
		}
		if slotted.InstanceID != "" && slotted.Durability == 0 {
			continue
		}
		def, ok := reg.Armor(slotted.ItemDefID)
		if !ok {
			continue
		}
		slotAC := def.ACBonus
		switch slotted.Modifier {
		case "tuned":
			slotAC++
		case "defective":
			slotAC--
		case "cursed":
			slotAC -= 2
		}
		stats.ACBonus += slotAC

		// Apply proficiency-based rules.
		rank := ""
		if profs != nil {
			rank = profs[def.ProficiencyCategory]
		}
		if rank != "" {
			// Trained: proficiency bonus, no penalties.
			stats.ACBonus += armorProfBonus(level, rank)
		} else {
			// Untrained: apply check and speed penalties.
			stats.CheckPenalty += def.CheckPenalty
			stats.SpeedPenalty += def.SpeedPenalty
		}

		if def.StrengthReq > stats.StrengthReq {
			stats.StrengthReq = def.StrengthReq
		}
		if !hasDexCap || def.DexCap < stats.EffectiveDex {
			stats.EffectiveDex = def.DexCap
			hasDexCap = true
		}
		for dmgType, val := range def.Resistances {
			if val > stats.Resistances[dmgType] {
				stats.Resistances[dmgType] = val
			}
		}
		for dmgType, val := range def.Weaknesses {
			stats.Weaknesses[dmgType] += val
		}
	}
	if stats.EffectiveDex > dexMod {
		stats.EffectiveDex = dexMod
	}
	return stats
}

// ComputedDefensesWithProficienciesAndSetBonuses computes proficiency-aware defense stats
// and applies the active set bonus ACBonus on top (REQ-EM-35).
//
// Precondition: reg must be non-nil; profs may be nil (treated as all untrained).
// Postcondition: ACBonus includes per-slot proficiency bonus for trained categories and sb.ACBonus.
func (e *Equipment) ComputedDefensesWithProficienciesAndSetBonuses(reg *Registry, dexMod int, profs map[string]string, level int, sb SetBonusSummary) DefenseStats {
	stats := e.ComputedDefensesWithProficiencies(reg, dexMod, profs, level)
	stats.ACBonus += sb.ACBonus
	return stats
}
