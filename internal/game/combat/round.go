package combat

import (
	"context"
	"fmt"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/detection"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/effect"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/reaction"
)

// Reaction-related RoundEvent labels (GH #244 Task 9). These strings appear as
// the Narrative-prefix tag for events emitted by fireTrigger so downstream
// systems can distinguish Ready / feat-reaction events from regular combat.
const (
	// EventTypeReactionFired labels a RoundEvent emitted when a Ready entry or a
	// feat reaction fires. When ReadyEntry is non-nil the gameserver layer is
	// expected to resolve the prepared action after ResolveRound returns.
	EventTypeReactionFired = "reaction_fired"
	// EventTypeReadyFizzled labels a RoundEvent emitted when a Ready entry matched
	// its trigger but revalidation failed (e.g., the queued target is now dead).
	// The ReactionBudget has been refunded; no prepared action runs.
	EventTypeReadyFizzled = "ready_fizzled"
)

// CheckReactiveStrikes returns attack events from all NPCs that are adjacent
// (≤5 ft) to mover before the stride and whose position did not change
// (i.e., they were not the one striding).
//
// This is a legacy wrapper that constructs a MoveContext with Cause set to
// MoveCauseStride. New callers SHOULD use CheckReactiveStrikesCtx so they can
// pass the correct cause (e.g. MoveCauseMoveTrait suppresses reactions per
// WMOVE-12).
//
// Precondition: cbt non-nil; moverID non-empty; oldX/oldY is mover's grid position before stride.
// Postcondition: Returns zero or more RoundEvent{ActionType: ActionAttack} events.
// targetUpdater(id, hp) is called after each damage application; may be nil (no-op).
func CheckReactiveStrikes(cbt *Combat, moverID string, oldX, oldY int, rng Source, targetUpdater func(id string, hp int)) []RoundEvent {
	return CheckReactiveStrikesCtx(cbt, ReactionMoveContext{
		MoverID: moverID,
		FromX:   oldX,
		FromY:   oldY,
		Cause:   MoveCauseStride,
	}, rng, targetUpdater)
}

// CheckReactiveStrikesCtx is the cause-aware variant of CheckReactiveStrikes.
// It accepts a MoveContext carrying the originating MoveCause and short-circuits
// when the cause suppresses reactions (WMOVE-12).
//
// Precondition: cbt non-nil; ctx.MoverID non-empty.
// Postcondition: returns nil when ctx.Cause == MoveCauseMoveTrait. Otherwise
// returns zero or more RoundEvent{ActionType: ActionAttack} events identical
// to the legacy CheckReactiveStrikes path.
func CheckReactiveStrikesCtx(cbt *Combat, ctx ReactionMoveContext, rng Source, targetUpdater func(id string, hp int)) []RoundEvent {
	// WMOVE-12: Mobile-trait granted movement does not provoke reactive strikes.
	if ctx.Cause == MoveCauseMoveTrait {
		return nil
	}
	if targetUpdater == nil {
		targetUpdater = func(id string, hp int) {}
	}
	moverID := ctx.MoverID
	oldX := ctx.FromX
	oldY := ctx.FromY
	mover := findCombatantByID(cbt, moverID)
	if mover == nil {
		return nil
	}
	newX, newY := mover.GridX, mover.GridY

	var events []RoundEvent
	for _, c := range cbt.Combatants {
		if c.ID == moverID || c.IsDead() {
			continue
		}
		// Distance before stride: use oldX/oldY for the mover's position.
		oldMover := Combatant{GridX: oldX, GridY: oldY}
		if CombatRange(*c, oldMover) > 5 {
			// NPC was not adjacent before the stride — no reactive strike.
			continue
		}
		// Check that the mover actually moved away (new distance > 5).
		newMover := Combatant{GridX: newX, GridY: newY}
		if CombatRange(*c, newMover) <= 5 {
			// Mover didn't move away from this combatant.
			continue
		}

		// Simplified reactive-strike roll: no hooks, conditions, resistances, or weapon dice.
		d20 := rng.Intn(20) + 1
		atkTotal := d20 + c.Level
		outcome := OutcomeFor(atkTotal, mover.AC)
		dmg := 0
		hitNarrative := fmt.Sprintf("%s makes a reactive strike against %s!", c.Name, mover.Name)
		switch outcome {
		case CritSuccess:
			dmg = rng.Intn(6) + 1 + rng.Intn(6) + 1 // double damage die on crit
			mover.ApplyDamage(dmg)
			targetUpdater(mover.ID, mover.CurrentHP)
			hitNarrative = fmt.Sprintf("%s makes a reactive strike against %s! *** CRITICAL HIT! *** Deals %d damage (total %d)!", c.Name, mover.Name, dmg, atkTotal)
		case Success:
			dmg = rng.Intn(6) + 1
			mover.ApplyDamage(dmg)
			targetUpdater(mover.ID, mover.CurrentHP)
			hitNarrative = fmt.Sprintf("%s makes a reactive strike against %s! Hit for %d damage (total %d).", c.Name, mover.Name, dmg, atkTotal)
		default:
			hitNarrative = fmt.Sprintf("%s makes a reactive strike against %s! Miss (total %d).", c.Name, mover.Name, atkTotal)
		}

		r := AttackResult{
			AttackTotal: atkTotal,
			BaseDamage:  dmg,
			Outcome:     outcome,
		}
		events = append(events, RoundEvent{
			AttackResult: &r,
			ActionType:   ActionAttack,
			ActorID:      c.ID,
			ActorName:    c.Name,
			TargetID:     moverID,
			Narrative:    hitNarrative,
		})
	}
	return events
}

// CompassDelta returns the (dx, dy) movement delta for one stride step.
// Compass directions: n/s/e/w/ne/nw/se/sw move exactly 1 square.
// "toward" moves one step toward the opponent (Y reduced first, then X).
// "away" is the inverse of "toward".
//
// Precondition: opponent may be nil (only used for toward/away).
// Postcondition: Returns (dx, dy) where each component is -1, 0, or 1.
func CompassDelta(dir string, actor *Combatant, opponent *Combatant) (int, int) {
	switch dir {
	case "n":
		return 0, -1
	case "s":
		return 0, 1
	case "e":
		return 1, 0
	case "w":
		return -1, 0
	case "ne":
		return 1, -1
	case "nw":
		return -1, -1
	case "se":
		return 1, 1
	case "sw":
		return -1, 1
	case "toward":
		if opponent == nil {
			return 0, 0
		}
		return towardDelta(actor.GridX, actor.GridY, opponent.GridX, opponent.GridY)
	case "away":
		if opponent == nil {
			return 0, 0
		}
		dx, dy := towardDelta(actor.GridX, actor.GridY, opponent.GridX, opponent.GridY)
		return -dx, -dy
	default:
		return 0, 0
	}
}

// towardDelta returns a (dx, dy) step of magnitude ≤1 toward (tx, ty) from (fx, fy).
// Ties resolved by reducing Y distance first, then X.
func towardDelta(fx, fy, tx, ty int) (int, int) {
	dx, dy := 0, 0
	if tx > fx {
		dx = 1
	} else if tx < fx {
		dx = -1
	}
	if ty > fy {
		dy = 1
	} else if ty < fy {
		dy = -1
	}
	return dx, dy
}

// attackNarrative builds a human-readable attack result string.
// When dmg > 0 (a hit landed), the damage dealt is included.
// When weaponName is non-empty, the weapon is included as "with a <weaponName>".
// REQ-67-1: The raw d20 result MUST appear as "1d20 (N)".
// REQ-67-2: The target's effective AC MUST appear as "vs AC N".
func attackNarrative(actorName, verb, targetName, weaponName string, outcome Outcome, d20Roll, total, targetAC, dmg int) string {
	with := ""
	if weaponName != "" {
		with = " with a " + weaponName
	}
	mod := total - d20Roll
	var rollBreakdown string
	switch {
	case mod > 0:
		rollBreakdown = fmt.Sprintf("[1d20 (%d) +%d = %d vs AC %d]", d20Roll, mod, total, targetAC)
	case mod < 0:
		rollBreakdown = fmt.Sprintf("[1d20 (%d) %d = %d vs AC %d]", d20Roll, mod, total, targetAC)
	default:
		rollBreakdown = fmt.Sprintf("[1d20 (%d) = %d vs AC %d]", d20Roll, total, targetAC)
	}
	switch outcome {
	case CritSuccess:
		if dmg > 0 {
			return fmt.Sprintf("*** CRITICAL HIT! *** %s %s %s%s %s for %d damage!", actorName, verb, targetName, with, rollBreakdown, dmg)
		}
		return fmt.Sprintf("*** CRITICAL HIT! *** %s %s %s%s %s!", actorName, verb, targetName, with, rollBreakdown)
	case CritFailure:
		return fmt.Sprintf("*** CRITICAL MISS! *** %s fumbles against %s %s!", actorName, targetName, rollBreakdown)
	case Success:
		if dmg > 0 {
			return fmt.Sprintf("%s %s %s%s %s for %d damage.", actorName, verb, targetName, with, rollBreakdown, dmg)
		}
		return fmt.Sprintf("%s %s %s%s %s.", actorName, verb, targetName, with, rollBreakdown)
	default: // Failure
		return fmt.Sprintf("%s %s %s%s %s — miss.", actorName, verb, targetName, with, rollBreakdown)
	}
}

