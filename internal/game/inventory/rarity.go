package inventory

// ModifierProbs holds the probability breakdown for modifier assignment at item spawn.
type ModifierProbs struct {
	// Tuned is the probability [0,1) of spawning with a "tuned" modifier.
	Tuned float64
	// Defective is the probability [0,1) of spawning with a "defective" modifier.
	Defective float64
	// Cursed is the probability [0,1) of spawning with a "cursed" modifier.
	Cursed float64
}

// RarityDef defines immutable game constants for a single rarity tier.
type RarityDef struct {
	// ID is the rarity tier identifier (e.g. "salvage", "street").
	ID string
	// StatMultiplier is the factor applied to base weapon/armor stats at load time.
	StatMultiplier float64
	// FeatureSlots is the number of feature slots available on items of this rarity.
	FeatureSlots int
	// FeatureEffectiveness is the multiplier applied to feature effects for this rarity.
	FeatureEffectiveness float64
	// MinLevel is the minimum character level required to equip items of this rarity.
	MinLevel int
	// MaxDurability is the maximum durability for items of this rarity.
	MaxDurability int
	// DestructionChance is the probability [0,1) of permanent destruction when durability reaches 0.
	DestructionChance float64
	// ModifierProbs holds the spawn probabilities for modifier types.
	ModifierProbs ModifierProbs
}

// rarityRegistry is the immutable set of all rarity tier definitions.
// These are game constants and are never loaded from YAML.
var rarityRegistry = map[string]RarityDef{
	"salvage": {
		ID:                   "salvage",
		StatMultiplier:       1.0,
		FeatureSlots:         0,
		FeatureEffectiveness: 1.00,
		MinLevel:             0,
		MaxDurability:        20,
		DestructionChance:    0.50,
		ModifierProbs:        ModifierProbs{Tuned: 0.00, Defective: 0.30, Cursed: 0.10},
	},
	"street": {
		ID:                   "street",
		StatMultiplier:       1.2,
		FeatureSlots:         1,
		FeatureEffectiveness: 1.10,
		MinLevel:             1,
		MaxDurability:        40,
		DestructionChance:    0.30,
		ModifierProbs:        ModifierProbs{Tuned: 0.05, Defective: 0.15, Cursed: 0.05},
	},
	"mil_spec": {
		ID:                   "mil_spec",
		StatMultiplier:       1.5,
		FeatureSlots:         2,
		FeatureEffectiveness: 1.25,
		MinLevel:             5,
		MaxDurability:        60,
		DestructionChance:    0.15,
		ModifierProbs:        ModifierProbs{Tuned: 0.10, Defective: 0.10, Cursed: 0.03},
	},
	"black_market": {
		ID:                   "black_market",
		StatMultiplier:       1.8,
		FeatureSlots:         3,
		FeatureEffectiveness: 1.40,
		MinLevel:             10,
		MaxDurability:        80,
		DestructionChance:    0.05,
		ModifierProbs:        ModifierProbs{Tuned: 0.20, Defective: 0.05, Cursed: 0.02},
	},
	"ghost": {
		ID:                   "ghost",
		StatMultiplier:       2.2,
		FeatureSlots:         4,
		FeatureEffectiveness: 1.60,
		MinLevel:             15,
		MaxDurability:        100,
		DestructionChance:    0.01,
		ModifierProbs:        ModifierProbs{Tuned: 0.30, Defective: 0.02, Cursed: 0.01},
	},
}

// LookupRarity returns the RarityDef for the given rarity ID.
//
// Precondition: id is a non-empty string.
// Postcondition: returns (RarityDef, true) if id is a known rarity tier, (RarityDef{}, false) otherwise.
func LookupRarity(id string) (RarityDef, bool) {
	def, ok := rarityRegistry[id]
	return def, ok
}

// RollModifier determines the modifier for a newly spawned item given a rarity def and a
// random float in [0.0, 1.0). The roll is compared against modifier probability thresholds
// in order: tuned, defective, cursed. Anything above all thresholds is normal ("").
//
// Precondition: roll is in [0.0, 1.0); def is a valid RarityDef.
// Postcondition: returns one of "", "tuned", "defective", "cursed".
func RollModifier(def RarityDef, roll float64) string {
	p := def.ModifierProbs
	if roll < p.Tuned {
		return "tuned"
	}
	if roll < p.Tuned+p.Defective {
		return "defective"
	}
	if roll < p.Tuned+p.Defective+p.Cursed {
		return "cursed"
	}
	return ""
}

// rarityColorCodes maps each rarity ID to its ANSI escape color code.
// Colors per spec: salvage=gray, street=white, mil_spec=green, black_market=purple, ghost=gold.
var rarityColorCodes = map[string]string{
	"salvage":      "\033[90m", // dark gray
	"street":       "\033[97m", // bright white
	"mil_spec":     "\033[32m", // green
	"black_market": "\033[35m", // purple/magenta
	"ghost":        "\033[33m", // gold/yellow
}

// AnsiReset is the ANSI escape sequence to reset text formatting.
const AnsiReset = "\033[0m"

// RarityColorCode returns the ANSI color escape sequence for the given rarity ID.
//
// Postcondition: returns the registered code, or AnsiReset if the rarity is unknown.
func RarityColorCode(rarity string) string {
	if code, ok := rarityColorCodes[rarity]; ok {
		return code
	}
	return AnsiReset
}

// RarityColoredName returns itemName wrapped with the ANSI color for the rarity tier,
// reset to default at the end. For use in inventory and equipment display (REQ-EM-4).
//
// Postcondition: returns a non-empty string containing itemName.
func RarityColoredName(rarity, itemName string) string {
	return RarityColorCode(rarity) + itemName + AnsiReset
}
