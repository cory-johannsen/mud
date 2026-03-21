package gameserver

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/session"
	"go.uber.org/zap"
)

// mockDecaySaver records all Upsert calls.
type mockDecaySaver struct {
	calls []struct {
		characterID int64
		zoneID      string
		level       int
	}
}

func (m *mockDecaySaver) Upsert(_ context.Context, characterID int64, zoneID string, level int) error {
	m.calls = append(m.calls, struct {
		characterID int64
		zoneID      string
		level       int
	}{characterID, zoneID, level})
	return nil
}

// mockSessionLister returns a fixed list of sessions.
type mockSessionLister struct {
	players []*session.PlayerSession
}

func (m *mockSessionLister) AllPlayers() []*session.PlayerSession {
	return m.players
}

// TestDecayWantedLevels_DecaysWhenNoViolationToday verifies that a player
// with WantedLevel=2 and LastViolationDay < currentDay has their level decremented.
func TestDecayWantedLevels_DecaysWhenNoViolationToday(t *testing.T) {
	const zoneID = "zone-decay-1"
	sess := &session.PlayerSession{
		UID:              "uid-decay-1",
		CharacterID:      10,
		WantedLevel:      map[string]int{zoneID: 2},
		SafeViolations:   make(map[string]int),
		LastViolationDay: map[string]int{zoneID: 3},
	}
	lister := &mockSessionLister{players: []*session.PlayerSession{sess}}
	saver := &mockDecaySaver{}
	logger := zap.NewNop()

	decayWantedLevels(lister, saver, 4, logger) // currentDay=4 > lastViolationDay=3

	if sess.WantedLevel[zoneID] != 1 {
		t.Errorf("WantedLevel[%q] = %d; want 1 (decremented)", zoneID, sess.WantedLevel[zoneID])
	}
	if len(saver.calls) != 1 {
		t.Fatalf("Upsert called %d times; want 1", len(saver.calls))
	}
	if saver.calls[0].level != 1 {
		t.Errorf("Upsert level = %d; want 1", saver.calls[0].level)
	}
}

// TestDecayWantedLevels_NoDecayWhenViolatedToday verifies that a player
// with LastViolationDay == currentDay does NOT have their level decremented.
func TestDecayWantedLevels_NoDecayWhenViolatedToday(t *testing.T) {
	const zoneID = "zone-decay-2"
	sess := &session.PlayerSession{
		UID:              "uid-decay-2",
		CharacterID:      11,
		WantedLevel:      map[string]int{zoneID: 2},
		SafeViolations:   make(map[string]int),
		LastViolationDay: map[string]int{zoneID: 5},
	}
	lister := &mockSessionLister{players: []*session.PlayerSession{sess}}
	saver := &mockDecaySaver{}
	logger := zap.NewNop()

	decayWantedLevels(lister, saver, 5, logger) // currentDay=5 == lastViolationDay=5

	if sess.WantedLevel[zoneID] != 2 {
		t.Errorf("WantedLevel[%q] = %d; want 2 (no decay)", zoneID, sess.WantedLevel[zoneID])
	}
	if len(saver.calls) != 0 {
		t.Errorf("Upsert called %d times; want 0 (no decay)", len(saver.calls))
	}
}
