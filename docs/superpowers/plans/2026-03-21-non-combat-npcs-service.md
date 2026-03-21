# Non-Combat NPCs — Service NPCs (Healer + Job Trainer) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Healer and Job Trainer non-combat NPC types with full command support, runtime state persistence, and named NPC content for Rustbucket Ridge.

**Architecture:** The Healer NPC tracks a `HealerRuntimeState` (CapacityUsed, reset on daily calendar tick) stored in a `healerRuntimeStates` map on `GameServiceServer`, mirroring the Merchant/Banker patterns. The Job Trainer introduces three new player commands (`train`, `jobs`, `setjob`); job ownership is tracked via new `Jobs` (map of job_id→level) and `ActiveJobID` fields on `PlayerSession`. Both types wire new proto messages through the `ClientMessage.oneof payload` and dispatch through `grpc_service.go` to dedicated handler files.

**Tech Stack:** Go 1.26, `github.com/stretchr/testify`, `pgregory.net/rapid`

---

## File Map

### New files
| File | Purpose |
|------|---------|
| `internal/gameserver/grpc_service_healer.go` | handleHeal handler + healer runtime state helpers |
| `internal/gameserver/grpc_service_healer_test.go` | TDD tests for healer commands |
| `internal/gameserver/grpc_service_job_trainer.go` | handleTrain, handleJobs, handleSetJob handlers |
| `internal/gameserver/grpc_service_job_trainer_test.go` | TDD tests for job trainer commands |
| `content/npcs/clutch.yaml` | Clutch — healer NPC, The Tinker's Den |
| `content/npcs/tina_wires.yaml` | Tina Wires — healer NPC, Junker's Dream |
| `content/npcs/rio_wrench.yaml` | Rio Wrench — job trainer NPC, Wreckers Rest |

### Modified files
| File | Change |
|------|--------|
| `api/proto/game/v1/game.proto` | Add `HealRequest`, `HealAmountRequest`, `TrainJobRequest`, `ListJobsRequest`, `SetJobRequest` messages; add to `ClientMessage.oneof` at fields 94–98 |
| `internal/gameserver/gamev1/` | Regenerate proto (run `mise exec -- buf generate`) |
| `internal/game/session/manager.go` | Add `Jobs map[string]int` and `ActiveJobID string` fields to `PlayerSession`; add to `AddPlayerOptions` |
| `internal/gameserver/grpc_service.go` | Add `healerRuntimeStates map[string]*npc.HealerRuntimeState`; add dispatch cases for new proto messages |
| `internal/gameserver/grpc_service_npc_ticks.go` | Add `tickHealerCapacity()` called on daily tick (Hour == 0) |
| `internal/game/npc/noncombat.go` | Add `ComputeHealCost`, `ApplyHeal`, `CheckHealPrerequisites` pure functions; add `JobTrainerConfig.Validate(skillIDs map[string]bool)` |
| `internal/game/npc/noncombat_test.go` | Add TDD tests for new pure functions |
| `internal/game/npc/template.go` | Call `JobTrainerConfig.Validate(skillRegistry)` in `Template.Validate()` when skill registry is passed; add `ValidateWithSkills(skillIDs map[string]bool) error` |

---

## Prerequisite: Understand existing shape

Before starting any task, confirm the following are in place (already true from sub-projects 1–2):
- `HealerConfig`, `HealerRuntimeState`, `JobTrainerConfig`, `TrainableJob`, `JobPrerequisites` are defined in `internal/game/npc/noncombat.go`.
- `Template.Healer` and `Template.JobTrainer` are wired in `template.go`.
- `Template.Validate()` already checks `npc_type == "healer"` requires `Healer != nil`, and `npc_type == "job_trainer"` requires `JobTrainer != nil`.

---

## Task 1 — Healer pure functions (noncombat.go)

**Estimated time:** 5 minutes

### Steps

- [ ] **1a. Write failing tests** in `internal/game/npc/noncombat_test.go`:

```go
// TestComputeHealCost_FullHeal checks cost = PricePerHP × (MaxHP - CurrentHP).
func TestComputeHealCost_FullHeal(t *testing.T) {
    cfg := &HealerConfig{PricePerHP: 5, DailyCapacity: 100}
    cost := ComputeHealCost(cfg, 60, 100) // missing 40 HP
    assert.Equal(t, 200, cost)
}

// TestComputeHealCost_PartialHeal checks cost = PricePerHP × amount.
func TestComputeHealCost_PartialHeal(t *testing.T) {
    cfg := &HealerConfig{PricePerHP: 3, DailyCapacity: 100}
    cost := ComputeHealCost(cfg, 0, 10) // not used; amount-based
    _ = cost
    partialCost := ComputeHealAmountCost(cfg, 15)
    assert.Equal(t, 45, partialCost)
}

// TestCheckHealPrerequisites_InsufficientCredits verifies error when credits < cost.
func TestCheckHealPrerequisites_InsufficientCredits(t *testing.T) {
    cfg := &HealerConfig{PricePerHP: 10, DailyCapacity: 50}
    state := &HealerRuntimeState{CapacityUsed: 0}
    err := CheckHealPrerequisites(cfg, state, 50 /*current*/, 100 /*max*/, 400 /*credits*/)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "credits")
}

// TestCheckHealPrerequisites_CapacityExhausted verifies error when capacity is full.
func TestCheckHealPrerequisites_CapacityExhausted(t *testing.T) {
    cfg := &HealerConfig{PricePerHP: 1, DailyCapacity: 10}
    state := &HealerRuntimeState{CapacityUsed: 10}
    err := CheckHealPrerequisites(cfg, state, 80, 100, 9999)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "capacity")
}

// TestCheckHealPrerequisites_AlreadyFullHP verifies error when player is at full HP.
func TestCheckHealPrerequisites_AlreadyFullHP(t *testing.T) {
    cfg := &HealerConfig{PricePerHP: 5, DailyCapacity: 100}
    state := &HealerRuntimeState{CapacityUsed: 0}
    err := CheckHealPrerequisites(cfg, state, 100, 100, 9999)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "full health")
}

// TestApplyHeal_FullHeal verifies HP restored to MaxHP.
func TestApplyHeal_FullHeal(t *testing.T) {
    cfg := &HealerConfig{PricePerHP: 5, DailyCapacity: 100}
    state := &HealerRuntimeState{CapacityUsed: 0}
    newHP, cost, newUsed := ApplyHeal(cfg, state, 60, 100, 100 /*available capacity remains*/)
    assert.Equal(t, 100, newHP)
    assert.Equal(t, 200, cost)
    assert.Equal(t, 40, newUsed)
}

// TestApplyHeal_CapacityLimited verifies heal is capped at remaining capacity.
func TestApplyHeal_CapacityLimited(t *testing.T) {
    cfg := &HealerConfig{PricePerHP: 2, DailyCapacity: 50}
    state := &HealerRuntimeState{CapacityUsed: 45}
    // remaining capacity = 5; player missing = 40 HP
    newHP, cost, newUsed := ApplyHeal(cfg, state, 60, 100, 5)
    assert.Equal(t, 65, newHP)  // 60 + 5
    assert.Equal(t, 10, cost)   // 2 × 5
    assert.Equal(t, 50, newUsed)
}

// TestProperty_ComputeHealCost_NeverNegative property test.
func TestProperty_ComputeHealCost_NeverNegative(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        pricePerHP := rapid.IntRange(1, 100).Draw(rt, "price")
        current := rapid.IntRange(0, 1000).Draw(rt, "current")
        max := rapid.IntRange(current, 1000).Draw(rt, "max")
        cfg := &HealerConfig{PricePerHP: pricePerHP, DailyCapacity: 9999}
        cost := ComputeHealCost(cfg, current, max)
        if cost < 0 {
            rt.Fatalf("ComputeHealCost must be >= 0, got %d", cost)
        }
    })
}
```