// applyResistanceWeakness was deleted (MULT-17). Resistance/weakness handling moved
// into ResolveDamage's StageWeakness/StageResistance steps; see damage.go.

// RoundEvent records what happened when one action was resolved.
type RoundEvent struct {
	AttackResult *AttackResult // nil for pass
	ActionType   ActionType
	ActorID      string
	ActorName    string
	// TargetID is the combatant ID of the target, when applicable (e.g. reactive strikes).
	// Empty for non-targeted actions such as pass or stride.
	TargetID string
	// CoverEquipmentID is the item ID of the cover equipment hit in an ActionCoverHit event.
	// Empty for all other event types.
	CoverEquipmentID string
	Narrative        string
	// Flanking is true when the attacker benefits from the flanking +2 bonus on this attack.
	Flanking bool
	// AbilityID is the technology ID for ActionUseTech events; empty for all other event types.
	AbilityID string
	// TargetX and TargetY are the AoE burst center grid coordinates for ActionUseTech events.
	// -1 means unset (no AoE targeting).
	TargetX int32
	TargetY int32
	// ReadyEntry is non-nil on EventTypeReactionFired events that fire a prepared
	// Ready action (GH #244 REACTION-8). The gameserver layer resolves the
	// prepared action after ResolveRound returns; inline execution is deferred
	// to keep this package's responsibilities focused on dispatch.
	ReadyEntry *reaction.ReadyEntry
	// Damage is the post-pipeline damage applied by this event (TERRAIN-9..14).
	// Used by hazard-damage events; zero for non-damaging events.
	Damage int
	// DamageBreakdown is the inline-formatted breakdown line (MULT-14).
	// Empty string when the breakdown was trivial (only StageBase).
	DamageBreakdown string
	// BreakdownSteps is the structured breakdown for verbose rendering (MULT-15).
	BreakdownSteps []DamageBreakdownStep
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
// Postcondition: Returns true if the condition was applied; false if blocked by hook or registry error.
//
//	Skipped silently if condID is not in the registry (content configuration error).
func applyConditionIfAllowed(cbt *Combat, uid, condID string, stacks, duration int) bool {
	if !conditionApplyAllowed(cbt, uid, condID, stacks) {
		return false
	}
	if err := cbt.ApplyCondition(uid, condID, stacks, duration); err != nil {
		// Condition ID not found in registry; skip silently.
		// This indicates a content configuration error at startup.
		return false
	}
	return true
}

// mapPenaltyFor returns the Multiple Attack Penalty for the Nth attack in a
// round, where attacksMade is the count of prior attack rolls this combatant
// has already resolved this round (0 for the first attack, 1 for the second,
// etc). PF2e MAP is -5 for the 2nd attack, -10 for the 3rd and beyond.
//
// Postcondition: returns 0 when attacksMade <= 0; -5 when attacksMade == 1;
// -10 when attacksMade >= 2.
func mapPenaltyFor(attacksMade int) int {
	switch {
	case attacksMade <= 0:
		return 0
	case attacksMade == 1:
		return -5
	default:
		return -10
	}
}

// applyAttackConditions applies conditions triggered by an attack result:
//   - CritFailure: attacker gains prone (permanent, -2 to attacks)
//   - CritSuccess: target gains flat_footed (1 round)
//   - Player target at 0 HP (not already dying): gains dying(1 + wounded stacks)
//
// Precondition: cbt, target, and r must be valid; cbt.condRegistry must be non-nil.
// Postcondition: Conditions are applied in-place on cbt, subject to on_condition_apply hooks.
// Returns a slice of human-readable narratives describing each condition applied.
func applyAttackConditions(cbt *Combat, actor, target *Combatant, r AttackResult) []string {
	var notes []string
	switch r.Outcome {
	case CritFailure:
		if applyConditionIfAllowed(cbt, actor.ID, "prone", 1, -1) {
			notes = append(notes, actor.Name+" falls prone! (-2 to attacks; stand up costs 1 AP)")
		}
	case CritSuccess:
		if applyConditionIfAllowed(cbt, target.ID, "flat_footed", 1, 1) {
			notes = append(notes, target.Name+" is flat-footed! (-2 AC this round)")
		}
	}
	// Only apply dying if the target is a player, at 0 HP, and NOT already dying
	if target.CurrentHP <= 0 && target.Kind == KindPlayer && !cbt.HasCondition(target.ID, "dying") {
		woundedStacks := cbt.Conditions[target.ID].Stacks("wounded")
		applyConditionIfAllowed(cbt, target.ID, "dying", 1+woundedStacks, -1)
	}
	return notes
}

// applyPassiveFeats evaluates all active passive feats for actor against target and returns
// the total bonus damage to add.
// Precondition: actor and target must be non-nil; dmg is the base hit damage (0 on miss).
// Postcondition: Returns 0 when actor is not a player, sessionGetter is nil, or no feats are active.
//
//	Attacking always breaks concealment: actor.Hidden is cleared before feat checks run.
//
// Postcondition: actor.Hidden is set to false if it was true on entry (attacking always breaks concealment).
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
		ps.FavoredTargetMu.RLock()
		favoredTarget := ps.FavoredTarget
		ps.FavoredTargetMu.RUnlock()
		peMet := favoredTarget != "" && target.NPCType == favoredTarget && dmg > 0
		if peMet {
			peBonus = src.Intn(8) + 1
		}
		bonus += hookPassiveFeatCheck(cbt, actor.ID, target.ID, "predators_eye", peBonus, peMet)
	}
	return bonus
}

// AidOutcome classifies an Aid roll total into one of four DC 20 outcome bands.
//
// Precondition: total is the sum of d20 + best ability modifier.
// Postcondition: Returns "critical_success" (>=30), "success" (20-29),
// "failure" (10-19), or "critical_failure" (<=9).
func AidOutcome(total int) string {
	switch {
	case total >= 30:
		return "critical_success"
	case total >= 20:
		return "success"
	case total >= 10:
		return "failure"
	default:
		return "critical_failure"
	}
}

// DefaultReactionTimeout is the fallback prompt timeout used by ResolveRound
// when a caller passes a non-positive reactionTimeout. Matches
// config.DefaultReactionPromptTimeout (kept as a local literal to avoid a cyclic
// import of the config package from this low-level engine package).
const DefaultReactionTimeout = 3 * time.Second

// fireTrigger is the sole reaction dispatch helper for every trigger point in
// the resolver (GH #244 Task 9). Priority order (REACTION-8):
//  1. Ready entries win over feat reactions.
//  2. Only one reaction spends per combatant per round (enforced via
//     Combatant.ReactionBudget.TrySpend/Refund).
//
// cbt / uid identify the player who may react; trigger + rctx describe the
// trigger; sourceUID is used to match the Ready entry's optional TriggerTgt.
// reactionFn is the player-interactive callback (may be nil for no feat path).
// reactionTimeout bounds the interactive prompt (<= 0 maps to DefaultReactionTimeout).
// reactRegistry is an optional pre-filter source for feat-reaction candidates;
// when nil the callback is responsible for its own candidate lookup (legacy
// behavior compatible with sess.ReactionFn that filters internally).
//
// Returns any RoundEvents emitted. The caller MUST append them to its own
// events slice.
//
// Precondition: cbt must not be nil.
// Postcondition: Returns nil when no reaction fires. When a Ready entry matched
// but revalidation failed, returns a single EventTypeReadyFizzled narrative
// event and refunds the budget. When a Ready entry fires, returns an
// EventTypeReactionFired event carrying the ReadyEntry pointer. When a feat
// reaction fires, returns an EventTypeReactionFired narrative event.
func fireTrigger(
	cbt *Combat,
	uid string,
	trigger reaction.ReactionTriggerType,
	rctx reaction.ReactionContext,
	sourceUID string,
	reactionFn reaction.ReactionCallback,
	reactionTimeout time.Duration,
	reactRegistry *reaction.ReactionRegistry,
) []RoundEvent {
	combatant := findCombatantByID(cbt, uid)
	if combatant == nil || combatant.ReactionBudget == nil {
		return nil
	}
	if reactionTimeout <= 0 {
		reactionTimeout = DefaultReactionTimeout
	}

	// 1) Ready-first (REACTION-8).
	if cbt.ReadyRegistry != nil {
		if entry := cbt.ReadyRegistry.Consume(uid, trigger, sourceUID); entry != nil {
			if !combatant.ReactionBudget.TrySpend() {
				// Budget exhausted; the Ready entry was already consumed from the
				// registry by Consume. Emit a fizzled event so callers know the
				// Ready was skipped rather than silently dropped.
				return []RoundEvent{{
					ActorID:   uid,
					ActorName: combatant.Name,
					Narrative: fmt.Sprintf("[%s] %s's ready is skipped: reaction budget exhausted.", EventTypeReadyFizzled, combatant.Name),
				}}
			}
			if !revalidateReadyEntry(cbt, entry) {
				combatant.ReactionBudget.Refund()
				return []RoundEvent{{
					ActorID:   uid,
					ActorName: combatant.Name,
					Narrative: fmt.Sprintf("[%s] %s's ready fizzles: target no longer valid.", EventTypeReadyFizzled, combatant.Name),
				}}
			}
			// Inline execution of the prepared action is deferred to the
			// gameserver layer; emit an event carrying the ReadyEntry pointer.
			return []RoundEvent{{
				ActorID:    uid,
				ActorName:  combatant.Name,
				Narrative:  fmt.Sprintf("[%s] %s's ready triggers (%s).", EventTypeReactionFired, combatant.Name, entry.Action.Type),
				ReadyEntry: entry,
			}}
		}
	}

	// 2) Feat reactions.
	if reactionFn == nil {
		return nil
	}
	var candidates []reaction.PlayerReaction
	if reactRegistry != nil {
		candidates = reactRegistry.Filter(uid, trigger, func(req string) bool {
			// Per-requirement checks are the callback's responsibility; the
			// pre-filter here accepts any registered entry.
			return true
		})
		if len(candidates) == 0 {
			return nil
		}
	}
	if !combatant.ReactionBudget.TrySpend() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), reactionTimeout)
	defer cancel()
	spent, chosen, err := reactionFn(ctx, uid, trigger, rctx, candidates)
	if err != nil && ctx.Err() != context.DeadlineExceeded {
		combatant.ReactionBudget.Refund()
		return nil
	}
	if !spent || chosen == nil {
		combatant.ReactionBudget.Refund()
		return nil
	}
	return []RoundEvent{{
		ActorID:   uid,
		ActorName: combatant.Name,
		Narrative: fmt.Sprintf("[%s] %s reacts with %s.", EventTypeReactionFired, combatant.Name, chosen.FeatName),
	}}
}

