package command

import (
	"fmt"
	"strings"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/skillcheck"
)

// AffixSession provides the player-session view needed by HandleAffix.
//
// Precondition: Session must not be nil.
type AffixSession struct {
	Session *session.PlayerSession
}

// AffixOutcome represents the four possible crafting check outcomes, plus an unspecified sentinel.
type AffixOutcome int

const (
	// AffixOutcomeUnspecified indicates the check was not attempted (precondition failure).
	AffixOutcomeUnspecified AffixOutcome = iota
	// AffixOutcomeCriticalFailure: check failed by 10 or more; material destroyed.
	AffixOutcomeCriticalFailure
	// AffixOutcomeFailure: check failed; material undamaged but not affixed.
	AffixOutcomeFailure
	// AffixOutcomeSuccess: check succeeded; material consumed and affixed.
	AffixOutcomeSuccess
	// AffixOutcomeCriticalSuccess: check succeeded by 10 or more; material returned intact.
	AffixOutcomeCriticalSuccess
)

// AffixResult carries the outcome of an affix operation for persistence routing.
type AffixResult struct {
	// Message is the player-facing result text.
	Message string
	// Outcome is the crafting check tier, or AffixOutcomeUnspecified if a precondition failed.
	Outcome AffixOutcome
	// MaterialConsumed is true when the material item was removed from the backpack.
	MaterialConsumed bool
	// MaterialReturned is true when a critical success returns the material to the backpack.
	MaterialReturned bool
	// TargetIsWeapon is true when the affixed target was a weapon (false = armor).
	TargetIsWeapon bool
}

