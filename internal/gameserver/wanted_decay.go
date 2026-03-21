package gameserver

import (
	"context"

	"github.com/cory-johannsen/mud/internal/game/session"
	"go.uber.org/zap"
)

// SessionLister provides read access to all online player sessions.
type SessionLister interface {
	AllPlayers() []*session.PlayerSession
}

// StartWantedDecay subscribes to the calendar and decrements WantedLevel
// for all online players once per in-game day.
// It MUST be called after GameServiceServer is fully initialized.
// Precondition: cal MUST NOT be nil.
// Postcondition: returns a stop function; call it to unsubscribe and stop the goroutine.
func StartWantedDecay(cal *GameCalendar, sessions SessionLister, wantedRepo WantedSaver, logger *zap.Logger) func() {
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	var lastDecayDay int
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case dt := <-ch:
				if dt.Day == lastDecayDay {
					continue
				}
				lastDecayDay = dt.Day
				decayWantedLevels(sessions, wantedRepo, dt.Day, logger)
			case <-stop:
				cal.Unsubscribe(ch)
				return
			}
		}
	}()
	return func() { close(stop) }
}

// decayWantedLevels decrements WantedLevel by 1 for each zone where the player
// has not violated safe-room rules on or after currentDay.
//
// Precondition: sessions and wantedRepo MUST NOT be nil; currentDay MUST be >= 0.
// Postcondition: each online player's WantedLevel is decremented by 1 for zones
// not violated on currentDay; changes are persisted via wantedRepo.Upsert.
func decayWantedLevels(sessions SessionLister, wantedRepo WantedSaver, currentDay int, logger *zap.Logger) {
	for _, sess := range sessions.AllPlayers() {
		for zoneID, level := range sess.WantedLevel {
			if level <= 0 {
				continue
			}
			if sess.LastViolationDay[zoneID] >= currentDay {
				continue // violated today or in the future; no decay
			}
			newLevel := level - 1
			sess.WantedLevel[zoneID] = newLevel
			if err := wantedRepo.Upsert(context.Background(), sess.CharacterID, zoneID, newLevel); err != nil {
				logger.Warn("failed to persist wanted decay",
					zap.String("uid", sess.UID),
					zap.String("zone", zoneID),
					zap.Error(err),
				)
			}
		}
	}
}
