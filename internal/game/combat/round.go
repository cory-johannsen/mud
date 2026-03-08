package combat

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/inventory"
)

// attackNarrative builds a human-readable attack result string.
// When dmg > 0 (a hit landed), the damage dealt is included.
func attackNarrative(actorName, verb, targetName string, outcome Outcome, total, dmg int) string {
	switch outcome {
	case CritSuccess:
		if dmg > 0 {
			return fmt.Sprintf("*** CRITICAL HIT! *** %s %s %s (total %d) for %d damage!", actorName, verb, targetName, total, dmg)
		}
		return fmt.Sprintf("*** CRITICAL HIT! *** %s %s %s (total %d)!", actorName, verb, targetName, total)
	case CritFailure:
		return fmt.Sprintf("*** CRITICAL MISS! *** %s fumbles against %s (total %d)!", actorName, targetName, total)
	case Success:
		if dmg > 0 {
			return fmt.Sprintf("%s %s %s (total %d) for %d damage.", actorName, verb, targetName, total, dmg)
		}
		return fmt.Sprintf("%s %s %s (total %d).", actorName, verb, targetName, total)
	default: // Failure
		return fmt.Sprintf("%s %s %s (total %d) — miss.", actorName, verb, targetName, total)
	}
}

// applyResistanceWeakness adjusts baseDmg based on target's resistances and weaknesses
// for the given damageType. Returns the final damage (minimum 0) and annotation strings.
//
// Precondition: baseDmg >= 0; damageType may be empty (returns baseDmg unchanged).
// Postcondition: returned damage >= 0.
func applyResistanceWeakness(target *Combatant, damageType string, baseDmg int) (int, []string) {
	if damageType == "" || baseDmg == 0 {
		return baseDmg, nil
	}
	var annotations []string
	result := baseDmg
	if r, ok := target.Resistances[damageType]; ok && r > 0 {
		result -= r
		if result < 0 {
			result = 0
		}
		annotations = append(annotations, fmt.Sprintf("resisted %d %s", r, damageType))
	}
	if w, ok := target.Weaknesses[damageType]; ok && w > 0 {
		result += w
		annotations = append(annotations, fmt.Sprintf("weak to %s +%d", damageType, w))
	}
	return result, annotations
}

// RoundEvent records what happened when one action was resolved.
type RoundEvent struct {
	AttackResult *AttackResult // nil for pass
	ActionType   ActionType
	ActorID      string
	ActorName    string
	Narrative    string
}

