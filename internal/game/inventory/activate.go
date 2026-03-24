package inventory

import "strings"

// ActivateSession provides the equipped item view needed by HandleActivate
// and satisfies ConsumableTarget so the caller can pass it to ApplyConsumable.
//
// Precondition: all methods return valid, non-nil values.
type ActivateSession interface {
	ConsumableTarget // GetTeam, GetStatModifier, ApplyHeal, ApplyCondition, RemoveCondition, ApplyDisease, ApplyToxin

	// EquippedInstances returns all ItemInstances currently equipped.
	// The implementation MUST resolve instances from:
	//  1. WeaponPreset.MainHand.InstanceID → backpack lookup
	//  2. WeaponPreset.OffHand.InstanceID → backpack lookup
	//  3. Equipment.Armor[slot].InstanceID → backpack lookup (all non-nil slots)
	//  4. Equipment.Accessories[slot].InstanceID → backpack lookup (all non-nil slots)
	// Items whose InstanceID is not found in the backpack are silently skipped.
	EquippedInstances() []*ItemInstance
}

// ActivateResult is the outcome of a successful HandleActivate call.
type ActivateResult struct {
	AP               int               // AP cost consumed (equals ItemDef.ActivationCost)
	ItemDefID        string            // ID of the activated item's def
	ActivationEffect *ConsumableEffect // non-nil if consumable-style effect should be applied by caller
	Script           string            // non-empty if Lua hook should be invoked by caller
	Destroyed        bool              // true if item was destroyed on depletion
}

// HandleActivate resolves query across all equipped item instances, validates charge
// and AP state, decrements ChargesRemaining, and marks the item Expended or Destroyed.
// Does NOT apply effects or persist state — the caller is responsible for both.
//
// Precondition: sess, reg non-nil; query non-empty; currentAP >= 0.
// Postcondition: On success, ChargesRemaining is decremented in the matched
// ItemInstance and ActivateResult is returned with errMsg == "".
// On failure, ItemInstance is unchanged and errMsg is non-empty.
func HandleActivate(sess ActivateSession, reg *Registry, query string, inCombat bool, currentAP int) (result ActivateResult, errMsg string) {
	// Resolve the equipped instance matching query.
	var matched *ItemInstance
	for _, inst := range sess.EquippedInstances() {
		def, ok := reg.Item(inst.ItemDefID)
		if !ok {
			continue
		}
		if strings.EqualFold(def.ID, query) || strings.EqualFold(def.Name, query) {
			matched = inst
			break
		}
	}
	if matched == nil {
		return result, "No activatable item matching \"" + query + "\" found in your equipped gear."
	}

	def, ok := reg.Item(matched.ItemDefID)
	if !ok || def.ActivationCost == 0 {
		return result, "That item cannot be activated."
	}

	// REQ-ACT-13: initialize sentinel.
	if matched.ChargesRemaining == -1 {
		matched.ChargesRemaining = def.Charges
	}

	// REQ-ACT-3: block expended items.
	if matched.Expended {
		return result, "That item is expended and has no charges remaining."
	}
	if matched.ChargesRemaining == 0 {
		return result, "That item has no charges remaining."
	}

	// REQ-ACT-4: in combat, check AP.
	if inCombat && currentAP < def.ActivationCost {
		return result, "Not enough AP to activate that item."
	}

	// Decrement charge.
	matched.ChargesRemaining--

	// Determine depletion behavior.
	var destroyed bool
	if matched.ChargesRemaining == 0 {
		// REQ-ACT-12: when recharge entries exist, always use expend semantics.
		useExpend := len(def.Recharge) > 0 || def.OnDeplete != "destroy"
		if useExpend {
			matched.Expended = true
		} else {
			destroyed = true
		}
	}

	return ActivateResult{
		AP:               def.ActivationCost,
		ItemDefID:        def.ID,
		ActivationEffect: def.ActivationEffect,
		Script:           def.ActivationScript,
		Destroyed:        destroyed,
	}, ""
}

// TickRecharge restores charges to all item instances whose ItemDef has a recharge
// entry matching trigger. Charges are capped at ItemDef.Charges. Expended is
// cleared when charges become > 0 after recharge.
// Returns the subset of instances that were modified.
//
// Precondition: instances and reg non-nil; trigger is a known RechargeEntry.Trigger value.
// Postcondition: ChargesRemaining and Expended are updated in-place on modified instances.
func TickRecharge(instances []*ItemInstance, reg *Registry, trigger string) []*ItemInstance {
	var modified []*ItemInstance
	for _, inst := range instances {
		def, ok := reg.Item(inst.ItemDefID)
		if !ok {
			continue
		}
		for _, re := range def.Recharge {
			if re.Trigger != trigger {
				continue
			}
			before := inst.ChargesRemaining
			wasExpended := inst.Expended
			inst.ChargesRemaining += re.Amount
			if inst.ChargesRemaining > def.Charges {
				inst.ChargesRemaining = def.Charges
			}
			if inst.ChargesRemaining > 0 {
				inst.Expended = false
			}
			if inst.ChargesRemaining != before || inst.Expended != wasExpended {
				modified = append(modified, inst)
			}
			break // first matching trigger wins per item
		}
	}
	return modified
}
