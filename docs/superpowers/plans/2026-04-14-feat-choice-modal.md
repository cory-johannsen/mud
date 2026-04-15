# Feat Choice Modal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the raw "Choose N: feat1, feat2, ..." string in the Job Drawer with structured pending feat choice data and a clickable modal in the web client.

**Architecture:** The server's `handleJobGrants()` is modified to emit structured `PendingFeatChoice` messages instead of raw label strings. A new `handleChooseFeat()` handler persists the player's selection. On the web client, `CharacterPanel` shows a notification badge when choices are pending, which opens a `FeatChoiceModal` to resolve them. The `JobDrawer` is updated to show a "Pending choice" badge instead of raw text.

**Tech Stack:** Go (protobuf, grpc), React 18 + TypeScript (Vite), existing dark-theme monospace styling.

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `api/proto/game/v1/game.proto` | Modify | Add `FeatOption`, `PendingFeatChoice`, `ChooseFeatRequest`; extend `JobGrantsResponse` |
| `internal/gameserver/gamev1/game.pb.go` | Regenerate | `make proto` |
| `internal/gameserver/grpc_service.go` | Modify | Update `handleJobGrants()` to populate `PendingFeatChoices`; add dispatch case |
| `internal/gameserver/grpc_service_feat_choice.go` | Create | `handleChooseFeat()` handler |
| `internal/gameserver/grpc_service_feat_choice_test.go` | Create | Handler unit tests |
| `cmd/webclient/handlers/websocket_dispatch.go` | Modify | Register `ChooseFeatRequest` in typeMap and wrap function |
| `cmd/webclient/ui/src/proto/index.ts` | Modify | Add `FeatOption`, `PendingFeatChoice`, `ChooseFeatRequest` TS types; extend `JobGrantsResponse` |
| `cmd/webclient/ui/src/game/drawers/FeatChoiceModal.tsx` | Create | Feat selection modal component |
| `cmd/webclient/ui/src/game/panels/CharacterPanel.tsx` | Modify | Add pending feat notification badge + modal trigger |
| `cmd/webclient/ui/src/game/drawers/JobDrawer.tsx` | Modify | Replace raw "Choose N: ..." text with "Pending choice" badge |

---

### Task 1: Proto — FeatOption, PendingFeatChoice, ChooseFeatRequest

**Files:**
- Modify: `api/proto/game/v1/game.proto:1636-1640`

- [ ] **Step 1: Add new messages after `JobGrantsResponse` (line 1641)**

Open `api/proto/game/v1/game.proto`. After line 1640 (`}`), insert:

```protobuf
// FeatOption describes a single selectable feat in a pending choice pool.
//
// Precondition: feat_id is a valid feat ID in the server's feat registry.
message FeatOption {
  string feat_id     = 1;
  string name        = 2;
  string description = 3;
  string category    = 4;
}

// PendingFeatChoice describes one unresolved feat choice pool at a specific grant level.
//
// Precondition: count >= 1; options is non-empty; grant_level >= 1.
message PendingFeatChoice {
  int32             grant_level = 1;
  int32             count       = 2;
  repeated FeatOption options   = 3;
}

// ChooseFeatRequest asks the server to resolve one pending feat choice slot.
//
// Precondition: grant_level >= 1; feat_id is non-empty.
message ChooseFeatRequest {
  int32  grant_level = 1;
  string feat_id     = 2;
}
```

- [ ] **Step 2: Add `pending_feat_choices` field to `JobGrantsResponse` (line 1637-1640)**

Change:

```protobuf
message JobGrantsResponse {
  repeated JobFeatGrant feat_grants = 1;
  repeated JobTechGrant tech_grants = 2;
}
```

To:

```protobuf
message JobGrantsResponse {
  repeated JobFeatGrant      feat_grants          = 1;
  repeated JobTechGrant      tech_grants          = 2;
  repeated PendingFeatChoice pending_feat_choices = 3;
}
```

- [ ] **Step 3: Add `ChooseFeatRequest` to `ClientMessage` oneof**

The highest existing field in `ClientMessage` is `train_tech = 138` (line 170). Add at line 171 (just before the closing `}`):

```protobuf
    ChooseFeatRequest    choose_feat           = 139;
```

- [ ] **Step 4: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud && make proto 2>&1 | tail -10
```

Expected: no errors; `internal/gameserver/gamev1/game.pb.go` updated.

- [ ] **Step 5: Verify build**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go
git commit -m "feat(proto): add FeatOption, PendingFeatChoice, ChooseFeatRequest for feat choice modal"
```

---

### Task 2: Server — Modify handleJobGrants() to populate PendingFeatChoices

**Files:**
- Modify: `internal/gameserver/grpc_service.go:7186-7282`

Context: The `addFeatGrants` closure (starting around line 7235) handles unresolved choice pool entries at lines 7270-7282 by building a raw `"Choose N: feat1, feat2, ..."` label. We change the unresolved branch to emit a structured `PendingFeatChoice` instead, and the return value at lines 7443-7450 to include the pending choices.

- [ ] **Step 1: Write the failing test**

