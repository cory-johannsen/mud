package mentalstate

import (
	"fmt"
	"sync"
)

// trackState holds the current severity and how many rounds the player has been at that level.
type trackState struct {
	Severity     Severity
	RoundsActive int
}

// playerState holds the four track states for a single player.
type playerState struct {
	Tracks [4]trackState
}

// Manager manages mental state for all players.
type Manager struct {
	mu      sync.Mutex
	players map[string]*playerState
}

// NewManager creates a new Manager.
func NewManager() *Manager {
	return &Manager{
		players: make(map[string]*playerState),
	}
}

// getOrCreate returns the playerState for uid, creating it if necessary.
// Caller must hold m.mu.
func (m *Manager) getOrCreate(uid string) *playerState {
	ps, ok := m.players[uid]
	if !ok {
		ps = &playerState{}
		m.players[uid] = ps
	}
	return ps
}

// ApplyTrigger advances the given track to at least sev.
// It is a no-op if the current severity is already >= sev.
// Resets RoundsActive on change.
func (m *Manager) ApplyTrigger(uid string, track Track, sev Severity) []StateChange {
	m.mu.Lock()
	defer m.mu.Unlock()

	ps := m.getOrCreate(uid)
	ts := &ps.Tracks[track]

	if ts.Severity >= sev {
		return nil
	}

	oldSev := ts.Severity
	ts.Severity = sev
	ts.RoundsActive = 0

	return []StateChange{makeWorsening(track, oldSev, sev)}
}

// AdvanceRound increments RoundsActive for all active tracks and fires
// escalation or auto-recovery if thresholds are reached.
// Escalation takes priority over auto-recovery when both thresholds are reached.
func (m *Manager) AdvanceRound(uid string) []StateChange {
	m.mu.Lock()
	defer m.mu.Unlock()

	ps, ok := m.players[uid]
	if !ok {
		return nil
	}

	var changes []StateChange
	for i := Track(0); i < 4; i++ {
		ts := &ps.Tracks[i]
		if ts.Severity == SeverityNone {
			continue
		}

		ts.RoundsActive++

		escalate := escalateAfterRounds[i][ts.Severity]
		recover := autoRecoverAfterRounds[i][ts.Severity]

		escalateReached := escalate > 0 && ts.RoundsActive >= escalate
		recoverReached := recover > 0 && ts.RoundsActive >= recover

		if escalateReached && ts.Severity < SeveritySevere {
			// Escalation fires first (priority rule)
			oldSev := ts.Severity
			ts.Severity++
			ts.RoundsActive = 0
			changes = append(changes, makeWorsening(i, oldSev, ts.Severity))
		} else if recoverReached {
			oldSev := ts.Severity
			ts.Severity--
			ts.RoundsActive = 0
			changes = append(changes, makeImprovement(i, oldSev, ts.Severity))
		}
	}
	return changes
}

// Recover steps the given track down one severity level.
// It is a no-op if the current severity is SeverityNone.
func (m *Manager) Recover(uid string, track Track) []StateChange {
	m.mu.Lock()
	defer m.mu.Unlock()

	ps, ok := m.players[uid]
	if !ok {
		return nil
	}

	ts := &ps.Tracks[track]
	if ts.Severity == SeverityNone {
		return nil
	}

	oldSev := ts.Severity
	ts.Severity--
	ts.RoundsActive = 0
	return []StateChange{makeImprovement(track, oldSev, ts.Severity)}
}

// ClearTrack resets the given track to SeverityNone.
func (m *Manager) ClearTrack(uid string, track Track) []StateChange {
	m.mu.Lock()
	defer m.mu.Unlock()

	ps, ok := m.players[uid]
	if !ok {
		return nil
	}

	ts := &ps.Tracks[track]
	if ts.Severity == SeverityNone {
		return nil
	}

	oldSev := ts.Severity
	ts.Severity = SeverityNone
	ts.RoundsActive = 0

	return []StateChange{{
		Track:          track,
		OldConditionID: conditionIDs[track][oldSev],
		NewConditionID: "",
		Message:        clearMessages[track],
	}}
}

// CurrentSeverity returns the current severity for the given track.
func (m *Manager) CurrentSeverity(uid string, track Track) Severity {
	m.mu.Lock()
	defer m.mu.Unlock()

	ps, ok := m.players[uid]
	if !ok {
		return SeverityNone
	}
	return ps.Tracks[track].Severity
}

// WorstActiveTrack returns the track with the highest severity.
// Returns (TrackFear, SeverityNone) if all tracks are inactive.
func (m *Manager) WorstActiveTrack(uid string) (Track, Severity) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ps, ok := m.players[uid]
	if !ok {
		return TrackFear, SeverityNone
	}

	worst := TrackFear
	worstSev := SeverityNone
	for i := Track(0); i < 4; i++ {
		if ps.Tracks[i].Severity > worstSev {
			worstSev = ps.Tracks[i].Severity
			worst = i
		}
	}
	return worst, worstSev
}

// Remove deletes all mental state for the given player.
func (m *Manager) Remove(uid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.players, uid)
}

// makeWorsening constructs a StateChange for an increase in severity.
func makeWorsening(track Track, oldSev, newSev Severity) StateChange {
	return StateChange{
		Track:          track,
		OldConditionID: conditionIDs[track][oldSev],
		NewConditionID: conditionIDs[track][newSev],
		Message:        fmt.Sprintf("Your mental state worsens — you are now %s!", severityNames[track][newSev]),
	}
}

// makeImprovement constructs a StateChange for a decrease in severity.
func makeImprovement(track Track, oldSev, newSev Severity) StateChange {
	if newSev == SeverityNone {
		return StateChange{
			Track:          track,
			OldConditionID: conditionIDs[track][oldSev],
			NewConditionID: "",
			Message:        clearMessages[track],
		}
	}
	return StateChange{
		Track:          track,
		OldConditionID: conditionIDs[track][oldSev],
		NewConditionID: conditionIDs[track][newSev],
		Message:        fmt.Sprintf("Your mental state improves — you are now %s.", severityNames[track][newSev]),
	}
}