- [ ] **1b. Run tests** — verify they fail:
  ```sh
  mise exec -- go test ./internal/game/npc/... -run "TestComputeHealCost|TestCheckHealPrerequisites|TestApplyHeal|TestProperty_ComputeHealCost" -v 2>&1 | head -30
  ```

- [ ] **1c. Implement** the following functions in `internal/game/npc/noncombat.go` (add after `HealerRuntimeState`):

```go
// ComputeHealCost returns the credit cost to restore a player from currentHP to maxHP.
//
// Precondition: cfg must not be nil; currentHP <= maxHP; both >= 0.
// Postcondition: Returns cfg.PricePerHP × (maxHP - currentHP).
func ComputeHealCost(cfg *HealerConfig, currentHP, maxHP int) int {
    return cfg.PricePerHP * (maxHP - currentHP)
}

// ComputeHealAmountCost returns the credit cost to restore exactly amount HP.
//
// Precondition: cfg must not be nil; amount >= 0.
// Postcondition: Returns cfg.PricePerHP × amount.
func ComputeHealAmountCost(cfg *HealerConfig, amount int) int {
    return cfg.PricePerHP * amount
}

// CheckHealPrerequisites validates whether a full-heal is allowed.
// Returns a descriptive error if the player is already at full health,
// capacity is exhausted, or the player cannot afford the cost.
//
// Precondition: cfg and state must not be nil; currentHP <= maxHP; credits >= 0.
// Postcondition: Returns nil iff heal is allowed.
func CheckHealPrerequisites(cfg *HealerConfig, state *HealerRuntimeState, currentHP, maxHP, credits int) error {
    if currentHP >= maxHP {
        return fmt.Errorf("you are already at full health")
    }
    remaining := cfg.DailyCapacity - state.CapacityUsed
    if remaining <= 0 {
        return fmt.Errorf("%s has exhausted their daily healing capacity", "the healer")
    }
    healAmount := maxHP - currentHP
    if healAmount > remaining {
        healAmount = remaining
    }
    cost := cfg.PricePerHP * healAmount
    if credits < cost {
        return fmt.Errorf("you need %d credits but only have %d", cost, credits)
    }
    return nil
}

// ApplyHeal computes the result of healing a player, capped at availableCapacity.
// Returns (newHP, creditCost, newCapacityUsed).
//
// Precondition: cfg and state must not be nil; currentHP <= maxHP; availableCapacity >= 0.
// Postcondition: newHP <= maxHP; creditCost = cfg.PricePerHP × healAmount;
// newCapacityUsed = state.CapacityUsed + healAmount.
func ApplyHeal(cfg *HealerConfig, state *HealerRuntimeState, currentHP, maxHP, availableCapacity int) (newHP, creditCost, newCapacityUsed int) {
    missing := maxHP - currentHP
    healAmount := missing
    if healAmount > availableCapacity {
        healAmount = availableCapacity
    }
    cost := cfg.PricePerHP * healAmount
    return currentHP + healAmount, cost, state.CapacityUsed + healAmount
}
```

- [ ] **1d. Run tests** — verify they pass:
  ```sh
  mise exec -- go test ./internal/game/npc/... -run "TestComputeHealCost|TestCheckHealPrerequisites|TestApplyHeal|TestProperty_ComputeHealCost" -v 2>&1 | tail -15
  ```

- [ ] **1e. Commit:**
  ```sh
  git add internal/game/npc/noncombat.go internal/game/npc/noncombat_test.go
  git commit -m "feat: add healer pure functions (ComputeHealCost, ApplyHeal, CheckHealPrerequisites)"
  ```

---

## Task 2 — Job Trainer pure functions + Validate(skillIDs)

**Estimated time:** 5 minutes

### Steps

- [ ] **2a. Write failing tests** in `internal/game/npc/noncombat_test.go`:

```go
// TestJobTrainerConfig_Validate_UnknownSkill verifies unknown skill ID is a fatal error.
func TestJobTrainerConfig_Validate_UnknownSkill(t *testing.T) {
    cfg := &JobTrainerConfig{
        OfferedJobs: []TrainableJob{
            {
                JobID: "scavenger", TrainingCost: 100,
                Prerequisites: JobPrerequisites{
                    MinSkillRanks: map[string]string{"ghost_skill_xyz": "trained"},
                },
            },
        },
    }
    knownSkills := map[string]bool{"smooth_talk": true, "hustle": true}
    err := cfg.Validate(knownSkills)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "ghost_skill_xyz")
}

// TestJobTrainerConfig_Validate_ValidSkill verifies known skill passes.
func TestJobTrainerConfig_Validate_ValidSkill(t *testing.T) {
    cfg := &JobTrainerConfig{
        OfferedJobs: []TrainableJob{
            {
                JobID: "scavenger", TrainingCost: 100,
                Prerequisites: JobPrerequisites{
                    MinSkillRanks: map[string]string{"smooth_talk": "trained"},
                },
            },
        },
    }
    knownSkills := map[string]bool{"smooth_talk": true}
    err := cfg.Validate(knownSkills)
    assert.NoError(t, err)
}

// TestJobTrainerConfig_Validate_EmptyOfferedJobs allows empty job list.
func TestJobTrainerConfig_Validate_EmptyOfferedJobs(t *testing.T) {
    cfg := &JobTrainerConfig{OfferedJobs: nil}
    err := cfg.Validate(map[string]bool{})
    assert.NoError(t, err)
}

// TestCheckJobPrerequisites_MinLevel verifies level gate.
func TestCheckJobPrerequisites_MinLevel(t *testing.T) {
    job := TrainableJob{
        JobID: "infiltrator", TrainingCost: 200,
        Prerequisites: JobPrerequisites{MinLevel: 5},
    }
    playerLevel := 3
    playerJobs := map[string]int{}
    playerAttrs := map[string]int{}
    playerSkills := map[string]string{}
    err := CheckJobPrerequisites(job, playerLevel, playerJobs, playerAttrs, playerSkills)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "level 5")
}

// TestCheckJobPrerequisites_AlreadyHasJob verifies duplicate job error.
func TestCheckJobPrerequisites_AlreadyHasJob(t *testing.T) {
    job := TrainableJob{JobID: "scavenger", TrainingCost: 100}
    playerJobs := map[string]int{"scavenger": 2}
    err := CheckJobPrerequisites(job, 1, playerJobs, nil, nil)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "already trained")
}

// TestCheckJobPrerequisites_RequiredJobMissing verifies required job gate.
func TestCheckJobPrerequisites_RequiredJobMissing(t *testing.T) {
    job := TrainableJob{
        JobID: "veteran", TrainingCost: 300,
        Prerequisites: JobPrerequisites{RequiredJobs: []string{"soldier"}},
    }
    err := CheckJobPrerequisites(job, 10, map[string]int{}, nil, nil)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "soldier")
}

// TestCheckJobPrerequisites_MinSkillRank verifies skill rank gate.
func TestCheckJobPrerequisites_MinSkillRank(t *testing.T) {
    job := TrainableJob{
        JobID: "infiltrator", TrainingCost: 150,
        Prerequisites: JobPrerequisites{
            MinSkillRanks: map[string]string{"sneak": "expert"},
        },
    }
    err := CheckJobPrerequisites(job, 5, map[string]int{}, nil, map[string]string{"sneak": "trained"})
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "sneak")
}

// TestCheckJobPrerequisites_AllMet verifies no error when all prerequisites are met.
func TestCheckJobPrerequisites_AllMet(t *testing.T) {
    job := TrainableJob{
        JobID: "infiltrator", TrainingCost: 150,
        Prerequisites: JobPrerequisites{
            MinLevel:      3,
            RequiredJobs:  []string{"scavenger"},
            MinSkillRanks: map[string]string{"sneak": "trained"},
        },
    }
    err := CheckJobPrerequisites(job, 5,
        map[string]int{"scavenger": 2},
        nil,
        map[string]string{"sneak": "expert"},
    )
    assert.NoError(t, err)
}
```

