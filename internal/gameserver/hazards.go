package gameserver

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/game/condition"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/world"
)

// HazardMessageFn is a callback that delivers a hazard message to the player.
// The message is already formatted; the callback is responsible for delivery
// (e.g. via pushMessageToUID or session entity push).
type HazardMessageFn func(msg string)

// ApplyHazards fires all hazards matching trigger for the given player session.
// For each matching hazard, damage is applied directly to sess.CurrentHP and
// messageFn is called with the hazard message (when non-empty).
//
// Precondition: room, sess must not be nil; trigger must be "on_enter" or "round_start".
// Postcondition: each matching hazard reduces sess.CurrentHP and optionally applies a condition.
func ApplyHazards(
	room *world.Room,
	sess *session.PlayerSession,
	trigger string,
	diceRoller *dice.Roller,
	condRegistry *condition.Registry,
	messageFn HazardMessageFn,
	logger *zap.Logger,
) {
	for _, hazard := range room.Hazards {
		if hazard.Trigger != trigger {
			continue
		}
		if diceRoller == nil {
			continue
		}
		dmgResult, err := diceRoller.RollExpr(hazard.DamageExpr)
		if err != nil {
			if logger != nil {
				logger.Warn("hazard dice roll failed",
					zap.String("hazard_id", hazard.ID),
					zap.String("expr", hazard.DamageExpr),
					zap.Error(err),
				)
			}
			continue
		}
		dmg := dmgResult.Total()
		if dmg < 0 {
			dmg = 0
		}
		sess.CurrentHP -= dmg
		if sess.CurrentHP < 0 {
			sess.CurrentHP = 0
		}
		if hazard.Message != "" && messageFn != nil {
			messageFn(fmt.Sprintf("%s (-%d HP)\r\n", hazard.Message, dmg))
		}
		if hazard.ConditionID != "" && condRegistry != nil && sess.Conditions != nil {
			if cond, ok := condRegistry.Get(hazard.ConditionID); ok {
				_ = sess.Conditions.Apply(sess.UID, cond, 1, -1)
			}
		}
	}
}
