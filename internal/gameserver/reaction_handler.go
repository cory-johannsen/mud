package gameserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/reaction"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// pushServerEventToUID marshals evt and pushes it onto the player's entity
// stream, mirroring (*CombatHandler).pushMessageToUID but for any
// ServerEvent oneof. Returns true iff the event was successfully enqueued;
// false means the player session or entity was unreachable.
func (s *GameServiceServer) pushServerEventToUID(uid string, evt *gamev1.ServerEvent) bool {
	if s == nil || s.sessions == nil || evt == nil {
		return false
	}
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok || sess == nil || sess.Entity == nil {
		return false
	}
	data, err := proto.Marshal(evt)
	if err != nil {
		return false
	}
	return sess.Entity.Push(data) == nil
}

// triggerDescriptions provides human-readable descriptions for each trigger type.
var triggerDescriptions = map[reaction.ReactionTriggerType]string{
	reaction.TriggerOnSaveFail:          "you failed a saving throw",
	reaction.TriggerOnSaveCritFail:      "you critically failed a saving throw",
	reaction.TriggerOnDamageTaken:       "you are about to take damage",
	reaction.TriggerOnEnemyMoveAdjacent: "an enemy moved adjacent to you",
	reaction.TriggerOnConditionApplied:  "a condition is being applied to you",
	reaction.TriggerOnAllyDamaged:       "an ally took damage",
	reaction.TriggerOnFall:              "you are about to fall",
}

// CheckReactionRequirement returns true if sess meets the given requirement string.
//
// Precondition: sess must not be nil.
// Postcondition: Returns true when req is empty or "none". Returns false for unknown requirements.
func CheckReactionRequirement(sess *session.PlayerSession, req string) bool {
	switch req {
	case "", "none":
		return true
	case "wielding_melee_weapon":
		if sess.LoadoutSet == nil {
			return false
		}
		preset := sess.LoadoutSet.ActivePreset()
		if preset == nil || preset.MainHand == nil || preset.MainHand.Def == nil {
			return false
		}
		return preset.MainHand.Def.IsMelee()
	case "wielding_shield":
		if sess.LoadoutSet == nil {
			return false
		}
		preset := sess.LoadoutSet.ActivePreset()
		if preset == nil || preset.OffHand == nil || preset.OffHand.Def == nil {
			return false
		}
		return preset.OffHand.Def.IsShield()
	default:
		return false
	}
}

// ApplyReactionEffect executes the reaction effect, modifying ctx in place.
//
// Precondition: sess and ctx must not be nil.
// Postcondition: ctx is modified according to effect.Type; no side effects on unknown types.
func ApplyReactionEffect(sess *session.PlayerSession, effect reaction.ReactionEffect, ctx *reaction.ReactionContext) {
	switch effect.Type {
	case reaction.ReactionEffectRerollSave:
		if ctx.SaveOutcome == nil {
			return
		}
		// Only "better" (or empty, defaulting to "better") keep strategy is supported.
		// Unknown values are treated as no-op to avoid silent data contract violations.
		if effect.Keep != "" && effect.Keep != "better" {
			return
		}
		// Reroll: generate new outcome in [0,3]. Keep the better (lower) value.
		// 0=CritSuccess, 1=Success, 2=Failure, 3=CritFailure.
		reroll := mrand.Intn(4)
		if reroll < *ctx.SaveOutcome {
			*ctx.SaveOutcome = reroll
		}
	case reaction.ReactionEffectReduceDamage:
		if ctx.DamagePending == nil {
			return
		}
		hardness := shieldHardness(sess)
		*ctx.DamagePending -= hardness
		if *ctx.DamagePending < 0 {
			*ctx.DamagePending = 0
		}
	case reaction.ReactionEffectStrike:
		// Strike execution deferred to sub-project 2 (Reactive Strike).
	}
}