- [ ] **2b. Run tests** — verify they fail:
  ```sh
  mise exec -- go test ./internal/game/npc/... -run "TestJobTrainerConfig|TestCheckJobPrerequisites" -v 2>&1 | head -30
  ```

- [ ] **2c. Implement** in `internal/game/npc/noncombat.go` (add after `JobPrerequisites`):

```go
// proficiencyRank maps a rank name to a comparable integer.
// Higher integer = higher rank. Unknown → 0.
func proficiencyRank(rank string) int {
    switch rank {
    case "trained":
        return 1
    case "expert":
        return 2
    case "master":
        return 3
    case "legendary":
        return 4
    default:
        return 0
    }
}

// Validate checks that all skill IDs referenced in MinSkillRanks exist in the
// provided skill registry. Returns an error naming the first unknown skill ID.
//
// REQ-NPC-2a: unknown skill IDs MUST be a fatal load error.
//
// Precondition: knownSkills may be nil (treated as empty set, all IDs unknown).
// Postcondition: Returns nil iff all referenced skill IDs are present in knownSkills.
func (c *JobTrainerConfig) Validate(knownSkills map[string]bool) error {
    for _, job := range c.OfferedJobs {
        for skillID := range job.Prerequisites.MinSkillRanks {
            if !knownSkills[skillID] {
                return fmt.Errorf("job_trainer: unknown skill ID %q in job %q prerequisites", skillID, job.JobID)
            }
        }
    }
    return nil
}

// CheckJobPrerequisites validates all prerequisites for training a job.
// Returns the first unmet prerequisite as a descriptive error, or nil.
//
// Precondition: job, playerJobs, playerAttrs, playerSkills must not be nil (pass empty maps, not nil).
// Postcondition: Returns nil iff all prerequisites are satisfied and the player does not already hold the job.
func CheckJobPrerequisites(job TrainableJob, playerLevel int, playerJobs map[string]int, playerAttrs map[string]int, playerSkills map[string]string) error {
    if _, has := playerJobs[job.JobID]; has {
        return fmt.Errorf("you have already trained %q", job.JobID)
    }
    p := job.Prerequisites
    if p.MinLevel > 0 && playerLevel < p.MinLevel {
        return fmt.Errorf("you must be at least level %d (you are level %d)", p.MinLevel, playerLevel)
    }
    for reqJob := range p.MinJobLevel {
        lvl, has := playerJobs[reqJob]
        if !has || lvl < p.MinJobLevel[reqJob] {
            return fmt.Errorf("you must have %q at level %d", reqJob, p.MinJobLevel[reqJob])
        }
    }
    for attr, minVal := range p.MinAttributes {
        if playerAttrs[attr] < minVal {
            return fmt.Errorf("you need %s %d (you have %d)", attr, minVal, playerAttrs[attr])
        }
    }
    for skillID, minRank := range p.MinSkillRanks {
        if proficiencyRank(playerSkills[skillID]) < proficiencyRank(minRank) {
            return fmt.Errorf("you need skill %q at rank %q (you have %q)", skillID, minRank, playerSkills[skillID])
        }
    }
    for _, reqJob := range p.RequiredJobs {
        if _, has := playerJobs[reqJob]; !has {
            return fmt.Errorf("you must have trained %q first", reqJob)
        }
    }
    return nil
}
```

- [ ] **2d. Run tests** — verify they pass:
  ```sh
  mise exec -- go test ./internal/game/npc/... -run "TestJobTrainerConfig|TestCheckJobPrerequisites" -v 2>&1 | tail -15
  ```

- [ ] **2e. Commit:**
  ```sh
  git add internal/game/npc/noncombat.go internal/game/npc/noncombat_test.go
  git commit -m "feat: add job trainer pure functions (Validate, CheckJobPrerequisites)"
  ```

---

## Task 3 — PlayerSession: Jobs + ActiveJobID fields

**Estimated time:** 5 minutes

### Steps

- [ ] **3a. Write failing tests** in `internal/game/session/` (new file `manager_jobs_test.go`):

```go
package session_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/cory-johannsen/mud/internal/game/session"
)

func TestAddPlayer_JobsInitialized(t *testing.T) {
    mgr := session.NewManager()
    _, err := mgr.AddPlayer(session.AddPlayerOptions{
        UID: "u1", Username: "a", CharName: "A", RoomID: "r1",
        CurrentHP: 10, MaxHP: 10, Role: "player",
    })
    require.NoError(t, err)
    sess, ok := mgr.GetPlayer("u1")
    require.True(t, ok)
    assert.NotNil(t, sess.Jobs, "Jobs map must be initialized on AddPlayer")
    assert.Equal(t, "", sess.ActiveJobID, "ActiveJobID must default to empty string")
}

func TestPlayerSession_Jobs_TrackMultipleJobs(t *testing.T) {
    mgr := session.NewManager()
    _, err := mgr.AddPlayer(session.AddPlayerOptions{
        UID: "u2", Username: "b", CharName: "B", RoomID: "r1",
        CurrentHP: 10, MaxHP: 10, Role: "player",
    })
    require.NoError(t, err)
    sess, _ := mgr.GetPlayer("u2")
    sess.Jobs["scavenger"] = 1
    sess.Jobs["infiltrator"] = 3
    assert.Equal(t, 2, len(sess.Jobs))
    assert.Equal(t, 1, sess.Jobs["scavenger"])
    assert.Equal(t, 3, sess.Jobs["infiltrator"])
}
```

- [ ] **3b. Run tests** — verify they fail:
  ```sh
  mise exec -- go test ./internal/game/session/... -run "TestAddPlayer_JobsInitialized|TestPlayerSession_Jobs" -v 2>&1 | head -20
  ```

- [ ] **3c. Implement** — add to `PlayerSession` struct in `internal/game/session/manager.go` after `StashBalance`:

```go
// Jobs maps job_id to the player's current level in that job.
// Initialized to an empty map at session creation.
// REQ-NPC-9: players acquire jobs via training; REQ-NPC-11: all held jobs grant feats/proficiencies.
Jobs map[string]int
// ActiveJobID is the currently active job that earns XP (REQ-NPC-10).
// Empty string means no active job. Set to the first trained job automatically (REQ-NPC-9).
ActiveJobID string
```

Then in the `NewManager` or `AddPlayer` body where other maps are initialized (after `WantedLevel: make(map[string]int)`), add:

```go
Jobs: make(map[string]int),
```

(ActiveJobID defaults to `""` automatically.)

- [ ] **3d. Run tests** — verify they pass:
  ```sh
  mise exec -- go test ./internal/game/session/... -run "TestAddPlayer_JobsInitialized|TestPlayerSession_Jobs" -v 2>&1 | tail -10
  ```

- [ ] **3e. Run full session test suite** to confirm no regressions:
  ```sh
  mise exec -- go test ./internal/game/session/... 2>&1 | tail -5
  ```

- [ ] **3f. Commit:**
  ```sh
  git add internal/game/session/manager.go internal/game/session/manager_jobs_test.go
  git commit -m "feat: add Jobs and ActiveJobID fields to PlayerSession"
  ```

---

## Task 4 — Proto: Add new message types and oneof entries

**Estimated time:** 5 minutes

### Steps

- [ ] **4a. Edit** `api/proto/game/v1/game.proto`:

Add after `StashBalanceRequest` message definition (near line 176):

```protobuf
// HealRequest asks the healer NPC to fully restore the player's HP.
message HealRequest {
  string npc_name = 1; // name of the healer NPC to address
}

// HealAmountRequest asks the healer NPC to restore a specific number of HP.
message HealAmountRequest {
  string npc_name = 1;
  int32  amount   = 2;
}

// TrainJobRequest asks a job trainer NPC to train the player in a job.
message TrainJobRequest {
  string npc_name = 1;
  string job_id   = 2;
}

// ListJobsRequest asks the server to list all jobs held by the player.
message ListJobsRequest {}

// SetJobRequest asks the server to change the player's active job.
// REQ-NPC-17: available from any room.
message SetJobRequest {
  string job_id = 1;
}
```

Add to `ClientMessage.oneof payload` after field 93 (`stash_balance = 93`):

```protobuf
    HealRequest       heal            = 94;
    HealAmountRequest heal_amount     = 95;
    TrainJobRequest   train_job       = 96;
    ListJobsRequest   list_jobs       = 97;
    SetJobRequest     set_job         = 98;
```

- [ ] **4b. Regenerate proto:**
  ```sh
  cd /home/cjohannsen/src/mud && mise exec -- buf generate
  ```

- [ ] **4c. Verify compilation:**
  ```sh
  mise exec -- go build ./... 2>&1 | head -20
  ```

- [ ] **4d. Commit:**
  ```sh
  git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
  git commit -m "feat: add proto messages for heal and job trainer commands"
  ```

---

## Task 5 — Healer runtime state infrastructure

**Estimated time:** 5 minutes

### Steps

- [ ] **5a. Add** `healerRuntimeStates map[string]*npc.HealerRuntimeState` to `GameServiceServer` struct in `internal/gameserver/grpc_service.go` after the `bankerRuntimeStates` field:

```go
// healerRuntimeStates maps NPC instance ID to active healer runtime state.
healerRuntimeStates map[string]*npc.HealerRuntimeState
```

- [ ] **5b. Initialize** the map in `NewGameServiceServer` alongside `bankerRuntimeStates`:

```go
s.healerRuntimeStates = make(map[string]*npc.HealerRuntimeState)
```

- [ ] **5c. Create** `internal/gameserver/grpc_service_healer.go`:

```go
package gameserver

import (
    "fmt"
    "sync"

    "github.com/cory-johannsen/mud/internal/game/npc"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var healerRuntimeMu sync.RWMutex

// initHealerRuntimeState initialises runtime state for a healer instance if absent.
//
// Precondition: inst must be non-nil.
// Postcondition: healerRuntimeStates[inst.ID] is set iff inst.NPCType == "healer".
func (s *GameServiceServer) initHealerRuntimeState(inst *npc.Instance) {
    if inst.NPCType != "healer" {
        return
    }
    healerRuntimeMu.Lock()
    defer healerRuntimeMu.Unlock()
    if _, ok := s.healerRuntimeStates[inst.ID]; !ok {
        s.healerRuntimeStates[inst.ID] = &npc.HealerRuntimeState{}
    }
}

// healerStateFor returns the HealerRuntimeState for instID, or nil if absent.
//
// Precondition: none.
// Postcondition: Returns nil when instID is not in healerRuntimeStates.
func (s *GameServiceServer) healerStateFor(instID string) *npc.HealerRuntimeState {
    healerRuntimeMu.RLock()
    defer healerRuntimeMu.RUnlock()
    return s.healerRuntimeStates[instID]
}

// findHealerInRoom returns the first healer NPC matching npcName in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findHealerInRoom(roomID, npcName string) (*npc.Instance, string) {
    inst := s.npcMgr.FindInRoom(roomID, npcName)
    if inst == nil {
        return nil, fmt.Sprintf("You don't see %q here.", npcName)
    }
    if inst.NPCType != "healer" {
        return nil, fmt.Sprintf("%s is not a healer.", inst.Name())
    }
    if inst.Cowering {
        return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
    }
    return inst, ""
}

// handleHeal restores the player to full HP via a healer NPC.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, sess.CurrentHP == sess.MaxHP and sess.Currency is reduced by cost.
func (s *GameServiceServer) handleHeal(uid string, req *gamev1.HealRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("player not found"), nil
    }
    inst, errMsg := s.findHealerInRoom(sess.RoomID, req.GetNpcName())
    if inst == nil {
        return messageEvent(errMsg), nil
    }
    tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
    if tmpl == nil || tmpl.Healer == nil {
        return messageEvent("This healer has no configuration."), nil
    }
    state := s.healerStateFor(inst.ID)
    if state == nil {
        s.initHealerRuntimeState(inst)
        state = s.healerStateFor(inst.ID)
    }
    if err := npc.CheckHealPrerequisites(tmpl.Healer, state, sess.CurrentHP, sess.MaxHP, sess.Currency); err != nil {
        // Check if capacity is partially available and offer prorated heal.
        remaining := tmpl.Healer.DailyCapacity - state.CapacityUsed
        missing := sess.MaxHP - sess.CurrentHP
        if remaining > 0 && remaining < missing {
            cost := tmpl.Healer.PricePerHP * remaining
            if sess.Currency >= cost {
                return messageEvent(fmt.Sprintf(
                    "%s can only heal %d HP today. That would cost %d credits. Use 'heal %d %s' to accept.",
                    inst.Name(), remaining, cost, remaining, inst.Name(),
                )), nil
            }
        }
        return messageEvent(err.Error()), nil
    }
    remaining := tmpl.Healer.DailyCapacity - state.CapacityUsed
    newHP, cost, newUsed := npc.ApplyHeal(tmpl.Healer, state, sess.CurrentHP, sess.MaxHP, remaining)
    healerRuntimeMu.Lock()
    state.CapacityUsed = newUsed
    healerRuntimeMu.Unlock()
    sess.CurrentHP = newHP
    sess.Currency -= cost
    return messageEvent(fmt.Sprintf(
        "%s patches you up. HP restored to %d/%d. Cost: %d credits.",
        inst.Name(), sess.CurrentHP, sess.MaxHP, cost,
    )), nil
}

// handleHealAmount restores the player by a specific HP amount via a healer NPC.
//
// Precondition: uid identifies an active player session; req is non-nil; req.Amount > 0.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, sess.CurrentHP is increased by the healed amount (capped at MaxHP and capacity).
func (s *GameServiceServer) handleHealAmount(uid string, req *gamev1.HealAmountRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("player not found"), nil
    }
    amount := int(req.GetAmount())
    if amount <= 0 {
        return messageEvent("Specify a positive amount of HP to heal."), nil
    }
    inst, errMsg := s.findHealerInRoom(sess.RoomID, req.GetNpcName())
    if inst == nil {
        return messageEvent(errMsg), nil
    }
    tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
    if tmpl == nil || tmpl.Healer == nil {
        return messageEvent("This healer has no configuration."), nil
    }
    state := s.healerStateFor(inst.ID)
    if state == nil {
        s.initHealerRuntimeState(inst)
        state = s.healerStateFor(inst.ID)
    }
    if sess.CurrentHP >= sess.MaxHP {
        return messageEvent("You are already at full health."), nil
    }
    remaining := tmpl.Healer.DailyCapacity - state.CapacityUsed
    if remaining <= 0 {
        return messageEvent(fmt.Sprintf("%s has exhausted their daily healing capacity.", inst.Name())), nil
    }
    // Cap amount at: missing HP, remaining capacity.
    missing := sess.MaxHP - sess.CurrentHP
    if amount > missing {
        amount = missing
    }
    if amount > remaining {
        amount = remaining
    }
    cost := npc.ComputeHealAmountCost(tmpl.Healer, amount)
    if sess.Currency < cost {
        return messageEvent(fmt.Sprintf("You need %d credits to heal %d HP but only have %d.", cost, amount, sess.Currency)), nil
    }
    healerRuntimeMu.Lock()
    state.CapacityUsed += amount
    healerRuntimeMu.Unlock()
    sess.CurrentHP += amount
    sess.Currency -= cost
    return messageEvent(fmt.Sprintf(
        "%s heals you for %d HP (%d/%d). Cost: %d credits.",
        inst.Name(), amount, sess.CurrentHP, sess.MaxHP, cost,
    )), nil
}
```