// HandleAffix processes the "affix <material> <target>" command.
//
// Preconditions:
//   - as and reg must not be nil.
//   - materialQuery identifies a KindPreciousMaterial ItemDef ID (or name) in the backpack.
//   - targetQuery identifies an equipped weapon (by Def.ID or name) or armor (by ItemDefID or name).
//
// Postconditions:
//   - On AffixOutcomeCriticalSuccess: material entry appended to target, material remains in backpack.
//   - On AffixOutcomeSuccess: material entry appended to target, material removed from backpack.
//   - On AffixOutcomeFailure: no state change.
//   - On AffixOutcomeCriticalFailure: material removed from backpack, target unchanged.
//   - On AffixOutcomeUnspecified: no state change.
func HandleAffix(as *AffixSession, reg *inventory.Registry, materialQuery, targetQuery string, rng inventory.Roller) AffixResult {
	sess := as.Session

	// REQ-GA-6: cannot affix during combat.
	if sess.Status == 2 {
		return AffixResult{Message: "You cannot affix materials during combat."}
	}

	// Locate the material item in the backpack by def ID or name.
	matInst, matItem, matDef, found := findMaterialInBackpack(sess, reg, materialQuery)
	if !found {
		return AffixResult{Message: fmt.Sprintf("You don't have %s in your pack.", materialQuery)}
	}

	// Locate the equipped target.
	target := findRepairTarget(sess, targetQuery)
	if target == nil {
		return AffixResult{Message: fmt.Sprintf("%s is not equipped.", targetQuery)}
	}

	targetIsWeapon := target.weapon != nil
	var (
		targetAffixed      *[]string
		targetUpgradeSlots int
		targetName         string
		targetDurability   int
		targetMaxDurBonus  *int
	)

	if targetIsWeapon {
		targetAffixed = &target.weapon.AffixedMaterials
		targetUpgradeSlots = target.weapon.Def.UpgradeSlots
		targetName = target.weapon.Def.Name
		targetDurability = target.weapon.Durability
		targetMaxDurBonus = &target.weapon.MaterialMaxDurabilityBonus
	} else {
		armorDef, ok := reg.Armor(target.armorItem.ItemDefID)
		if !ok {
			return AffixResult{Message: fmt.Sprintf("%s is not equipped.", targetQuery)}
		}
		targetAffixed = &target.armorItem.AffixedMaterials
		targetUpgradeSlots = armorDef.UpgradeSlots
		targetName = target.armorItem.Name
		targetDurability = target.armorItem.Durability
		targetMaxDurBonus = &target.armorItem.MaterialMaxDurabilityBonus
	}

	// REQ-GA-7: target must not be broken.
	if targetDurability == 0 {
		return AffixResult{Message: "You cannot affix materials to broken equipment. Repair it first."}
	}

	// REQ-GA-8: validate AppliesTo compatibility.
	if targetIsWeapon {
		if !containsAppliesTo(matDef.AppliesTo, inventory.MaterialAppliesToWeapon) {
			return AffixResult{Message: fmt.Sprintf("%s cannot be affixed to weapons.", matDef.Name)}
		}
	} else {
		if !containsAppliesTo(matDef.AppliesTo, inventory.MaterialAppliesToArmor) {
			return AffixResult{Message: fmt.Sprintf("%s cannot be affixed to armor.", matDef.Name)}
		}
	}

	// REQ-GA-9: no duplicate material type on the same item.
	for _, entry := range *targetAffixed {
		parts := strings.SplitN(entry, ":", 2)
		if len(parts) == 2 && parts[0] == matDef.MaterialID {
			return AffixResult{Message: fmt.Sprintf("%s already has %s affixed.", targetName, matDef.Name)}
		}
	}

	// REQ-GA-10: must have at least one upgrade slot remaining.
	if len(*targetAffixed) >= targetUpgradeSlots {
		return AffixResult{Message: fmt.Sprintf("%s has no upgrade slots remaining.", targetName)}
	}

	// REQ-GA-9: perform the crafting check (PF2E rules: natural 20 upgrades outcome by one tier,
	// natural 1 downgrades outcome by one tier).
	dc := inventory.DCForMaterial(matDef.Tier, matDef.GradeID)
	roll := rng.RollD20()
	abilityMod := sess.Abilities.Modifier(sess.Abilities.Reasoning)
	craftingRank := ""
	if sess.Skills != nil {
		craftingRank = sess.Skills["crafting"]
	}
	checkResult := skillcheck.Resolve(roll, abilityMod, craftingRank, dc, skillcheck.TriggerDef{})
	// Apply PF2E natural 20 / natural 1 tier adjustments.
	outcome := adjustOutcomeForNaturalRoll(checkResult.Outcome, roll)

	affixEntry := matDef.MaterialID + ":" + matDef.GradeID
	displayName := matDef.Name + " (" + matDef.GradeName + ")"

	switch outcome {
	case skillcheck.CritSuccess:
		// Affix material; material returned intact (not consumed).
		*targetAffixed = append(*targetAffixed, affixEntry)
		applyAffixDirectBonuses(matDef, targetIsWeapon, targetMaxDurBonus)
		recomputeAffixPassiveMaterials(sess, reg)
		return AffixResult{
			Message:          fmt.Sprintf("Exceptional work. %s affixed to %s — material returned intact.", displayName, targetName),
			Outcome:          AffixOutcomeCriticalSuccess,
			MaterialReturned: true,
			TargetIsWeapon:   targetIsWeapon,
		}
	case skillcheck.Success:
		// Affix material; material consumed.
		*targetAffixed = append(*targetAffixed, affixEntry)
		applyAffixDirectBonuses(matDef, targetIsWeapon, targetMaxDurBonus)
		_ = sess.Backpack.Remove(matInst.InstanceID, 1)
		recomputeAffixPassiveMaterials(sess, reg)
		_ = matItem // suppress unused warning; matItem used for kind validation above
		return AffixResult{
			Message:          fmt.Sprintf("%s affixed to %s.", displayName, targetName),
			Outcome:          AffixOutcomeSuccess,
			MaterialConsumed: true,
			TargetIsWeapon:   targetIsWeapon,
		}
	case skillcheck.Failure:
		// Material undamaged; affix fails.
		return AffixResult{
			Message: "Your hands slip. The material is undamaged but the affix fails.",
			Outcome: AffixOutcomeFailure,
		}
	default: // skillcheck.CritFailure
		// Material destroyed.
		_ = sess.Backpack.Remove(matInst.InstanceID, 1)
		return AffixResult{
			Message:          fmt.Sprintf("You ruin the material. %s is destroyed.", displayName),
			Outcome:          AffixOutcomeCriticalFailure,
			MaterialConsumed: true,
			TargetIsWeapon:   targetIsWeapon,
		}
	}
}

