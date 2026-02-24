package combat

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"github.com/cory-johannsen/mud/internal/game/condition"
)

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
				r.AttackTotal = hookAttackRoll(cbt, actor, target, r.AttackTotal)
				r.Outcome = OutcomeFor(r.AttackTotal, target.AC)
				dmg := r.EffectiveDamage()
				dmg = hookDamageRoll(cbt, actor, target, dmg)
				if dmg > 0 {
					target.ApplyDamage(dmg)
					targetUpdater(target.ID, target.CurrentHP)
				}
				applyAttackConditions(cbt, actor, target, r)
				events = append(events, RoundEvent{
					AttackResult: &r,
					ActionType:   ActionAttack,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    fmt.Sprintf("%s attacks %s: %s (total %d).", actor.Name, target.Name, r.Outcome, r.AttackTotal),
				})

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
				r1.AttackTotal = hookAttackRoll(cbt, actor, target, r1.AttackTotal)
				r1.Outcome = OutcomeFor(r1.AttackTotal, target.AC)
				dmg1 := r1.EffectiveDamage()
				dmg1 = hookDamageRoll(cbt, actor, target, dmg1)
				if dmg1 > 0 {
					target.ApplyDamage(dmg1)
					targetUpdater(target.ID, target.CurrentHP)
				}
				applyAttackConditions(cbt, actor, target, r1)
				events = append(events, RoundEvent{
					AttackResult: &r1,
					ActionType:   ActionStrike,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    fmt.Sprintf("%s strikes %s: %s (total %d).", actor.Name, target.Name, r1.Outcome, r1.AttackTotal),
				})

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
				r2.AttackTotal -= 5
				r2.AttackTotal = hookAttackRoll(cbt, actor, target, r2.AttackTotal)
				r2.Outcome = OutcomeFor(r2.AttackTotal, target.AC)
				dmg2 := r2.EffectiveDamage()
				dmg2 = hookDamageRoll(cbt, actor, target, dmg2)
				if dmg2 > 0 {
					target.ApplyDamage(dmg2)
					targetUpdater(target.ID, target.CurrentHP)
				}
				applyAttackConditions(cbt, actor, target, r2)
				events = append(events, RoundEvent{
					AttackResult: &r2,
					ActionType:   ActionStrike,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    fmt.Sprintf("%s strikes %s again (MAP): %s (total %d).", actor.Name, target.Name, r2.Outcome, r2.AttackTotal),
				})
			}
		}
	}

	return events
}
