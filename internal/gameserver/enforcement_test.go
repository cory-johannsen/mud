package gameserver

import (
	"context"
	"strings"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

type mockWantedSaver struct {
	upserted map[string]int
}

type spyGuardCombat struct {
	calledUID        string
	calledZoneID     string
	calledWantedLevel int
}

func (s *spyGuardCombat) InitiateGuardCombat(uid, zoneID string, wantedLevel int) {
	s.calledUID = uid
	s.calledZoneID = zoneID
	s.calledWantedLevel = wantedLevel
}

func (m *mockWantedSaver) Upsert(_ context.Context, _ int64, zoneID string, level int) error {
	if m.upserted == nil {
		m.upserted = make(map[string]int)
	}
	m.upserted[zoneID] = level
	return nil
}

func newTestSession() *session.PlayerSession {
	return &session.PlayerSession{
		UID:              "test-uid",
		CharacterID:      42,
		WantedLevel:      make(map[string]int),
		SafeViolations:   make(map[string]int),
		LastViolationDay: make(map[string]int),
	}
}

func TestCheckSafeViolation_FirstViolation_Warning(t *testing.T) {
	sess := newTestSession()
	saver := &mockWantedSaver{}
	const zoneID = "test_zone"

	events, err := CheckSafeViolation(sess, zoneID, string(danger.Safe), "", 5, nil, saver, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 warning event, got %d", len(events))
	}
	if !strings.Contains(events[0].Narrative, "Warning") {
		t.Errorf("warning event narrative %q must contain \"Warning\"", events[0].Narrative)
	}
	if sess.SafeViolations[zoneID] != 1 {
		t.Errorf("SafeViolations[%q] = %d; want 1", zoneID, sess.SafeViolations[zoneID])
	}
	if len(saver.upserted) != 0 {
		t.Errorf("Upsert called on first violation; want no DB write")
	}
}

func TestCheckSafeViolation_SecondViolation_IncrementsWanted(t *testing.T) {
	sess := newTestSession()
	sess.SafeViolations["test_zone"] = 1
	saver := &mockWantedSaver{}
	spy := &spyGuardCombat{}
	const zoneID = "test_zone"

	events, err := CheckSafeViolation(sess, zoneID, string(danger.Safe), "", 5, spy, saver, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("want no events on second violation, got %d", len(events))
	}
	if sess.WantedLevel[zoneID] != 1 {
		t.Errorf("WantedLevel[%q] = %d; want 1", zoneID, sess.WantedLevel[zoneID])
	}
	if sess.SafeViolations[zoneID] != 0 {
		t.Errorf("SafeViolations[%q] = %d; want 0 (reset)", zoneID, sess.SafeViolations[zoneID])
	}
	if saver.upserted[zoneID] != 1 {
		t.Errorf("Upsert zoneID=%q level=%d; want 1", zoneID, saver.upserted[zoneID])
	}
	if spy.calledUID != sess.UID {
		t.Errorf("InitiateGuardCombat uid=%q; want %q", spy.calledUID, sess.UID)
	}
	if spy.calledZoneID != zoneID {
		t.Errorf("InitiateGuardCombat zoneID=%q; want %q", spy.calledZoneID, zoneID)
	}
	if spy.calledWantedLevel != 1 {
		t.Errorf("InitiateGuardCombat wantedLevel=%d; want 1", spy.calledWantedLevel)
	}
}

func TestCheckSafeViolation_NonSafeRoom_NoOp(t *testing.T) {
	sess := newTestSession()
	saver := &mockWantedSaver{}
	const zoneID = "test_zone"

	events, err := CheckSafeViolation(sess, zoneID, string(danger.Sketchy), "", 5, nil, saver, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("want no events for non-safe room, got %d", len(events))
	}
	if sess.SafeViolations[zoneID] != 0 {
		t.Errorf("SafeViolations should not change for non-safe room")
	}
}

// Compile-time assertion that mockWantedSaver satisfies WantedSaver.
var _ WantedSaver = (*mockWantedSaver)(nil)

// Compile-time assertion that []*gamev1.CombatEvent is returned (type is used).
var _ []*gamev1.CombatEvent = nil