// findMaterialInBackpack locates a KindPreciousMaterial item in the session backpack by def ID or name.
//
// Precondition: sess and reg must not be nil.
// Postcondition: returns (instance, itemDef, materialDef, true) when found; (zero, nil, nil, false) otherwise.
func findMaterialInBackpack(sess *session.PlayerSession, reg *inventory.Registry, query string) (inventory.ItemInstance, *inventory.ItemDef, *inventory.MaterialDef, bool) {
	// Search backpack instances for matching def ID first, then name.
	for _, inst := range sess.Backpack.Items() {
		item, ok := reg.Item(inst.ItemDefID)
		if !ok || item.Kind != inventory.KindPreciousMaterial {
			continue
		}
		if inst.ItemDefID == query || strings.EqualFold(item.Name, query) {
			matDef, ok := reg.Material(item.MaterialID, item.GradeID)
			if !ok {
				continue
			}
			return inst, item, matDef, true
		}
	}
	return inventory.ItemInstance{}, nil, nil, false
}

// adjustOutcomeForNaturalRoll applies PF2E natural 20 / natural 1 tier shifts.
// A natural 20 upgrades the outcome by one tier (max: CritSuccess).
// A natural 1 downgrades the outcome by one tier (min: CritFailure).
//
// Precondition: roll is in [1, 20].
// Postcondition: returns adjusted CheckOutcome.
func adjustOutcomeForNaturalRoll(base skillcheck.CheckOutcome, roll int) skillcheck.CheckOutcome {
	if roll == 20 {
		switch base {
		case skillcheck.CritFailure:
			return skillcheck.Failure
		case skillcheck.Failure:
			return skillcheck.Success
		case skillcheck.Success:
			return skillcheck.CritSuccess
		default:
			return skillcheck.CritSuccess
		}
	}
	if roll == 1 {
		switch base {
		case skillcheck.CritSuccess:
			return skillcheck.Success
		case skillcheck.Success:
			return skillcheck.Failure
		case skillcheck.Failure:
			return skillcheck.CritFailure
		default:
			return skillcheck.CritFailure
		}
	}
	return base
}

// containsAppliesTo reports whether the slice contains the given value.
func containsAppliesTo(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// applyAffixDirectBonuses applies material-specific stat bonuses at affix time.
// Currently handles carbide_alloy MaxDurability bonuses.
//
// Precondition: def and maxDurBonus must not be nil.
// Postcondition: *maxDurBonus is incremented by the grade-appropriate bonus for carbide_alloy weapons.
func applyAffixDirectBonuses(def *inventory.MaterialDef, targetIsWeapon bool, maxDurBonus *int) {
	if def.MaterialID == "carbide_alloy" && targetIsWeapon {
		switch def.GradeID {
		case "street_grade":
			*maxDurBonus += 2
		case "mil_spec_grade":
			*maxDurBonus += 4
		case "ghost_grade":
			*maxDurBonus += 6
		}
	}
}

// recomputeAffixPassiveMaterials refreshes the session's passive material summary after an affix.
//
// Precondition: sess and reg must not be nil.
// Postcondition: sess.PassiveMaterials reflects the current affixed materials on equipped items.
func recomputeAffixPassiveMaterials(sess *session.PlayerSession, reg *inventory.Registry) {
	if sess.LoadoutSet == nil || sess.Equipment == nil {
		return
	}
	active := sess.LoadoutSet.ActivePreset()
	if active == nil {
		return
	}
	equipped := []*inventory.EquippedWeapon{active.MainHand, active.OffHand}
	sess.PassiveMaterials = inventory.ComputePassiveMaterials(equipped, sess.Equipment.Armor, reg)
}