Add to `internal/gameserver/grpc_service_feat_choice_test.go` (new file — create it now with just this test):

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleJobGrants_UnresolvedChoicePool_PopulatesPendingFeatChoices verifies
// that handleJobGrants returns a PendingFeatChoice (not a raw label string) when
// the player has not yet selected from a feat choice pool (REQ-FCM-1, REQ-FCM-2).
//
// Precondition: Player has no feats from the choice pool; feat registry has pool feats.
// Postcondition: JobGrantsResponse.PendingFeatChoices has one entry; FeatGrant for
//                that level has empty feat_id and feat_name.
func TestHandleJobGrants_UnresolvedChoicePool_PopulatesPendingFeatChoices(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_job_grants", Username: "u_job_grants", CharName: "u_job_grants",
		RoomID: "room_a", Role: "player", CurrentHP: 20, MaxHP: 20,
	})
	require.NoError(t, err)
	sess.Level = 2

	// Build a job with a level-2 feat choice pool [rage, overpower].
	job := &ruleset.Job{
		ID:       "test_job",
		Name:     "Test Job",
		Archetype: "",
		LevelUpFeatGrants: map[int]*ruleset.FeatGrants{
			2: {
				Choices: &ruleset.FeatChoices{
					Pool:  []string{"rage", "overpower"},
					Count: 1,
				},
			},
		},
	}
	jobRegistry := ruleset.NewJobRegistry([]*ruleset.Job{job})
	sess.Class = "test_job"

	rageF := &ruleset.Feat{ID: "rage", Name: "Rage", Description: "Enter a rage.", Category: "job"}
	overpowerF := &ruleset.Feat{ID: "overpower", Name: "Overpower", Description: "Overpower your foe.", Category: "job"}
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{rageF, overpowerF})

	svc := newTestGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npc.NewManager(), nil, nil, nil),
		NewChatHandler(sessMgr),
		zaptest.NewLogger(t),
		nil, dice.NewLoggedRoller(&fixedDiceSource{val: 10}, zaptest.NewLogger(t)), nil,
		npc.NewManager(),
		NewCombatHandler(combat.NewEngine(), npc.NewManager(), sessMgr,
			dice.NewLoggedRoller(&fixedDiceSource{val: 10}, zaptest.NewLogger(t)),
			func(_ string, _ []*gamev1.CombatEvent) {}, testRoundDuration,
			makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil,
			mentalstate.NewManager(),
		),
		nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, makeTestConditionRegistry(), nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		[]*ruleset.Feat{rageF, overpowerF}, featRegistry, &stubFeatsRepo{data: map[int64][]string{sess.CharacterID: {}}},
		nil, nil, nil, nil, nil, nil, jobRegistry,
		mentalstate.NewManager(), nil,
		nil, nil,
		nil, nil,
	)

	evt, err := svc.handleJobGrants("u_job_grants")
	require.NoError(t, err)
	require.NotNil(t, evt)

	resp := evt.GetJobGrantsResponse()
	require.NotNil(t, resp)

	// The level-2 feat grant row must have empty feat_id and feat_name (not "Choose 1: Rage, Overpower").
	var level2Grant *gamev1.JobFeatGrant
	for _, g := range resp.FeatGrants {
		if g.GrantLevel == 2 {
			level2Grant = g
			break
		}
	}
	require.NotNil(t, level2Grant, "expected a feat grant row for level 2")
	assert.Empty(t, level2Grant.FeatId, "REQ-FCM-1: unresolved grant must have empty feat_id")
	assert.Empty(t, level2Grant.FeatName, "REQ-FCM-1: unresolved grant must have empty feat_name")

	// PendingFeatChoices must contain one entry for level 2 with both pool feats.
	require.Len(t, resp.PendingFeatChoices, 1, "REQ-FCM-2: one PendingFeatChoice for level 2")
	pfc := resp.PendingFeatChoices[0]
	assert.Equal(t, int32(2), pfc.GrantLevel)
	assert.Equal(t, int32(1), pfc.Count)
	require.Len(t, pfc.Options, 2)
	assert.Equal(t, "rage", pfc.Options[0].FeatId)
	assert.Equal(t, "Rage", pfc.Options[0].Name)
	assert.Equal(t, "Enter a rage.", pfc.Options[0].Description)
	assert.Equal(t, "overpower", pfc.Options[1].FeatId)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleJobGrants_UnresolvedChoicePool -count=1 -timeout=60s 2>&1 | tail -20
