package gameserver

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/danger"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

type mockWantedSaver struct {
	upserted map[string]int
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
	if events[0].Narrative == "" {
		t.Error("warning event has empty narrative")
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
	const zoneID = "test_zone"

	events, err := CheckSafeViolation(sess, zoneID, string(danger.Safe), "", 5, nil, saver, nil)
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