- [ ] **5d. Verify compilation:**
  ```sh
  mise exec -- go build ./internal/gameserver/... 2>&1 | head -20
  ```

- [ ] **5e. Commit:**
  ```sh
  git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_healer.go
  git commit -m "feat: add healer runtime state infrastructure and handleHeal/handleHealAmount handlers"
  ```

---

## Task 6 — Healer dispatch + daily tick reset

**Estimated time:** 5 minutes

### Steps

- [ ] **6a. Add dispatch cases** in `internal/gameserver/grpc_service.go` in the `switch p := msg.Payload.(type)` block after the `stash_balance` case:

```go
case *gamev1.ClientMessage_Heal:
    return s.handleHeal(uid, p.Heal)
case *gamev1.ClientMessage_HealAmount:
    return s.handleHealAmount(uid, p.HealAmount)
```

- [ ] **6b. Add `tickHealerCapacity`** in `internal/gameserver/grpc_service_npc_ticks.go` after `tickBankerRates`:

```go
// tickHealerCapacity resets CapacityUsed to 0 for all healer instances.
// Called once per in-game day (when dt.Hour == 0). REQ-NPC-16.
//
// Precondition: s.healerRuntimeStates MUST NOT be nil.
// Postcondition: every HealerRuntimeState.CapacityUsed is set to 0.
func (s *GameServiceServer) tickHealerCapacity() {
    healerRuntimeMu.Lock()
    defer healerRuntimeMu.Unlock()
    for _, state := range s.healerRuntimeStates {
        state.CapacityUsed = 0
    }
}
```

- [ ] **6c. Call `tickHealerCapacity`** in `StartNPCTickHook` alongside `tickBankerRates`:

```go
if dt.Hour == 0 {
    s.tickBankerRates()
    s.tickHealerCapacity()
}
```

- [ ] **6d. Write failing tests** in `internal/gameserver/grpc_service_healer_test.go`:

```go
package gameserver

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/session"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// newHealerTestServer builds a GameServiceServer with a real healer NPC in room_a.
func newHealerTestServer(t *testing.T) (*GameServiceServer, string, *npc.Instance) {
    t.Helper()
    worldMgr, sessMgr := testWorldAndSession(t)
    npcManager := npc.NewManager()
    svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

    uid := "heal_u1"
    _, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
        UID: uid, Username: "heal_user", CharName: "HealChar",
        RoomID: "room_a", CurrentHP: 60, MaxHP: 100, Role: "player",
    })
    require.NoError(t, err)
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.Currency = 500

    tmpl := &npc.Template{
        ID: "test_healer", Name: "Clutch", NPCType: "healer",
        Level: 3, MaxHP: 20, AC: 10,
        Healer: &npc.HealerConfig{PricePerHP: 5, DailyCapacity: 100},
    }
    inst, err := npcManager.Spawn(tmpl, "room_a")
    require.NoError(t, err)
    return svc, uid, inst
}

func TestHandleHeal_FullHeal(t *testing.T) {
    svc, uid, _ := newHealerTestServer(t)
    evt, err := svc.handleHeal(uid, &gamev1.HealRequest{NpcName: "Clutch"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "100/100")
    sess, _ := svc.sessions.GetPlayer(uid)
    assert.Equal(t, 100, sess.CurrentHP)
    assert.Equal(t, 300, sess.Currency) // 500 - 5×40
}

func TestHandleHeal_AlreadyFullHP(t *testing.T) {
    svc, uid, _ := newHealerTestServer(t)
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.CurrentHP = 100
    evt, err := svc.handleHeal(uid, &gamev1.HealRequest{NpcName: "Clutch"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "full health")
}

func TestHandleHeal_InsufficientCredits(t *testing.T) {
    svc, uid, _ := newHealerTestServer(t)
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.Currency = 10 // need 200 (40 HP × 5)
    evt, err := svc.handleHeal(uid, &gamev1.HealRequest{NpcName: "Clutch"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "credits")
}

func TestHandleHeal_CapacityExhausted(t *testing.T) {
    svc, uid, inst := newHealerTestServer(t)
    svc.healerRuntimeStates[inst.ID] = &npc.HealerRuntimeState{CapacityUsed: 100}
    evt, err := svc.handleHeal(uid, &gamev1.HealRequest{NpcName: "Clutch"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "capacity")
}

func TestHandleHealAmount_PartialHeal(t *testing.T) {
    svc, uid, _ := newHealerTestServer(t)
    evt, err := svc.handleHealAmount(uid, &gamev1.HealAmountRequest{NpcName: "Clutch", Amount: 10})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "70/100")
    sess, _ := svc.sessions.GetPlayer(uid)
    assert.Equal(t, 70, sess.CurrentHP)
    assert.Equal(t, 450, sess.Currency) // 500 - 5×10
}

func TestTickHealerCapacity_ResetsCapacityUsed(t *testing.T) {
    svc, _, inst := newHealerTestServer(t)
    svc.initHealerRuntimeState(inst)
    svc.healerRuntimeStates[inst.ID] = &npc.HealerRuntimeState{CapacityUsed: 75}
    svc.tickHealerCapacity()
    assert.Equal(t, 0, svc.healerRuntimeStates[inst.ID].CapacityUsed)
}

func TestProperty_HandleHeal_NeverExceedsMaxHP(t *testing.T) {
    // Property: after any heal, CurrentHP <= MaxHP.
    svc, uid, _ := newHealerTestServer(t)
    sess, _ := svc.sessions.GetPlayer(uid)
    for _, startHP := range []int{0, 1, 50, 99, 100} {
        sess.CurrentHP = startHP
        sess.Currency = 99999
        svc.healerRuntimeStates = make(map[string]*npc.HealerRuntimeState)
        _, _ = svc.handleHeal(uid, &gamev1.HealRequest{NpcName: "Clutch"})
        assert.LessOrEqual(t, sess.CurrentHP, sess.MaxHP,
            "CurrentHP must not exceed MaxHP after heal, startHP=%d", startHP)
    }
}
```