```

Expected: FAIL — `PendingFeatChoices` is nil/empty; `FeatName` still has "Choose 1: ..." string.

- [ ] **Step 3: Modify handleJobGrants() — declare pendingFeatChoices accumulator**

In `internal/gameserver/grpc_service.go` at line ~7187, change the variable block:

```go
var featGrants []*gamev1.JobFeatGrant
var techGrants []*gamev1.JobTechGrant
```

To:

```go
var featGrants []*gamev1.JobFeatGrant
var techGrants []*gamev1.JobTechGrant
var pendingFeatChoices []*gamev1.PendingFeatChoice
```

- [ ] **Step 4: Replace the unresolved choice branch in addFeatGrants**

In `addFeatGrants` closure, find the `else` branch (~line 7270) that builds the raw label:

```go
} else {
    // Player has not yet chosen — show the pool label.
    poolNames := make([]string, 0, len(fg.Choices.Pool))
    for _, id := range fg.Choices.Pool {
        poolNames = append(poolNames, featName(id))
    }
    label := fmt.Sprintf("Choose %d: %s", fg.Choices.Count, strings.Join(poolNames, ", "))
    featGrants = append(featGrants, &gamev1.JobFeatGrant{
        GrantLevel: int32(level),
        FeatId:     "",
        FeatName:   label,
    })
}
```

Replace with:

```go
} else {
    // Player has not yet chosen — emit an empty grant row and a structured PendingFeatChoice.
    featGrants = append(featGrants, &gamev1.JobFeatGrant{
        GrantLevel: int32(level),
        FeatId:     "",
        FeatName:   "",
    })
    opts := make([]*gamev1.FeatOption, 0, len(fg.Choices.Pool))
    for _, id := range fg.Choices.Pool {
        opt := &gamev1.FeatOption{FeatId: id, Name: featName(id)}
        if s.featRegistry != nil {
            if f, ok := s.featRegistry.Feat(id); ok {
                opt.Description = f.Description
                opt.Category = f.Category
            }
        }
        opts = append(opts, opt)
    }
    pendingFeatChoices = append(pendingFeatChoices, &gamev1.PendingFeatChoice{
        GrantLevel: int32(level),
        Count:      int32(fg.Choices.Count),
        Options:    opts,
    })
}
```

- [ ] **Step 5: Update the return value to include pendingFeatChoices**

Find the return statement (~line 7443):

```go
return &gamev1.ServerEvent{
    Payload: &gamev1.ServerEvent_JobGrantsResponse{
        JobGrantsResponse: &gamev1.JobGrantsResponse{
            FeatGrants: featGrants,
            TechGrants: techGrants,
        },
    },
```

Change to:

```go
return &gamev1.ServerEvent{
    Payload: &gamev1.ServerEvent_JobGrantsResponse{
        JobGrantsResponse: &gamev1.JobGrantsResponse{
            FeatGrants:         featGrants,
            TechGrants:         techGrants,
            PendingFeatChoices: pendingFeatChoices,
        },
    },
```

- [ ] **Step 6: Run test to verify it passes**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestHandleJobGrants_UnresolvedChoicePool -count=1 -timeout=60s 2>&1 | tail -10
```

Expected: PASS.

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -count=1 -timeout=120s 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_feat_choice_test.go
git commit -m "feat(gameserver): populate PendingFeatChoices in handleJobGrants instead of raw label string"
```

---

### Task 3: Server — handleChooseFeat() handler

**Files:**
- Create: `internal/gameserver/grpc_service_feat_choice.go`
- Modify: `internal/gameserver/grpc_service_feat_choice_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/gameserver/grpc_service_feat_choice_test.go`:

```go
// TestHandleChooseFeat_ValidSelection_StoresFeat verifies that a valid feat choice
// persists the feat, marks the level granted, and pushes updated events (REQ-FCM-8, REQ-FCM-9).
//
// Precondition: Player at level 2 with unresolved pool [rage, overpower]; feats repo is empty.
// Postcondition: feat "rage" added to feats repo; level 2 marked granted.
func TestHandleChooseFeat_ValidSelection_StoresFeat(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_choose_feat", Username: "u_choose_feat", CharName: "u_choose_feat",
		RoomID: "room_a", Role: "player", CurrentHP: 20, MaxHP: 20,
	})
	require.NoError(t, err)
	sess.Level = 2

	job := &ruleset.Job{
		ID: "test_job", Name: "Test Job",
		LevelUpFeatGrants: map[int]*ruleset.FeatGrants{
			2: {Choices: &ruleset.FeatChoices{Pool: []string{"rage", "overpower"}, Count: 1}},
		},
	}
	rageF := &ruleset.Feat{ID: "rage", Name: "Rage", Description: "Enter a rage.", Category: "job"}
	overpowerF := &ruleset.Feat{ID: "overpower", Name: "Overpower", Description: "Overpower foe.", Category: "job"}
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{rageF, overpowerF})
	featsRepo := &stubFeatsRepo{data: map[int64][]string{sess.CharacterID: {}}}
	featLevelGrantsRepo := &stubFeatLevelGrantsRepo{}
	sess.Class = "test_job"

	svc := buildChooseFeatSvc(t, worldMgr, sessMgr, job, []*ruleset.Feat{rageF, overpowerF}, featRegistry, featsRepo, featLevelGrantsRepo)

	evt, err := svc.handleChooseFeat("u_choose_feat", 2, "rage")
	require.NoError(t, err)
	require.NotNil(t, evt)

	// Feat must be stored.
	assert.Contains(t, featsRepo.data[sess.CharacterID], "rage", "REQ-FCM-9: feat must be persisted")
	// Level must be marked granted.
	granted, _ := featLevelGrantsRepo.IsLevelGranted(context.Background(), sess.CharacterID, 2)
	assert.True(t, granted, "REQ-FCM-9: level 2 must be marked granted")
}

// TestHandleChooseFeat_FeatNotInPool_ReturnsDenial verifies that choosing a feat
// not in the pool returns a denial event and does not modify state (REQ-FCM-8).
//
// Precondition: Player at level 2 with pool [rage, overpower]; feat_id "unknown_feat" not in pool.
// Postcondition: Returns non-nil event with denial message; feats repo unchanged.
func TestHandleChooseFeat_FeatNotInPool_ReturnsDenial(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_choose_feat_bad", Username: "u_choose_feat_bad", CharName: "u_choose_feat_bad",
		RoomID: "room_a", Role: "player", CurrentHP: 20, MaxHP: 20,
	})
	require.NoError(t, err)
	sess.Level = 2
	sess.Class = "test_job"

	job := &ruleset.Job{
		ID: "test_job", Name: "Test Job",
		LevelUpFeatGrants: map[int]*ruleset.FeatGrants{
			2: {Choices: &ruleset.FeatChoices{Pool: []string{"rage", "overpower"}, Count: 1}},
		},
	}
	rageF := &ruleset.Feat{ID: "rage", Name: "Rage", Description: "Enter a rage.", Category: "job"}
	overpowerF := &ruleset.Feat{ID: "overpower", Name: "Overpower", Description: "Overpower foe.", Category: "job"}
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{rageF, overpowerF})
	featsRepo := &stubFeatsRepo{data: map[int64][]string{sess.CharacterID: {}}}
	featLevelGrantsRepo := &stubFeatLevelGrantsRepo{}

	svc := buildChooseFeatSvc(t, worldMgr, sessMgr, job, []*ruleset.Feat{rageF, overpowerF}, featRegistry, featsRepo, featLevelGrantsRepo)

	evt, err := svc.handleChooseFeat("u_choose_feat_bad", 2, "unknown_feat")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg, "REQ-FCM-8: denial must return a non-empty message")
	assert.Empty(t, featsRepo.data[sess.CharacterID], "REQ-FCM-8: feats repo must be unchanged on denial")
}