// revalidateReadyEntry returns true when a Ready entry's queued action is still
// valid at trigger time. Used by fireTrigger to filter out entries whose target
// has been killed or removed since enqueue. Currently validates "attack"-type
// Ready actions: the named target must be alive in cbt. Other action types are
// treated as always-valid (the downstream resolver handles their sanity checks).
func revalidateReadyEntry(cbt *Combat, entry *reaction.ReadyEntry) bool {
	if entry == nil {
		return false
	}
	if entry.Action.Type == "attack" && entry.Action.Target != "" {
		for _, c := range cbt.Combatants {
			if c.Name == entry.Action.Target && !c.IsDead() {
				return true
			}
		}
		return false
	}
	return true
}

// reactionDispatchFn is the callback type the sub-resolvers use to fire
// reactions. It returns any RoundEvents the caller must append.
type reactionDispatchFn func(uid string, trigger reaction.ReactionTriggerType, rctx reaction.ReactionContext) []RoundEvent

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
// coverDegrader(roomID, equipID) is called when an attack misses the target but would have
// hit without the cover penalty (i.e., the cover absorbed the shot). Returns true when the
// cover is destroyed. May be nil (no-op; treated as always returning false).
//
// reactionTimeout bounds the reaction-prompt callback (REACTION-13). Non-positive
// values fall back to DefaultReactionTimeout.
func ResolveRound(cbt *Combat, src Source, targetUpdater func(id string, hp int), reactionFn reaction.ReactionCallback, reactionTimeout time.Duration, coverDegraderArgs ...func(roomID, equipID string) bool) []RoundEvent {
	if targetUpdater == nil {
		targetUpdater = func(id string, hp int) {}
	}
	coverDegrader := func(roomID, equipID string) bool { return false }
	if len(coverDegraderArgs) > 0 && coverDegraderArgs[0] != nil {
		coverDegrader = coverDegraderArgs[0]
	}
	// fireReaction dispatches via fireTrigger so Ready entries win over feat
	// reactions (REACTION-8). Any events emitted by fireTrigger are appended
	// to the outer events slice by the caller via the returned slice.
	fireReaction := func(uid string, trigger reaction.ReactionTriggerType, rctx reaction.ReactionContext) []RoundEvent {
		return fireTrigger(cbt, uid, trigger, rctx, rctx.SourceUID, reactionFn, reactionTimeout, nil)
	}

	var events []RoundEvent

	// Snapshot start-of-round positions for simultaneous resolution.
	// When resolving stride "toward"/"away", use the position the opponent
	// occupied when actions were committed, not where they moved to after
	// acting earlier in the same round. This eliminates positional foreknowledge.
	type gridPos struct{ x, y int }
	startPos := make(map[string]gridPos, len(cbt.Combatants))
	for _, c := range cbt.Combatants {
		startPos[c.ID] = gridPos{c.GridX, c.GridY}
	}

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
			case ActionStride:
				dir := action.Direction
				if dir == "" {
					dir = "toward"
				}
				width := cbt.GridWidth
				if width == 0 {
					width = 10
				}
				height := cbt.GridHeight
				if height == 0 {
					height = 10
				}

				// Find the first living opponent (used for "toward"/"away" legacy directions).
				// Use the start-of-round snapshot position so later-acting combatants
				// target where opponents were when actions committed, not where they
				// moved to after acting earlier this round (simultaneous resolution).
				var opponent *Combatant
				for _, c := range cbt.Combatants {
					if c.ID != actor.ID && c.Kind != actor.Kind && !c.IsDead() {
						snap := *c
						if sp, ok := startPos[c.ID]; ok {
							snap.GridX = sp.x
							snap.GridY = sp.y
						}
						opponent = &snap
						break
					}
				}

				// TERRAIN-6/7/8: Move up to SpeedBudget() points per stride. Each cell
				// consumed costs EntryCost(newX, newY); TerrainGreaterDifficult is
				// impassable. SpeedBudget defaults to 5 (25 ft) when SpeedFt is 0.
				// Each step recomputes direction for "toward"/"away" since position changes.
				budget := actor.SpeedBudget()
				strideNarrative := fmt.Sprintf("%s strides %s.", actor.Name, dir)
				stepsTaken := 0
				for budget > 0 {
					// REQ-STRIDE-STOP: For "toward" strides, stop when already adjacent (≤ 5 ft).
					if dir == "toward" && opponent != nil && CombatRange(*actor, *opponent) <= 5 {
						break
					}

					dx, dy := CompassDelta(dir, actor, opponent)
					if dx == 0 && dy == 0 {
						break
					}

					newX := actor.GridX + dx
					newY := actor.GridY + dy
					// Clamp to grid bounds.
					if newX < 0 {
						newX = 0
					} else if newX >= width {
						newX = width - 1
					}
					if newY < 0 {
						newY = 0
					} else if newY >= height {
						newY = height - 1
					}
					// Stop if clamping produced no movement (actor is at a grid edge in this direction).
					if newX == actor.GridX && newY == actor.GridY {
						break
					}
					// REQ-STRIDE-NOOVERLAP: Do not move onto a cell occupied by
					// another living combatant or a cover object (GH #227).
					if CellBlocked(cbt, actor.ID, newX, newY) {
						break
					}
					// TERRAIN-8/17: Greater-difficult cells are impassable.
					cost, passable := cbt.EntryCost(newX, newY)
					if !passable {
						if stepsTaken == 0 {
							strideNarrative = fmt.Sprintf("%s tries to move but the terrain blocks the way.", actor.Name)
						} else {
							strideNarrative = fmt.Sprintf("%s strides %s and stops at the terrain.", actor.Name, dir)
						}
						break
					}
					// TERRAIN-7/18: Insufficient budget to enter the next cell.
					if cost > budget {
						if stepsTaken == 0 {
							strideNarrative = fmt.Sprintf("%s cannot afford to move — not enough movement speed.", actor.Name)
						} else {
							strideNarrative = fmt.Sprintf("%s strides %s and stops — terrain too rough to continue.", actor.Name, dir)
						}
						break
					}
					budget -= cost
					actor.GridX = newX
					actor.GridY = newY
					stepsTaken++

					// TERRAIN-9: fire on_enter hazard for hazardous cells.
					if tc := cbt.TerrainAt(newX, newY); tc.Type == TerrainHazardous && tc.Hazard != nil {
						events = append(events, applyCellHazard(cbt, actor, tc, "on_enter", src)...)
					}

					// REQ-RXN19: TriggerOnEnemyMoveAdjacent fires when an NPC moves into melee range of a player.
					if actor.Kind == KindNPC {
						for _, c := range cbt.Combatants {
							if c.Kind == KindPlayer && !c.IsDead() {
								if CombatRange(*actor, *c) <= 5 {
									events = append(events, fireReaction(c.ID, reaction.TriggerOnEnemyMoveAdjacent, reaction.ReactionContext{
										TriggerUID: c.ID,
										SourceUID:  actor.ID,
									})...)
								}
							}
						}
					}
				}
				events = append(events, RoundEvent{
					ActionType: ActionStride,
					ActorID:    actor.ID,
					ActorName:  actor.Name,
					Narrative:  strideNarrative,
				})

			case ActionMoveTraitStride:
				// WMOVE-7/12: a free Stride granted by the Mobile weapon trait.
				// Resolves like a normal Stride but: (a) does not consume AP
				// (already enforced at queue time via QueueFreeAction), (b) does
				// NOT provoke reactive strikes for the moving actor, (c) does
				// NOT fire TriggerOnEnemyMoveAdjacent for adjacent players when
				// the mover is an NPC. Movement is bounded by SpeedBudget()
				// (WMOVE-G1) and travels toward (TargetX, TargetY).
				width := cbt.GridWidth
				if width == 0 {
					width = 10
				}
				height := cbt.GridHeight
				if height == 0 {
					height = 10
				}
				targetX := int(action.TargetX)
				targetY := int(action.TargetY)
				budget := actor.SpeedBudget()
				stepsTaken := 0
				moveNarrative := fmt.Sprintf("%s strides freely (move trait).", actor.Name)
				for budget > 0 {
					if actor.GridX == targetX && actor.GridY == targetY {
						break
					}
					dx, dy := towardDelta(actor.GridX, actor.GridY, targetX, targetY)
					if dx == 0 && dy == 0 {
						break
					}
					newX := actor.GridX + dx
					newY := actor.GridY + dy
					if newX < 0 {
						newX = 0
					} else if newX >= width {
						newX = width - 1
					}
					if newY < 0 {
						newY = 0
					} else if newY >= height {
						newY = height - 1
					}
					if newX == actor.GridX && newY == actor.GridY {
						break
					}
					if CellBlocked(cbt, actor.ID, newX, newY) {
						break
					}
					cost, passable := cbt.EntryCost(newX, newY)
					if !passable {
						break
					}
					if cost > budget {
						break
					}
					budget -= cost
					actor.GridX = newX
					actor.GridY = newY
					stepsTaken++
					if tc := cbt.TerrainAt(newX, newY); tc.Type == TerrainHazardous && tc.Hazard != nil {
						events = append(events, applyCellHazard(cbt, actor, tc, "on_enter", src)...)
					}
					// WMOVE-12: explicitly do NOT fire TriggerOnEnemyMoveAdjacent
					// here — the Mobile trait suppresses reaction triggers on
					// the granted movement.
				}
				if stepsTaken == 0 {
					moveNarrative = fmt.Sprintf("%s does not move (move trait).", actor.Name)
				}
				events = append(events, RoundEvent{
					ActionType: ActionMoveTraitStride,
					ActorID:    actor.ID,
					ActorName:  actor.Name,
					Narrative:  moveNarrative,
				})

			case ActionPass:
				events = append(events, RoundEvent{
					ActionType: ActionPass,
					ActorID:    actor.ID,
					ActorName:  actor.Name,
					Narrative:  fmt.Sprintf("%s passes.", actor.Name),
				})
				// Clear combat-start flat_footed from NPC combatants after their first
				// action resolves (sucker_punch window). Mid-round flat_footed (crit,
				// Feint) has no "combat_start" source and is expired by Tick at the
				// start of the target's next round, not here.
				if actor.Kind == KindNPC && cbt.Conditions[actor.ID] != nil &&
					cbt.Conditions[actor.ID].Source("flat_footed") == "combat_start" {
					cbt.Conditions[actor.ID].Remove(actor.ID, "flat_footed")
					SyncConditionRemove(actor, "flat_footed")
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
				// DETECT-19: Strike/Attack is an auditory action.
				actor.MadeSoundThisRound = true
				// Hidden flat check: NPC attacking a hidden player must pass DC 11.
				if actor.Kind == KindNPC && target.Kind == KindPlayer && target.Hidden {
					target.Hidden = false // being targeted always breaks concealment
					// Mirror the legacy clear into the per-pair map (DETECT-3).
					if cbt.DetectionStates != nil {
						cbt.DetectionStates.Clear(actor.ID, target.ID)
					}
					flatRoll := src.Intn(20) + 1
					if flatRoll <= 10 {
						events = append(events, RoundEvent{
							ActionType: ActionAttack,
							ActorID:    actor.ID,
							ActorName:  actor.Name,
							Narrative:  fmt.Sprintf("%s attacks %s but fails to locate them (flat check %d)!", actor.Name, target.Name, flatRoll),
						})
						continue
					}
					// Flat check passed — attack proceeds normally against now-revealed target.
				}
				// Hidden NPC flat check: player attacking a hidden NPC must pass DC 11,
				// unless the NPC has been revealed by Seek (RevealedUntilRound > cbt.Round).
				if actor.Kind == KindPlayer && target.Kind == KindNPC && target.Hidden && target.RevealedUntilRound <= cbt.Round {
					flatRoll := src.Intn(20) + 1
					if flatRoll <= 10 {
						events = append(events, RoundEvent{
							ActionType: ActionAttack,
							ActorID:    actor.ID,
							ActorName:  actor.Name,
							TargetID:   target.ID,
							Narrative:  fmt.Sprintf("%s swings at %s but can't locate them (flat check %d)!", actor.Name, target.Name, flatRoll),
						})
						continue
					}
					// Flat check passed — attack proceeds, hidden is cleared.
					target.Hidden = false
					if cbt.DetectionStates != nil {
						cbt.DetectionStates.Clear(actor.ID, target.ID)
					}
				}
				// Range enforcement: determine weapon type and enforce distance rules.
				var mainHandDef *inventory.WeaponDef
				if actor.Loadout != nil && actor.Loadout.MainHand != nil {
					mainHandDef = actor.Loadout.MainHand.Def
				}
				dist := CombatRange(*actor, *target)
				isMelee := mainHandDef == nil || mainHandDef.RangeIncrement == 0
				if isMelee {
					// Melee weapons (including unarmed) cannot attack beyond 5ft.
					if dist > 5 {
						events = append(events, RoundEvent{
							ActionType: ActionAttack,
							ActorID:    actor.ID,
							ActorName:  actor.Name,
							Narrative:  fmt.Sprintf("%s swings but %s is out of melee range.", actor.Name, target.Name),
						})
						if actor.Kind == KindNPC && cbt.Conditions[actor.ID] != nil &&
							cbt.Conditions[actor.ID].Source("flat_footed") == "combat_start" {
							cbt.Conditions[actor.ID].Remove(actor.ID, "flat_footed")
							SyncConditionRemove(actor, "flat_footed")
						}
						continue
					}
				} else {
					// Ranged weapon: enforce extreme range (beyond 4x RangeIncrement).
					if dist > 4*mainHandDef.RangeIncrement {
						events = append(events, RoundEvent{
							ActionType: ActionAttack,
							ActorID:    actor.ID,
							ActorName:  actor.Name,
							Narrative:  fmt.Sprintf("%s fires but %s is at extreme range.", actor.Name, target.Name),
						})
						if actor.Kind == KindNPC && cbt.Conditions[actor.ID] != nil &&
							cbt.Conditions[actor.ID].Source("flat_footed") == "combat_start" {
							cbt.Conditions[actor.ID].Remove(actor.ID, "flat_footed")
							SyncConditionRemove(actor, "flat_footed")
						}
						continue
					}
				}

				// COVER-4: apply ephemeral cover bonus before resolving AC.
				coverTier := DetermineCoverTier(target)
				if coverTier > NoCover && target.Effects != nil {
					target.Effects.Apply(BuildCoverEffect(coverTier))
				}

				atkBonus := effect.Resolve(actor.Effects, effect.StatAttack).Total
				// COVER-8: AC bonus moves to effectiveAC, no longer subtracted from AttackTotal.
				acBonus := effect.Resolve(target.Effects, effect.StatAC).Total

				// Flanking check: gather all living combatants on the attacker's side.
				// Flanking only applies to melee attacks (attacker must be adjacent).
				flanked := false
				if isMelee {
					var alliesAndSelf []Combatant
					alliesAndSelf = append(alliesAndSelf, *actor)
					for _, c := range cbt.Combatants {
						if c.Kind == actor.Kind && c.ID != actor.ID && !c.IsDead() {
							alliesAndSelf = append(alliesAndSelf, *c)
						}
					}
					flanked = IsFlanked(*target, alliesAndSelf)
				}

				var r AttackResult
				if !isMelee {
					// Ranged attack: compute range increments from combat distance.
					ri := 0
					if dist > mainHandDef.RangeIncrement {
						ri = (dist - mainHandDef.RangeIncrement) / mainHandDef.RangeIncrement
					}
					r = ResolveFirearmAttack(actor, target, mainHandDef, ri, src)
				} else {
					r = ResolveAttack(actor, target, src)
				}
				r.AttackTotal += atkBonus
				r.AttackTotal += actor.InitiativeBonus
				// GH #232: cross-action Multiple Attack Penalty.
				r.AttackTotal += mapPenaltyFor(actor.AttacksMadeThisRound)
				actor.AttacksMadeThisRound++
				if flanked {
					r.AttackTotal += 2
				}
				effectiveAC := target.AC + target.InitiativeBonus + acBonus
				r.AttackTotal = hookAttackRoll(cbt, actor, target, r.AttackTotal)
				r.Outcome = OutcomeFor(r.AttackTotal, effectiveAC)
				// COVER-11/12: cover absorb-miss — target was hit only because of cover.
				if (r.Outcome == Failure || r.Outcome == CritFailure) &&
					coverTier > NoCover && target.CoverEquipmentID != "" {
					coverACMag, _ := CoverBonus(coverTier)
					if coverACMag > 0 && r.AttackTotal >= effectiveAC-coverACMag {
						coverEquipID := target.CoverEquipmentID
						destroyed := coverDegrader(cbt.RoomID, coverEquipID)
						events = append(events, RoundEvent{
							ActionType:       ActionCoverHit,
							ActorID:          actor.ID,
							ActorName:        actor.Name,
							TargetID:         target.ID,
							CoverEquipmentID: coverEquipID,
							Narrative:        fmt.Sprintf("%s's attack hits %s's cover!", actor.Name, target.Name),
						})
						if destroyed {
							events = append(events, RoundEvent{
								ActionType: ActionCoverDestroy,
								ActorID:    actor.ID,
								ActorName:  actor.Name,
								TargetID:   target.ID,
								Narrative:  fmt.Sprintf("%s's cover is destroyed!", target.Name),
							})
						}
					}
				}
				// COVER-13: remove ephemeral cover effect after resolution.
				if coverTier > NoCover && target.Effects != nil {
					target.Effects.Remove(CoverSourceID(coverTier), "")
				}
				// MULT-17: damage now flows through ResolveDamage. Roll any extra weapon dice up
				// front; crit-doubling is handled by the pipeline's DamageMultiplier stage.
				var extraDiceRolled int
				if extraDice := condition.ExtraWeaponDice(cbt.Conditions[actor.ID]); extraDice > 0 && (r.Outcome == CritSuccess || r.Outcome == Success) {
					dieSides := 6 // unarmed fallback
					if mainHandDef != nil && mainHandDef.DamageDice != "" {
						if expr, parseErr := dice.Parse(mainHandDef.DamageDice); parseErr == nil {
							dieSides = expr.Sides
						}
					}
					for i := 0; i < extraDice; i++ {
						extraDiceRolled += src.Intn(dieSides) + 1
					}
				}
				// applyPassiveFeats gates on dmg > 0 to distinguish hit from miss; pass
				// BaseDamage on a hit, 0 on a miss. The hook still fires either way and
				// actor.Hidden is cleared regardless.
				passiveFeatGate := 0
				if r.Outcome == CritSuccess || r.Outcome == Success {
					passiveFeatGate = r.BaseDamage
				}
				passiveFeatBonus := applyPassiveFeats(cbt, actor, target, passiveFeatGate, src)
				di := BuildDamageInput(BuildDamageOpts{
					Actor:             actor,
					Target:            target,
					AttackResult:      r,
					ConditionDmgBonus: effect.Resolve(actor.Effects, effect.StatDamage).Total,
					WeaponModBonus:    weaponModifierDamageBonus(actor), // REQ-EM-23
					ExtraDiceRolled:   extraDiceRolled,
					PassiveFeatBonus:  passiveFeatBonus,
				})
				dmgResult := ResolveDamage(di)
				dmg := hookDamageRoll(cbt, actor, target, dmgResult.Final)
				var rwAnnotations []string
				// MULT-14: derive narrative annotations for resistance/weakness so the
				// "(...)" suffix on attack narratives still surfaces target defenses to the player.
				for _, step := range dmgResult.Breakdown {
					switch step.Stage {
					case StageWeakness:
						rwAnnotations = append(rwAnnotations, fmt.Sprintf("weakness +%d", step.Delta))
					case StageResistance:
						rwAnnotations = append(rwAnnotations, fmt.Sprintf("resistance %d", -step.Delta))
					}
				}
				// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
				if target.Kind == KindPlayer && dmg > 0 {
					events = append(events, fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
						TriggerUID:    target.ID,
						SourceUID:     actor.ID,
						DamagePending: &dmg,
					})...)
				}
				if dmg > 0 {
					target.ApplyDamage(dmg)
					targetUpdater(target.ID, target.CurrentHP)
					if actor.Kind == KindPlayer && target.Kind == KindNPC {
						cbt.RecordDamage(actor.ID, dmg)
					}
					// DETECT-27: damage advances the target's awareness of the
					// attacker one rung up the detection ladder.
					if cbt.DetectionStates != nil {
						detection.AdvanceTowardObserved(cbt.DetectionStates, target.ID, actor.ID)
					}
					// REQ-RXN19: TriggerOnAllyDamaged fires for other players when a player ally takes damage.
					if target.Kind == KindPlayer {
						for _, c := range cbt.Combatants {
							if c.Kind == KindPlayer && c.ID != target.ID && !c.IsDead() {
								events = append(events, fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
									TriggerUID:    c.ID,
									SourceUID:     actor.ID,
									DamagePending: nil,
								})...)
							}
						}
					}
				}
				condNotes := applyAttackConditions(cbt, actor, target, r)
				attackVerb1 := actor.AttackVerb
				if attackVerb1 == "" {
					attackVerb1 = "attacks"
				}
				narrative := attackNarrative(actor.Name, attackVerb1, target.Name, r.WeaponName, r.Outcome, r.AttackRoll, r.AttackTotal, effectiveAC, dmg)
				if len(rwAnnotations) > 0 {
					narrative += " (" + strings.Join(rwAnnotations, "; ") + ")"
				}
				if flanked {
					narrative += " (flanking +2)"
				}
				for _, note := range condNotes {
					narrative += " " + note
				}
				events = append(events, RoundEvent{
					AttackResult:    &r,
					ActionType:      ActionAttack,
					ActorID:         actor.ID,
					ActorName:       actor.Name,
					Narrative:       narrative,
					Flanking:        flanked,
					DamageBreakdown: FormatBreakdownInline(dmgResult.Breakdown),
					BreakdownSteps:  dmgResult.Breakdown,
				})
				// Consume one round of ammo for ranged attacks.
				if !isMelee && actor.Loadout != nil {
					if eq := actor.Loadout.MainHand; eq != nil && eq.Magazine != nil {
						_ = eq.Magazine.Consume(1)
					}
				}
				// Clear combat-start flat_footed from NPC combatants after their first
				// action resolves (sucker_punch window). Mid-round flat_footed (crit,
				// Feint) has no "combat_start" source and is expired by Tick at the
				// start of the target's next round, not here.
				if actor.Kind == KindNPC && cbt.Conditions[actor.ID] != nil &&
					cbt.Conditions[actor.ID].Source("flat_footed") == "combat_start" {
					cbt.Conditions[actor.ID].Remove(actor.ID, "flat_footed")
					SyncConditionRemove(actor, "flat_footed")
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
				// DETECT-19: Strike is an auditory action.
				actor.MadeSoundThisRound = true
				// Hidden flat check: NPC striking a hidden player must pass DC 11.
				// On failure, skip BOTH strikes with a single flat-check-fail event.
				if actor.Kind == KindNPC && target.Kind == KindPlayer && target.Hidden {
					target.Hidden = false // being targeted always breaks concealment
					if cbt.DetectionStates != nil {
						cbt.DetectionStates.Clear(actor.ID, target.ID)
					}
					flatRoll := src.Intn(20) + 1
					if flatRoll <= 10 {
						events = append(events, RoundEvent{
							ActionType: ActionStrike,
							ActorID:    actor.ID,
							ActorName:  actor.Name,
							Narrative:  fmt.Sprintf("%s strikes at %s but fails to locate them (flat check %d)!", actor.Name, target.Name, flatRoll),
						})
						continue
					}
					// Flat check passed — both strikes proceed normally against now-revealed target.
				}
				// First strike
				// COVER-4: apply ephemeral cover bonus before resolving AC.
				coverTier1 := DetermineCoverTier(target)
				if coverTier1 > NoCover && target.Effects != nil {
					target.Effects.Apply(BuildCoverEffect(coverTier1))
				}
				atkBonus1 := effect.Resolve(actor.Effects, effect.StatAttack).Total
				// COVER-8: AC bonus moves to effectiveAC, no longer subtracted from AttackTotal.
				acBonus1 := effect.Resolve(target.Effects, effect.StatAC).Total
				r1 := ResolveAttack(actor, target, src)
				r1.AttackTotal += atkBonus1
				r1.AttackTotal += actor.InitiativeBonus
				// GH #232: cross-action MAP. Within a Strike the first attack uses
				// the counter value at entry; the second uses the incremented value.
				r1.AttackTotal += mapPenaltyFor(actor.AttacksMadeThisRound)
				actor.AttacksMadeThisRound++
				effectiveAC1 := target.AC + target.InitiativeBonus + acBonus1
				r1.AttackTotal = hookAttackRoll(cbt, actor, target, r1.AttackTotal)
				r1.Outcome = OutcomeFor(r1.AttackTotal, effectiveAC1)
				// COVER-11/12: cover absorb-miss — first strike was a miss only because of cover.
				if (r1.Outcome == Failure || r1.Outcome == CritFailure) &&
					coverTier1 > NoCover && target.CoverEquipmentID != "" {
					coverACMag1, _ := CoverBonus(coverTier1)
					if coverACMag1 > 0 && r1.AttackTotal >= effectiveAC1-coverACMag1 {
						coverEquipID1 := target.CoverEquipmentID
						destroyed1 := coverDegrader(cbt.RoomID, coverEquipID1)
						events = append(events, RoundEvent{
							ActionType:       ActionCoverHit,
							ActorID:          actor.ID,
							ActorName:        actor.Name,
							TargetID:         target.ID,
							CoverEquipmentID: coverEquipID1,
							Narrative:        fmt.Sprintf("%s's strike hits %s's cover!", actor.Name, target.Name),
						})
						if destroyed1 {
							events = append(events, RoundEvent{
								ActionType: ActionCoverDestroy,
								ActorID:    actor.ID,
								ActorName:  actor.Name,
								TargetID:   target.ID,
								Narrative:  fmt.Sprintf("%s's cover is destroyed!", target.Name),
							})
						}
					}
				}
				// COVER-13: remove ephemeral cover effect after first-strike resolution.
				if coverTier1 > NoCover && target.Effects != nil {
					target.Effects.Remove(CoverSourceID(coverTier1), "")
				}
				// MULT-17: damage now flows through ResolveDamage.
				passiveFeatGate1 := 0
				if r1.Outcome == CritSuccess || r1.Outcome == Success {
					passiveFeatGate1 = r1.BaseDamage
				}
				passiveFeatBonus1 := applyPassiveFeats(cbt, actor, target, passiveFeatGate1, src)
				di1 := BuildDamageInput(BuildDamageOpts{
					Actor:             actor,
					Target:            target,
					AttackResult:      r1,
					ConditionDmgBonus: effect.Resolve(actor.Effects, effect.StatDamage).Total,
					WeaponModBonus:    weaponModifierDamageBonus(actor), // REQ-EM-23
					PassiveFeatBonus:  passiveFeatBonus1,
				})
				dmgResult1 := ResolveDamage(di1)
				dmg1 := hookDamageRoll(cbt, actor, target, dmgResult1.Final)
				var rwAnnotations1 []string
				// MULT-14: derive narrative annotations for resistance/weakness so the
				// "(...)" suffix on attack narratives still surfaces target defenses to the player.
				for _, step := range dmgResult1.Breakdown {
					switch step.Stage {
					case StageWeakness:
						rwAnnotations1 = append(rwAnnotations1, fmt.Sprintf("weakness +%d", step.Delta))
					case StageResistance:
						rwAnnotations1 = append(rwAnnotations1, fmt.Sprintf("resistance %d", -step.Delta))
					}
				}
				// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
				if target.Kind == KindPlayer && dmg1 > 0 {
					events = append(events, fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
						TriggerUID:    target.ID,
						SourceUID:     actor.ID,
						DamagePending: &dmg1,
					})...)
				}
				if dmg1 > 0 {
					target.ApplyDamage(dmg1)
					targetUpdater(target.ID, target.CurrentHP)
					if actor.Kind == KindPlayer && target.Kind == KindNPC {
						cbt.RecordDamage(actor.ID, dmg1)
					}
					// DETECT-27: damage advances the target's awareness of the
					// attacker one rung up the detection ladder.
					if cbt.DetectionStates != nil {
						detection.AdvanceTowardObserved(cbt.DetectionStates, target.ID, actor.ID)
					}
					// REQ-RXN19: TriggerOnAllyDamaged fires for other players when a player ally takes damage.
					if target.Kind == KindPlayer {
						for _, c := range cbt.Combatants {
							if c.Kind == KindPlayer && c.ID != target.ID && !c.IsDead() {
								events = append(events, fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
									TriggerUID:    c.ID,
									SourceUID:     actor.ID,
									DamagePending: nil,
								})...)
							}
						}
					}
				}
				condNotes1 := applyAttackConditions(cbt, actor, target, r1)
				strikeVerb1 := actor.AttackVerb
				if strikeVerb1 == "" {
					strikeVerb1 = "strikes"
				}
				narrative1 := attackNarrative(actor.Name, strikeVerb1, target.Name, r1.WeaponName, r1.Outcome, r1.AttackRoll, r1.AttackTotal, effectiveAC1, dmg1)
				if len(rwAnnotations1) > 0 {
					narrative1 += " (" + strings.Join(rwAnnotations1, "; ") + ")"
				}
				for _, note := range condNotes1 {
					narrative1 += " " + note
				}
				events = append(events, RoundEvent{
					AttackResult:    &r1,
					ActionType:      ActionStrike,
					ActorID:         actor.ID,
					ActorName:       actor.Name,
					Narrative:       narrative1,
					DamageBreakdown: FormatBreakdownInline(dmgResult1.Breakdown),
					BreakdownSteps:  dmgResult1.Breakdown,
				})
				// Clear combat-start flat_footed from NPC combatants after their first
				// action resolves (sucker_punch window). Mid-round flat_footed (crit,
				// Feint) has no "combat_start" source and is expired by Tick at the
				// start of the target's next round, not here.
				if actor.Kind == KindNPC && cbt.Conditions[actor.ID] != nil &&
					cbt.Conditions[actor.ID].Source("flat_footed") == "combat_start" {
					cbt.Conditions[actor.ID].Remove(actor.ID, "flat_footed")
					SyncConditionRemove(actor, "flat_footed")
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
				// COVER-4: apply ephemeral cover bonus before resolving AC for the second strike.
				coverTier2 := DetermineCoverTier(target)
				if coverTier2 > NoCover && target.Effects != nil {
					target.Effects.Apply(BuildCoverEffect(coverTier2))
				}
				atkBonus2 := effect.Resolve(actor.Effects, effect.StatAttack).Total
				// COVER-8: AC bonus moves to effectiveAC, no longer subtracted from AttackTotal.
				acBonus2 := effect.Resolve(target.Effects, effect.StatAC).Total
				r2 := ResolveAttack(actor, target, src)
				r2.AttackTotal += atkBonus2
				r2.AttackTotal += actor.InitiativeBonus
				// GH #232: cross-action MAP. The counter now reflects the first
				// strike, so this attack picks up -5 (or -10 if there were prior
				// attack actions this round).
				mapBonus2 := mapPenaltyFor(actor.AttacksMadeThisRound)
				r2.AttackTotal += mapBonus2
				actor.AttacksMadeThisRound++
				// REQ-BUG121: snap_shot waives the MAP penalty when the first strike missed.
				if (r1.Outcome == Failure || r1.Outcome == CritFailure) && cbt.sessionGetter != nil {
					if ps, ok := cbt.sessionGetter(actor.ID); ok && ps.PassiveFeats["snap_shot"] {
						r2.AttackTotal -= mapBonus2
					}
				}
				effectiveAC2 := target.AC + target.InitiativeBonus + acBonus2
				r2.AttackTotal = hookAttackRoll(cbt, actor, target, r2.AttackTotal)
				r2.Outcome = OutcomeFor(r2.AttackTotal, effectiveAC2)
				// COVER-11/12: cover absorb-miss — second strike was a miss only because of cover.
				if (r2.Outcome == Failure || r2.Outcome == CritFailure) &&
					coverTier2 > NoCover && target.CoverEquipmentID != "" {
					coverACMag2, _ := CoverBonus(coverTier2)
					if coverACMag2 > 0 && r2.AttackTotal >= effectiveAC2-coverACMag2 {
						coverEquipID2 := target.CoverEquipmentID
						destroyed2 := coverDegrader(cbt.RoomID, coverEquipID2)
						events = append(events, RoundEvent{
							ActionType:       ActionCoverHit,
							ActorID:          actor.ID,
							ActorName:        actor.Name,
							TargetID:         target.ID,
							CoverEquipmentID: coverEquipID2,
							Narrative:        fmt.Sprintf("%s's strike hits %s's cover!", actor.Name, target.Name),
						})
						if destroyed2 {
							events = append(events, RoundEvent{
								ActionType: ActionCoverDestroy,
								ActorID:    actor.ID,
								ActorName:  actor.Name,
								TargetID:   target.ID,
								Narrative:  fmt.Sprintf("%s's cover is destroyed!", target.Name),
							})
						}
					}
				}
				// COVER-13: remove ephemeral cover effect after second-strike resolution.
				if coverTier2 > NoCover && target.Effects != nil {
					target.Effects.Remove(CoverSourceID(coverTier2), "")
				}
				// MULT-17: damage now flows through ResolveDamage.
				passiveFeatGate2 := 0
				if r2.Outcome == CritSuccess || r2.Outcome == Success {
					passiveFeatGate2 = r2.BaseDamage
				}
				passiveFeatBonus2 := applyPassiveFeats(cbt, actor, target, passiveFeatGate2, src)
				di2 := BuildDamageInput(BuildDamageOpts{
					Actor:             actor,
					Target:            target,
					AttackResult:      r2,
					ConditionDmgBonus: effect.Resolve(actor.Effects, effect.StatDamage).Total,
					WeaponModBonus:    weaponModifierDamageBonus(actor), // REQ-EM-23
					PassiveFeatBonus:  passiveFeatBonus2,
				})
				dmgResult2 := ResolveDamage(di2)
				dmg2 := hookDamageRoll(cbt, actor, target, dmgResult2.Final)
				var rwAnnotations2 []string
				// MULT-14: derive narrative annotations for resistance/weakness so the
				// "(...)" suffix on attack narratives still surfaces target defenses to the player.
				for _, step := range dmgResult2.Breakdown {
					switch step.Stage {
					case StageWeakness:
						rwAnnotations2 = append(rwAnnotations2, fmt.Sprintf("weakness +%d", step.Delta))
					case StageResistance:
						rwAnnotations2 = append(rwAnnotations2, fmt.Sprintf("resistance %d", -step.Delta))
					}
				}
				// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
				if target.Kind == KindPlayer && dmg2 > 0 {
					events = append(events, fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
						TriggerUID:    target.ID,
						SourceUID:     actor.ID,
						DamagePending: &dmg2,
					})...)
				}
				if dmg2 > 0 {
					target.ApplyDamage(dmg2)
					targetUpdater(target.ID, target.CurrentHP)
					if actor.Kind == KindPlayer && target.Kind == KindNPC {
						cbt.RecordDamage(actor.ID, dmg2)
					}
					// REQ-RXN19: TriggerOnAllyDamaged fires for other players when a player ally takes damage.
					if target.Kind == KindPlayer {
						for _, c := range cbt.Combatants {
							if c.Kind == KindPlayer && c.ID != target.ID && !c.IsDead() {
								events = append(events, fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
									TriggerUID:    c.ID,
									SourceUID:     actor.ID,
									DamagePending: nil,
								})...)
							}
						}
					}
				}
				condNotes2 := applyAttackConditions(cbt, actor, target, r2)
				strikeVerb2 := actor.AttackVerb
				if strikeVerb2 == "" {
					strikeVerb2 = "strikes"
				}
				narrative2 := attackNarrative(actor.Name, strikeVerb2, target.Name, r2.WeaponName, r2.Outcome, r2.AttackRoll, r2.AttackTotal, effectiveAC2, dmg2)
				if len(rwAnnotations2) > 0 {
					narrative2 += " (" + strings.Join(rwAnnotations2, "; ") + ")"
				}
				for _, note := range condNotes2 {
					narrative2 += " " + note
				}
				events = append(events, RoundEvent{
					AttackResult:    &r2,
					ActionType:      ActionStrike,
					ActorID:         actor.ID,
					ActorName:       actor.Name,
					Narrative:       narrative2,
					DamageBreakdown: FormatBreakdownInline(dmgResult2.Breakdown),
					BreakdownSteps:  dmgResult2.Breakdown,
				})
			case ActionReload:
				events = append(events, resolveReload(cbt, actor, action))
			case ActionFireBurst:
				events = append(events, resolveFireBurst(cbt, actor, action, src, coverDegrader, fireReaction)...)
			case ActionFireAutomatic:
				events = append(events, resolveFireAutomatic(cbt, actor, action, src, coverDegrader, fireReaction)...)
			case ActionThrow:
				events = append(events, resolveThrow(cbt, actor, action, src, fireReaction)...)
			case ActionAid:
				target := findCombatantByNameOrID(cbt, action.Target)
				if target == nil || target.IsDead() {
					events = append(events, RoundEvent{
						ActionType: ActionAid,
						ActorID:    actor.ID,
						ActorName:  actor.Name,
						Narrative:  fmt.Sprintf("%s tries to aid but %s is already down.", actor.Name, action.Target),
					})
					continue
				}
				d20 := src.Intn(20) + 1
				bestMod := actor.GritMod
				if actor.QuicknessMod > bestMod {
					bestMod = actor.QuicknessMod
				}
				if actor.SavvyMod > bestMod {
					bestMod = actor.SavvyMod
				}
				total := d20 + bestMod
				outcome := AidOutcome(total)
				var narrative string
				switch outcome {
				case "critical_success":
					applyConditionIfAllowed(cbt, target.ID, "aided_strong", 1, 1)
					narrative = fmt.Sprintf("%s provides critical aid to %s (total %d)!", actor.Name, target.Name, total)
				case "success":
					applyConditionIfAllowed(cbt, target.ID, "aided", 1, 1)
					narrative = fmt.Sprintf("%s aids %s (total %d).", actor.Name, target.Name, total)
				case "failure":
					narrative = fmt.Sprintf("%s fails to aid %s (total %d).", actor.Name, target.Name, total)
				case "critical_failure":
					applyConditionIfAllowed(cbt, target.ID, "aided_penalty", 1, 1)
					narrative = fmt.Sprintf("%s fumbles the aid attempt on %s (total %d)!", actor.Name, target.Name, total)
				}
				events = append(events, RoundEvent{
					ActionType: ActionAid,
					ActorID:    actor.ID,
					ActorName:  actor.Name,
					TargetID:   target.ID,
					Narrative:  narrative,
				})
			case ActionUseTech:
				// Record the tech use for post-round effect resolution.
				// Effects are applied by the server after round resolution via techUseResolverFn.
				events = append(events, RoundEvent{
					ActionType: ActionUseTech,
					ActorID:    actor.ID,
					ActorName:  actor.Name,
					TargetID:   action.Target,
					AbilityID:  action.AbilityID,
					TargetX:    action.TargetX,
					TargetY:    action.TargetY,
					Narrative:  fmt.Sprintf("%s uses %s.", actor.Name, action.AbilityID),
				})
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
// coverDegrader is called when a shot misses the target but would have hit without cover; may be nil.
func resolveFireBurst(cbt *Combat, actor *Combatant, qa QueuedAction, src Source, coverDegrader func(roomID, equipID string) bool, fireReaction reactionDispatchFn) []RoundEvent {
	target := findCombatantByNameOrID(cbt, qa.Target)
	if target == nil || target.IsDead() {
		return []RoundEvent{{ActionType: ActionFireBurst, ActorID: actor.ID, ActorName: actor.Name,
			Narrative: fmt.Sprintf("%s fires burst but target not found.", actor.Name)}}
	}
	if coverDegrader == nil {
		coverDegrader = func(roomID, equipID string) bool { return false }
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
		// COVER-4: apply ephemeral cover bonus before resolving AC.
		coverTierBurst := DetermineCoverTier(target)
		if coverTierBurst > NoCover && target.Effects != nil {
			target.Effects.Apply(BuildCoverEffect(coverTierBurst))
		}
		// COVER-9: include attacker's StatAttack effect bonus (was missing prior to #247).
		atkBonus := effect.Resolve(actor.Effects, effect.StatAttack).Total
		result.AttackTotal += atkBonus
		// COVER-8: AC bonus moves to effectiveAC, no longer subtracted from AttackTotal.
		acBonus := effect.Resolve(target.Effects, effect.StatAC).Total
		// GH #232: each burst shot counts toward cross-action MAP.
		result.AttackTotal += mapPenaltyFor(actor.AttacksMadeThisRound)
		actor.AttacksMadeThisRound++
		effectiveAC := target.AC + target.InitiativeBonus + acBonus
		result.Outcome = OutcomeFor(result.AttackTotal, effectiveAC)
		// COVER-11/12: cover absorb-miss — burst shot was a miss only because of cover.
		if (result.Outcome == Failure || result.Outcome == CritFailure) &&
			coverTierBurst > NoCover && target.CoverEquipmentID != "" {
			coverACMagBurst, _ := CoverBonus(coverTierBurst)
			if coverACMagBurst > 0 && result.AttackTotal >= effectiveAC-coverACMagBurst {
				coverEquipIDBurst := target.CoverEquipmentID
				destroyed := coverDegrader(cbt.RoomID, coverEquipIDBurst)
				events = append(events, RoundEvent{
					ActionType:       ActionCoverHit,
					ActorID:          actor.ID,
					ActorName:        actor.Name,
					TargetID:         target.ID,
					CoverEquipmentID: coverEquipIDBurst,
					Narrative:        fmt.Sprintf("%s's burst fire hits %s's cover!", actor.Name, target.Name),
				})
				if destroyed {
					events = append(events, RoundEvent{
						ActionType: ActionCoverDestroy,
						ActorID:    actor.ID,
						ActorName:  actor.Name,
						TargetID:   target.ID,
						Narrative:  fmt.Sprintf("%s's cover is destroyed!", target.Name),
					})
				}
			}
		}
		// COVER-13: remove ephemeral cover effect after resolution.
		if coverTierBurst > NoCover && target.Effects != nil {
			target.Effects.Remove(CoverSourceID(coverTierBurst), "")
		}
		// MULT-17: damage now flows through ResolveDamage.
		di := BuildDamageInput(BuildDamageOpts{
			Actor:        actor,
			Target:       target,
			AttackResult: result,
		})
		dmgResult := ResolveDamage(di)
		dmg := hookDamageRoll(cbt, actor, target, dmgResult.Final)
		_ = dmgResult // breakdown reserved for Task 6 narrative work
		// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
		if target.Kind == KindPlayer && dmg > 0 {
			events = append(events, fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
				TriggerUID:    target.ID,
				SourceUID:     actor.ID,
				DamagePending: &dmg,
			})...)
		}
		if dmg > 0 {
			target.ApplyDamage(dmg)
			if actor.Kind == KindPlayer && target.Kind == KindNPC {
				cbt.RecordDamage(actor.ID, dmg)
			}
			// REQ-RXN19: TriggerOnAllyDamaged fires for other players when a player ally takes damage.
			if target.Kind == KindPlayer {
				for _, c := range cbt.Combatants {
					if c.Kind == KindPlayer && c.ID != target.ID && !c.IsDead() {
						events = append(events, fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
							TriggerUID:    c.ID,
							SourceUID:     actor.ID,
							DamagePending: nil,
						})...)
					}
				}
			}
		}
		if weapon != nil && actor.Loadout != nil {
			if eq := actor.Loadout.MainHand; eq != nil && eq.Magazine != nil {
				_ = eq.Magazine.Consume(1)
			}
		}
		result.BaseDamage = dmg
		events = append(events, RoundEvent{
			AttackResult:    &result,
			ActionType:      ActionFireBurst,
			ActorID:         actor.ID,
			ActorName:       actor.Name,
			Narrative:       buildNarrative(actor, target, result, dmg),
			DamageBreakdown: FormatBreakdownInline(dmgResult.Breakdown),
			BreakdownSteps:  dmgResult.Breakdown,
		})
		if target.IsDead() {
			break
		}
	}
	return events
}