- [ ] **6e. Run tests** — verify they pass:
  ```sh
  mise exec -- go test ./internal/gameserver/... -run "TestHandleHeal|TestHandleHealAmount|TestTickHealerCapacity|TestProperty_HandleHeal" -v 2>&1 | tail -20
  ```

- [ ] **6f. Run full test suite:**
  ```sh
  mise exec -- go test ./... 2>&1 | tail -10
  ```

- [ ] **6g. Commit:**
  ```sh
  git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_npc_ticks.go internal/gameserver/grpc_service_healer.go internal/gameserver/grpc_service_healer_test.go
  git commit -m "feat: wire healer command dispatch and daily capacity tick reset"
  ```

---

## Task 7 — Job Trainer handlers

**Estimated time:** 10 minutes

### Steps

- [ ] **7a. Write failing tests** in `internal/gameserver/grpc_service_job_trainer_test.go`:

```go
package gameserver

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/cory-johannsen/mud/internal/game/npc"
    "github.com/cory-johannsen/mud/internal/game/session"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

func newJobTrainerTestServer(t *testing.T) (*GameServiceServer, string) {
    t.Helper()
    worldMgr, sessMgr := testWorldAndSession(t)
    npcManager := npc.NewManager()
    svc := testServiceWithNPCMgr(t, worldMgr, sessMgr, npcManager)

    uid := "jt_u1"
    _, err := svc.sessions.AddPlayer(session.AddPlayerOptions{
        UID: uid, Username: "jt_user", CharName: "JTChar",
        RoomID: "room_a", CurrentHP: 50, MaxHP: 50, Role: "player",
        Level: 5,
    })
    require.NoError(t, err)
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.Currency = 1000
    sess.Level = 5

    tmpl := &npc.Template{
        ID: "test_trainer", Name: "Rio Wrench", NPCType: "job_trainer",
        Level: 4, MaxHP: 25, AC: 11,
        JobTrainer: &npc.JobTrainerConfig{
            OfferedJobs: []npc.TrainableJob{
                {JobID: "scavenger", TrainingCost: 200, Prerequisites: npc.JobPrerequisites{MinLevel: 1}},
                {JobID: "infiltrator", TrainingCost: 500, Prerequisites: npc.JobPrerequisites{MinLevel: 3, RequiredJobs: []string{"scavenger"}}},
            },
        },
    }
    _, err = npcManager.Spawn(tmpl, "room_a")
    require.NoError(t, err)
    return svc, uid
}

func TestHandleTrainJob_Success_FirstJob(t *testing.T) {
    svc, uid := newJobTrainerTestServer(t)
    evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "scavenger"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "scavenger")
    sess, _ := svc.sessions.GetPlayer(uid)
    assert.Equal(t, 1, sess.Jobs["scavenger"])
    assert.Equal(t, "scavenger", sess.ActiveJobID, "first trained job becomes active")
    assert.Equal(t, 800, sess.Currency)
}

func TestHandleTrainJob_AlreadyTrained(t *testing.T) {
    svc, uid := newJobTrainerTestServer(t)
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.Jobs["scavenger"] = 2
    evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "scavenger"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "already trained")
}

func TestHandleTrainJob_InsufficientCredits(t *testing.T) {
    svc, uid := newJobTrainerTestServer(t)
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.Currency = 50
    evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "scavenger"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "credits")
}

func TestHandleTrainJob_MissingPrerequisite(t *testing.T) {
    svc, uid := newJobTrainerTestServer(t)
    evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "infiltrator"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "scavenger")
}

func TestHandleTrainJob_UnknownJob(t *testing.T) {
    svc, uid := newJobTrainerTestServer(t)
    evt, err := svc.handleTrainJob(uid, &gamev1.TrainJobRequest{NpcName: "Rio Wrench", JobId: "ninja"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "ninja")
}

func TestHandleListJobs_Empty(t *testing.T) {
    svc, uid := newJobTrainerTestServer(t)
    evt, err := svc.handleListJobs(uid, &gamev1.ListJobsRequest{})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "no jobs")
}

func TestHandleListJobs_WithJobs(t *testing.T) {
    svc, uid := newJobTrainerTestServer(t)
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.Jobs["scavenger"] = 3
    sess.ActiveJobID = "scavenger"
    evt, err := svc.handleListJobs(uid, &gamev1.ListJobsRequest{})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "scavenger")
    assert.Contains(t, evt.GetMessage().GetText(), "[active]")
}

func TestHandleSetJob_Success(t *testing.T) {
    svc, uid := newJobTrainerTestServer(t)
    sess, _ := svc.sessions.GetPlayer(uid)
    sess.Jobs["scavenger"] = 1
    sess.Jobs["infiltrator"] = 2
    sess.ActiveJobID = "scavenger"
    evt, err := svc.handleSetJob(uid, &gamev1.SetJobRequest{JobId: "infiltrator"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "infiltrator")
    assert.Equal(t, "infiltrator", sess.ActiveJobID)
}

func TestHandleSetJob_NotHeld(t *testing.T) {
    svc, uid := newJobTrainerTestServer(t)
    evt, err := svc.handleSetJob(uid, &gamev1.SetJobRequest{JobId: "infiltrator"})
    require.NoError(t, err)
    assert.Contains(t, evt.GetMessage().GetText(), "not trained")
}
```

- [ ] **7b. Run tests** — verify they fail:
  ```sh
  mise exec -- go test ./internal/gameserver/... -run "TestHandleTrain|TestHandleListJobs|TestHandleSetJob" -v 2>&1 | head -20
  ```

- [ ] **7c. Create** `internal/gameserver/grpc_service_job_trainer.go`:

```go
package gameserver

import (
    "fmt"
    "sort"
    "strings"

    "github.com/cory-johannsen/mud/internal/game/npc"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// findJobTrainerInRoom locates a job_trainer NPC by name in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findJobTrainerInRoom(roomID, npcName string) (*npc.Instance, string) {
    inst := s.npcMgr.FindInRoom(roomID, npcName)
    if inst == nil {
        return nil, fmt.Sprintf("You don't see %q here.", npcName)
    }
    if inst.NPCType != "job_trainer" {
        return nil, fmt.Sprintf("%s is not a job trainer.", inst.Name())
    }
    if inst.Cowering {
        return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
    }
    return inst, ""
}

// handleTrainJob processes a player's request to learn a new job from a trainer.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, the job is added to sess.Jobs at level 1; if it is the first job,
// sess.ActiveJobID is set to the trained job ID (REQ-NPC-9).
func (s *GameServiceServer) handleTrainJob(uid string, req *gamev1.TrainJobRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("player not found"), nil
    }
    inst, errMsg := s.findJobTrainerInRoom(sess.RoomID, req.GetNpcName())
    if inst == nil {
        return messageEvent(errMsg), nil
    }
    tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
    if tmpl == nil || tmpl.JobTrainer == nil {
        return messageEvent("This trainer has no jobs configured."), nil
    }
    jobID := req.GetJobId()
    var trainable *npc.TrainableJob
    for i := range tmpl.JobTrainer.OfferedJobs {
        if tmpl.JobTrainer.OfferedJobs[i].JobID == jobID {
            trainable = &tmpl.JobTrainer.OfferedJobs[i]
            break
        }
    }
    if trainable == nil {
        return messageEvent(fmt.Sprintf("%s doesn't offer training for %q.", inst.Name(), jobID)), nil
    }
    playerAttrs := map[string]int{
        "brutality": sess.Abilities.Brutality,
        "grit":      sess.Abilities.Grit,
        "quickness": sess.Abilities.Quickness,
        "reasoning": sess.Abilities.Reasoning,
        "savvy":     sess.Abilities.Savvy,
        "flair":     sess.Abilities.Flair,
    }
    if err := npc.CheckJobPrerequisites(*trainable, sess.Level, sess.Jobs, playerAttrs, sess.Skills); err != nil {
        return messageEvent(err.Error()), nil
    }
    if sess.Currency < trainable.TrainingCost {
        return messageEvent(fmt.Sprintf("Training costs %d credits but you only have %d.", trainable.TrainingCost, sess.Currency)), nil
    }
    sess.Currency -= trainable.TrainingCost
    sess.Jobs[jobID] = 1
    if sess.ActiveJobID == "" {
        sess.ActiveJobID = jobID
    }
    return messageEvent(fmt.Sprintf(
        "%s trains you in %q. Cost: %d credits. Your jobs: %s.",
        inst.Name(), jobID, trainable.TrainingCost, formatJobList(sess.Jobs, sess.ActiveJobID),
    )), nil
}

// handleListJobs lists all jobs held by the player with level and active marker.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleListJobs(uid string, req *gamev1.ListJobsRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("player not found"), nil
    }
    if len(sess.Jobs) == 0 {
        return messageEvent("You have no jobs trained yet. Find a job trainer to begin."), nil
    }
    var sb strings.Builder
    sb.WriteString("Your jobs:\n")
    ids := make([]string, 0, len(sess.Jobs))
    for id := range sess.Jobs {
        ids = append(ids, id)
    }
    sort.Strings(ids)
    for _, id := range ids {
        marker := ""
        if id == sess.ActiveJobID {
            marker = " [active]"
        }
        sb.WriteString(fmt.Sprintf("  %-20s level %d%s\n", id, sess.Jobs[id], marker))
    }
    return messageEvent(sb.String()), nil
}

// handleSetJob changes the player's active job. REQ-NPC-17: available from any room.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, sess.ActiveJobID is updated.
func (s *GameServiceServer) handleSetJob(uid string, req *gamev1.SetJobRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return messageEvent("player not found"), nil
    }
    jobID := req.GetJobId()
    if _, has := sess.Jobs[jobID]; !has {
        return messageEvent(fmt.Sprintf("You have not trained %q. Use 'jobs' to see your trained jobs.", jobID)), nil
    }
    sess.ActiveJobID = jobID
    return messageEvent(fmt.Sprintf("Active job set to %q (level %d).", jobID, sess.Jobs[jobID])), nil
}

// formatJobList returns a compact comma-separated listing of job:level pairs with active marker.
func formatJobList(jobs map[string]int, activeID string) string {
    ids := make([]string, 0, len(jobs))
    for id := range jobs {
        ids = append(ids, id)
    }
    sort.Strings(ids)
    parts := make([]string, 0, len(ids))
    for _, id := range ids {
        entry := fmt.Sprintf("%s(L%d)", id, jobs[id])
        if id == activeID {
            entry += "*"
        }
        parts = append(parts, entry)
    }
    return strings.Join(parts, ", ")
}
```

- [ ] **7d. Add dispatch cases** in `internal/gameserver/grpc_service.go` after the heal cases:

```go
case *gamev1.ClientMessage_TrainJob:
    return s.handleTrainJob(uid, p.TrainJob)
case *gamev1.ClientMessage_ListJobs:
    return s.handleListJobs(uid, p.ListJobs)
case *gamev1.ClientMessage_SetJob:
    return s.handleSetJob(uid, p.SetJob)
```

- [ ] **7e. Run tests** — verify they pass:
  ```sh
  mise exec -- go test ./internal/gameserver/... -run "TestHandleTrain|TestHandleListJobs|TestHandleSetJob" -v 2>&1 | tail -20
  ```

- [ ] **7f. Run full test suite:**
  ```sh
  mise exec -- go test ./... 2>&1 | tail -10
  ```

- [ ] **7g. Commit:**
  ```sh
  git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_job_trainer.go internal/gameserver/grpc_service_job_trainer_test.go
  git commit -m "feat: implement job trainer commands (train, jobs, setjob)"
  ```

---

## Task 8 — AddPlayerOptions: Level field

**Estimated time:** 3 minutes

> **Note:** This task ensures `AddPlayerOptions` exposes a `Level` field so the job trainer tests can seed the player level. If `Level` is already in `AddPlayerOptions`, skip this task.

### Steps

- [ ] **8a. Check** if `Level int` is in `AddPlayerOptions` in `internal/game/session/manager.go`:
  ```sh
  grep -n "Level" /home/cjohannsen/src/mud/internal/game/session/manager.go | head -10
  ```

- [ ] **8b. If missing**, add `Level int` to `AddPlayerOptions` and pass it through to `PlayerSession.Level` in the `AddPlayer` function body.

- [ ] **8c. Verify compilation:**
  ```sh
  mise exec -- go build ./... 2>&1 | head -10
  ```

- [ ] **8d. Commit if changed:**
  ```sh
  git add internal/game/session/manager.go
  git commit -m "feat: expose Level field in AddPlayerOptions"
  ```

---

## Task 9 — Named NPC YAML content

**Estimated time:** 5 minutes

### Steps

- [ ] **9a. Create** `content/npcs/clutch.yaml`:

```yaml
id: clutch
name: Clutch
npc_type: healer
description: >
  A compact woman with grease-stained hands and a battered medkit strapped across her back.
  She operates out of a converted tool locker in The Tinker's Den and charges reasonable rates.
max_hp: 22
ac: 11
level: 4
awareness: 4
personality: neutral
healer:
  price_per_hp: 4
  daily_capacity: 150
```

- [ ] **9b. Create** `content/npcs/tina_wires.yaml`:

```yaml
id: tina_wires
name: Tina Wires
npc_type: healer
description: >
  A lanky technician with a neural-lace visible at her temple, Tina runs a no-questions-asked
  patching service out of Junker's Dream. Her prices are fair; her bedside manner is not.
max_hp: 18
ac: 10
level: 3
awareness: 3
personality: neutral
healer:
  price_per_hp: 3
  daily_capacity: 120
```

- [ ] **9c. Create** `content/npcs/rio_wrench.yaml`:

```yaml
id: rio_wrench
name: Rio Wrench
npc_type: job_trainer
description: >
  A weathered veteran of Rustbucket Ridge with callused hands and a knowing squint.
  Rio has worked every trade worth working in the scrap district and will teach the
  willing — for the right price.
max_hp: 30
ac: 12
level: 5
awareness: 5
personality: neutral
job_trainer:
  offered_jobs:
    - job_id: scavenger
      training_cost: 150
      prerequisites:
        min_level: 1
    - job_id: drifter
      training_cost: 250
      prerequisites:
        min_level: 2
    - job_id: enforcer
      training_cost: 350
      prerequisites:
        min_level: 3
        min_job_level:
          scavenger: 2
```