// TestHandleChooseFeat_AlreadyOwned_ReturnsDenial verifies that choosing a feat
// the player already owns returns a denial and does not modify state (REQ-FCM-8).
//
// Precondition: Player at level 2 with pool [rage, overpower]; player already owns "rage".
// Postcondition: Returns denial; feats repo not duplicated; level not re-marked.
func TestHandleChooseFeat_AlreadyOwned_ReturnsDenial(t *testing.T) {
	worldMgr, sessMgr := testWorldAndSession(t)
	sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: "u_choose_already", Username: "u_choose_already", CharName: "u_choose_already",
		RoomID: "room_a", Role: "player", CurrentHP: 20, MaxHP: 20,
	})
	require.NoError(t, err)
	sess.Level = 2
	sess.Class = "test_job"

	job := &ruleset.Job{
		ID: "test_job", Name: "Test Job",
		LevelUpFeatGrants: map[int]*ruleset.FeatGrants{
			2: {Choices: &ruleset.FeatChoices{Pool: []string{"rage", "overpower"}, Count: 1}},
		},
	}
	rageF := &ruleset.Feat{ID: "rage", Name: "Rage", Description: "Enter a rage.", Category: "job"}
	overpowerF := &ruleset.Feat{ID: "overpower", Name: "Overpower", Description: "Overpower foe.", Category: "job"}
	featRegistry := ruleset.NewFeatRegistry([]*ruleset.Feat{rageF, overpowerF})
	// Player already owns "rage".
	featsRepo := &stubFeatsRepo{data: map[int64][]string{sess.CharacterID: {"rage"}}}
	featLevelGrantsRepo := &stubFeatLevelGrantsRepo{}

	svc := buildChooseFeatSvc(t, worldMgr, sessMgr, job, []*ruleset.Feat{rageF, overpowerF}, featRegistry, featsRepo, featLevelGrantsRepo)

	evt, err := svc.handleChooseFeat("u_choose_already", 2, "rage")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.NotEmpty(t, msg, "REQ-FCM-8: denial must return a non-empty message")
	// feats repo still has exactly ["rage"] — not duplicated or grown.
	assert.Len(t, featsRepo.data[sess.CharacterID], 1, "REQ-FCM-8: feats repo must not be modified")
}

// stubFeatLevelGrantsRepo is an in-memory CharacterFeatLevelGrantsRepo for tests.
type stubFeatLevelGrantsRepo struct {
	granted map[int64]map[int]bool
}

func (r *stubFeatLevelGrantsRepo) IsLevelGranted(ctx context.Context, charID int64, level int) (bool, error) {
	if r.granted == nil {
		return false, nil
	}
	return r.granted[charID][level], nil
}

func (r *stubFeatLevelGrantsRepo) MarkLevelGranted(ctx context.Context, charID int64, level int) error {
	if r.granted == nil {
		r.granted = make(map[int64]map[int]bool)
	}
	if r.granted[charID] == nil {
		r.granted[charID] = make(map[int]bool)
	}
	r.granted[charID][level] = true
	return nil
}

// buildChooseFeatSvc is a test helper that wires a GameServiceServer with the
// minimum dependencies needed to exercise handleChooseFeat.
func buildChooseFeatSvc(
	t *testing.T,
	worldMgr interface{ /* worldMgr */ },
	sessMgr *session.Manager,
	job *ruleset.Job,
	feats []*ruleset.Feat,
	featRegistry *ruleset.FeatRegistry,
	featsRepo *stubFeatsRepo,
	featLevelGrantsRepo *stubFeatLevelGrantsRepo,
) *GameServiceServer {
	t.Helper()
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	condReg := makeTestConditionRegistry()
	roller := dice.NewLoggedRoller(&fixedDiceSource{val: 10}, logger)
	jobRegistry := ruleset.NewJobRegistry([]*ruleset.Job{job})
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, condReg, nil, nil, nil, nil, nil, nil,
		mentalstate.NewManager(),
	)
	featIDs := make([]string, len(feats))
	for i, f := range feats {
		featIDs[i] = f.ID
	}
	return newTestGameServiceServer(
		worldMgr.(interface{ /* worldMgr */ }),
		sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr.(interface{ /* worldMgr */ }), sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil,
		nil, nil, condReg, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		feats, featRegistry, featsRepo,
		nil, nil, nil, nil, nil, nil, jobRegistry,
		mentalstate.NewManager(), nil,
		nil, nil,
		nil, nil,
	)
}
```

**Note on `buildChooseFeatSvc`:** The `worldMgr` parameter type needs to match the actual type. Look at how `testWorldAndSession` is called in other tests in the same package and use the same type. If it returns `*world.Manager`, change the parameter type accordingly.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleChooseFeat" -count=1 -timeout=60s 2>&1 | tail -20
```

Expected: compile error or FAIL — `handleChooseFeat` not defined yet.

- [ ] **Step 3: Create grpc_service_feat_choice.go**

Create `internal/gameserver/grpc_service_feat_choice.go`:

```go
package gameserver

import (
	"context"
	"fmt"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// handleChooseFeat resolves one pending feat choice slot for a player.
//
// Precondition: uid non-empty; grantLevel >= 1; featID non-empty.
// Postcondition: On success — feat persisted, grant level marked, CharacterSheetView and
//                JobGrantsResponse pushed to player stream; success MessageEvent returned.
//                On denial — MessageEvent with reason returned; no state modified.
func (s *GameServiceServer) handleChooseFeat(uid string, grantLevel int, featID string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, fmt.Errorf("player %q not found", uid)
	}

	// Step 1: Get current pending choices from handleJobGrants.
	grantsEvt, err := s.handleJobGrants(uid)
	if err != nil {
		return nil, fmt.Errorf("handleChooseFeat: failed to load job grants: %w", err)
	}
	jobResp := grantsEvt.GetJobGrantsResponse()
	if jobResp == nil {
		return messageEvent("Feat grant data is not available."), nil
	}

	// Step 2: Find the PendingFeatChoice for the requested grant level.
	var pendingChoice *gamev1.PendingFeatChoice
	for _, pfc := range jobResp.PendingFeatChoices {
		if pfc.GrantLevel == int32(grantLevel) {
			pendingChoice = pfc
			break
		}
	}
	if pendingChoice == nil {
		return messageEvent(fmt.Sprintf("No pending feat choice at level %d.", grantLevel)), nil
	}

	// Step 3: Verify feat_id is in the pool.
	inPool := false
	for _, opt := range pendingChoice.Options {
		if opt.FeatId == featID {
			inPool = true
			break
		}
	}
	if !inPool {
		return messageEvent(fmt.Sprintf("%q is not a valid choice at level %d.", featID, grantLevel)), nil
	}

	// Step 4: Verify player does not already own the feat.
	if s.characterFeatsRepo != nil {
		existing, err := s.characterFeatsRepo.GetAll(context.Background(), sess.CharacterID)
		if err != nil {
			return nil, fmt.Errorf("handleChooseFeat: GetAll feats: %w", err)
		}
		for _, id := range existing {
			if id == featID {
				return messageEvent(fmt.Sprintf("You already have %q.", featID)), nil
			}
		}
	}

	// Step 5: Persist the feat.
	if s.characterFeatsRepo != nil {
		if err := s.characterFeatsRepo.Add(context.Background(), sess.CharacterID, featID); err != nil {
			return nil, fmt.Errorf("handleChooseFeat: Add feat: %w", err)
		}
	}

	// Step 6: Mark grant level as fulfilled.
	if s.featLevelGrantsRepo != nil {
		if err := s.featLevelGrantsRepo.MarkLevelGranted(context.Background(), sess.CharacterID, grantLevel); err != nil {
			return nil, fmt.Errorf("handleChooseFeat: MarkLevelGranted: %w", err)
		}
	}

	// Step 7: Push updated CharacterSheetView and JobGrantsResponse to the player stream.
	s.pushCharacterSheet(sess)
	if updatedGrantsEvt, err := s.handleJobGrants(uid); err == nil && updatedGrantsEvt != nil {
		s.pushEventToUID(uid, updatedGrantsEvt)
	}

	// Step 8: Return a success event (not pushed — returned via the dispatch loop).
	featName := featID
	if s.featRegistry != nil {
		if f, ok := s.featRegistry.Feat(featID); ok {
			featName = f.Name
		}
	}
	return messageEvent(fmt.Sprintf("You have learned %s.", featName)), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleChooseFeat" -count=1 -timeout=60s 2>&1 | tail -20
```

Expected: all three tests PASS.

- [ ] **Step 5: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -count=1 -timeout=120s 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/grpc_service_feat_choice.go internal/gameserver/grpc_service_feat_choice_test.go
git commit -m "feat(gameserver): implement handleChooseFeat handler (REQ-FCM-8, REQ-FCM-9)"
```

---

### Task 4: Server — Wire ChooseFeatRequest dispatch

**Files:**
- Modify: `internal/gameserver/grpc_service.go:2337-2338` (near `case *gamev1.ClientMessage_TrainTech:`)

- [ ] **Step 1: Add dispatch case**

In `internal/gameserver/grpc_service.go`, find the dispatch switch near line 2337:

```go
case *gamev1.ClientMessage_TrainTech:
    return s.handleTrainTech(uid, p.TrainTech.GetNpcName(), p.TrainTech.GetTechId())
```

Add immediately after:

```go
case *gamev1.ClientMessage_ChooseFeat:
    return s.handleChooseFeat(uid, int(p.ChooseFeat.GetGrantLevel()), p.ChooseFeat.GetFeatId())
```

- [ ] **Step 2: Build to verify**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 3: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -count=1 -timeout=120s 2>&1 | tail -10
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go
git commit -m "feat(gameserver): wire ChooseFeatRequest dispatch to handleChooseFeat"
```

---

### Task 5: Webclient Go — Register ChooseFeatRequest in dispatcher

**Files:**
- Modify: `cmd/webclient/handlers/websocket_dispatch.go:843-947`

- [ ] **Step 1: Add to typeMap**

In `websocket_dispatch.go`, find the typeMap around line 843 (near `"JobGrantsRequest"`):

```go
"JobGrantsRequest":       func() proto.Message { return &gamev1.JobGrantsRequest{} },
"QuestLogRequest":        func() proto.Message { return &gamev1.QuestLogRequest{} },
```

Add after `"QuestLogRequest"` line:

```go
"ChooseFeatRequest":      func() proto.Message { return &gamev1.ChooseFeatRequest{} },
```

