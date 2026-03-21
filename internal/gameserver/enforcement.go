package gameserver

import (
	"context"

	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// WantedSaver is the persistence interface for WantedLevel.
type WantedSaver interface {
	Upsert(ctx context.Context, characterID int64, zoneID string, level int) error
}

// CheckSafeViolation enforces safe-room combat rules for the given player.
//
// Preconditions:
//   - sess MUST be non-nil.
//   - zoneLevel and roomLevel MUST be valid DangerLevel strings (roomLevel may be empty).
//   - currentDay MUST be the current in-game day number.
//   - wantedRepo MUST be non-nil when the room is Safe.
//
// Postconditions:
//   - Returns nil, nil when the effective danger level is not Safe (no-op).
//   - First violation: increments SafeViolations, returns one warning CombatEvent; no DB write.
//   - Second+ violation: increments WantedLevel (capped at 4), resets SafeViolations to 0,
//     persists via wantedRepo, calls InitiateGuardCombat when combatH is non-nil, returns nil events.
//   - Returns a non-nil error only on wantedRepo.Upsert failure.
func CheckSafeViolation(
	sess *session.PlayerSession,
	zoneID string,
	zoneLevel, roomLevel string,
	currentDay int,
	combatH *CombatHandler,
	wantedRepo WantedSaver,
	broadcastFn func(roomID string, events []*gamev1.CombatEvent),
) ([]*gamev1.CombatEvent, error) {
	level := danger.EffectiveDangerLevel(zoneLevel, roomLevel)
	if level != danger.Safe {
		return nil, nil
	}

	sess.SafeViolations[zoneID]++
	sess.LastViolationDay[zoneID] = currentDay

	if sess.SafeViolations[zoneID] == 1 {
		// First violation: warn only; caller MUST NOT proceed with the attack.
		return []*gamev1.CombatEvent{
			{
				Narrative: "Warning: combat is not permitted in this area.",
			},
		}, nil
	}

	// Second+ violation: escalate WantedLevel (cap at 4), reset violation counter.
	newLevel := sess.WantedLevel[zoneID] + 1
	if newLevel > 4 {
		newLevel = 4
	}
	sess.WantedLevel[zoneID] = newLevel
	sess.SafeViolations[zoneID] = 0

	if err := wantedRepo.Upsert(context.Background(), sess.CharacterID, zoneID, newLevel); err != nil {
		return nil, err
	}

	if combatH != nil {
		combatH.InitiateGuardCombat(sess.UID, zoneID, newLevel)
	}

	return nil, nil
}