- [ ] **9d. Validate YAML loads** (runs LoadTemplates which calls Validate):
  ```sh
  mise exec -- go test ./internal/game/npc/... -run "TestLoadTemplates" -v 2>&1 | tail -10
  ```
  If `TestLoadTemplates` does not exist, write a quick smoke check:
  ```sh
  # Quick validation: parse each new file
  mise exec -- go run -C /home/cjohannsen/src/mud ./cmd/mud --validate-npcs 2>&1 | head -5
  ```
  If neither option applies, verify with a short inline test or build check:
  ```sh
  mise exec -- go build ./... 2>&1 | head -5
  ```

- [ ] **9e. Commit:**
  ```sh
  git add content/npcs/clutch.yaml content/npcs/tina_wires.yaml content/npcs/rio_wrench.yaml
  git commit -m "content: add named healer and job trainer NPCs for Rustbucket Ridge"
  ```

---

## Task 10 — JobTrainerConfig.Validate in Template.Validate

**Estimated time:** 5 minutes

REQ-NPC-2a: `Template.Validate()` MUST verify skill IDs in config exist in the skill registry. This requires a `ValidateWithSkills` variant so callers that have the skill registry can pass it in.

### Steps

- [ ] **10a. Write failing tests** in `internal/game/npc/template_test.go`:

```go
// TestTemplate_ValidateWithSkills_UnknownSkill verifies fatal error for unknown skill.
func TestTemplate_ValidateWithSkills_UnknownSkill(t *testing.T) {
    tmpl := &npc.Template{
        ID: "trainer_x", Name: "Trainer X", NPCType: "job_trainer",
        Level: 2, MaxHP: 20, AC: 10,
        JobTrainer: &npc.JobTrainerConfig{
            OfferedJobs: []npc.TrainableJob{
                {
                    JobID: "hacker", TrainingCost: 200,
                    Prerequisites: npc.JobPrerequisites{
                        MinSkillRanks: map[string]string{"nonexistent_skill_abc": "trained"},
                    },
                },
            },
        },
    }
    knownSkills := map[string]bool{"smooth_talk": true}
    err := tmpl.ValidateWithSkills(knownSkills)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "nonexistent_skill_abc")
}

// TestTemplate_ValidateWithSkills_KnownSkill verifies known skill passes.
func TestTemplate_ValidateWithSkills_KnownSkill(t *testing.T) {
    tmpl := &npc.Template{
        ID: "trainer_y", Name: "Trainer Y", NPCType: "job_trainer",
        Level: 2, MaxHP: 20, AC: 10,
        JobTrainer: &npc.JobTrainerConfig{
            OfferedJobs: []npc.TrainableJob{
                {
                    JobID: "hacker", TrainingCost: 200,
                    Prerequisites: npc.JobPrerequisites{
                        MinSkillRanks: map[string]string{"smooth_talk": "trained"},
                    },
                },
            },
        },
    }
    knownSkills := map[string]bool{"smooth_talk": true}
    err := tmpl.ValidateWithSkills(knownSkills)
    assert.NoError(t, err)
}
```

- [ ] **10b. Run tests** — verify they fail:
  ```sh
  mise exec -- go test ./internal/game/npc/... -run "TestTemplate_ValidateWithSkills" -v 2>&1 | head -15
  ```

- [ ] **10c. Implement** `ValidateWithSkills` in `internal/game/npc/template.go` after `Validate`:

```go
// ValidateWithSkills runs Validate and then checks all skill IDs referenced
// in any JobTrainerConfig against the provided skill registry.
//
// REQ-NPC-2a: unknown skill IDs MUST be a fatal load error.
//
// Precondition: t must not be nil; knownSkills may be nil (treated as empty).
// Postcondition: Returns nil iff Validate passes and all skill IDs are known.
func (t *Template) ValidateWithSkills(knownSkills map[string]bool) error {
    if err := t.Validate(); err != nil {
        return err
    }
    if t.JobTrainer != nil {
        if err := t.JobTrainer.Validate(knownSkills); err != nil {
            return fmt.Errorf("npc template %q: %w", t.ID, err)
        }
    }
    return nil
}
```

- [ ] **10d. Run tests** — verify they pass:
  ```sh
  mise exec -- go test ./internal/game/npc/... -run "TestTemplate_ValidateWithSkills" -v 2>&1 | tail -10
  ```

- [ ] **10e. Run full test suite:**
  ```sh
  mise exec -- go test ./... 2>&1 | tail -10
  ```

- [ ] **10f. Commit:**
  ```sh
  git add internal/game/npc/template.go internal/game/npc/template_test.go
  git commit -m "feat: add Template.ValidateWithSkills for REQ-NPC-2a skill registry check"
  ```

---

## Task 11 — Final integration smoke test

**Estimated time:** 3 minutes

### Steps

- [ ] **11a. Run entire test suite:**
  ```sh
  mise exec -- go test ./... 2>&1 | tail -15
  ```

- [ ] **11b. Verify no compilation warnings:**
  ```sh
  mise exec -- go vet ./... 2>&1 | head -10
  ```

- [ ] **11c. Commit** (only if there are any minor fixups needed; otherwise skip):
  ```sh
  git add -p  # stage only fixup files
  git commit -m "fix: address post-integration review findings"
  ```

---

## Requirements Traceability

| Requirement | Task |
|-------------|------|
| REQ-NPC-2a: ValidateWithSkills for skill registry | Task 10 |
| REQ-NPC-9: Exactly one active job after first train | Task 7c (`if sess.ActiveJobID == ""`) |
| REQ-NPC-10: Active job earns XP (field set) | Task 3 (`ActiveJobID`) |
| REQ-NPC-11: Inactive jobs still provide feats/proficiencies | Session field `Jobs` map (Task 3); enforcement is in XP handler (pre-existing) |
| REQ-NPC-16: CapacityUsed resets on daily tick; restored from DB on restart | Task 6b (`tickHealerCapacity`); DB restore note: `initHealerRuntimeState` checks for existing state first |
| REQ-NPC-17: `setjob` available from any room | Task 7c (`handleSetJob` does not check room) |

---

## Implementation Notes

- **DB persistence for HealerRuntimeState and Jobs**: This plan implements in-memory runtime state following the identical pattern used for `MerchantRuntimeState` and `BankerRuntimeState`. Full DB persistence (REQ-NPC-16 "restored from DB on restart") requires a database schema and repository — those belong in a subsequent persistence sub-project and MUST be tracked as a follow-up task. The `initHealerRuntimeState` function initialises `CapacityUsed = 0` on server restart, which is safe because the daily tick will reset it anyway.
- **PlayerSession.Jobs persistence**: `Jobs` and `ActiveJobID` on `PlayerSession` are in-memory for this sub-project; persistence to the character table belongs in the DB sub-project.
- **rio_wrench.yaml job IDs**: The YAML uses `scavenger`, `drifter`, and `enforcer` as illustrative job IDs. Before deploying content, confirm these IDs exist in `content/jobs/`. Adjust to valid job IDs if needed.
- **`setjob` from any room**: `handleSetJob` intentionally does not call `findJobTrainerInRoom`; it operates purely on session state, satisfying REQ-NPC-17.
