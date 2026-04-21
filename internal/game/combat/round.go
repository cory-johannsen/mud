package combat

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/reaction"
)

// CheckReactiveStrikes returns attack events from all NPCs that are adjacent
// (≤5 ft) to mover before the stride and whose position did not change
// (i.e., they were not the one striding).
//
// Precondition: cbt non-nil; moverID non-empty; oldX/oldY is mover's grid position before stride.
// Postcondition: Returns zero or more RoundEvent{ActionType: ActionAttack} events.
// targetUpdater(id, hp) is called after each damage application; may be nil (no-op).
func CheckReactiveStrikes(cbt *Combat, moverID string, oldX, oldY int, rng Source, targetUpdater func(id string, hp int)) []RoundEvent {
	if targetUpdater == nil {
		targetUpdater = func(id string, hp int) {}
	}
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
func ResolveRound(cbt *Combat, src Source, targetUpdater func(id string, hp int), reactionFn reaction.ReactionCallback, coverDegraderArgs ...func(roomID, equipID string) bool) []RoundEvent {
	if targetUpdater == nil {
		targetUpdater = func(id string, hp int) {}
	}
	coverDegrader := func(roomID, equipID string) bool { return false }
	if len(coverDegraderArgs) > 0 && coverDegraderArgs[0] != nil {
		coverDegrader = coverDegraderArgs[0]
	}
	fireReaction := func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext) {
		if reactionFn == nil {
			return
		}
		_, _ = reactionFn(uid, trigger, ctx)
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

				// REQ-STRIDE-SPEED: Move up to SpeedSquares() cells per stride (default 5 = 25 ft).
				// Each step recomputes direction for "toward"/"away" since position changes.
				steps := actor.SpeedSquares()
				for step := 0; step < steps; step++ {
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
					// REQ-STRIDE-NOOVERLAP: Do not move onto a cell occupied by another living combatant.
					if CellOccupied(cbt, actor.ID, newX, newY) {
						break
					}
					actor.GridX = newX
					actor.GridY = newY

					// REQ-RXN19: TriggerOnEnemyMoveAdjacent fires when an NPC moves into melee range of a player.
					if actor.Kind == KindNPC {
						for _, c := range cbt.Combatants {
							if c.Kind == KindPlayer && !c.IsDead() {
								if CombatRange(*actor, *c) <= 5 {
									fireReaction(c.ID, reaction.TriggerOnEnemyMoveAdjacent, reaction.ReactionContext{
										TriggerUID: c.ID,
										SourceUID:  actor.ID,
									})
								}
							}
						}
					}
				}
				events = append(events, RoundEvent{
					ActionType: ActionStride,
					ActorID:    actor.ID,
					ActorName:  actor.Name,
					Narrative:  fmt.Sprintf("%s strides %s.", actor.Name, dir),
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
				// Hidden flat check: NPC attacking a hidden player must pass DC 11.
				if actor.Kind == KindNPC && target.Kind == KindPlayer && target.Hidden {
					target.Hidden = false // being targeted always breaks concealment
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
						}
						continue
					}
				}

				atkBonus := condition.AttackBonus(cbt.Conditions[actor.ID])
				acBonus := condition.ACBonus(cbt.Conditions[target.ID])

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
				r.AttackTotal += acBonus
				r.AttackTotal += actor.InitiativeBonus
				if flanked {
					r.AttackTotal += 2
				}
				effectiveAC := target.AC + target.InitiativeBonus
				r.AttackTotal = hookAttackRoll(cbt, actor, target, r.AttackTotal)
				r.Outcome = OutcomeFor(r.AttackTotal, effectiveAC)
				// Crossfire degradation: attack missed but would have hit without cover penalty.
				if (r.Outcome == Failure || r.Outcome == CritFailure) &&
					target.CoverTier != "" && target.CoverEquipmentID != "" {
					attackWithoutCoverPenalty := r.AttackTotal - acBonus // acBonus <= 0, so this raises the effective roll
					if attackWithoutCoverPenalty >= effectiveAC {
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
				dmg := r.EffectiveDamage()
				dmg += weaponModifierDamageBonus(actor) // REQ-EM-23
				dmg += condition.DamageBonus(cbt.Conditions[actor.ID])
				// Extra weapon dice from conditions (e.g. brutal_surge_active / Overpower).
				// Only applied on a hit or crit; crits double the extra dice as well.
				if extraDice := condition.ExtraWeaponDice(cbt.Conditions[actor.ID]); extraDice > 0 && (r.Outcome == CritSuccess || r.Outcome == Success) {
					dieSides := 6 // unarmed fallback
					if mainHandDef != nil && mainHandDef.DamageDice != "" {
						if expr, parseErr := dice.Parse(mainHandDef.DamageDice); parseErr == nil {
							dieSides = expr.Sides
						}
					}
					extraDmg := 0
					for i := 0; i < extraDice; i++ {
						extraDmg += src.Intn(dieSides) + 1
					}
					if r.Outcome == CritSuccess {
						extraDmg *= 2
					}
					dmg += extraDmg
				}
				dmg += applyPassiveFeats(cbt, actor, target, dmg, src)
				dmg = hookDamageRoll(cbt, actor, target, dmg)
				dmg, rwAnnotations := applyResistanceWeakness(target, r.DamageType, dmg)
				// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
				if target.Kind == KindPlayer && dmg > 0 {
					fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
						TriggerUID:    target.ID,
						SourceUID:     actor.ID,
						DamagePending: &dmg,
					})
				}
				if dmg > 0 {
					target.ApplyDamage(dmg)
					targetUpdater(target.ID, target.CurrentHP)
					if actor.Kind == KindPlayer && target.Kind == KindNPC {
						cbt.RecordDamage(actor.ID, dmg)
					}
					// REQ-RXN19: TriggerOnAllyDamaged fires for other players when a player ally takes damage.
					if target.Kind == KindPlayer {
						for _, c := range cbt.Combatants {
							if c.Kind == KindPlayer && c.ID != target.ID && !c.IsDead() {
								fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
									TriggerUID:    c.ID,
									SourceUID:     actor.ID,
									DamagePending: nil,
								})
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
					AttackResult: &r,
					ActionType:   ActionAttack,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    narrative,
					Flanking:     flanked,
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
				// Hidden flat check: NPC striking a hidden player must pass DC 11.
				// On failure, skip BOTH strikes with a single flat-check-fail event.
				if actor.Kind == KindNPC && target.Kind == KindPlayer && target.Hidden {
					target.Hidden = false // being targeted always breaks concealment
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
				atkBonus1 := condition.AttackBonus(cbt.Conditions[actor.ID])
				acBonus1 := condition.ACBonus(cbt.Conditions[target.ID])
				r1 := ResolveAttack(actor, target, src)
				r1.AttackTotal += atkBonus1
				r1.AttackTotal += acBonus1
				r1.AttackTotal += actor.InitiativeBonus
				effectiveAC1 := target.AC + target.InitiativeBonus
				r1.AttackTotal = hookAttackRoll(cbt, actor, target, r1.AttackTotal)
				r1.Outcome = OutcomeFor(r1.AttackTotal, effectiveAC1)
				// Crossfire degradation: first strike missed but would have hit without cover penalty.
				if (r1.Outcome == Failure || r1.Outcome == CritFailure) &&
					target.CoverTier != "" && target.CoverEquipmentID != "" {
					attackWithoutCoverPenalty1 := r1.AttackTotal - acBonus1
					if attackWithoutCoverPenalty1 >= effectiveAC1 {
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
				dmg1 := r1.EffectiveDamage()
				dmg1 += weaponModifierDamageBonus(actor) // REQ-EM-23
				dmg1 += condition.DamageBonus(cbt.Conditions[actor.ID])
				dmg1 += applyPassiveFeats(cbt, actor, target, dmg1, src)
				dmg1 = hookDamageRoll(cbt, actor, target, dmg1)
				dmg1, rwAnnotations1 := applyResistanceWeakness(target, r1.DamageType, dmg1)
				// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
				if target.Kind == KindPlayer && dmg1 > 0 {
					fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
						TriggerUID:    target.ID,
						SourceUID:     actor.ID,
						DamagePending: &dmg1,
					})
				}
				if dmg1 > 0 {
					target.ApplyDamage(dmg1)
					targetUpdater(target.ID, target.CurrentHP)
					if actor.Kind == KindPlayer && target.Kind == KindNPC {
						cbt.RecordDamage(actor.ID, dmg1)
					}
					// REQ-RXN19: TriggerOnAllyDamaged fires for other players when a player ally takes damage.
					if target.Kind == KindPlayer {
						for _, c := range cbt.Combatants {
							if c.Kind == KindPlayer && c.ID != target.ID && !c.IsDead() {
								fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
									TriggerUID:    c.ID,
									SourceUID:     actor.ID,
									DamagePending: nil,
								})
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
					AttackResult: &r1,
					ActionType:   ActionStrike,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    narrative1,
				})
				// Clear combat-start flat_footed from NPC combatants after their first
				// action resolves (sucker_punch window). Mid-round flat_footed (crit,
				// Feint) has no "combat_start" source and is expired by Tick at the
				// start of the target's next round, not here.
				if actor.Kind == KindNPC && cbt.Conditions[actor.ID] != nil &&
					cbt.Conditions[actor.ID].Source("flat_footed") == "combat_start" {
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
				// REQ-BUG121: snap_shot waives the MAP penalty when the first strike missed.
				if (r1.Outcome == Failure || r1.Outcome == CritFailure) && cbt.sessionGetter != nil {
					if ps, ok := cbt.sessionGetter(actor.ID); ok && ps.PassiveFeats["snap_shot"] {
						r2.AttackTotal += 5
					}
				}
				effectiveAC2 := target.AC + target.InitiativeBonus
				r2.AttackTotal = hookAttackRoll(cbt, actor, target, r2.AttackTotal)
				r2.Outcome = OutcomeFor(r2.AttackTotal, effectiveAC2)
				// Crossfire degradation: second strike missed but would have hit without cover penalty.
				if (r2.Outcome == Failure || r2.Outcome == CritFailure) &&
					target.CoverTier != "" && target.CoverEquipmentID != "" {
					attackWithoutCoverPenalty2 := r2.AttackTotal - acBonus2
					if attackWithoutCoverPenalty2 >= effectiveAC2 {
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
				dmg2 := r2.EffectiveDamage()
				dmg2 += weaponModifierDamageBonus(actor) // REQ-EM-23
				dmg2 += condition.DamageBonus(cbt.Conditions[actor.ID])
				dmg2 += applyPassiveFeats(cbt, actor, target, dmg2, src)
				dmg2 = hookDamageRoll(cbt, actor, target, dmg2)
				dmg2, rwAnnotations2 := applyResistanceWeakness(target, r2.DamageType, dmg2)
				// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
				if target.Kind == KindPlayer && dmg2 > 0 {
					fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
						TriggerUID:    target.ID,
						SourceUID:     actor.ID,
						DamagePending: &dmg2,
					})
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
								fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
									TriggerUID:    c.ID,
									SourceUID:     actor.ID,
									DamagePending: nil,
								})
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
					AttackResult: &r2,
					ActionType:   ActionStrike,
					ActorID:      actor.ID,
					ActorName:    actor.Name,
					Narrative:    narrative2,
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
func resolveFireBurst(cbt *Combat, actor *Combatant, qa QueuedAction, src Source, coverDegrader func(roomID, equipID string) bool, fireReaction func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext)) []RoundEvent {
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
		acBonus := condition.ACBonus(cbt.Conditions[target.ID])
		result.AttackTotal += acBonus
		effectiveAC := target.AC + target.InitiativeBonus
		result.Outcome = OutcomeFor(result.AttackTotal, effectiveAC)
		// Crossfire degradation: burst shot missed but would have hit without cover penalty.
		if (result.Outcome == Failure || result.Outcome == CritFailure) &&
			target.CoverTier != "" && target.CoverEquipmentID != "" {
			attackWithoutCoverPenalty := result.AttackTotal - acBonus
			if attackWithoutCoverPenalty >= effectiveAC {
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
		dmg := result.EffectiveDamage()
		dmg = hookDamageRoll(cbt, actor, target, dmg)
		// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
		if target.Kind == KindPlayer && dmg > 0 {
			fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
				TriggerUID:    target.ID,
				SourceUID:     actor.ID,
				DamagePending: &dmg,
			})
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
						fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
							TriggerUID:    c.ID,
							SourceUID:     actor.ID,
							DamagePending: nil,
						})
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
// coverDegrader is called when a shot misses a target but would have hit without cover; may be nil.
func resolveFireAutomatic(cbt *Combat, actor *Combatant, qa QueuedAction, src Source, coverDegrader func(roomID, equipID string) bool, fireReaction func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext)) []RoundEvent {
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
		acBonus := condition.ACBonus(cbt.Conditions[target.ID])
		result.AttackTotal += acBonus
		effectiveAC := target.AC + target.InitiativeBonus
		result.Outcome = OutcomeFor(result.AttackTotal, effectiveAC)
		// Crossfire degradation: automatic shot missed but would have hit without cover penalty.
		if (result.Outcome == Failure || result.Outcome == CritFailure) &&
			target.CoverTier != "" && target.CoverEquipmentID != "" {
			attackWithoutCoverPenalty := result.AttackTotal - acBonus
			if attackWithoutCoverPenalty >= effectiveAC {
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
		dmg := result.EffectiveDamage()
		dmg = hookDamageRoll(cbt, actor, target, dmg)
		// REQ-RXN19: TriggerOnDamageTaken fires before damage is applied so reduce_damage can modify it.
		if target.Kind == KindPlayer && dmg > 0 {
			fireReaction(target.ID, reaction.TriggerOnDamageTaken, reaction.ReactionContext{
				TriggerUID:    target.ID,
				SourceUID:     actor.ID,
				DamagePending: &dmg,
			})
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
						fireReaction(c.ID, reaction.TriggerOnAllyDamaged, reaction.ReactionContext{
							TriggerUID:    c.ID,
							SourceUID:     actor.ID,
							DamagePending: nil,
						})
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
			AttackResult: &result,
			ActionType:   ActionFireAutomatic,
			ActorID:      actor.ID,
			ActorName:    actor.Name,
			Narrative:    buildNarrative(actor, target, result, dmg),
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
func resolveThrow(cbt *Combat, actor *Combatant, qa QueuedAction, src Source, fireReaction func(uid string, trigger reaction.ReactionTriggerType, ctx reaction.ReactionContext)) []RoundEvent {
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
				fireReaction(target.ID, reaction.TriggerOnSaveFail, reaction.ReactionContext{
					TriggerUID: target.ID,
					SourceUID:  actor.ID,
				})
			case CritFailure:
				fireReaction(target.ID, reaction.TriggerOnSaveCritFail, reaction.ReactionContext{
					TriggerUID: target.ID,
					SourceUID:  actor.ID,
				})
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