// shieldHardness returns the hardness of the player's equipped off-hand shield, or 0.
//
// Precondition: sess must not be nil.
// Postcondition: Returns a non-negative integer.
func shieldHardness(sess *session.PlayerSession) int {
	if sess.LoadoutSet == nil {
		return 0
	}
	preset := sess.LoadoutSet.ActivePreset()
	if preset == nil || preset.OffHand == nil || preset.OffHand.Def == nil {
		return 0
	}
	if !preset.OffHand.Def.IsShield() {
		return 0
	}
	return preset.OffHand.Def.Hardness
}

// matchesReadyTrigger reports whether the player's readied trigger string corresponds to the
// fired reaction trigger type.
//
// Precondition: readiedTrigger is the string stored in PlayerSession.ReadiedTrigger.
// Postcondition: Returns true iff the strings are semantically equivalent.
func matchesReadyTrigger(readiedTrigger string, firedTrigger reaction.ReactionTriggerType) bool {
	switch readiedTrigger {
	case "enemy_enters":
		return firedTrigger == reaction.TriggerOnEnemyEntersRoom
	case "enemy_attacks_me":
		return firedTrigger == reaction.TriggerOnDamageTaken
	case "ally_attacked":
		return firedTrigger == reaction.TriggerOnAllyDamaged
	default:
		return false
	}
}

// executeReadiedStrike fires the readied Strike action.
//
// Precondition: s, uid, and sess must not be nil; sess.ReadiedAction == "strike".
// Postcondition: Strike is queued against the first NPC combatant found; a message is pushed to the player.
func executeReadiedStrike(s *GameServiceServer, uid string, sess *session.PlayerSession) {
	if s.combatH == nil {
		s.pushMessageToUID(uid, "Your readied Strike finds no target.")
		return
	}
	cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
	if !ok {
		s.pushMessageToUID(uid, "Your readied Strike finds no target.")
		return
	}
	target := ""
	for _, c := range cbt.Combatants {
		if c.Kind == combat.KindNPC {
			target = c.ID
			break
		}
	}
	if target == "" {
		s.pushMessageToUID(uid, "Your readied Strike finds no target.")
		return
	}
	events, err := s.combatH.Strike(uid, target)
	if err != nil {
		s.pushMessageToUID(uid, "Your readied Strike fires but misses: "+err.Error())
		return
	}
	for _, evt := range events {
		s.pushMessageToUID(uid, "Readied Strike: "+evt.Narrative)
	}
}

// executeReadiedStep fires the readied Step action.
//
// Precondition: s, uid, and sess must not be nil; sess.ReadiedAction == "step".
// Postcondition: Player combatant's position advances 5 ft toward the NPC; a message is pushed.
func executeReadiedStep(s *GameServiceServer, uid string, sess *session.PlayerSession) {
	if s.combatH == nil {
		s.pushMessageToUID(uid, "Your readied Step fires.")
		return
	}
	cbt, ok := s.combatH.GetCombatForRoom(sess.RoomID)
	if !ok {
		s.pushMessageToUID(uid, "Your readied Step fires.")
		return
	}
	combatant := cbt.GetCombatant(uid)
	if combatant == nil {
		s.pushMessageToUID(uid, "Your readied Step fires — you move 5 feet.")
		return
	}
	// Find nearest living enemy and move one grid cell toward them.
	var nearestEnemy *combat.Combatant
	for _, c := range cbt.Combatants {
		if c.Kind != combatant.Kind && !c.IsDead() {
			if nearestEnemy == nil || combat.CombatRange(*combatant, *c) < combat.CombatRange(*combatant, *nearestEnemy) {
				nearestEnemy = c
			}
		}
	}
	dx, dy := combat.CompassDelta("toward", combatant, nearestEnemy)
	width, height := cbt.GridWidth, cbt.GridHeight
	if width == 0 {
		width = 10
	}
	if height == 0 {
		height = 10
	}
	newX := combatant.GridX + dx
	newY := combatant.GridY + dy
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
	combatant.GridX = newX
	combatant.GridY = newY
	s.pushMessageToUID(uid, "Your readied Step fires — you move 5 feet.")
}

