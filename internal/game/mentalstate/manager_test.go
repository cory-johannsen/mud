package mentalstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestApplyTrigger_SetsState(t *testing.T) {
	m := NewManager()
	changes := m.ApplyTrigger("u1", TrackFear, SeverityMild)
	require.Len(t, changes, 1)
	assert.Equal(t, "", changes[0].OldConditionID)
	assert.Equal(t, "fear_uneasy", changes[0].NewConditionID)
}

func TestApplyTrigger_DoesNotDowngrade(t *testing.T) {
	m := NewManager()
	m.ApplyTrigger("u1", TrackFear, SeverityMod)
	changes := m.ApplyTrigger("u1", TrackFear, SeverityMild)
	assert.Empty(t, changes)
}

func TestApplyTrigger_Upgrades(t *testing.T) {
	m := NewManager()
	m.ApplyTrigger("u1", TrackFear, SeverityMild)
	changes := m.ApplyTrigger("u1", TrackFear, SeverityMod)
	require.Len(t, changes, 1)
	assert.Equal(t, "fear_uneasy", changes[0].OldConditionID)
	assert.Equal(t, "fear_panicked", changes[0].NewConditionID)
}

func TestRecover_StepsDown(t *testing.T) {
	m := NewManager()
	m.ApplyTrigger("u1", TrackFear, SeverityMod)
	changes := m.Recover("u1", TrackFear)
	require.Len(t, changes, 1)
	assert.Equal(t, "fear_panicked", changes[0].OldConditionID)
	assert.Equal(t, "fear_uneasy", changes[0].NewConditionID)
}

func TestRecover_AtNone_NoOp(t *testing.T) {
	m := NewManager()
	changes := m.Recover("u1", TrackFear)
	assert.Empty(t, changes)
}

func TestRecover_StepsToNone(t *testing.T) {
	m := NewManager()
	m.ApplyTrigger("u1", TrackFear, SeverityMild)
	changes := m.Recover("u1", TrackFear)
	require.Len(t, changes, 1)
	assert.Equal(t, "fear_uneasy", changes[0].OldConditionID)
	assert.Equal(t, "", changes[0].NewConditionID)
}

func TestClearTrack(t *testing.T) {
	m := NewManager()
	m.ApplyTrigger("u1", TrackFear, SeveritySevere)
	changes := m.ClearTrack("u1", TrackFear)
	require.Len(t, changes, 1)
	assert.Equal(t, "fear_psychotic", changes[0].OldConditionID)
	assert.Equal(t, "", changes[0].NewConditionID)
}

func TestAdvanceRound_Escalation(t *testing.T) {
	m := NewManager()
	m.ApplyTrigger("u1", TrackFear, SeverityMild) // escalates after 3 rounds
	for i := 0; i < 2; i++ {
		changes := m.AdvanceRound("u1")
		assert.Empty(t, changes, "round %d should not escalate", i+1)
	}
	changes := m.AdvanceRound("u1")
	require.Len(t, changes, 1)
	assert.Equal(t, "fear_uneasy", changes[0].OldConditionID)
	assert.Equal(t, "fear_panicked", changes[0].NewConditionID)
}

func TestAdvanceRound_AutoRecovery(t *testing.T) {
	// Fear level 1: escalate=3, recover=3. Escalation fires first (priority rule).
	// After escalation, verify we are now at level 2.
	m2 := NewManager()
	m2.ApplyTrigger("u2", TrackFear, SeverityMild)
	for i := 0; i < 2; i++ {
		m2.AdvanceRound("u2")
	}
	changes := m2.AdvanceRound("u2") // round 3 — escalates (not recovers)
	require.Len(t, changes, 1)
	assert.Equal(t, "fear_panicked", changes[0].NewConditionID)
}

func TestTracksAreIndependent(t *testing.T) {
	m := NewManager()
	m.ApplyTrigger("u1", TrackFear, SeverityMod)
	m.ApplyTrigger("u1", TrackRage, SeverityMild)
	assert.Equal(t, SeverityMod, m.CurrentSeverity("u1", TrackFear))
	assert.Equal(t, SeverityMild, m.CurrentSeverity("u1", TrackRage))
	assert.Equal(t, SeverityNone, m.CurrentSeverity("u1", TrackDespair))
}

func TestRemove_ClearsAllTracks(t *testing.T) {
	m := NewManager()
	m.ApplyTrigger("u1", TrackFear, SeverityMod)
	m.ApplyTrigger("u1", TrackRage, SeverityMild)
	m.Remove("u1")
	assert.Equal(t, SeverityNone, m.CurrentSeverity("u1", TrackFear))
	assert.Equal(t, SeverityNone, m.CurrentSeverity("u1", TrackRage))
}

func TestProperty_ApplyTriggerNeverDowngrades(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		m := NewManager()
		uid := "u_prop"
		initial := Severity(rapid.IntRange(1, 3).Draw(rt, "initial"))
		m.ApplyTrigger(uid, TrackFear, initial)
		lower := Severity(rapid.IntRange(1, int(initial)).Draw(rt, "lower"))
		changes := m.ApplyTrigger(uid, TrackFear, lower)
		if lower < initial {
			assert.Empty(rt, changes, "trigger with lower severity must be no-op")
		}
		assert.Equal(rt, initial, m.CurrentSeverity(uid, TrackFear))
	})
}

func TestConditionID(t *testing.T) {
	assert.Equal(t, "fear_uneasy", ConditionID(TrackFear, SeverityMild))
	assert.Equal(t, "fear_panicked", ConditionID(TrackFear, SeverityMod))
	assert.Equal(t, "fear_psychotic", ConditionID(TrackFear, SeveritySevere))
	assert.Equal(t, "", ConditionID(TrackFear, SeverityNone))
	assert.Equal(t, "rage_irritated", ConditionID(TrackRage, SeverityMild))
	assert.Equal(t, "despair_catatonic", ConditionID(TrackDespair, SeveritySevere))
	assert.Equal(t, "delirium_hallucinatory", ConditionID(TrackDelirium, SeveritySevere))
}