// resolveFireAutomatic handles ActionFireAutomatic: one attack against each living enemy (up to 3).
// coverDegrader is called when a shot misses a target but would have hit without cover; may be nil.
func resolveFireAutomatic(cbt *Combat, actor *Combatant, qa QueuedAction, src Source, coverDegrader func(roomID, equipID string) bool, fireReaction reactionDispatchFn) []RoundEvent {
	enemies := livingEnemiesOf(cbt, actor)
	if len(enemies) == 0 {
		return []RoundEvent{{ActionType: ActionFireAutomatic, ActorID: actor.ID, ActorName: actor.Name,
			Narrative: fmt.Sprintf("%s lays down suppressive fire.", actor.Name)}}
	}
	if coverDegrader == nil {
		coverDegrader = func(roomID, equipID string) bool { return false }
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
		// COVER-4: apply ephemeral cover bonus before resolving AC.
		coverTierAutomatic := DetermineCoverTier(target)
		if coverTierAutomatic > NoCover && target.Effects != nil {
			target.Effects.Apply(BuildCoverEffect(coverTierAutomatic))
		}
		// COVER-9: include attacker's StatAttack effect bonus (was missing prior to #247).
		atkBonus := effect.Resolve(actor.Effects, effect.StatAttack).Total
		result.AttackTotal += atkBonus
		// COVER-8: AC bonus moves to effectiveAC, no longer subtracted from AttackTotal.
		acBonus := effect.Resolve(target.Effects, effect.StatAC).Total
		// GH #232: each automatic-fire shot counts toward cross-action MAP.
		result.AttackTotal += mapPenaltyFor(actor.AttacksMadeThisRound)
		actor.AttacksMadeThisRound++
		effectiveAC := target.AC + target.InitiativeBonus + acBonus
		result.Outcome = OutcomeFor(result.AttackTotal, effectiveAC)
		// COVER-11/12: cover absorb-miss — automatic shot was a miss only because of cover.
		if (result.Outcome == Failure || result.Outcome == CritFailure) &&
			coverTierAutomatic > NoCover && target.CoverEquipmentID != "" {
			coverACMagAutomatic, _ := CoverBonus(coverTierAutomatic)
			if coverACMagAutomatic > 0 && result.AttackTotal >= effectiveAC-coverACMagAutomatic {
				coverEquipIDAutomatic := target.CoverEquipmentID
				destroyed := coverDegrader(cbt.RoomID, coverEquipIDAutomatic)
				events = append(events, RoundEvent{
					ActionType:       ActionCoverHit,
					ActorID:          actor.ID,
					ActorName:        actor.Name,
					TargetID:         target.ID,
					CoverEquipmentID: coverEquipIDAutomatic,
					Narrative:        fmt.Sprintf("%s's automatic fire hits %s's cover!", actor.Name, target.Name),
				})
				if destroyed {
					events = append(events, RoundEvent{
						ActionType: ActionCoverDestroy,
						ActorID:    actor.ID,
						ActorName:  actor.Name,
						TargetID:   target.ID,
						Narrative:  fmt.Sprintf("%s's cover is destroyed!", target.Name),
					})
				}
			}
		}
		// COVER-13: remove ephemeral cover effect after resolution.
		if coverTierAutomatic > NoCover && target.Effects != nil {
			target.Effects.Remove(CoverSourceID(coverTierAutomatic), "")
		}
		// MULT-17: damage now flows through ResolveDamage.
		di := BuildDamageInput(BuildDamageOpts{
			Actor:        actor,
			Target:       target,
			AttackResult: result,
		})
		dmgResult := ResolveDamage(di)
		dmg := hookDamageRoll(cbt, actor, target, dmgResult.Final)
		_ = dmgResult // breakdown reserved for Task 6 narrative work
		// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
		if target.Kind == KindPlayer && dmg > 0 {
			events = append(events, fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
				TriggerUID:    target.ID,
				SourceUID:     actor.ID,
				DamagePending: &dmg,
			})...)
		}
		if dmg > 0 {
			target.ApplyDamage(dmg)
			if actor.Kind == KindPlayer && target.Kind == KindNPC {
				cbt.RecordDamage(actor.ID, dmg)
			}
			// REQ-RXN19: TriggerOnAllyDamaged fires for other players when a player ally takes damage.
			if target.Kind == KindPlayer {
				for _, c := range cbt.Combatants {
					if c.Kind == KindPlayer && c.ID != target.ID && !c.IsDead() {
						events = append(events, fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
							TriggerUID:    c.ID,
							SourceUID:     actor.ID,
							DamagePending: nil,
						})...)
					}
				}
			}
		}
		if weapon != nil && actor.Loadout != nil {
			if eq := actor.Loadout.MainHand; eq != nil && eq.Magazine != nil {
				_ = eq.Magazine.Consume(1)
			}
		}
		result.BaseDamage = dmg
		shots--
		events = append(events, RoundEvent{
			AttackResult:    &result,
			ActionType:      ActionFireAutomatic,
			ActorID:         actor.ID,
			ActorName:       actor.Name,
			Narrative:       buildNarrative(actor, target, result, dmg),
			DamageBreakdown: FormatBreakdownInline(dmgResult.Breakdown),
			BreakdownSteps:  dmgResult.Breakdown,
		})
	}
	return events
}

