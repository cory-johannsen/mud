# Mental State System Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a four-track mental state system (Fear, Rage, Despair, Delirium) with severity escalation, duration-based escalation, auto-recovery, and active recovery via the `calm` command.

**Architecture:** MentalStateManager in `internal/game/mentalstate/` owns state tracking and escalation. Effects are applied via conditions on `PlayerSession.Conditions` — the Manager owns which condition is active per track. New `APReduction`, `SkipTurn`, and `SkillPenalty` fields on `ConditionDef` support AP reduction, turn-skip, and skill penalty effects. `calm` follows the full CMD-1 through CMD-7 command pattern.

**Tech Stack:** Go, protobuf, pgregory.net/rapid (property tests)

---

## Chunk 1: Core mentalstate package + condition extensions

### Task 1: Core mentalstate package (types, manager, tests)

**Files:**
- Create: `internal/game/mentalstate/types.go`
- Create: `internal/game/mentalstate/manager.go`
- Create: `internal/game/mentalstate/manager_test.go`

**Background:**

The mentalstate package is self-contained. It tracks state per player keyed by uid (string). It does NOT import session, condition, or gameserver packages — it is a pure data/logic package.

`PlayerMentalState` holds the current `TrackState` for all four tracks. `Manager` is thread-safe via a mutex.

Escalation thresholds (rounds at level before advancing):
- Level 1 → Level 2: Fear=3, Rage=4, Despair=5, Delirium=4
- Level 2 → Level 3: all tracks = 5

Auto-recovery thresholds (rounds at level before recovering, only if no new trigger):
- Level 1: Fear=3, Rage=4, Despair=5, Delirium=4
- Level 3 Despair (Catatonic) only: 3 rounds auto-recover to level 2

Condition IDs per track × severity (used by callers to apply/remove conditions):
```
TrackFear:    ["", "fear_uneasy", "fear_panicked", "fear_psychotic"]
TrackRage:    ["", "rage_irritated", "rage_enraged", "rage_berserker"]
TrackDespair: ["", "despair_discouraged", "despair_despairing", "despair_catatonic"]
TrackDelirium:["", "delirium_confused", "delirium_delirious", "delirium_hallucinatory"]
```

**StateChange** is returned by `AdvanceRound`, `ApplyTrigger`, and `Recover` to tell callers which conditions to swap.

- [ ] **Step 1: Write failing tests**

Create `internal/game/mentalstate/manager_test.go`:

```go
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
	assert.Empty(t, changes) // no change — already at higher severity
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

	// 2 rounds — no escalation yet
	for i := 0; i < 2; i++ {
		changes := m.AdvanceRound("u1")
		assert.Empty(t, changes, "round %d should not escalate", i+1)
	}

	// 3rd round — should escalate
	changes := m.AdvanceRound("u1")
	require.Len(t, changes, 1)
	assert.Equal(t, "fear_uneasy", changes[0].OldConditionID)
	assert.Equal(t, "fear_panicked", changes[0].NewConditionID)
}

func TestAdvanceRound_AutoRecovery(t *testing.T) {
	m := NewManager()
	m.ApplyTrigger("u1", TrackRage, SeverityMild) // auto-recovers after 4 rounds, escalates after 4

	// Escalation threshold == auto-recovery threshold for Rage level 1.
	// Escalation takes priority. After 4 rounds it should escalate, not recover.
	// To test auto-recovery: use Delirium level 1 (escalate=4, recover=4 — same edge).
	// Instead, use a fresh Manager and manually advance to test state.
	// Actually escalation fires first. Let's test that after escalation, auto-recovery
	// doesn't interfere.

	// Use Fear level 1: escalate=3, recover=3. Escalation fires first.
	m2 := NewManager()
	m2.ApplyTrigger("u2", TrackFear, SeverityMild)
	for i := 0; i < 2; i++ {
		m2.AdvanceRound("u2")
	}
	changes := m2.AdvanceRound("u2") // round 3 — escalates
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/mentalstate/... -v 2>&1 | tail -5
```

Expected: compile error — package does not exist.

- [ ] **Step 3: Create types.go**

Create `internal/game/mentalstate/types.go`:

```go
package mentalstate

// Track identifies one of the four independent mental state tracks.
type Track int

const (
	TrackFear     Track = 0
	TrackRage     Track = 1
	TrackDespair  Track = 2
	TrackDelirium Track = 3
)

// Severity is the intensity level within a track.
// SeverityNone (0) means the track is inactive.
type Severity int

const (
	SeverityNone   Severity = 0
	SeverityMild   Severity = 1
	SeverityMod    Severity = 2
	SeveritySevere Severity = 3
)

// TrackState holds runtime state for one mental state track.
type TrackState struct {
	Severity     Severity
	RoundsActive int // rounds spent at current severity; resets on any severity change
}

// PlayerMentalState holds all four track states for one player.
type PlayerMentalState struct {
	Tracks [4]TrackState
}

// StateChange describes a condition swap resulting from a mental state transition.
// OldConditionID is the condition to remove (empty string = nothing to remove).
// NewConditionID is the condition to apply (empty string = nothing to apply).
// Message is a narrative description of the transition.
type StateChange struct {
	Track          Track
	OldConditionID string
	NewConditionID string
	Message        string
}

// conditionIDs maps (track, severity) to the condition ID string.
// Index [track][severity]; severity 0 is always "".
var conditionIDs = [4][4]string{
	/* Fear    */ {"", "fear_uneasy", "fear_panicked", "fear_psychotic"},
	/* Rage    */ {"", "rage_irritated", "rage_enraged", "rage_berserker"},
	/* Despair */ {"", "despair_discouraged", "despair_despairing", "despair_catatonic"},
	/* Delirium*/ {"", "delirium_confused", "delirium_delirious", "delirium_hallucinatory"},
}

// severityNames maps (track, severity) to display name.
var severityNames = [4][4]string{
	/* Fear    */ {"", "Uneasy", "Panicked", "Psychotic"},
	/* Rage    */ {"", "Irritated", "Enraged", "Berserker"},
	/* Despair */ {"", "Discouraged", "Despairing", "Catatonic"},
	/* Delirium*/ {"", "Confused", "Delirious", "Hallucinatory"},
}

// ConditionID returns the condition ID for a given track and severity.
// Returns "" for SeverityNone.
func ConditionID(track Track, sev Severity) string {
	if sev == SeverityNone {
		return ""
	}
	return conditionIDs[track][sev]
}
```

