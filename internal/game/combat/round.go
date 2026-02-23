package combat

import (
	"fmt"

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

// applyAttackConditions applies conditions triggered by an attack result:
//   - CritFailure: attacker gains prone (permanent)
//   - CritSuccess: target gains flat_footed (1 round)
//   - Player target at 0 HP (not already dying): gains dying(1 + wounded stacks)
//
// Precondition: cbt, target, and r must be valid; cbt.condRegistry must be non-nil.
// Postcondition: Conditions are applied in-place on cbt.
func applyAttackConditions(cbt *Combat, actor, target *Combatant, r AttackResult) {
	switch r.Outcome {
	case CritFailure:
		_ = cbt.ApplyCondition(actor.ID, "prone", 1, -1)
	case CritSuccess:
		_ = cbt.ApplyCondition(target.ID, "flat_footed", 1, 1)
	}
	// Only apply dying if the target is a player, at 0 HP, and NOT already dying
	if target.CurrentHP <= 0 && target.Kind == KindPlayer && !cbt.HasCondition(target.ID, "dying") {
		woundedStacks := cbt.Conditions[target.ID].Stacks("wounded")
		_ = cbt.ApplyCondition(target.ID, "dying", 1+woundedStacks, -1)
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
				r.Outcome = OutcomeFor(r.AttackTotal, target.AC)
				dmg := r.EffectiveDamage()
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
				r1.Outcome = OutcomeFor(r1.AttackTotal, target.AC)
				dmg1 := r1.EffectiveDamage()
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
				r2.Outcome = OutcomeFor(r2.AttackTotal, target.AC)
				dmg2 := r2.EffectiveDamage()
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