// executeReadiedRaiseShield fires the readied Raise Shield action.
//
// Precondition: s, uid, and sess must not be nil; sess.ReadiedAction == "raise_shield".
// Postcondition: Shield bonus is applied; a message is pushed to the player.
func executeReadiedRaiseShield(s *GameServiceServer, uid string, sess *session.PlayerSession) {
	if s.combatH != nil {
		if err := s.combatH.ApplyCombatantACMod(uid, uid, +2); err != nil {
			s.pushMessageToUID(uid, "Your readied Raise Shield failed.")
			return
		}
	}
	s.pushMessageToUID(uid, "Your readied Raise Shield fires — shield bonus applied.")
}

// executeReadiedAction dispatches the readied action stored in sess.
//
// Precondition: s, uid, and sess must not be nil; sess.ReadiedAction is non-empty.
// Postcondition: The appropriate action is executed and a message is pushed to the player.
func executeReadiedAction(s *GameServiceServer, uid string, sess *session.PlayerSession) {
	switch sess.ReadiedAction {
	case "strike":
		executeReadiedStrike(s, uid, sess)
	case "step":
		executeReadiedStep(s, uid, sess)
	case "raise_shield":
		executeReadiedRaiseShield(s, uid, sess)
	}
}

// newPromptID returns an opaque unique identifier for a reaction prompt.
// Uses crypto/rand for non-predictability (prevents a client from spoofing
// prompt IDs to unblock other players' callbacks).
//
// Postcondition: returns a non-empty string on success. On read failure a
// timestamp-based fallback is used so the callback cannot wedge on randomness
// exhaustion.
func newPromptID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("rxn-%d", time.Now().UnixNano())
	}
	return "rxn-" + hex.EncodeToString(b[:])
}

// buildReactionCallback constructs the ReactionCallback for a player session.
//
// Precondition: uid and sess must not be nil.
// Postcondition: Returns a ReactionCallback that, when candidates is non-empty,
// emits a ReactionPromptEvent over the player's gRPC stream and blocks until
// either (a) the player responds with a ReactionResponse (routed via
// reactionPromptHub) or (b) the caller's ctx fires. On response, the chosen
// reaction is applied via ApplyReactionEffect; on deadline or decline the
// callback returns (false, nil, nil).
func (s *GameServiceServer) buildReactionCallback(
	uid string,
	sess *session.PlayerSession,
) reaction.ReactionCallback {
	return func(
		ctx context.Context,
		triggerUID string,
		trigger reaction.ReactionTriggerType,
		rctx reaction.ReactionContext,
		candidates []reaction.PlayerReaction,
	) (bool, *reaction.PlayerReaction, error) {
		if triggerUID != uid {
			return false, nil, nil
		}
		// Fire readied action if trigger matches (no prompt required).
		if sess.ReadiedTrigger != "" && matchesReadyTrigger(sess.ReadiedTrigger, trigger) {
			executeReadiedAction(s, uid, sess)
			clearReadiedAction(sess)
		}
		if sess.ReactionsRemaining <= 0 {
			return false, nil, nil
		}
		// Requirement-filter candidates: the registry's pre-filter accepts all
		// entries and defers requirement checks to this callback.
		valid := make([]reaction.PlayerReaction, 0, len(candidates))
		for _, c := range candidates {
			if CheckReactionRequirement(sess, c.Def.Requirement) {
				valid = append(valid, c)
			}
		}
		if len(valid) == 0 {
			return false, nil, nil
		}

		chosen := s.promptReaction(ctx, uid, valid)
		if chosen == nil {
			return false, nil, nil
		}

		sess.ReactionsRemaining--
		desc, ok := triggerDescriptions[trigger]
		if !ok {
			desc = string(trigger)
		}
		damageBeforeEffect := 0
		if rctx.DamagePending != nil {
			damageBeforeEffect = *rctx.DamagePending
		}
		// rctx is passed by value but DamagePending and SaveOutcome are pointers into the caller's
		// data. ApplyReactionEffect mutates through these pointers, so effects propagate to the
		// caller.
		ApplyReactionEffect(sess, chosen.Def.Effect, &rctx)

		switch chosen.Def.Effect.Type {
		case reaction.ReactionEffectReduceDamage:
			if rctx.DamagePending != nil {
				blocked := damageBeforeEffect - *rctx.DamagePending
				if *rctx.DamagePending <= 0 {
					s.pushMessageToUID(uid, fmt.Sprintf("Reaction — %s triggered (%s): fully blocked the attack.", chosen.FeatName, desc))
				} else {
					s.pushMessageToUID(uid, fmt.Sprintf("Reaction — %s triggered (%s): blocked %d damage.", chosen.FeatName, desc, blocked))
				}
			}
		case reaction.ReactionEffectRerollSave:
			s.pushMessageToUID(uid, fmt.Sprintf("Reaction — %s triggered (%s): saving throw rerolled, keeping better result.", chosen.FeatName, desc))
		default:
			s.pushMessageToUID(uid, fmt.Sprintf("Reaction — %s triggered (%s).", chosen.FeatName, desc))
		}
		// chosen already points at a PlayerReaction copy owned by
		// promptReaction; pass it back as the caller's chosen candidate.
		return true, chosen, nil
	}
}

