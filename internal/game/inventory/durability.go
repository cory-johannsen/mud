package inventory

// DeductResult is the outcome of a single DeductDurability call.
type DeductResult struct {
	// NewDurability is the durability value after the deduction.
	NewDurability int
	// BecameBroken is true if durability just reached 0 in this call.
	BecameBroken bool
	// Destroyed is true if BecameBroken is true AND the destruction roll succeeded.
	// The caller MUST remove the item from inventory/equipment permanently when this is true.
	Destroyed bool
}

// DeductDurability reduces inst's durability by 1 and returns the outcome.
//
// Preconditions:
//   - inst must be non-nil.
//   - rng must be non-nil.
//   - inst.MaxDurability must be set (not -1 sentinel) before calling.
//
// Postconditions:
//   - If inst.Durability was already 0, returns zero-value DeductResult (no-op).
//   - If new durability > 0: BecameBroken=false, Destroyed=false.
//   - If new durability == 0 and destruction roll < DestructionChance: Destroyed=true.
//   - If new durability == 0 and destruction roll >= DestructionChance: Destroyed=false.
//   - Persistence (DB writes) is the responsibility of the caller.
func DeductDurability(inst *ItemInstance, rng Roller) DeductResult {
	if inst.Durability <= 0 {
		return DeductResult{NewDurability: inst.Durability}
	}
	inst.Durability--
	if inst.Durability > 0 {
		return DeductResult{NewDurability: inst.Durability}
	}
	// Durability just hit 0 — make a destruction roll.
	// Look up the destruction chance directly from the rarity registry.
	destructionChance := 0.0
	if def, ok := LookupRarity(inst.Rarity); ok {
		destructionChance = def.DestructionChance
	}
	roll := rng.RollFloat()
	destroyed := roll < destructionChance
	return DeductResult{
		NewDurability: 0,
		BecameBroken:  true,
		Destroyed:     destroyed,
	}
}

// RepairField restores 1d6 durability points (capped at MaxDurability) using a repair kit.
// Returns the number of durability points actually restored.
//
// Preconditions:
//   - inst must be non-nil.
//   - rng must be non-nil.
//   - The caller MUST have already consumed a repair_kit item before invoking this function.
//
// Postconditions:
//   - inst.Durability is increased by min(1d6, MaxDurability-Durability).
//   - Returns the actual points restored (>= 0).
func RepairField(inst *ItemInstance, rng Roller) int {
	roll := rng.Roll("1d6")
	space := inst.MaxDurability - inst.Durability
	if roll > space {
		roll = space
	}
	inst.Durability += roll
	return roll
}

// RepairFull restores the item to its MaxDurability.
//
// Preconditions:
//   - inst must be non-nil.
//
// Postconditions:
//   - inst.Durability == inst.MaxDurability.
func RepairFull(inst *ItemInstance) {
	inst.Durability = inst.MaxDurability
}

// InitDurability sets Durability = MaxDurability for the item's rarity if the
// sentinel value -1 is detected. This handles legacy rows loaded from the database.
//
// Preconditions:
//   - inst must be non-nil.
//   - rarity must be a valid rarity ID or empty string.
//
// Postconditions:
//   - If inst.Durability != -1, no change is made.
//   - If inst.Durability == -1, inst.MaxDurability and inst.Durability are set
//     to the rarity's MaxDurability constant (0 if the rarity is unknown).
func InitDurability(inst *ItemInstance, rarity string) {
	if inst.Durability != -1 {
		return
	}
	def, ok := LookupRarity(rarity)
	maxDur := 0
	if ok {
		maxDur = def.MaxDurability
	}
	inst.MaxDurability = maxDur
	inst.Durability = maxDur
}