// explosiveTargetsOf returns living targets for an explosive based on its FriendlyFire flag.
//
// Precondition: cbt, actor, and grenade must be non-nil.
// Postcondition: Returns all living non-actor combatants; if grenade.FriendlyFire is false,
// only enemy-kind (different Kind from actor) combatants are returned.
func explosiveTargetsOf(cbt *Combat, actor *Combatant, grenade *inventory.ExplosiveDef) []*Combatant {
	var out []*Combatant
	for _, c := range cbt.Combatants {
		if c.IsDead() || c.ID == actor.ID {
			continue
		}
		if !grenade.FriendlyFire && c.Kind == actor.Kind {
			continue
		}
		out = append(out, c)
	}
	return out
}

// resolveThrow handles ActionThrow: explosive area effect against all living enemies.
func resolveThrow(cbt *Combat, actor *Combatant, qa QueuedAction, src Source, fireReaction reactionDispatchFn) []RoundEvent {
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
	targets := explosiveTargetsOf(cbt, actor, grenade)
	effectiveDC := grenade.SaveDC + actor.Level
	results := ResolveExplosive(grenade, targets, effectiveDC, src)
	var events []RoundEvent
	for i, r := range results {
		target := targets[i]
		// REQ-RXN19: TriggerOnSaveFail / TriggerOnSaveCritFail for explosive saves.
		if target.Kind == KindPlayer {
			switch r.SaveResult {
			case Failure:
				events = append(events, fireReaction(target.ID, reaction.TriggerOnSaveFail, reaction.ReactionContext{
					TriggerUID: target.ID,
					SourceUID:  actor.ID,
				})...)
			case CritFailure:
				events = append(events, fireReaction(target.ID, reaction.TriggerOnSaveCritFail, reaction.ReactionContext{
					TriggerUID: target.ID,
					SourceUID:  actor.ID,
				})...)
			}
		}
		if r.BaseDamage > 0 {
			target.ApplyDamage(r.BaseDamage)
			if actor.Kind == KindPlayer && target.Kind == KindNPC {
				cbt.RecordDamage(actor.ID, r.BaseDamage)
			}
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

// weaponModifierDamageBonus returns the flat damage adjustment for the actor's
// equipped main-hand weapon modifier (REQ-EM-23):
//
//   - "tuned"    → +1
//   - "defective" → -1
//   - "cursed"   → -2
//   - ""         → 0
//
// Precondition: actor may have a nil Loadout or nil MainHand.
// Postcondition: Returns 0 when no loadout or no modifier is set.
func weaponModifierDamageBonus(actor *Combatant) int {
	if actor.Loadout == nil || actor.Loadout.MainHand == nil {
		return 0
	}
	switch actor.Loadout.MainHand.Modifier {
	case "tuned":
		return 1
	case "defective":
		return -1
	case "cursed":
		return -2
	}
	return 0
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

// applyCellHazard fires a hazard on victim if its trigger matches.
// Routes damage through ResolveDamage (#246 pipeline).
// Returns zero or more RoundEvents (damage narrative, condition narrative, etc.).
//
// Precondition: tc.Type == TerrainHazardous; tc.Hazard must not be nil.
func applyCellHazard(_ *Combat, victim *Combatant, tc TerrainCell, trigger string, src Source) []RoundEvent {
	if tc.Hazard == nil || tc.Hazard.Def == nil {
		return nil
	}
	def := tc.Hazard.Def
	if def.Trigger != trigger {
		return nil
	}
	var events []RoundEvent
	if def.DamageExpr != "" {
		rollResult, err := dice.RollExpr(def.DamageExpr, src)
		if err == nil && rollResult.Total() > 0 {
			in := DamageInput{
				Additives: []DamageAdditive{
					{Label: def.ID, Value: rollResult.Total(), Source: "hazard:" + def.ID},
				},
				DamageType: def.DamageType,
				Weakness:   victim.WeaknessFor(def.DamageType),
				Resistance: victim.ResistanceFor(def.DamageType),
			}
			result := ResolveDamage(in)
			victim.ApplyDamage(result.Final)
			narrative := fmt.Sprintf("%s is hit by %s! (%s → %d damage)",
				victim.Name, def.ID, def.DamageExpr, result.Final)
			if def.Message != "" {
				narrative = fmt.Sprintf("%s — %s (%s → %d damage)", def.Message, victim.Name, def.DamageExpr, result.Final)
			}
			events = append(events, RoundEvent{
				ActionType: ActionHazardDamage,
				ActorID:    victim.ID,
				ActorName:  victim.Name,
				Damage:     result.Final,
				Narrative:  narrative,
			})
		}
	}
	return events
}

// ApplyCellHazardForTest exposes applyCellHazard for package-external tests.
func ApplyCellHazardForTest(cbt *Combat, victim *Combatant, tc TerrainCell, trigger string, src Source) []RoundEvent {
	return applyCellHazard(cbt, victim, tc, trigger, src)
}