// promptReaction sends a ReactionPromptEvent to uid's session stream and
// blocks until a matching ReactionResponse is delivered via reactionPromptHub
// or ctx fires. Returns the chosen PlayerReaction or nil for skip/timeout.
//
// Precondition: candidates must be non-empty. The first candidate whose Feat
// matches the client's chosen option ID is returned.
// Postcondition: the hub entry for the allocated prompt_id is always
// Unregistered before return.
func (s *GameServiceServer) promptReaction(
	ctx context.Context,
	uid string,
	candidates []reaction.PlayerReaction,
) *reaction.PlayerReaction {
	if s.reactionPromptHub == nil {
		// No hub (tests may construct a bare server). Safe fallback: auto-fire
		// the first candidate, preserving the pre-#244-T13 behaviour.
		c := candidates[0]
		return &c
	}

	promptID := newPromptID()
	ch := s.reactionPromptHub.Register(promptID)
	defer s.reactionPromptHub.Unregister(promptID)

	// Build the option list from valid candidates.
	opts := make([]*gamev1.ReactionPromptOption, 0, len(candidates))
	for _, c := range candidates {
		opts = append(opts, &gamev1.ReactionPromptOption{
			Id:    c.Feat,
			Label: c.FeatName,
		})
	}

	timeout := s.reactionPromptTimeout
	if timeout <= 0 {
		// Fallback: honour the ctx deadline via select; use a visible default
		// so the client can render a countdown bar even if the server forgot
		// to call SetReactionPromptTimeout.
		timeout = 3 * time.Second
	}
	deadline := time.Now().Add(timeout).UnixMilli()

	event := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_ReactionPrompt{
			ReactionPrompt: &gamev1.ReactionPromptEvent{
				PromptId:       promptID,
				DeadlineUnixMs: deadline,
				Options:        opts,
			},
		},
	}
	if !s.pushServerEventToUID(uid, event) {
		// Could not deliver the prompt (player disconnected, entity missing).
		// Return nil so the caller refunds the reaction budget.
		return nil
	}

	select {
	case <-ctx.Done():
		return nil
	case resp := <-ch:
		if resp == nil || resp.GetChosen() == "" {
			return nil
		}
		for i := range candidates {
			if candidates[i].Feat == resp.GetChosen() {
				c := candidates[i]
				return &c
			}
		}
		return nil
	}
}