- [ ] **Step 2: Add to wrapProtoAsClientMessage**

Find the switch at line ~940:

```go
case *gamev1.JobGrantsRequest:
    cm.Payload = &gamev1.ClientMessage_JobGrantsRequest{JobGrantsRequest: m}
case *gamev1.QuestLogRequest:
    cm.Payload = &gamev1.ClientMessage_QuestLogRequest{QuestLogRequest: m}
default:
```

Add before `default:`:

```go
case *gamev1.ChooseFeatRequest:
    cm.Payload = &gamev1.ClientMessage_ChooseFeat{ChooseFeat: m}
```

- [ ] **Step 3: Build to verify**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add cmd/webclient/handlers/websocket_dispatch.go
git commit -m "feat(webclient): register ChooseFeatRequest in websocket dispatcher"
```

---

### Task 6: TypeScript — Update proto/index.ts types

**Files:**
- Modify: `cmd/webclient/ui/src/proto/index.ts:614-641`

- [ ] **Step 1: Add FeatOption and PendingFeatChoice interfaces**

In `proto/index.ts`, after line 621 (`}` closing `JobFeatGrant`), add:

```typescript
export interface FeatOption {
  featId?: string
  feat_id?: string
  name?: string
  description?: string
  category?: string
}

export interface PendingFeatChoice {
  grantLevel?: number
  grant_level?: number
  count?: number
  options?: FeatOption[]
}

export interface ChooseFeatRequest {
  grantLevel?: number
  grant_level?: number
  featId?: string
  feat_id?: string
}
```

- [ ] **Step 2: Extend JobGrantsResponse**

Change (lines 636-641):

```typescript
export interface JobGrantsResponse {
  featGrants?: JobFeatGrant[]
  feat_grants?: JobFeatGrant[]
  techGrants?: JobTechGrant[]
  tech_grants?: JobTechGrant[]
}
```

To:

```typescript
export interface JobGrantsResponse {
  featGrants?: JobFeatGrant[]
  feat_grants?: JobFeatGrant[]
  techGrants?: JobTechGrant[]
  tech_grants?: JobTechGrant[]
  pendingFeatChoices?: PendingFeatChoice[]
  pending_feat_choices?: PendingFeatChoice[]
}
```

- [ ] **Step 3: Verify TypeScript compiles**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run build 2>&1 | tail -20
```

Expected: no TypeScript errors.

- [ ] **Step 4: Commit**

```bash
git add cmd/webclient/ui/src/proto/index.ts
git commit -m "feat(webclient): add FeatOption, PendingFeatChoice, ChooseFeatRequest TypeScript types"
```

---

### Task 7: Webclient — FeatChoiceModal component

**Files:**
- Create: `cmd/webclient/ui/src/game/drawers/FeatChoiceModal.tsx`

- [ ] **Step 1: Write the component**

Create `cmd/webclient/ui/src/game/drawers/FeatChoiceModal.tsx`:

```tsx
import { useState } from 'react'
import { useGame } from '../GameContext'
import type { PendingFeatChoice } from '../../proto/index'

// FeatChoiceModal presents a feat selection modal for pending feat choices.
//
// REQ-FCM-4: Opens when player clicks the notification badge.
// REQ-FCM-5: Shows each feat option with name, category badge, and description.
// REQ-FCM-6: Confirm button disabled until selected count === choice.count.
// REQ-FCM-7: On confirm, sends one ChooseFeatRequest per selected feat.
// REQ-FCM-11: Dark-theme monospace styling matching AbilityBoostModal.

interface Props {
  choices: PendingFeatChoice[]
  onClose: () => void
}

export function FeatChoiceModal({ choices, onClose }: Props) {
  const { sendMessage } = useGame()
  const [choiceIndex, setChoiceIndex] = useState(0)
  const [selected, setSelected] = useState<string[]>([])

  if (choices.length === 0) return null
  const choice = choices[choiceIndex]
  const required = choice.count ?? 1
  const options = choice.options ?? []
  const grantLevel = choice.grantLevel ?? choice.grant_level ?? 1

  function toggleFeat(featId: string) {
    setSelected(prev => {
      if (prev.includes(featId)) return prev.filter(id => id !== featId)
      if (prev.length >= required) return prev
      return [...prev, featId]
    })
  }

  function handleConfirm() {
    for (const featId of selected) {
      sendMessage('ChooseFeatRequest', { grantLevel, featId })
    }
    if (choiceIndex + 1 < choices.length) {
      setChoiceIndex(i => i + 1)
      setSelected([])
    } else {
      onClose()
    }
  }

  const categoryColor = (cat?: string) => {
    if (cat === 'job') return '#ffcc88'
    if (cat === 'skill') return '#88ccff'
    return '#aaa'
  }

  return (
    <div style={styles.overlay}>
      <div style={styles.modal} onClick={e => e.stopPropagation()}>
        <div style={styles.header}>
          <span style={styles.title}>
            Choose {required} Feat{required !== 1 ? 's' : ''} — Level {grantLevel}
          </span>
          {choices.length > 1 && (
            <span style={styles.pager}>{choiceIndex + 1} / {choices.length}</span>
          )}
        </div>
        <div style={styles.body}>
          {options.map(opt => {
            const featId = opt.featId ?? opt.feat_id ?? ''
            const isSelected = selected.includes(featId)
            return (
              <button
                key={featId}
                type="button"
                style={{ ...styles.card, ...(isSelected ? styles.cardSelected : {}) }}
                onClick={() => toggleFeat(featId)}
              >
                <div style={styles.cardHeader}>
                  <span style={styles.featName}>{opt.name ?? featId}</span>
                  {opt.category && (
                    <span style={{ ...styles.categoryBadge, color: categoryColor(opt.category), borderColor: categoryColor(opt.category) }}>
                      {opt.category}
                    </span>
                  )}
                </div>
                {opt.description && (
                  <div style={styles.description}>{opt.description}</div>
                )}
              </button>
            )
          })}
        </div>
        <div style={styles.footer}>
          <span style={styles.counter}>{selected.length} / {required} selected</span>
          <button
            type="button"
            style={{ ...styles.confirmBtn, ...(selected.length !== required ? styles.confirmBtnDisabled : {}) }}
            disabled={selected.length !== required}
            onClick={handleConfirm}
          >
            Confirm
          </button>
        </div>
      </div>
    </div>
  )
}

const styles: Record<string, React.CSSProperties> = {
  overlay: {
    position: 'fixed',
    inset: 0,
    background: 'rgba(0,0,0,0.8)',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 300,
  },
  modal: {
    background: '#111',
    border: '1px solid #4a6a2a',
    borderRadius: '6px',
    width: 'min(560px, 95vw)',
    maxHeight: '80vh',
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
    fontFamily: 'monospace',
  },
  header: {
    padding: '0.75rem 1rem',
    borderBottom: '1px solid #2a3a1a',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    flexShrink: 0,
  },
  title: {
    color: '#e0c060',
    fontSize: '0.95rem',
    fontWeight: 600,
  },
  pager: {
    color: '#666',
    fontSize: '0.8rem',
  },
  body: {
    padding: '0.75rem 1rem',
    display: 'flex',
    flexDirection: 'column',
    gap: '0.5rem',
    overflowY: 'auto',
  },
  card: {
    background: '#1a2a1a',
    border: '1px solid #333',
    borderRadius: '4px',
    padding: '0.6rem 0.75rem',
    textAlign: 'left',
    cursor: 'pointer',
    fontFamily: 'monospace',
    color: '#ccc',
    width: '100%',
  },
  cardSelected: {
    background: '#1a3a1a',
    border: '1px solid #8d4',
  },
  cardHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: '0.5rem',
    marginBottom: '0.25rem',
  },
  featName: {
    fontWeight: 700,
    color: '#eee',
    fontSize: '0.9rem',
  },
  categoryBadge: {
    fontSize: '0.7rem',
    border: '1px solid',
    borderRadius: '4px',
    padding: '0 4px',
  },
  description: {
    color: '#999',
    fontSize: '0.8rem',
    lineHeight: 1.4,
  },
  footer: {
    padding: '0.5rem 1rem',
    borderTop: '1px solid #2a3a1a',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    flexShrink: 0,
  },
  counter: {
    color: '#888',
    fontSize: '0.8rem',
  },
  confirmBtn: {
    background: '#3a5a1a',
    border: '1px solid #8d4',
    color: '#8d4',
    borderRadius: '4px',
    padding: '0.3rem 0.9rem',
    fontFamily: 'monospace',
    fontSize: '0.85rem',
    cursor: 'pointer',
  },
  confirmBtnDisabled: {
    background: '#222',
    border: '1px solid #444',
    color: '#555',
    cursor: 'not-allowed',
  },
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run build 2>&1 | tail -20
```

