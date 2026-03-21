package gameserver

import (
	"fmt"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// actionAliasesReady maps user-supplied action strings to canonical action keys.
var actionAliasesReady = map[string]string{
	"strike": "strike", "attack": "strike", "atk": "strike",
	"step": "step", "move": "step",
	"raise_shield": "raise_shield", "shield": "raise_shield", "rs": "raise_shield",
}

// triggerAliasesReady maps user-supplied trigger strings to canonical trigger keys.
var triggerAliasesReady = map[string]string{
	"enemy_enters": "enemy_enters", "enters": "enemy_enters", "enter": "enemy_enters",
	"enemy_attacks_me": "enemy_attacks_me", "attacks": "enemy_attacks_me", "attacked": "enemy_attacks_me",
	"ally_attacked": "ally_attacked", "ally": "ally_attacked",
}

// actionNamesReady maps canonical action keys to display names.
var actionNamesReady = map[string]string{
	"strike": "Strike", "step": "Step", "raise_shield": "Raise Shield",
}

// triggerDescriptionsReady maps canonical trigger keys to human-readable descriptions.
var triggerDescriptionsReady = map[string]string{
	"enemy_enters":     "an enemy enters the room",
	"enemy_attacks_me": "an enemy attacks you",
	"ally_attacked":    "an ally is attacked",
}

// handleReady processes a ready action request, spending 2 AP to set a (trigger, action) pair
// that fires as a Reaction during the current round.
//
// Precondition: uid identifies a connected player; req.Action and req.Trigger are non-empty strings.
// Postcondition: On success, 2 AP are spent and sess.ReadiedTrigger/ReadiedAction are set.
func (s *GameServiceServer) handleReady(uid string, req *gamev1.ReadyRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("You are not in the game."), nil
	}

	if sess.Status != statusInCombat {
		return messageEvent("You must be in combat to ready an action."), nil
	}

	if s.combatH.RemainingAP(uid) < 2 {
		return messageEvent("You need at least 2 AP to ready an action."), nil
	}

	canonicalAction, actionOK := actionAliasesReady[req.Action]
	if !actionOK {
		return messageEvent(fmt.Sprintf("Unknown action '%s'. Valid: strike, step, raise_shield.", req.Action)), nil
	}

	canonicalTrigger, triggerOK := triggerAliasesReady[req.Trigger]
	if !triggerOK {
		return messageEvent(fmt.Sprintf("Unknown trigger '%s'. Valid: enemy_enters, enemy_attacks_me, ally_attacked.", req.Trigger)), nil
	}

	if sess.ReadiedTrigger != "" {
		return messageEvent("You already have a readied action. Wait for the round to end."), nil
	}

	if err := s.combatH.SpendAP(uid, 2); err != nil {
		return messageEvent("Not enough AP to ready an action."), nil
	}

	sess.ReadiedTrigger = canonicalTrigger
	sess.ReadiedAction = canonicalAction

	msg := fmt.Sprintf("You ready a %s for when %s.", actionNamesReady[canonicalAction], triggerDescriptionsReady[canonicalTrigger])
	return messageEvent(msg), nil
}