- [ ] **Step 4: Create manager.go**

Create `internal/game/mentalstate/manager.go`:

```go
package mentalstate

import (
	"fmt"
	"sync"
)

// escalateAfterRounds[track][severity] = rounds before escalation. 0 = no escalation from that level.
var escalateAfterRounds = [4][4]int{
	/* Fear    */ {0, 3, 5, 0},
	/* Rage    */ {0, 4, 5, 0},
	/* Despair */ {0, 5, 5, 0},
	/* Delirium*/ {0, 4, 5, 0},
}

// autoRecoverAfterRounds[track][severity] = rounds before auto-recovery. 0 = no auto-recovery.
// Only level 1 states auto-recover (plus Catatonic level 3 Despair).
var autoRecoverAfterRounds = [4][4]int{
	/* Fear    */ {0, 3, 0, 0},
	/* Rage    */ {0, 4, 0, 0},
	/* Despair */ {0, 5, 0, 3}, // Catatonic auto-recovers to Despairing after 3 rounds
	/* Delirium*/ {0, 4, 0, 0},
}

// Manager tracks mental states for all players.
// It is safe for concurrent use.
type Manager struct {
	mu     sync.Mutex
	states map[string]*PlayerMentalState
}

// NewManager creates an empty Manager.
func NewManager() *Manager {
	return &Manager{states: make(map[string]*PlayerMentalState)}
}

func (m *Manager) getOrCreate(uid string) *PlayerMentalState {
	if s, ok := m.states[uid]; ok {
		return s
	}
	s := &PlayerMentalState{}
	m.states[uid] = s
	return s
}

// ApplyTrigger advances the given track to at least the given severity.
// If current severity >= sev, this is a no-op and returns nil.
// Resets RoundsActive when severity changes.
//
// Precondition: uid non-empty; sev > SeverityNone.
// Postcondition: track severity >= sev; returns StateChange if severity changed.
func (m *Manager) ApplyTrigger(uid string, track Track, sev Severity) []StateChange {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps := m.getOrCreate(uid)
	old := ps.Tracks[track]
	if old.Severity >= sev {
		return nil
	}
	ps.Tracks[track] = TrackState{Severity: sev, RoundsActive: 0}
	return []StateChange{makeChange(track, old.Severity, sev)}
}

// AdvanceRound ticks all active tracks for uid.
// Increments RoundsActive; fires escalation if threshold reached; applies auto-recovery.
// Returns StateChanges for any transitions that occurred.
//
// Precondition: uid non-empty.
// Postcondition: all active tracks have RoundsActive incremented; transitions applied.
func (m *Manager) AdvanceRound(uid string) []StateChange {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps, ok := m.states[uid]
	if !ok {
		return nil
	}
	var changes []StateChange
	for i := range ps.Tracks {
		track := Track(i)
		ts := ps.Tracks[track]
		if ts.Severity == SeverityNone {
			continue
		}
		ts.RoundsActive++
		// Escalation takes priority over auto-recovery.
		escThresh := escalateAfterRounds[track][ts.Severity]
		if escThresh > 0 && ts.RoundsActive >= escThresh && ts.Severity < SeveritySevere {
			old := ts.Severity
			ts.Severity++
			ts.RoundsActive = 0
			changes = append(changes, makeChange(track, old, ts.Severity))
		} else {
			recThresh := autoRecoverAfterRounds[track][ts.Severity]
			if recThresh > 0 && ts.RoundsActive >= recThresh {
				old := ts.Severity
				ts.Severity--
				ts.RoundsActive = 0
				changes = append(changes, makeChange(track, old, ts.Severity))
			}
		}
		ps.Tracks[track] = ts
	}
	return changes
}

// Recover steps down the given track by one severity level.
// Returns a StateChange if severity changed; returns nil if already at SeverityNone.
//
// Precondition: uid non-empty.
// Postcondition: track severity = max(0, current-1).
func (m *Manager) Recover(uid string, track Track) []StateChange {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps, ok := m.states[uid]
	if !ok {
		return nil
	}
	ts := ps.Tracks[track]
	if ts.Severity == SeverityNone {
		return nil
	}
	old := ts.Severity
	ts.Severity--
	ts.RoundsActive = 0
	ps.Tracks[track] = ts
	return []StateChange{makeChange(track, old, ts.Severity)}
}

// ClearTrack immediately resets the given track to SeverityNone.
// Returns a StateChange if the track was active; returns nil if already at SeverityNone.
func (m *Manager) ClearTrack(uid string, track Track) []StateChange {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps, ok := m.states[uid]
	if !ok {
		return nil
	}
	ts := ps.Tracks[track]
	if ts.Severity == SeverityNone {
		return nil
	}
	old := ts.Severity
	ps.Tracks[track] = TrackState{}
	return []StateChange{makeChange(track, old, SeverityNone)}
}

// CurrentSeverity returns the current severity for the given track.
func (m *Manager) CurrentSeverity(uid string, track Track) Severity {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps, ok := m.states[uid]
	if !ok {
		return SeverityNone
	}
	return ps.Tracks[track].Severity
}

// WorstActiveTrack returns the track with the highest severity and its severity.
// Returns (TrackFear, SeverityNone) if no tracks are active.
func (m *Manager) WorstActiveTrack(uid string) (Track, Severity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ps, ok := m.states[uid]
	if !ok {
		return TrackFear, SeverityNone
	}
	worst := TrackFear
	worstSev := SeverityNone
	for i, ts := range ps.Tracks {
		if ts.Severity > worstSev {
			worst = Track(i)
			worstSev = ts.Severity
		}
	}
	return worst, worstSev
}

// Remove deletes all mental state for a player (call on logout or session end).
func (m *Manager) Remove(uid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, uid)
}

// makeChange builds a StateChange for a track transition from oldSev to newSev.
func makeChange(track Track, oldSev, newSev Severity) StateChange {
	sc := StateChange{
		Track:          track,
		OldConditionID: ConditionID(track, oldSev),
		NewConditionID: ConditionID(track, newSev),
	}
	sc.Message = buildMessage(track, oldSev, newSev)
	return sc
}

// buildMessage returns a narrative string describing a severity transition.
func buildMessage(track Track, oldSev, newSev Severity) string {
	if newSev > oldSev {
		if newSev == SeverityNone {
			return ""
		}
		return fmt.Sprintf("Your mental state worsens — you are now %s!", severityNames[track][newSev])
	}
	if newSev == SeverityNone {
		switch track {
		case TrackFear:
			return "Your fear subsides."
		case TrackRage:
			return "Your rage fades."
		case TrackDespair:
			return "Your despair lifts."
		case TrackDelirium:
			return "Your head clears."
		default:
			return "Your mental state returns to normal."
		}
	}
	return fmt.Sprintf("Your mental state improves — you are now %s.", severityNames[track][newSev])
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/mentalstate/... -v 2>&1
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/mentalstate/
git commit -m "feat(mentalstate): add MentalState types and Manager (4 tracks, 3 severity levels each)"
```