// findCombatantByName returns the first Combatant in cbt whose Name matches name, or nil.
func findCombatantByName(cbt *Combat, name string) *Combatant {
	for _, c := range cbt.Combatants {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// findCombatantByID returns the first Combatant in cbt whose ID matches id, or nil.
// Precondition: cbt must be non-nil.
// Postcondition: Returns nil when no combatant with the given ID exists.
func findCombatantByID(cbt *Combat, id string) *Combatant {
	for _, c := range cbt.Combatants {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// hookAttackRoll invokes the on_attack_roll Lua hook (if a scriptMgr is present) and returns
// the (possibly overridden) attack total.
// Precondition: actor and target must be non-nil.
// Postcondition: Returns atkTotal unchanged when no hook is defined or hook returns nil/non-number.
func hookAttackRoll(cbt *Combat, actor, target *Combatant, atkTotal int) int {
	if cbt.scriptMgr == nil {
		return atkTotal
	}
	ret, _ := cbt.scriptMgr.CallHook(cbt.zoneID, "on_attack_roll",
		lua.LString(actor.ID), lua.LString(target.ID),
		lua.LNumber(float64(atkTotal)), lua.LNumber(float64(target.AC)))
	if n, ok := ret.(lua.LNumber); ok {
		return int(n)
	}
	return atkTotal
}

// hookPassiveFeatCheck fires the on_passive_feat_check Lua hook for a passive feat evaluation.
// It is called once per passive feat, whether or not the condition was met.
//
// Precondition: cbt.scriptMgr may be nil (hook is silently skipped when nil).
// Postcondition: Returns the (possibly overridden) damage bonus from the hook; returns bonus unchanged
//
//	when no scripting manager is present, or the hook is absent or returns nil/non-number.
func hookPassiveFeatCheck(cbt *Combat, actorID, targetID, featID string, bonus int, met bool) int {
	if cbt.scriptMgr == nil {
		return bonus
	}
	outcome := "not_met"
	if met {
		outcome = "met"
	}
	targetType := ""
	if t := findCombatantByID(cbt, targetID); t != nil {
		targetType = t.NPCType
	}
	ctx := map[string]lua.LValue{
		"target_uid":   lua.LString(targetID),
		"damage_bonus": lua.LNumber(float64(bonus)),
		"outcome":      lua.LString(outcome),
		"target_type":  lua.LString(targetType),
	}
	// Error is always nil; CallHookWithContext logs Lua runtime errors internally and never propagates them.
	ret, _ := cbt.scriptMgr.CallHookWithContext(cbt.zoneID, "on_passive_feat_check", actorID, featID, ctx)
	if n, ok := ret.(lua.LNumber); ok {
		return int(n)
	}
	return bonus
}

// hookDamageRoll invokes the on_damage_roll Lua hook (if a scriptMgr is present) and returns
// the (possibly overridden) damage value.
// Precondition: actor and target must be non-nil; dmg >= 0.
// Postcondition: Returns dmg unchanged if cbt.scriptMgr is nil, dmg <= 0, or hook is absent/returns nil.
//
//	Returns hook's integer return value when hook returns a Lua number.
func hookDamageRoll(cbt *Combat, actor, target *Combatant, dmg int) int {
	if cbt.scriptMgr == nil || dmg <= 0 {
		return dmg
	}
	ret, _ := cbt.scriptMgr.CallHook(cbt.zoneID, "on_damage_roll",
		lua.LString(actor.ID), lua.LString(target.ID),
		lua.LNumber(float64(dmg)))
	if n, ok := ret.(lua.LNumber); ok {
		return int(n)
	}
	return dmg
}

// conditionApplyAllowed invokes the on_condition_apply Lua hook.
// Returns false (cancelling the application) only when the hook explicitly returns false.
// Precondition: uid and condID must be non-empty.
// Postcondition: Returns true when no hook is defined or hook does not return false.
func conditionApplyAllowed(cbt *Combat, uid, condID string, stacks int) bool {
	if cbt.scriptMgr == nil {
		return true
	}
	ret, _ := cbt.scriptMgr.CallHook(cbt.zoneID, "on_condition_apply",
		lua.LString(uid), lua.LString(condID), lua.LNumber(float64(stacks)))
	if ret == lua.LFalse {
		return false
	}
	return true
}

// applyConditionIfAllowed applies a condition to uid only when the on_condition_apply hook permits.
// Precondition: uid and condID must be non-empty; stacks >= 1.
// Postcondition: Condition is applied; skipped if hook returns false.
//
//	Skipped silently if condID is not in the registry (content configuration error).
func applyConditionIfAllowed(cbt *Combat, uid, condID string, stacks, duration int) {
	if !conditionApplyAllowed(cbt, uid, condID, stacks) {
		return
	}
	if err := cbt.ApplyCondition(uid, condID, stacks, duration); err != nil {
		// Condition ID not found in registry; skip silently.
		// This indicates a content configuration error at startup.
		return
	}
}

// applyAttackConditions applies conditions triggered by an attack result:
//   - CritFailure: attacker gains prone (permanent)
//   - CritSuccess: target gains flat_footed (1 round)
//   - Player target at 0 HP (not already dying): gains dying(1 + wounded stacks)
//
// Precondition: cbt, target, and r must be valid; cbt.condRegistry must be non-nil.
// Postcondition: Conditions are applied in-place on cbt, subject to on_condition_apply hooks.
func applyAttackConditions(cbt *Combat, actor, target *Combatant, r AttackResult) {
	switch r.Outcome {
	case CritFailure:
		applyConditionIfAllowed(cbt, actor.ID, "prone", 1, -1)
	case CritSuccess:
		applyConditionIfAllowed(cbt, target.ID, "flat_footed", 1, 1)
	}
	// Only apply dying if the target is a player, at 0 HP, and NOT already dying
	if target.CurrentHP <= 0 && target.Kind == KindPlayer && !cbt.HasCondition(target.ID, "dying") {
		woundedStacks := cbt.Conditions[target.ID].Stacks("wounded")
		applyConditionIfAllowed(cbt, target.ID, "dying", 1+woundedStacks, -1)
	}
}

// applyPassiveFeats evaluates all active passive feats for actor against target and returns
// the total bonus damage to add.
// Precondition: actor and target must be non-nil; dmg is the base hit damage (0 on miss).
// Postcondition: Returns 0 when actor is not a player, sessionGetter is nil, or no feats are active.
//
//	Attacking always breaks concealment: actor.Hidden is cleared before feat checks run.
func applyPassiveFeats(cbt *Combat, actor, target *Combatant, dmg int, src Source) int {
	if actor.Kind != KindPlayer || cbt.sessionGetter == nil {
		return 0
	}
	ps, ok := cbt.sessionGetter(actor.ID)
	if !ok {
		return 0
	}

	// Capture hidden state before clearing — attacking always breaks concealment.
	actorHidden := actor.Hidden
	if actorHidden {
		actor.Hidden = false
	}

	bonus := 0
	if ps.PassiveFeats["sucker_punch"] {
		spBonus := 0
		targetOffGuard := cbt.Conditions[target.ID] != nil &&
			(cbt.Conditions[target.ID].Has("flat_footed") || cbt.Conditions[target.ID].Has("grabbed"))
		spMet := (targetOffGuard || actorHidden) && dmg > 0
		if spMet {
			spBonus = src.Intn(6) + 1
		}
		bonus += hookPassiveFeatCheck(cbt, actor.ID, target.ID, "sucker_punch", spBonus, spMet)
	}
	if ps.PassiveFeats["predators_eye"] {
		peBonus := 0
		peMet := ps.FavoredTarget != "" && target.NPCType == ps.FavoredTarget && dmg > 0
		if peMet {
			peBonus = src.Intn(8) + 1
		}
		bonus += hookPassiveFeatCheck(cbt, actor.ID, target.ID, "predators_eye", peBonus, peMet)
	}
	return bonus
}

// ResolveRound processes all queued actions for cbt in Combatants order (initiative-sorted).
// For each living combatant, iterates their QueuedActions():
//   - ActionAttack: one ResolveAttack call; damage applied to target combatant; targetUpdater called.
//   - ActionStrike: two ResolveAttack calls; second attack has AttackTotal reduced by 5 (MAP) and
//     Outcome re-evaluated with OutcomeFor(adjustedTotal, target.AC).
//   - ActionPass: narrative event, no damage, nil AttackResult.
//
// Dead combatants are skipped entirely (no events).
// If a target is dead before a follow-up strike hit, emit a narrative "hit nothing" event.
// If the ActionStrike target is nil or already dead at the start of the strike, both the first and
// second attack produce "hit nothing" narrative events with nil AttackResult.
//
// targetUpdater(id string, hp int) is called after each damage application; may be nil (no-op).
//
// Lua hooks fired per attack (when cbt.scriptMgr != nil):
//   - on_attack_roll(attacker_uid, target_uid, roll_total, ac) → number: override attack total.
//   - on_damage_roll(attacker_uid, target_uid, damage) → number: override damage value.
//   - on_condition_apply(uid, cond_id, stacks) → false: cancel condition application.
//
// Precondition: cbt and src must not be nil.
// Postcondition: Returns ordered RoundEvents; damage applied in-place on Combatants.
func ResolveRound(cbt *Combat, src Source, targetUpdater func(id string, hp int)) []RoundEvent {
	if targetUpdater == nil {
		targetUpdater = func(id string, hp int) {}
	}

	var events []RoundEvent

	for _, actor := range cbt.Combatants {
		if actor.IsDead() {
			continue
		}
		q, ok := cbt.ActionQueues[actor.ID]
		if !ok {
			continue
		}
		for _, action := range q.QueuedActions() {
			switch action.Type {
			case ActionPass:
				events = append(events, RoundEvent{
					ActionType: ActionPass,
					ActorID:    actor.ID,
					ActorName:  actor.Name,
					Narrative:  fmt.Sprintf("%s passes.", actor.Name),
				})
				// Clear flat_footed from NPC combatants after their first action resolves.
				if actor.Kind == KindNPC && cbt.Conditions[actor.ID] != nil {
					cbt.Conditions[actor.ID].Remove(actor.ID, "flat_footed")
				}

			case ActionAttack:
				target := findCombatantByName(cbt, action.Target)
				if target == nil || target.IsDead() {
					events = append(events, RoundEvent{
						ActionType: ActionAttack,
						ActorID:    actor.ID,
						ActorName:  actor.Name,
						Narrative:  fmt.Sprintf("%s attacks but hits nothing.", actor.Name),
					})
					continue
				}
				atkBonus := condition.AttackBonus(cbt.Conditions[actor.ID])
				acBonus := condition.ACBonus(cbt.Conditions[target.ID])
				r := ResolveAttack(actor, target, src)
				r.AttackTotal += atkBonus
				r.AttackTotal += acBonus
				r.AttackTotal += actor.InitiativeBonus
				effectiveAC := target.AC + target.InitiativeBonus
				r.AttackTotal = hookAttackRoll(cbt, actor, target, r.AttackTotal)
				r.Outcome = OutcomeFor(r.AttackTotal, effectiveAC)
				dmg := r.EffectiveDamage()
				dmg += condition.DamageBonus(cbt.Conditions[actor.ID])
				dmg += applyPassiveFeats(cbt, actor, target, dmg, src)
				dmg = hookDamageRoll(cbt, actor, target, dmg)
				dmg, rwAnnotations := applyResistanceWeakness(target, r.DamageType, dmg)
				if dmg > 0 {
					target.ApplyDamage(dmg)
					targetUpdater(target.ID, target.CurrentHP)
				}
				applyAttackConditions(cbt, actor, target, r)
				narrative := attackNarrative(actor.Name, "attacks", target.Name, r.Outcome, r.AttackTotal, dmg)
				if len(rwAnnotations) > 0 {
					narrative += " (" + strings.Join(rwAnnotations, "; ") + ")"
				}
				events = append(events, RoundEvent{
					AttackResult: &r,
					ActionType:   ActionAttack,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    narrative,
				})
				// Clear flat_footed from NPC combatants after their first action resolves.
				if actor.Kind == KindNPC && cbt.Conditions[actor.ID] != nil {
					cbt.Conditions[actor.ID].Remove(actor.ID, "flat_footed")
				}

			case ActionStrike:
				target := findCombatantByName(cbt, action.Target)
				if target == nil || target.IsDead() {
					events = append(events, RoundEvent{
						ActionType: ActionStrike,
						ActorID:    actor.ID,
						ActorName:  actor.Name,
						Narrative:  fmt.Sprintf("%s strikes but hits nothing.", actor.Name),
					})
					// Emit a second event for the follow-up that also hits nothing.
					events = append(events, RoundEvent{
						ActionType: ActionStrike,
						ActorID:    actor.ID,
						ActorName:  actor.Name,
						Narrative:  fmt.Sprintf("%s strikes again but hits nothing.", actor.Name),
					})
					continue
				}
				// First strike
				atkBonus1 := condition.AttackBonus(cbt.Conditions[actor.ID])
				acBonus1 := condition.ACBonus(cbt.Conditions[target.ID])
				r1 := ResolveAttack(actor, target, src)
				r1.AttackTotal += atkBonus1
				r1.AttackTotal += acBonus1
				r1.AttackTotal += actor.InitiativeBonus
				effectiveAC1 := target.AC + target.InitiativeBonus
				r1.AttackTotal = hookAttackRoll(cbt, actor, target, r1.AttackTotal)
				r1.Outcome = OutcomeFor(r1.AttackTotal, effectiveAC1)
				dmg1 := r1.EffectiveDamage()
				dmg1 += condition.DamageBonus(cbt.Conditions[actor.ID])
				dmg1 += applyPassiveFeats(cbt, actor, target, dmg1, src)
				dmg1 = hookDamageRoll(cbt, actor, target, dmg1)
				dmg1, rwAnnotations1 := applyResistanceWeakness(target, r1.DamageType, dmg1)
				if dmg1 > 0 {
					target.ApplyDamage(dmg1)
					targetUpdater(target.ID, target.CurrentHP)
				}
				applyAttackConditions(cbt, actor, target, r1)
				narrative1 := attackNarrative(actor.Name, "strikes", target.Name, r1.Outcome, r1.AttackTotal, dmg1)
				if len(rwAnnotations1) > 0 {
					narrative1 += " (" + strings.Join(rwAnnotations1, "; ") + ")"
				}
				events = append(events, RoundEvent{
					AttackResult: &r1,
					ActionType:   ActionStrike,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    narrative1,
				})
				// Clear flat_footed from NPC combatants after their first action resolves.
				if actor.Kind == KindNPC && cbt.Conditions[actor.ID] != nil {
					cbt.Conditions[actor.ID].Remove(actor.ID, "flat_footed")
				}

				// Second strike with MAP penalty
				if target.IsDead() {
					events = append(events, RoundEvent{
						ActionType: ActionStrike,
						ActorID:    actor.ID,
						ActorName:  actor.Name,
						Narrative:  fmt.Sprintf("%s follows up but %s is already dead.", actor.Name, target.Name),
					})
					continue
				}
				atkBonus2 := condition.AttackBonus(cbt.Conditions[actor.ID])
				acBonus2 := condition.ACBonus(cbt.Conditions[target.ID])
				r2 := ResolveAttack(actor, target, src)
				r2.AttackTotal += atkBonus2
				r2.AttackTotal += acBonus2
				r2.AttackTotal += actor.InitiativeBonus
				r2.AttackTotal -= 5
				effectiveAC2 := target.AC + target.InitiativeBonus
				r2.AttackTotal = hookAttackRoll(cbt, actor, target, r2.AttackTotal)
				r2.Outcome = OutcomeFor(r2.AttackTotal, effectiveAC2)
				dmg2 := r2.EffectiveDamage()
				dmg2 += condition.DamageBonus(cbt.Conditions[actor.ID])
				dmg2 += applyPassiveFeats(cbt, actor, target, dmg2, src)
				dmg2 = hookDamageRoll(cbt, actor, target, dmg2)
				dmg2, rwAnnotations2 := applyResistanceWeakness(target, r2.DamageType, dmg2)
				if dmg2 > 0 {
					target.ApplyDamage(dmg2)
					targetUpdater(target.ID, target.CurrentHP)
				}
				applyAttackConditions(cbt, actor, target, r2)
				narrative2 := attackNarrative(actor.Name, "strikes", target.Name, r2.Outcome, r2.AttackTotal, dmg2)
				if len(rwAnnotations2) > 0 {
					narrative2 += " (" + strings.Join(rwAnnotations2, "; ") + ")"
				}
				events = append(events, RoundEvent{
					AttackResult: &r2,
					ActionType:   ActionStrike,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    narrative2,
				})
			case ActionReload:
				events = append(events, resolveReload(cbt, actor, action))
			case ActionFireBurst:
				events = append(events, resolveFireBurst(cbt, actor, action, src)...)
			case ActionFireAutomatic:
				events = append(events, resolveFireAutomatic(cbt, actor, action, src)...)
			case ActionThrow:
				events = append(events, resolveThrow(cbt, actor, action, src)...)
			}
		}
	}

	return events
}

// resolveReload handles ActionReload: calls on_reload Lua hook and restores magazine.
func resolveReload(cbt *Combat, actor *Combatant, qa QueuedAction) RoundEvent {
	narrative := actor.Name + " reloads."
	if actor.Loadout != nil {
		if eq := actor.Loadout.MainHand; eq != nil && eq.Magazine != nil {
			eq.Magazine.Reload()
			narrative = fmt.Sprintf("%s reloads %s.", actor.Name, eq.Def.Name)
		}
	}
	if cbt.scriptMgr != nil {
		_, _ = cbt.scriptMgr.CallHook(cbt.zoneID, "on_reload",
			lua.LString(actor.ID), lua.LString(qa.WeaponID))
	}
	return RoundEvent{ActionType: ActionReload, ActorID: actor.ID, ActorName: actor.Name, Narrative: narrative}
}

// resolveFireBurst handles ActionFireBurst: two ranged attacks against the same target.
func resolveFireBurst(cbt *Combat, actor *Combatant, qa QueuedAction, src Source) []RoundEvent {
	target := findCombatantByNameOrID(cbt, qa.Target)
	if target == nil || target.IsDead() {
		return []RoundEvent{{ActionType: ActionFireBurst, ActorID: actor.ID, ActorName: actor.Name,
			Narrative: fmt.Sprintf("%s fires burst but target not found.", actor.Name)}}
	}
	weapon := primaryFirearm(actor, qa.WeaponID)
	var events []RoundEvent
	for i := 0; i < 2; i++ {
		var result AttackResult
		if weapon != nil {
			result = ResolveFirearmAttack(actor, target, weapon, 0, src)
		} else {
			result = ResolveAttack(actor, target, src)
		}
		result.AttackTotal = hookAttackRoll(cbt, actor, target, result.AttackTotal)
		result.Outcome = OutcomeFor(result.AttackTotal, target.AC)
		dmg := result.EffectiveDamage()
		dmg = hookDamageRoll(cbt, actor, target, dmg)
		if dmg > 0 {
			target.ApplyDamage(dmg)
		}
		if weapon != nil && actor.Loadout != nil {
			if eq := actor.Loadout.MainHand; eq != nil && eq.Magazine != nil {
				_ = eq.Magazine.Consume(1)
			}
		}
		result.BaseDamage = dmg
		events = append(events, RoundEvent{
			AttackResult: &result,
			ActionType:   ActionFireBurst,
			ActorID:      actor.ID,
			ActorName:    actor.Name,
			Narrative:    buildNarrative(actor, target, result, dmg),
		})
		if target.IsDead() {
			break
		}
	}
	return events
}

// resolveFireAutomatic handles ActionFireAutomatic: one attack against each living enemy (up to 3).
func resolveFireAutomatic(cbt *Combat, actor *Combatant, qa QueuedAction, src Source) []RoundEvent {
	enemies := livingEnemiesOf(cbt, actor)
	if len(enemies) == 0 {
		return []RoundEvent{{ActionType: ActionFireAutomatic, ActorID: actor.ID, ActorName: actor.Name,
			Narrative: fmt.Sprintf("%s lays down suppressive fire.", actor.Name)}}
	}
	weapon := primaryFirearm(actor, qa.WeaponID)
	var events []RoundEvent
	shots := 3
	for _, target := range enemies {
		if shots <= 0 {
			break
		}
		var result AttackResult
		if weapon != nil {
			result = ResolveFirearmAttack(actor, target, weapon, 0, src)
		} else {
			result = ResolveAttack(actor, target, src)
		}
		result.AttackTotal = hookAttackRoll(cbt, actor, target, result.AttackTotal)
		result.Outcome = OutcomeFor(result.AttackTotal, target.AC)
		dmg := result.EffectiveDamage()
		dmg = hookDamageRoll(cbt, actor, target, dmg)
		if dmg > 0 {
			target.ApplyDamage(dmg)
		}
		if weapon != nil && actor.Loadout != nil {
			if eq := actor.Loadout.MainHand; eq != nil && eq.Magazine != nil {
				_ = eq.Magazine.Consume(1)
			}
		}
		result.BaseDamage = dmg
		shots--
		events = append(events, RoundEvent{
			AttackResult: &result,
			ActionType:   ActionFireAutomatic,
			ActorID:      actor.ID,
			ActorName:    actor.Name,
			Narrative:    buildNarrative(actor, target, result, dmg),
		})
	}
	return events
}

// resolveThrow handles ActionThrow: explosive area effect against all living enemies.
func resolveThrow(cbt *Combat, actor *Combatant, qa QueuedAction, src Source) []RoundEvent {
	if cbt.invRegistry == nil {
		return []RoundEvent{{ActionType: ActionThrow, ActorID: actor.ID, ActorName: actor.Name,
			Narrative: fmt.Sprintf("%s fumbles the throw.", actor.Name)}}
	}
	grenade := cbt.invRegistry.Explosive(qa.ExplosiveID)
	if grenade == nil {
		return []RoundEvent{{ActionType: ActionThrow, ActorID: actor.ID, ActorName: actor.Name,
			Narrative: fmt.Sprintf("%s reaches for an explosive but finds nothing.", actor.Name)}}
	}
	if cbt.scriptMgr != nil {
		_, _ = cbt.scriptMgr.CallHook(cbt.zoneID, "on_explosive_throw",
			lua.LString(actor.ID), lua.LString(qa.ExplosiveID))
	}
	enemies := livingEnemiesOf(cbt, actor)
	results := ResolveExplosive(grenade, enemies, src)
	var events []RoundEvent
	for i, r := range results {
		target := enemies[i]
		if r.BaseDamage > 0 {
			target.ApplyDamage(r.BaseDamage)
		}
		events = append(events, RoundEvent{
			ActionType: ActionThrow,
			ActorID:    actor.ID,
			ActorName:  actor.Name,
			Narrative: fmt.Sprintf("%s throws %s at %s for %d damage (hustle save: %s).",
				actor.Name, grenade.Name, target.Name, r.BaseDamage, r.SaveResult),
		})
	}
	if len(events) == 0 {
		events = append(events, RoundEvent{ActionType: ActionThrow, ActorID: actor.ID, ActorName: actor.Name,
			Narrative: fmt.Sprintf("%s throws %s but no targets are in range.", actor.Name, grenade.Name)})
	}
	return events
}

// findCombatantByNameOrID returns the first combatant matching name or ID (case-insensitive).
func findCombatantByNameOrID(cbt *Combat, nameOrID string) *Combatant {
	lower := strings.ToLower(nameOrID)
	for _, c := range cbt.Combatants {
		if strings.ToLower(c.Name) == lower || strings.ToLower(c.ID) == lower {
			return c
		}
	}
	return nil
}

// livingEnemiesOf returns all living combatants of a different Kind from actor.
func livingEnemiesOf(cbt *Combat, actor *Combatant) []*Combatant {
	var out []*Combatant
	for _, c := range cbt.Combatants {
		if !c.IsDead() && c.ID != actor.ID && c.Kind != actor.Kind {
			out = append(out, c)
		}
	}
	return out
}

// primaryFirearm returns the primary slot weapon if it is a firearm matching weaponID.
func primaryFirearm(actor *Combatant, weaponID string) *inventory.WeaponDef {
	if actor.Loadout == nil {
		return nil
	}
	eq := actor.Loadout.MainHand
	if eq == nil || !eq.Def.IsFirearm() {
		return nil
	}
	if weaponID != "" && eq.Def.ID != weaponID {
		return nil
	}
	return eq.Def
}

// buildNarrative returns a human-readable attack narrative string.
func buildNarrative(actor, target *Combatant, result AttackResult, dmg int) string {
	switch result.Outcome {
	case CritSuccess:
		if dmg > 0 {
			return fmt.Sprintf("*** CRITICAL HIT! *** %s hits %s for %d damage!", actor.Name, target.Name, dmg)
		}
		return fmt.Sprintf("*** CRITICAL HIT! *** %s hits %s!", actor.Name, target.Name)
	case Success:
		if dmg > 0 {
			return fmt.Sprintf("%s hits %s for %d damage.", actor.Name, target.Name, dmg)
		}
		return fmt.Sprintf("%s hits %s.", actor.Name, target.Name)
	case Failure:
		return fmt.Sprintf("%s misses %s.", actor.Name, target.Name)
	case CritFailure:
		return fmt.Sprintf("*** CRITICAL MISS! *** %s fumbles against %s!", actor.Name, target.Name)
	default:
		return fmt.Sprintf("%s attacks %s.", actor.Name, target.Name)
	}
}