Expected: no TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/webclient/ui/src/game/drawers/FeatChoiceModal.tsx
git commit -m "feat(webclient): add FeatChoiceModal component (REQ-FCM-4, REQ-FCM-5, REQ-FCM-6, REQ-FCM-7)"
```

---

### Task 8: Webclient — CharacterPanel notification badge

**Files:**
- Modify: `cmd/webclient/ui/src/game/panels/CharacterPanel.tsx`

The `CharacterPanel` currently reads `state.jobGrants` from `useGame()`. It does NOT currently fetch `JobGrantsRequest`. We add a fetch call alongside the existing `CharacterSheetRequest` in the `useEffect`, extend the `Modal` type to include `'feat'`, and add the badge + modal.

- [ ] **Step 1: Update imports and Modal type**

At the top of `CharacterPanel.tsx`, the `FeatChoiceModal` import is needed. Add after existing imports:

```tsx
import { FeatChoiceModal } from '../drawers/FeatChoiceModal'
```

Change (line 125):

```tsx
type Modal = 'boost' | 'skill' | null
```

To:

```tsx
type Modal = 'boost' | 'skill' | 'feat' | null
```

- [ ] **Step 2: Add JobGrantsRequest to useEffect**

Find the `useEffect` starting at line 133. Change:

```tsx
useEffect(() => {
  if (!characterSheet) {
    sendMessage('CharacterSheetRequest', {})
    retryRef.current = setInterval(() => {
      sendMessage('CharacterSheetRequest', {})
    }, 3000)
  } else {
    if (retryRef.current !== null) {
      clearInterval(retryRef.current)
      retryRef.current = null
    }
  }
  return () => {
    if (retryRef.current !== null) {
      clearInterval(retryRef.current)
      retryRef.current = null
    }
  }
}, [characterSheet, sendMessage])
```

To:

```tsx
useEffect(() => {
  sendMessage('JobGrantsRequest', {})
  if (!characterSheet) {
    sendMessage('CharacterSheetRequest', {})
    retryRef.current = setInterval(() => {
      sendMessage('CharacterSheetRequest', {})
    }, 3000)
  } else {
    if (retryRef.current !== null) {
      clearInterval(retryRef.current)
      retryRef.current = null
    }
  }
  return () => {
    if (retryRef.current !== null) {
      clearInterval(retryRef.current)
      retryRef.current = null
    }
  }
}, [characterSheet, sendMessage])
```

- [ ] **Step 3: Derive pendingFeatChoices from state**

After line 186 (`const pendingSkillInc = characterSheet?.pendingSkillIncreases ?? 0`), add:

```tsx
const pendingFeatChoices = state.jobGrants?.pendingFeatChoices ?? state.jobGrants?.pending_feat_choices ?? []
```

- [ ] **Step 4: Add 'feat' modal render**

After line 194 (`{modal === 'skill' && (`):

```tsx
{modal === 'feat' && (
  <FeatChoiceModal
    choices={pendingFeatChoices}
    onClose={() => setModal(null)}
  />
)}
```

- [ ] **Step 5: Add notification badge**

After line 255 (`</button>` for skill increase), add:

```tsx
{pendingFeatChoices.length > 0 && (
  <button style={styles.pendingBtn} onClick={() => setModal('feat')} type="button">
    ★ {pendingFeatChoices.length} Feat Choice{pendingFeatChoices.length !== 1 ? 's' : ''} Available — Choose
  </button>
)}
```

- [ ] **Step 6: Verify TypeScript compiles**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run build 2>&1 | tail -20
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add cmd/webclient/ui/src/game/panels/CharacterPanel.tsx
git commit -m "feat(webclient): add pending feat choice notification badge to CharacterPanel (REQ-FCM-3, REQ-FCM-4)"
```

---

### Task 9: Webclient — Fix JobDrawer raw text

**Files:**
- Modify: `cmd/webclient/ui/src/game/drawers/JobDrawer.tsx:104-109`

- [ ] **Step 1: Replace raw feat name rendering for unresolved rows**

In `JobDrawer.tsx`, find lines 104-109:

```tsx
{feats.map((g, i) => (
  <div key={`feat-${i}`} style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '3px', paddingLeft: '4px' }}>
    <span style={{ fontSize: '0.7rem', color: '#a0c8ff', background: 'rgba(100,150,255,0.15)', border: '1px solid rgba(100,150,255,0.3)', borderRadius: '4px', padding: '0 4px' }}>feat</span>
    <span style={{ color: '#ddd', fontSize: '0.85rem' }}>{g.featName ?? g.feat_name ?? g.featId ?? g.feat_id}</span>
  </div>
))}
```

Replace with:

```tsx
{feats.map((g, i) => {
  const fid = g.featId ?? g.feat_id ?? ''
  const fname = g.featName ?? g.feat_name ?? ''
  const isPending = !fid && !fname
  return (
    <div key={`feat-${i}`} style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '3px', paddingLeft: '4px' }}>
      <span style={{ fontSize: '0.7rem', color: '#a0c8ff', background: 'rgba(100,150,255,0.15)', border: '1px solid rgba(100,150,255,0.3)', borderRadius: '4px', padding: '0 4px' }}>feat</span>
      {isPending
        ? <span style={{ color: '#e0c060', fontSize: '0.85rem', fontStyle: 'italic' }}>Pending choice</span>
        : <span style={{ color: '#ddd', fontSize: '0.85rem' }}>{fname || fid}</span>
      }
    </div>
  )
})}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npm run build 2>&1 | tail -20
```

Expected: no errors.

- [ ] **Step 3: Run full Go test suite one last time**

```bash
cd /home/cjohannsen/src/mud && go test ./... -count=1 -timeout=120s 2>&1 | tail -20
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/webclient/ui/src/game/drawers/JobDrawer.tsx
git commit -m "fix(webclient): replace raw Choose-N label with Pending-choice badge in JobDrawer (REQ-FCM-10)"
```

---

## Self-Review

**Spec coverage:**
- REQ-FCM-1: Task 2 — `JobFeatGrant` for unresolved pool emits empty `feat_id`/`feat_name` ✅
- REQ-FCM-2: Task 2 — `PendingFeatChoices` populated with full option pool ✅
- REQ-FCM-3: Task 8 — `CharacterPanel` notification badge ✅
- REQ-FCM-4: Task 8 — clicking badge opens `FeatChoiceModal` ✅
- REQ-FCM-5: Task 7 — modal shows name, category badge, description ✅
- REQ-FCM-6: Task 7 — Confirm disabled until `selected.length === required` ✅
- REQ-FCM-7: Task 7 — on confirm, sends `ChooseFeatRequest` per selection ✅
- REQ-FCM-8: Task 3 — validation: pool membership, already-owned ✅
- REQ-FCM-9: Task 3 — on success: feat stored, level granted, push CharacterSheetView + JobGrantsResponse ✅
- REQ-FCM-10: Task 9 — JobDrawer shows "Pending choice" badge, not raw string ✅
- REQ-FCM-11: Task 7 — dark-theme monospace styling ✅

**Placeholder scan:** No TBDs or TODOs. All code blocks are complete.

**Type consistency:**
- `PendingFeatChoice.GrantLevel` (int32) used as `int32(grantLevel)` throughout ✅
- `ChooseFeatRequest.GrantLevel` / `.FeatId` field names match proto definition ✅
- `FeatOption.FeatId` / `.Name` / `.Description` / `.Category` consistent across proto, Go, TS ✅
- `handleChooseFeat(uid string, grantLevel int, featID string)` signature matches dispatch case ✅

**One note on Task 3 tests:** The `buildChooseFeatSvc` helper uses `newTestGameServiceServer` which has a large parameter list. The exact parameter order must match the function signature in `grpc_service.go`. The implementer should read the current `newTestGameServiceServer` definition and verify the parameter order before writing the helper — the plan shows the approximate order based on existing tests, but the implementer must confirm it matches exactly.