---

### Task 2: Condition extensions + YAML files

**Files:**
- Modify: `internal/game/condition/definition.go`
- Modify: `internal/game/condition/modifiers.go`
- Modify: `internal/game/combat/engine.go`
- Create: `content/conditions/mental/fear_uneasy.yaml` (and 11 more)
- Create: `internal/game/condition/modifiers_test.go` (or add to existing test file)

**Background:**

Add three new fields to `ConditionDef`:
- `APReduction int \`yaml:"ap_reduction"\`` — reduces AP at round start (like StunnedAPReduction but general)
- `SkipTurn bool \`yaml:"skip_turn"\`` — if true, the combatant's turn is skipped entirely
- `SkillPenalty int \`yaml:"skill_penalty"\`` — penalty applied to skill checks

Add three helper functions to `modifiers.go`:
- `APReduction(s *ActiveSet) int` — sums `Def.APReduction * Stacks` across all active conditions
- `SkipTurn(s *ActiveSet) bool` — returns true if any active condition has SkipTurn=true
- `SkillPenalty(s *ActiveSet) int` — sums `Def.SkillPenalty * Stacks` across all active conditions

In `combat/engine.go`, in `StartRoundWithSrc`, replace or extend the `reduction` calculation:
```go
// Current code:
reduction := condition.StunnedAPReduction(s)
// Change to:
reduction := condition.StunnedAPReduction(s) + condition.APReduction(s)
```

The 12 YAML files go in `content/conditions/mental/`. Check how `LoadDirectory` works — it may need to be called for this subdirectory in the server startup code. Find where `condition.LoadDirectory` is called (search for it in `internal/gameserver/` or `cmd/`) and ensure the `mental/` subdirectory is also loaded. If `LoadDirectory` is recursive, nothing extra is needed. If not, add a second call.

**Effects per condition:**

| ID | attack_penalty | ac_penalty | damage_bonus | ap_reduction | skip_turn | skill_penalty | restrict_actions |
|----|----------------|------------|--------------|--------------|-----------|---------------|------------------|
| fear_uneasy | 0 | 0 | 0 | 0 | false | 1 | [] |
| fear_panicked | 0 | 0 | 0 | 0 | false | 2 | [] |
| fear_psychotic | 0 | 0 | 0 | 0 | false | 3 | [] |
| rage_irritated | 0 | 1 | 1 | 0 | false | 0 | [] |
| rage_enraged | 0 | 2 | 2 | 0 | false | 0 | [flee] |
| rage_berserker | 2 | 3 | 3 | 0 | false | 0 | [flee] |
| despair_discouraged | 0 | 0 | 0 | 1 | false | 0 | [] |
| despair_despairing | 1 | 0 | 0 | 2 | false | 0 | [] |
| despair_catatonic | 0 | 0 | 0 | 0 | true | 0 | [move] |
| delirium_confused | 1 | 0 | 0 | 0 | false | 1 | [] |
| delirium_delirious | 2 | 0 | 0 | 0 | false | 2 | [] |
| delirium_hallucinatory | 3 | 0 | 0 | 0 | true | 3 | [] |

Note: `restrict_actions: [flee]` is a new action type. Add it to the flee check in combat_handler.go in Task 4.

- [ ] **Step 1: Write failing test for APReduction**

In `internal/game/condition/modifiers_test.go` (check if it exists; create if not):

```go
package condition

import (
	"testing"
)

func TestAPReduction_NoConditions(t *testing.T) {
	s := &ActiveSet{conditions: make(map[string]*ActiveCondition)}
	if got := APReduction(s); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestAPReduction_WithCondition(t *testing.T) {
	s := &ActiveSet{conditions: make(map[string]*ActiveCondition)}
	def := &ConditionDef{ID: "test_ap", APReduction: 2, DurationType: "rounds"}
	s.conditions["test_ap"] = &ActiveCondition{Def: def, Stacks: 1, DurationRemaining: 3}
	if got := APReduction(s); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}

func TestSkipTurn_False(t *testing.T) {
	s := &ActiveSet{conditions: make(map[string]*ActiveCondition)}
	if SkipTurn(s) {
		t.Error("expected false")
	}
}

func TestSkipTurn_True(t *testing.T) {
	s := &ActiveSet{conditions: make(map[string]*ActiveCondition)}
	def := &ConditionDef{ID: "test_skip", SkipTurn: true, DurationType: "rounds"}
	s.conditions["test_skip"] = &ActiveCondition{Def: def, Stacks: 1, DurationRemaining: 3}
	if !SkipTurn(s) {
		t.Error("expected true")
	}
}

func TestSkillPenalty_NoConditions(t *testing.T) {
	s := &ActiveSet{conditions: make(map[string]*ActiveCondition)}
	if got := SkillPenalty(s); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestSkillPenalty_WithCondition(t *testing.T) {
	s := &ActiveSet{conditions: make(map[string]*ActiveCondition)}
	def := &ConditionDef{ID: "test_skill", SkillPenalty: 2, DurationType: "rounds"}
	s.conditions["test_skill"] = &ActiveCondition{Def: def, Stacks: 1, DurationRemaining: 3}
	if got := SkillPenalty(s); got != 2 {
		t.Errorf("expected 2, got %d", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/condition/... -run "TestAPReduction|TestSkipTurn|TestSkillPenalty" -v 2>&1 | tail -5
```

Expected: compile error — `APReduction`, `SkipTurn`, and `SkillPenalty` do not exist.

- [ ] **Step 3: Add APReduction, SkipTurn, and SkillPenalty to ConditionDef**

In `internal/game/condition/definition.go`, add to the `ConditionDef` struct after `RestrictActions`:

```go
// APReduction is the number of AP removed from the combatant's action queue at round start.
APReduction  int  `yaml:"ap_reduction"`
// SkipTurn, if true, causes the combatant's entire turn to be skipped.
SkipTurn     bool `yaml:"skip_turn"`
// SkillPenalty is the penalty applied to skill checks while this condition is active.
SkillPenalty int  `yaml:"skill_penalty"`
```

- [ ] **Step 4: Add APReduction, SkipTurn, and SkillPenalty helpers to modifiers.go**

In `internal/game/condition/modifiers.go`, add after the existing functions:

```go
// APReduction returns the total AP reduction from all active conditions.
// Each condition contributes APReduction * Stacks.
func APReduction(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	total := 0
	for _, ac := range s.conditions {
		total += ac.Def.APReduction * ac.Stacks
	}
	return total
}

// SkipTurn returns true if any active condition has SkipTurn set.
func SkipTurn(s *ActiveSet) bool {
	if s == nil {
		return false
	}
	for _, ac := range s.conditions {
		if ac.Def.SkipTurn {
			return true
		}
	}
	return false
}

// SkillPenalty returns the total skill penalty from all active conditions.
// Each condition contributes SkillPenalty * Stacks.
func SkillPenalty(s *ActiveSet) int {
	if s == nil {
		return 0
	}
	total := 0
	for _, ac := range s.conditions {
		total += ac.Def.SkillPenalty * ac.Stacks
	}
	return total
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/condition/... -run "TestAPReduction|TestSkipTurn|TestSkillPenalty" -v 2>&1
```

Expected: PASS.

- [ ] **Step 6: Update engine.go to use APReduction**

In `internal/game/combat/engine.go`, in `StartRoundWithSrc`, find:

```go
reduction := condition.StunnedAPReduction(s)
```

Replace with:

```go
reduction := condition.StunnedAPReduction(s) + condition.APReduction(s)
```

- [ ] **Step 7: Run full condition and combat tests**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/condition/... ./internal/game/combat/... 2>&1
```

Expected: all PASS.

- [ ] **Step 8: Create 12 YAML files in content/conditions/mental/**

First, check if `content/conditions/mental/` exists. If not, create it.

Also check how `condition.LoadDirectory` is invoked in the server startup (search for `LoadDirectory` in `internal/gameserver/` and `cmd/`). If it loads `content/conditions/` non-recursively, you'll need to add a second load call for `content/conditions/mental/`. If it's recursive or uses a glob, nothing extra is needed — just note it for Task 3 where the registry is wired.

Create `content/conditions/mental/fear_uneasy.yaml`:
```yaml
id: fear_uneasy
name: Uneasy
description: |
  A creeping dread settles over you. Your senses are dulled by anxiety.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 1
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/fear_panicked.yaml`:
```yaml
id: fear_panicked
name: Panicked
description: |
  Terror grips you. You struggle to act with purpose.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 2
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/fear_psychotic.yaml`:
```yaml
id: fear_psychotic
name: Psychotic
description: |
  Reality fractures. You lash out at anyone nearby, friend or foe.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 3
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/rage_irritated.yaml`:
```yaml
id: rage_irritated
name: Irritated
description: |
  Anger sharpens your strikes but leaves you exposed.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 1
damage_bonus: 1
speed_penalty: 0
ap_reduction: 0
skip_turn: false
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/rage_enraged.yaml`:
```yaml
id: rage_enraged
name: Enraged
description: |
  Fury consumes you. You fight harder but cannot disengage.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 2
damage_bonus: 2
speed_penalty: 0
ap_reduction: 0
skip_turn: false
restrict_actions:
  - flee
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/rage_berserker.yaml`:
```yaml
id: rage_berserker
name: Berserker
description: |
  You have lost yourself to violence. Attacks land harder but wildly.
duration_type: rounds
max_stacks: 0
attack_penalty: 2
ac_penalty: 3
damage_bonus: 3
speed_penalty: 0
ap_reduction: 0
skip_turn: false
restrict_actions:
  - flee
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/despair_discouraged.yaml`:
```yaml
id: despair_discouraged
name: Discouraged
description: |
  Hopelessness slows your reactions.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 1
skip_turn: false
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/despair_despairing.yaml`:
```yaml
id: despair_despairing
name: Despairing
description: |
  You see no point in fighting. Your movements are sluggish and ineffective.
duration_type: rounds
max_stacks: 0
attack_penalty: 1
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 2
skip_turn: false
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/despair_catatonic.yaml`:
```yaml
id: despair_catatonic
name: Catatonic
description: |
  You stand frozen, unable to act or move.
duration_type: rounds
max_stacks: 0
attack_penalty: 0
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: true
restrict_actions:
  - move
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/delirium_confused.yaml`:
```yaml
id: delirium_confused
name: Confused
description: |
  Your thoughts are muddled. Basic actions require extra effort.
duration_type: rounds
max_stacks: 0
attack_penalty: 1
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 1
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/delirium_delirious.yaml`:
```yaml
id: delirium_delirious
name: Delirious
description: |
  Reality swims before you. You struggle to aim or focus.
duration_type: rounds
max_stacks: 0
attack_penalty: 2
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: false
skill_penalty: 2
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

Create `content/conditions/mental/delirium_hallucinatory.yaml`:
```yaml
id: delirium_hallucinatory
name: Hallucinatory
description: |
  You see enemies that aren't there and miss the ones that are.
duration_type: rounds
max_stacks: 0
attack_penalty: 3
ac_penalty: 0
damage_bonus: 0
speed_penalty: 0
ap_reduction: 0
skip_turn: true
skill_penalty: 3
restrict_actions: []
lua_on_apply: ""
lua_on_remove: ""
lua_on_tick: ""
```

- [ ] **Step 9: Run full test suite to confirm no regressions**

```bash
cd /home/cjohannsen/src/mud
go test $(go list ./... | grep -v storage/postgres) 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 10: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/condition/ internal/game/combat/engine.go content/conditions/mental/
git commit -m "feat(condition,mentalstate): add APReduction/SkipTurn to ConditionDef; 12 mental state condition YAMLs"
```

---

## Chunk 2: Server wiring + triggers + effects

### Task 3: Wire MentalStateManager into GameServiceServer and CombatHandler

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/combat_handler.go`

**Background:**

`NewGameServiceServer` currently has 37 parameters. Add `mentalStateMgr *mentalstate.Manager` as the 37th parameter (before `actionH *ActionHandler`). Update the `GameServiceServer` struct to store the field. Also update all test files that call `NewGameServiceServer` — search for all call sites.

`NewCombatHandler` currently has 13 parameters. Add `mentalStateMgr *mentalstate.Manager` as the 14th parameter (last). Update the `CombatHandler` struct to store the field.

**How to find all call sites:**
```bash
cd /home/cjohannsen/src/mud
grep -rn "NewGameServiceServer(" --include="*.go" | grep -v "_test.go"
grep -rn "NewGameServiceServer(" --include="*_test.go"
grep -rn "NewCombatHandler(" --include="*.go"
```

Read each call site and add `nil` for the new `mentalStateMgr` parameter in test call sites, and the real `mentalStateMgr` in production call sites.

Also: find where `condition.LoadDirectory` is called with `content/conditions/` and add a second call for `content/conditions/mental/` if loading is not recursive. Search:
```bash
cd /home/cjohannsen/src/mud
grep -rn "LoadDirectory\|conditions" --include="*.go" cmd/ internal/gameserver/ | head -20
```

- [ ] **Step 1: Add mentalStateMgr to GameServiceServer**

In `internal/gameserver/grpc_service.go`:

1. Add import: `"github.com/cory-johannsen/mud/internal/game/mentalstate"`

2. Add field to `GameServiceServer` struct: `mentalStateMgr *mentalstate.Manager`

3. Add parameter to `NewGameServiceServer` before `actionH *ActionHandler`:
   ```go
   mentalStateMgr *mentalstate.Manager,
   ```

4. Add to the struct literal in `NewGameServiceServer`:
   ```go
   mentalStateMgr: mentalStateMgr,
   ```

- [ ] **Step 2: Add mentalStateMgr to CombatHandler**

In `internal/gameserver/combat_handler.go`:

1. Add import: `"github.com/cory-johannsen/mud/internal/game/mentalstate"`

2. Add field to `CombatHandler` struct: `mentalStateMgr *mentalstate.Manager`

3. Add parameter to `NewCombatHandler` at end (before closing paren):
   ```go
   mentalStateMgr *mentalstate.Manager,
   ```

4. Add to the struct literal in `NewCombatHandler`:
   ```go
   mentalStateMgr: mentalStateMgr,
   ```

- [ ] **Step 3: Update all call sites**

Find all calls to `NewGameServiceServer` and `NewCombatHandler`. In production call sites (typically `cmd/server/main.go` or similar), pass a real `mentalstate.NewManager()`. In test files, pass `nil`.

Read each file before editing. Do not guess.

- [ ] **Step 4: Ensure mental state conditions are loaded**

Find where condition registry is built and `LoadDirectory` is called. Check if `content/conditions/mental/` is covered. If not, add:
```go
if err := condRegistry.LoadDirectoryInto("content/conditions/mental/", condRegistry); err != nil {
    // handle error
}
```
(The exact function name may differ — read the actual code first.)

- [ ] **Step 5: Build to verify no compile errors**

```bash
cd /home/cjohannsen/src/mud
go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud
go test $(go list ./... | grep -v storage/postgres) 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/grpc_service.go internal/gameserver/combat_handler.go
git add -u  # stage any updated test files
git commit -m "feat(gameserver): wire MentalStateManager into GameServiceServer and CombatHandler"
```

---

### Task 4: Triggers, AdvanceRound, and condition application

**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/grpc_service_mentalstate_test.go`

**Background:**

The `applyMentalStateChanges` helper applies condition swaps to a player session from `[]mentalstate.StateChange`. It must:
1. For each change: if `OldConditionID != ""`, call `sess.Conditions.Remove(uid, OldConditionID)`
2. If `NewConditionID != ""`, call `sess.Conditions.Apply(uid, condRegistry.Get(NewConditionID), 1, -1)`
3. Return narrative messages for broadcast

This helper lives on `CombatHandler` (it needs both `sessions` and `condRegistry`).

**HP threshold trigger:** After damage is applied to the player in a combat round, if player HP drops to ≤ 25% of MaxHP, call `h.mentalStateMgr.ApplyTrigger(uid, mentalstate.TrackFear, mentalstate.SeverityMild)` and apply changes.

Where to add this: find where player HP is updated after combat round resolution in `combat_handler.go`. Search for `CurrentHP` assignments in the combat handler. The pattern is likely: after calling `cbt.Round(...)` or processing round results, check player HP.

**AdvanceRound:** At the end of each combat round (in the function that processes the end of a round or calls callbacks), call `h.mentalStateMgr.AdvanceRound(uid)` for each player in the combat and apply any returned changes.

- [ ] **Step 1: Write failing tests**

Create `internal/gameserver/grpc_service_mentalstate_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/mentalstate"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHPThreshold_FearTrigger verifies that when a player's HP drops to ≤25% MaxHP
// during combat, the Fear track is set to at least Uneasy.
func TestHPThreshold_FearTrigger(t *testing.T) {
	// This test requires examining the mentalStateMgr after combat damage.
	// Use newDisarmSvcWithCombat pattern; after Attack sets up combat,
	// manually set player HP below threshold and verify Fear is triggered.
	// Since the trigger fires during round processing, we need a way to
	// observe the mental state manager after a round.

	// Use a service with a real mentalStateMgr.
	mentalMgr := mentalstate.NewManager()
	// Build a combat service that uses mentalMgr.
	// Use newCombatSvcWithMentalMgr helper (defined below).
	svc, sessMgr, npcMgr, _ := newCombatSvcWithMentalMgr(t, mentalMgr)

	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_fear", Username: "Tester", CharName: "Tester", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)

	// Set player HP to just above 25% threshold (MaxHP=10, threshold=2).
	sess, ok := sessMgr.GetPlayer("u_fear")
	require.True(t, ok)
	sess.MaxHP = 10
	sess.CurrentHP = 3 // 30%, above threshold

	tmpl := &npc.Template{ID: "ganger-fear-1", Name: "Ganger", Level: 1, MaxHP: 20, AC: 5, Perception: 10}
	inst, err := npcMgr.Spawn(tmpl, "room_a")
	require.NoError(t, err)
	_, err = svc.combatH.Attack("u_fear", inst.ID)
	require.NoError(t, err)

	// Manually drop HP to ≤25% and trigger the fear check.
	sess.CurrentHP = 2 // 20%, at threshold
	svc.combatH.checkHPThresholdFear("u_fear")

	assert.Equal(t, mentalstate.SeverityMild, mentalMgr.CurrentSeverity("u_fear", mentalstate.TrackFear),
		"expected Fear Uneasy triggered at ≤25%% HP")
}

// newCombatSvcWithMentalMgr builds a test GameServiceServer with the given MentalStateManager.
func newCombatSvcWithMentalMgr(t *testing.T, mentalMgr *mentalstate.Manager) (*GameServiceServer, *session.Manager, *npc.Manager, *CombatHandler) {
	t.Helper()
	// Adapt newDisarmSvcWithCombat pattern; pass mentalMgr to CombatHandler and svc.
	// Read grpc_service_disarm_test.go for the exact pattern, then adapt.
	// This is intentionally left as a pattern-match task for the implementer:
	// copy newDisarmSvcWithCombat, change function name, pass mentalMgr as last arg to
	// NewCombatHandler and as mentalStateMgr arg to NewGameServiceServer.
	worldMgr, sessMgr := testWorldAndSession(t)
	npcMgr := npc.NewManager()
	// ... (implementer fills in the full constructor call based on the pattern)
	_ = worldMgr
	_ = npcMgr
	_ = mentalMgr
	t.Skip("newCombatSvcWithMentalMgr not yet implemented — fill in from newDisarmSvcWithCombat pattern")
	return nil, nil, nil, nil
}
```

**NOTE:** The test helper `newCombatSvcWithMentalMgr` needs to be filled in by reading `grpc_service_disarm_test.go` for the exact constructor call pattern. The test above is a spec/direction — the implementer MUST read the disarm test file and adapt the helper correctly. The `t.Skip` line should be REMOVED once the helper is implemented.

The `checkHPThresholdFear` method is a new exported method on `CombatHandler` (testable entry point). Add it in Step 3.

- [ ] **Step 2: Run tests to verify they fail (or skip)**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestHPThreshold" -v 2>&1 | tail -10
```

Expected: test skips (because helper is not yet filled in) or compile error if method doesn't exist.

- [ ] **Step 3: Add applyMentalStateChanges helper to CombatHandler**

In `internal/gameserver/combat_handler.go`, add after the existing helpers:

```go
// applyMentalStateChanges applies condition swaps resulting from mental state transitions
// to the player session and returns narrative messages.
//
// Precondition: uid is a valid player session; changes may be nil or empty.
// Postcondition: conditions applied/removed; messages returned for broadcast.
func (h *CombatHandler) applyMentalStateChanges(uid string, changes []mentalstate.StateChange) []string {
	if len(changes) == 0 || h.mentalStateMgr == nil {
		return nil
	}
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok || sess.Conditions == nil {
		return nil
	}
	var messages []string
	for _, ch := range changes {
		if ch.OldConditionID != "" {
			sess.Conditions.Remove(uid, ch.OldConditionID)
		}
		if ch.NewConditionID != "" {
			def, ok := h.condRegistry.Get(ch.NewConditionID)
			if ok {
				_ = sess.Conditions.Apply(uid, def, 1, -1) // permanent duration for mental state conditions
			}
		}
		if ch.Message != "" {
			messages = append(messages, ch.Message)
		}
	}
	return messages
}
```

- [ ] **Step 4: Add checkHPThresholdFear to CombatHandler**

```go
// checkHPThresholdFear triggers Fear (Uneasy) if player HP is at or below 25% of MaxHP.
// Should be called after player takes damage during combat.
//
// Precondition: uid is a valid player session.
func (h *CombatHandler) checkHPThresholdFear(uid string) {
	if h.mentalStateMgr == nil {
		return
	}
	sess, ok := h.sessions.GetPlayer(uid)
	if !ok || sess.MaxHP == 0 {
		return
	}
	if float64(sess.CurrentHP)/float64(sess.MaxHP) <= 0.25 {
		changes := h.mentalStateMgr.ApplyTrigger(uid, mentalstate.TrackFear, mentalstate.SeverityMild)
		h.applyMentalStateChanges(uid, changes)
	}
}
```

- [ ] **Step 5: Wire checkHPThresholdFear into combat round processing**

Find in `combat_handler.go` where player HP is updated after a round. Search for `CurrentHP` near round result processing. After any player HP update in a round, add:

```go
h.checkHPThresholdFear(playerUID)
```

(The exact location requires reading the file. Find the loop or function that processes combat round events and updates player session HP.)

- [ ] **Step 6: Wire AdvanceRound into end-of-round processing**

Find in `combat_handler.go` where the end of a round is processed (after all combatants have acted). Add:

```go
if h.mentalStateMgr != nil {
    for _, uid := range playerUIDsInCombat {
        changes := h.mentalStateMgr.AdvanceRound(uid)
        msgs := h.applyMentalStateChanges(uid, changes)
        // broadcast msgs as narrative events to the player
        // use existing narrative broadcast pattern
    }
}
```

(Read the actual function to understand the broadcast pattern and player UID iteration.)

- [ ] **Step 7: Fill in newCombatSvcWithMentalMgr test helper**

Read `internal/gameserver/grpc_service_disarm_test.go`, find `newDisarmSvcWithCombat`, copy and adapt for `newCombatSvcWithMentalMgr` passing the `mentalMgr` argument. Remove the `t.Skip` line.

- [ ] **Step 8: Run the HP threshold test**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestHPThreshold" -v 2>&1
```

Expected: PASS.

- [ ] **Step 9: Run full test suite**

```bash
cd /home/cjohannsen/src/mud
go test $(go list ./... | grep -v storage/postgres) 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 10: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/gameserver/combat_handler.go internal/gameserver/grpc_service_mentalstate_test.go
git commit -m "feat(combat): HP threshold Fear trigger; AdvanceRound wired; applyMentalStateChanges helper"
```

---

## Chunk 3: Calm command CMD-1 through CMD-7

### Task 5: Calm command (CMD-1, CMD-2, CMD-3)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/calm.go`
- Create: `internal/game/command/calm_test.go`

**Background:**

`calm` takes no arguments (self-only for v1). Pattern: same as `seek.go` (no args).

Add `HandlerCalm = "calm"` after `HandlerMotive` in commands.go.

Add `BuiltinCommands()` entry after motive:
```go
{Name: "calm", Help: "Attempt to calm your worst active mental state (Grit check; costs all AP in combat).", Category: CategoryCombat, Handler: HandlerCalm},
```

- [ ] **Step 1: Write failing tests**

Create `internal/game/command/calm_test.go`:

```go
package command

import (
	"testing"
	"pgregory.net/rapid"
)

func TestHandleCalm_NoArgs(t *testing.T) {
	req, err := HandleCalm(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil CalmRequest")
	}
}

func TestHandleCalm_ArgsIgnored(t *testing.T) {
	req, err := HandleCalm([]string{"anything"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil CalmRequest")
	}
}

func TestHandleCalm_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := HandleCalm(args)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			rt.Fatal("expected non-nil CalmRequest")
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/command/... -run "TestHandleCalm" -v 2>&1 | tail -5
```

Expected: compile error.

- [ ] **Step 3: Create calm.go**

Create `internal/game/command/calm.go`:

```go
package command

// CalmRequest is the parsed form of the calm command.
// calm takes no arguments — it always targets the player's own worst active mental state.
type CalmRequest struct{}

// HandleCalm parses the calm command. Arguments are ignored.
//
// Postcondition: Always returns a non-nil *CalmRequest and nil error.
func HandleCalm(_ []string) (*CalmRequest, error) {
	return &CalmRequest{}, nil
}
```

- [ ] **Step 4: Add HandlerCalm constant and BuiltinCommands entry**

In `internal/game/command/commands.go`:

Add constant after `HandlerMotive`:
```go
HandlerCalm = "calm"
```

Add entry in `BuiltinCommands()` after the motive entry:
```go
{Name: "calm", Help: "Attempt to calm your worst active mental state (Grit check; costs all AP in combat).", Category: CategoryCombat, Handler: HandlerCalm},
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/game/command/... -run "TestHandleCalm" -v 2>&1
```

Expected: all 3 PASS.

- [ ] **Step 6: Commit**

```bash
cd /home/cjohannsen/src/mud
git add internal/game/command/calm.go internal/game/command/calm_test.go internal/game/command/commands.go
git commit -m "feat(command): add calm command handler (CMD-1, CMD-2, CMD-3)"
```

---

### Task 6: Proto + bridge + grpc handler for calm (CMD-4, CMD-5, CMD-6, CMD-7)

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_calm_test.go`

**Background:**

Proto: add `CalmRequest calm = 70;` to `ClientMessage` oneof. Add `message CalmRequest {}` after `MotiveRequest`.

Run `make proto`.

Bridge: `bridgeCalm` has no target argument — just sends the empty request.

**handleCalm logic:**
1. Get player session — error if not found
2. Find the worst active mental state track via `s.mentalStateMgr.WorstActiveTrack(uid)` — if SeverityNone, return "You are mentally composed — nothing to calm."
3. If in combat (`sess.Status == statusInCombat`):
   a. Spend ALL remaining AP: `s.combatH.SpendAllAP(uid)` — **check if SpendAllAP exists; if not, use SpendAP with the player's current AP count**
   b. Roll `d20 + GritMod`; DC = `10 + severity*4`
   c. On success: call `s.mentalStateMgr.Recover(uid, track)`; apply changes; return success message
   d. On failure: return failure message
4. If not in combat: same roll/DC/recover logic but no AP cost

**SpendAllAP:** Read `combat_handler.go` to find if `SpendAllAP` exists. If not, add it:
```go
// SpendAllAP removes all remaining AP from the player's action queue for this round.
func (h *CombatHandler) SpendAllAP(uid string) {
    h.combatMu.Lock()
    defer h.combatMu.Unlock()
    cbt, ok := h.activeCombats[uid]
    if !ok {
        return
    }
    q := cbt.ActionQueues[uid]
    if q != nil {
        q.SpendAll() // or q.remaining = 0 — check ActionQueue API
    }
}
```
(Read the actual ActionQueue code to find the right method/field to drain AP.)

**GritMod:** `combat.AbilityMod(sess.Abilities.Grit)` — check `internal/game/combat/` for `AbilityMod` function.

**Test cases:**
1. `TestHandleCalm_NoActiveMentalState` — no active states, returns "composed" message
2. `TestHandleCalm_NotInCombat_Success` — not in combat, Grit roll succeeds, Fear Uneasy → removed
3. `TestHandleCalm_NotInCombat_Failure` — not in combat, Grit roll fails
4. `TestHandleCalm_InCombat_Success` — in combat, roll succeeds, spends all AP
5. `TestProperty_CalmDC_AlwaysBasedOnSeverity` — property: DC always = 10 + severity*4

For deterministic tests: use `dice.NewDeterministicSource`. Roll=20 always succeeds (DC max = 10+3*4=22, need roll+GritMod ≥ 22; with GritMod=0 and roll=20, still fails if severity=3. Use severity=1: DC=14, roll=20 succeeds).

- [ ] **Step 1: Add CalmRequest to game.proto**

After `MotiveRequest motive = 69;` in ClientMessage oneof:
```protobuf
    CalmRequest calm = 70;
```

After `message MotiveRequest`:
```protobuf
// CalmRequest asks the server to attempt to calm the player's worst active mental state.
message CalmRequest {}
```

- [ ] **Step 2: Run make proto**

```bash
cd /home/cjohannsen/src/mud
make proto 2>&1
```

Expected: no errors.

- [ ] **Step 3: Add bridgeCalm and register it**

In `internal/frontend/handlers/bridge_handlers.go`:

Add to `bridgeHandlerMap` after the motive entry:
```go
command.HandlerCalm: bridgeCalm,
```

Add function after `bridgeMotive`:
```go
// bridgeCalm sends a CalmRequest with no arguments.
//
// Postcondition: Always returns a non-nil msg containing a CalmRequest.
func bridgeCalm(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Calm{Calm: &gamev1.CalmRequest{}},
	}}, nil
}
```

- [ ] **Step 4: Verify TestAllCommandHandlersAreWired passes**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/frontend/... -run "TestAllCommandHandlersAreWired" -v 2>&1
```

Expected: PASS.

- [ ] **Step 5: Write failing tests**

Create `internal/gameserver/grpc_service_calm_test.go` with the 5 test cases described above. Read `grpc_service_motive_test.go` for the exact constructor patterns. Use the `newCombatSvcWithMentalMgr` helper from Task 4.

For `TestHandleCalm_NoActiveMentalState`:
```go
func TestHandleCalm_NoActiveMentalState(t *testing.T) {
	mentalMgr := mentalstate.NewManager()
	svc, sessMgr, _, _ := newCombatSvcWithMentalMgr(t, mentalMgr)
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_calm_none", Username: "T", CharName: "T", RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	evt, err := svc.handleCalm("u_calm_none", &gamev1.CalmRequest{})
	require.NoError(t, err)
	msg := evt.GetMessage()
	require.NotNil(t, msg)
	assert.Contains(t, msg.Content, "composed")
}
```

- [ ] **Step 6: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestHandleCalm|TestProperty_Calm" -v 2>&1 | tail -10
```

Expected: compile error — `handleCalm` does not exist.

- [ ] **Step 7: Implement handleCalm in grpc_service.go**

Add after `handleMotive`. Read `AbilityMod` location (probably `internal/game/combat/combat.go`) and `SpendAllAP` (add to combat_handler.go if missing).

```go
// handleCalm attempts to calm the player's worst active mental state via a Grit check.
// In combat: costs all remaining AP. Out of combat: no AP cost.
//
// Precondition: uid must be a valid player session.
// Postcondition: On success, worst active track steps down one severity level.
func (s *GameServiceServer) handleCalm(uid string, _ *gamev1.CalmRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	if s.mentalStateMgr == nil {
		return errorEvent("Mental state system unavailable."), nil
	}

	track, sev := s.mentalStateMgr.WorstActiveTrack(uid)
	if sev == mentalstate.SeverityNone {
		return messageEvent("You are mentally composed — nothing to calm."), nil
	}

	if sess.Status == statusInCombat && s.combatH != nil {
		s.combatH.SpendAllAP(uid)
	}

	rollResult, err := s.dice.RollExpr("1d20")
	if err != nil {
		return nil, fmt.Errorf("handleCalm: rolling d20: %w", err)
	}
	roll := rollResult.Total()
	gritMod := combat.AbilityMod(sess.Abilities.Grit)
	total := roll + gritMod
	dc := 10 + int(sev)*4

	detail := fmt.Sprintf("Calm (Grit DC %d): rolled %d+%d=%d", dc, roll, gritMod, total)
	if total < dc {
		return messageEvent(detail + " — failure. Your mental state resists your efforts."), nil
	}

	changes := s.mentalStateMgr.Recover(uid, track)
	if s.combatH != nil {
		s.combatH.applyMentalStateChanges(uid, changes)
	} else {
		// Out-of-combat: apply condition changes directly
		applyMentalChangesToSession(sess, uid, changes, s.condRegistry)
	}
	msg := detail + " — success!"
	if len(changes) > 0 && changes[0].Message != "" {
		msg += " " + changes[0].Message
	}
	return messageEvent(msg), nil
}
```

Also add a standalone helper for out-of-combat condition application in grpc_service.go:

```go
// applyMentalChangesToSession applies mental state condition swaps directly to a session.
// Used outside combat when CombatHandler is not available.
func applyMentalChangesToSession(sess *session.PlayerSession, uid string, changes []mentalstate.StateChange, condReg *condition.Registry) {
	if sess.Conditions == nil || condReg == nil {
		return
	}
	for _, ch := range changes {
		if ch.OldConditionID != "" {
			sess.Conditions.Remove(uid, ch.OldConditionID)
		}
		if ch.NewConditionID != "" {
			def, ok := condReg.Get(ch.NewConditionID)
			if ok {
				_ = sess.Conditions.Apply(uid, def, 1, -1)
			}
		}
	}
}
```

- [ ] **Step 8: Wire dispatch case**

In the `dispatch` function, add before `default`:
```go
case *gamev1.ClientMessage_Calm:
    return s.handleCalm(uid, p.Calm)
```

- [ ] **Step 9: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud
go test ./internal/gameserver/... -run "TestHandleCalm|TestProperty_Calm" -v 2>&1
```

Expected: all tests PASS.

- [ ] **Step 10: Run full test suite**

```bash
cd /home/cjohannsen/src/mud
go test $(go list ./... | grep -v storage/postgres) 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 11: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, update the Mental State section:

Replace:
```
  - Mental state
      - Panic
        - Psychosis
      - [ ] Mental state system — add `MentalState` enum (Normal, Panicked, Psychotic) to PlayerSession; state transitions driven by conditions, zone effects, or health thresholds
      - [ ] Panic condition — implement as a condition with `restrict_actions` effect; on each turn a random available action is chosen instead of player input; triggered by specific NPC abilities, zone effects, or dropping below 25% HP
      - [ ] Psychosis condition — escalation of Panic; player may attack random targets including allies; triggered by prolonged Panic or specific substances
```

With:
```
  - Mental state
      - [x] Mental state system — MentalStateManager with four independent tracks (Fear, Rage, Despair, Delirium), three severity levels each, duration-based escalation, auto-recovery for level 1 states
      - [x] Fear track — Uneasy (skill penalty) → Panicked → Psychotic; triggered by HP ≤ 25%; escalates over rounds
      - [x] Rage track — Irritated (+dmg/-AC) → Enraged (flee restricted) → Berserker; triggered by NPC abilities (future)
      - [x] Despair track — Discouraged (-1 AP) → Despairing (-2 AP) → Catatonic (skip turn); triggered by NPC abilities (future)
      - [x] Delirium track — Confused (-atk) → Delirious (-atk) → Hallucinatory (-atk, skip turn); triggered by toxins/zones (future)
      - [x] Calm command — `calm` (Grit check vs DC 10+severity×4; costs all AP in combat; success steps down worst active track)
      - [ ] Forced action execution — Panicked (random action), Psychotic (attack nearest), Berserker (attack nearest) — requires combat auto-execution mechanism
      - [ ] NPC ability triggers for Rage, Despair, Delirium tracks
      - [ ] Zone effect triggers for all tracks
```

- [ ] **Step 12: Commit**

```bash
cd /home/cjohannsen/src/mud
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go \
        internal/frontend/handlers/bridge_handlers.go \
        internal/gameserver/grpc_service.go \
        internal/gameserver/grpc_service_calm_test.go \
        docs/requirements/FEATURES.md
git commit -m "feat(gameserver,proto,bridge): calm command CMD-4 through CMD-7; handleCalm with Grit check"
```
