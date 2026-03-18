# Prepared Tech Rearrangement at Rest Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `rest` command that allows a player to re-select which technologies fill their non-fixed prepared slots by aggregating all applicable grants and re-prompting from the pool.

**Architecture:** `RearrangePreparedTechs` in `technology_assignment.go` aggregates creation + level-up prepared grants, clears existing slots, and re-fills via `fillFromPreparedPool`. The `rest` command follows CMD-1–7 and wires into a pre-dispatch handler (like `handleStatus`) because it needs direct stream access to prompt the player interactively.

**Tech Stack:** Go modules (`github.com/cory-johannsen/mud`), `pgregory.net/rapid v1.2.0`, `github.com/stretchr/testify`, protobuf/gRPC

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/gameserver/technology_assignment.go` | Modify | Add `RearrangePreparedTechs` function |
| `internal/gameserver/technology_assignment_test.go` | Modify | REQ-RAR1–RAR5 unit tests |
| `internal/game/command/commands.go` | Modify | CMD-1/CMD-2: `HandlerRest` constant + `Command` entry |
| `internal/game/command/rest.go` | Create | CMD-3: `HandleRest` function |
| `api/proto/game/v1/game.proto` | Modify | CMD-4: `RestRequest` message + `ClientMessage` oneof field |
| `internal/frontend/handlers/bridge_handlers.go` | Modify | CMD-5: `bridgeRest` + `bridgeHandlerMap` registration |
| `internal/gameserver/grpc_service.go` | Modify | CMD-6: pre-dispatch check + `handleRest` function |
| `internal/gameserver/grpc_service_rest_test.go` | Create | REQ-RAR6–RAR7 integration tests (`package gameserver`) |

---

## Chunk 1: `RearrangePreparedTechs` and unit tests

### Task 1: `RearrangePreparedTechs` function and unit tests (REQ-RAR1–RAR5)

**Files:**
- Modify: `internal/gameserver/technology_assignment.go`
- Modify: `internal/gameserver/technology_assignment_test.go`

**Context:** The fakes `fakeHardwiredRepo`, `fakePreparedRepo`, `fakeSpontaneousRepo`, `fakeInnateRepo`, and `noPrompt` are already defined in `technology_assignment_test.go`. Do NOT redefine them. The `fillFromPreparedPool` function (existing) takes `(ctx, lvl, slots, startIdx int, grants *PreparedGrants, techReg, promptFn, characterID, repo)`.

- [ ] **Step 1: Write failing tests (REQ-RAR1–RAR5)**

Add these five tests to `internal/gameserver/technology_assignment_test.go`, after `TestLoadTechnologies`:

```go
// REQ-RAR1 (property): All chosen techs after RearrangePreparedTechs come from
// the aggregated pool or fixed entries for their level.
func TestPropertyRearrangePreparedTechs_ChosenFromPool(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numFixed := rapid.IntRange(0, 2).Draw(rt, "numFixed")
		numPool := rapid.IntRange(1, 3).Draw(rt, "numPool")
		numSlots := numFixed + numPool

		fixed := make([]ruleset.PreparedEntry, numFixed)
		for i := 0; i < numFixed; i++ {
			fixed[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("fixed_%d", i), Level: 1}
		}
		pool := make([]ruleset.PreparedEntry, numPool)
		for i := 0; i < numPool; i++ {
			pool[i] = ruleset.PreparedEntry{ID: fmt.Sprintf("pool_%d", i), Level: 1}
		}

		existingSlots := make([]*session.PreparedSlot, numSlots)
		for i := range existingSlots {
			existingSlots[i] = &session.PreparedSlot{TechID: "old"}
		}
		prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{1: existingSlots}}
		sess := &session.PlayerSession{
			PreparedTechs: map[int][]*session.PreparedSlot{1: existingSlots},
		}
		job := &ruleset.Job{
			TechnologyGrants: &ruleset.TechnologyGrants{
				Prepared: &ruleset.PreparedGrants{
					SlotsByLevel: map[int]int{1: numSlots},
					Fixed:        fixed,
					Pool:         pool,
				},
			},
		}

		err := gameserver.RearrangePreparedTechs(context.Background(), sess, 1, job, nil, noPrompt, prep)
		if err != nil {
			rt.Fatalf("RearrangePreparedTechs: %v", err)
		}

		validIDs := make(map[string]bool)
		for _, e := range fixed {
			validIDs[e.ID] = true
		}
		for _, e := range pool {
			validIDs[e.ID] = true
		}
		for _, slot := range sess.PreparedTechs[1] {
			if !validIDs[slot.TechID] {
				rt.Fatalf("tech %q not in valid set", slot.TechID)
			}
		}
	})
}

// REQ-RAR2: Fixed entries occupy indices 0..n-1; pool selections follow at n..m-1.
func TestRearrangePreparedTechs_FixedFirst(t *testing.T) {
	ctx := context.Background()
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "old1"}, {TechID: "old2"}},
	}}
	sess := &session.PlayerSession{
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {{TechID: "old1"}, {TechID: "old2"}},
		},
	}
	job := &ruleset.Job{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 2},
				Fixed:        []ruleset.PreparedEntry{{ID: "fixed_tech", Level: 1}},
				Pool:         []ruleset.PreparedEntry{{ID: "pool_tech", Level: 1}},
			},
		},
	}

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, noPrompt, prep)
	require.NoError(t, err)
	require.Len(t, sess.PreparedTechs[1], 2)
	assert.Equal(t, "fixed_tech", sess.PreparedTechs[1][0].TechID, "fixed at index 0")
	assert.Equal(t, "pool_tech", sess.PreparedTechs[1][1].TechID, "pool at index 1")
}

// REQ-RAR3: LevelUpGrants entries above sess.Level are excluded from the pool.
// With level 3 excluded, the pool has exactly 1 entry for 1 slot → auto-assign fires.
// If level 3 were included, pool would have 2 entries for 1 slot → prompt would fire.
func TestRearrangePreparedTechs_LevelUpGrantsFiltered(t *testing.T) {
	ctx := context.Background()
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "old"}},
	}}
	sess := &session.PlayerSession{
		Level: 2,
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {{TechID: "old"}},
		},
	}
	job := &ruleset.Job{
		LevelUpGrants: map[int]*ruleset.TechnologyGrants{
			2: {Prepared: &ruleset.PreparedGrants{
				Pool: []ruleset.PreparedEntry{{ID: "level2_pool", Level: 1}},
			}},
			3: {Prepared: &ruleset.PreparedGrants{
				Pool: []ruleset.PreparedEntry{{ID: "level3_pool", Level: 1}},
			}},
		},
	}

	promptCalled := false
	promptFn := func(options []string) (string, error) {
		promptCalled = true
		return options[0], nil
	}

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, promptFn, prep)
	require.NoError(t, err)

	// Level3 excluded → pool has 1 entry for 1 slot → auto-assign, no prompt
	assert.False(t, promptCalled, "auto-assign fires when level3 excluded (pool==open)")
	require.Len(t, sess.PreparedTechs[1], 1)
	assert.Equal(t, "level2_pool", sess.PreparedTechs[1][0].TechID)
}

// REQ-RAR4: Empty PreparedTechs is a no-op; DeleteAll is never called.
func TestRearrangePreparedTechs_EmptySession_NoOp(t *testing.T) {
	ctx := context.Background()
	// Populate repo to detect if DeleteAll is called (DeleteAll sets slots=nil).
	existingRepo := map[int][]*session.PreparedSlot{1: {{TechID: "db_slot"}}}
	prep := &fakePreparedRepo{slots: existingRepo}
	sess := &session.PlayerSession{} // no PreparedTechs

	job := &ruleset.Job{} // no grants

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, noPrompt, prep)
	require.NoError(t, err)
	// Repo unchanged: DeleteAll was not called
	assert.Equal(t, existingRepo, prep.slots, "repo must be unchanged on no-op")
}

// REQ-RAR5: Auto-assign fires when len(pool at level) == open slots; no prompt invoked.
func TestRearrangePreparedTechs_AutoAssignNoPrompt(t *testing.T) {
	ctx := context.Background()
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "old"}},
	}}
	sess := &session.PlayerSession{
		PreparedTechs: map[int][]*session.PreparedSlot{
			1: {{TechID: "old"}},
		},
	}
	job := &ruleset.Job{
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "only_option", Level: 1}},
			},
		},
	}

	promptCalled := false
	promptFn := func(options []string) (string, error) {
		promptCalled = true
		return options[0], nil
	}

	err := gameserver.RearrangePreparedTechs(ctx, sess, 1, job, nil, promptFn, prep)
	require.NoError(t, err)
	assert.False(t, promptCalled, "prompt must not be called when pool == open slots")
	require.Len(t, sess.PreparedTechs[1], 1)
	assert.Equal(t, "only_option", sess.PreparedTechs[1][0].TechID)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestPropertyRearrangePreparedTechs|TestRearrangePreparedTechs" -v 2>&1 | tail -10
```

Expected: compile error — `RearrangePreparedTechs` undefined.

- [ ] **Step 3: Implement `RearrangePreparedTechs`**

Add this function to `internal/gameserver/technology_assignment.go`, after `LevelUpTechnologies`:

```go
// RearrangePreparedTechs deletes all existing prepared slots and re-fills them
// by aggregating grants from job.TechnologyGrants and all job.LevelUpGrants
// entries for levels 1..sess.Level.
//
// Precondition: sess, job, prepRepo are non-nil. promptFn must be non-nil.
// Postcondition: sess.PreparedTechs and prepRepo reflect the re-selected slots.
// If sess.PreparedTechs is empty or all level slot counts are zero, returns nil (no-op).
func RearrangePreparedTechs(
	ctx context.Context,
	sess *session.PlayerSession,
	characterID int64,
	job *ruleset.Job,
	techReg *technology.Registry,
	promptFn TechPromptFn,
	prepRepo PreparedTechRepo,
) error {
	// Build SlotsByLevel from session (source of truth for slot counts).
	slotsByLevel := make(map[int]int)
	for lvl, slots := range sess.PreparedTechs {
		if len(slots) > 0 {
			slotsByLevel[lvl] = len(slots)
		}
	}
	// No-op guard must run before any mutation.
	if len(slotsByLevel) == 0 {
		return nil
	}

	// Aggregate Fixed and Pool from all applicable grants.
	var allFixed []ruleset.PreparedEntry
	var allPool []ruleset.PreparedEntry
	if job.TechnologyGrants != nil && job.TechnologyGrants.Prepared != nil {
		allFixed = append(allFixed, job.TechnologyGrants.Prepared.Fixed...)
		allPool = append(allPool, job.TechnologyGrants.Prepared.Pool...)
	}
	for lvl, grants := range job.LevelUpGrants {
		if lvl > sess.Level {
			continue
		}
		if grants != nil && grants.Prepared != nil {
			allFixed = append(allFixed, grants.Prepared.Fixed...)
			allPool = append(allPool, grants.Prepared.Pool...)
		}
	}

	merged := &ruleset.PreparedGrants{
		SlotsByLevel: slotsByLevel,
		Fixed:        allFixed,
		Pool:         allPool,
	}

	// Clear existing slots before re-filling.
	if err := prepRepo.DeleteAll(ctx, characterID); err != nil {
		return fmt.Errorf("RearrangePreparedTechs DeleteAll: %w", err)
	}
	sess.PreparedTechs = make(map[int][]*session.PreparedSlot)

	// Re-fill each level.
	for lvl, slots := range slotsByLevel {
		chosen, err := fillFromPreparedPool(ctx, lvl, slots, 0, merged, techReg, promptFn, characterID, prepRepo)
		if err != nil {
			return fmt.Errorf("RearrangePreparedTechs level %d: %w", lvl, err)
		}
		sess.PreparedTechs[lvl] = chosen
	}
	return nil
}
```

- [ ] **Step 4: Run the new tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestPropertyRearrangePreparedTechs|TestRearrangePreparedTechs" -v 2>&1 | tail -15
```

Expected: all 5 PASS.

- [ ] **Step 5: Run full gameserver test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -count=1 2>&1 | tail -5
```

Expected: PASS, 0 failures.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/technology_assignment.go internal/gameserver/technology_assignment_test.go
git commit -m "feat(gameserver): RearrangePreparedTechs — aggregate grants, delete+refill prepared slots (REQ-RAR1–RAR5)"
```

---

## Chunk 2: `rest` command wiring and integration tests

### Task 2: `rest` command (CMD-1–7) and integration tests (REQ-RAR6–RAR7)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/rest.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_rest_test.go`

**Context — dispatch architecture:**
Two patterns exist in `grpc_service.go`:
- **Simple handlers** (like `handleHide`): `func (s *GameServiceServer) handleHide(uid string) (*gamev1.ServerEvent, error)` — returned event goes through `dispatch()` → session loop.
- **Stream handlers** (like `handleStatus`): `func (s *GameServiceServer) handleStatus(uid, requestID string, stream gamev1.GameService_SessionServer) error` — called BEFORE `dispatch()` in a pre-dispatch check block (around line 1131); sends events directly via `stream.Send()`; session loop calls `continue` after.

`handleRest` must be a **stream handler** because it calls `promptFeatureChoice(stream, ...)` to interactively prompt the player.

**Context — `testServiceForGrant`:**
Defined in `grpc_service_grant_test.go` (`package gameserver`). Use the same helper for rest integration tests. The `SetPreparedTechRepo`, `SetJobRegistry` setters (added in the LevelUpTechnologies sprint) are available on `*GameServiceServer`.

**Context — proto field numbers:**
Current highest field in `ClientMessage` oneof is `KickRequest kick = 80`. Next available is **81**. Always verify in `game.proto` before writing and use the actual next number.

- [ ] **Step 1: Write failing integration tests (REQ-RAR6–RAR7)**

Create `internal/gameserver/grpc_service_rest_test.go` as `package gameserver`:

```go
package gameserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// fakeSessionStream is a minimal gamev1.GameService_SessionServer test double
// that records sent events and returns a canned recv message.
type fakeSessionStream struct {
	sent    []*gamev1.ServerEvent
	recvMsg *gamev1.ClientMessage
	ctx     context.Context
}

func newFakeSessionStream(ctx context.Context) *fakeSessionStream {
	return &fakeSessionStream{ctx: ctx}
}

func (f *fakeSessionStream) Send(evt *gamev1.ServerEvent) error {
	f.sent = append(f.sent, evt)
	return nil
}
func (f *fakeSessionStream) Recv() (*gamev1.ClientMessage, error)  { return f.recvMsg, nil }
func (f *fakeSessionStream) Context() context.Context              { return f.ctx }
func (f *fakeSessionStream) SendMsg(m interface{}) error           { return nil }
func (f *fakeSessionStream) RecvMsg(m interface{}) error           { return nil }
func (f *fakeSessionStream) SetHeader(_ interface{}) error         { return nil }
func (f *fakeSessionStream) SendHeader(_ interface{}) error        { return nil }
func (f *fakeSessionStream) SetTrailer(_ interface{})              {}

// lastMessage returns the last MessageEvent text sent on the stream, or "".
func lastMessage(f *fakeSessionStream) string {
	for i := len(f.sent) - 1; i >= 0; i-- {
		if m, ok := f.sent[i].Payload.(*gamev1.ServerEvent_Message); ok {
			return m.Message.Message
		}
	}
	return ""
}

// REQ-RAR6: Player in combat receives "can't rest" message; RearrangePreparedTechs not called.
func TestHandleRest_InCombat_Rejected(t *testing.T) {
	ctx := context.Background()
	svc := testServiceForGrant(t, grantTestOptions{})
	uid := addTargetForGrant(t, svc)
	svc.sessions.GetPlayer(uid) // warm up

	// Set player status to in-combat.
	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT)

	// Populate prepared tech repo so we can detect if DeleteAll is called.
	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "existing"}},
	}}
	svc.SetPreparedTechRepo(prep)

	stream := newFakeSessionStream(ctx)
	err := svc.handleRest(uid, "req1", stream)
	require.NoError(t, err)

	assert.Contains(t, lastMessage(stream), "can't rest")
	// Repo unchanged: RearrangePreparedTechs was not called
	require.NotNil(t, prep.slots)
	assert.Equal(t, "existing", prep.slots[1][0].TechID)
}

// REQ-RAR7: Player not in combat receives confirmation; PreparedTechs updated.
func TestHandleRest_NotInCombat_Rearranges(t *testing.T) {
	ctx := context.Background()
	svc := testServiceForGrant(t, grantTestOptions{})
	uid := addTargetForGrant(t, svc)

	sess, ok := svc.sessions.GetPlayer(uid)
	require.True(t, ok)
	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
	sess.Class = "test_job"
	sess.CharacterID = 10

	// Pre-populate one prepared slot.
	sess.PreparedTechs = map[int][]*session.PreparedSlot{
		1: {{TechID: "old_tech"}},
	}

	prep := &fakePreparedRepo{slots: map[int][]*session.PreparedSlot{
		1: {{TechID: "old_tech"}},
	}}
	svc.SetPreparedTechRepo(prep)

	job := &ruleset.Job{
		ID:   "test_job",
		Name: "Test Job",
		TechnologyGrants: &ruleset.TechnologyGrants{
			Prepared: &ruleset.PreparedGrants{
				SlotsByLevel: map[int]int{1: 1},
				Pool:         []ruleset.PreparedEntry{{ID: "new_tech", Level: 1}},
			},
		},
	}
	jobReg := ruleset.NewJobRegistry()
	jobReg.Register(job)
	svc.SetJobRegistry(jobReg)

	stream := newFakeSessionStream(ctx)
	err := svc.handleRest(uid, "req1", stream)
	require.NoError(t, err)

	assert.Contains(t, lastMessage(stream), "prepared")
	require.Len(t, sess.PreparedTechs[1], 1)
	assert.Equal(t, "new_tech", sess.PreparedTechs[1][0].TechID)
}
```

**Note:** `fakeSessionStream` must implement the full `gamev1.GameService_SessionServer` interface. Before writing, check what methods that interface requires:

```bash
grep -n "GameService_SessionServer\|SessionServer interface" /home/cjohannsen/src/mud/internal/gameserver/gamev1/game_grpc.pb.go | head -10
```

Add any missing methods as no-ops.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleRest" -v 2>&1 | tail -10
```

Expected: compile error — `handleRest` undefined or `fakeSessionStream` missing methods.

- [ ] **Step 3: CMD-1/CMD-2 — Add `HandlerRest` to `commands.go`**

In `internal/game/command/commands.go`, add after the last `Handler` constant (currently `HandlerKick`):

```go
HandlerRest = "rest"
```

Add a `Command` entry in `BuiltinCommands()` after the kick entry:

```go
{Name: "rest", Help: "Rest and rearrange your prepared technologies.", Category: CategoryGeneral, Handler: HandlerRest},
```

(Use `CategoryGeneral` or whichever category is appropriate — check existing non-combat commands for the right constant.)

- [ ] **Step 4: CMD-3 — Create `rest.go`**

Create `internal/game/command/rest.go`:

```go
package command

import "github.com/cory-johannsen/mud/internal/game/session"

// HandleRest handles the rest command. No arguments required.
//
// Precondition: none.
// Postcondition: returns a CommandResult directing the gameserver to process a rest.
func HandleRest(cmd Command, args []string, sess *session.PlayerSession) *CommandResult {
	return &CommandResult{Handler: HandlerRest}
}
```

- [ ] **Step 5: CMD-4 — Update proto**

In `api/proto/game/v1/game.proto`, add the message definition (place with the other request messages):

```protobuf
message RestRequest {}
```

Add to the `ClientMessage` oneof — verify the current highest field number first, then use the next one:

```bash
grep -n "= [0-9]*;" api/proto/game/v1/game.proto | tail -5
```

The current last entry should be `KickRequest kick = 80`. Add:

```protobuf
RestRequest rest = 81;
```

Run codegen:

```bash
cd /home/cjohannsen/src/mud && make proto 2>&1 | tail -5
```

Expected: no errors, `game.pb.go` and `game_grpc.pb.go` regenerated.

- [ ] **Step 6: CMD-5 — Add `bridgeRest` to `bridge_handlers.go`**

In `internal/frontend/handlers/bridge_handlers.go`, add the bridge function (follow `bridgeHide` as the pattern):

```go
func bridgeRest(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Rest{Rest: &gamev1.RestRequest{}},
	}}, nil
}
```

Add to `bridgeHandlerMap` (inside the map literal, after `HandlerKick`):

```go
command.HandlerRest: bridgeRest,
```

- [ ] **Step 7: Build to verify CMD-5 wiring**

```bash
cd /home/cjohannsen/src/mud && go build ./internal/frontend/... 2>&1
```

Expected: no errors. `TestAllCommandHandlersAreWired` will pass once the gameserver is also wired.

- [ ] **Step 8: CMD-6 — Implement `handleRest` in `grpc_service.go`**

Add the `handleRest` function to `internal/gameserver/grpc_service.go`:

```go
// handleRest processes the rest command for a player.
// Called pre-dispatch (before s.dispatch) because it requires direct stream access
// to prompt the player interactively via promptFeatureChoice.
//
// Precondition: uid identifies a valid player session.
// Postcondition: If player is not in combat, prepared tech slots are re-selected;
// a confirmation or error message is sent to the player's stream.
func (s *GameServiceServer) handleRest(uid string, requestID string, stream gamev1.GameService_SessionServer) error {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return fmt.Errorf("handleRest: player %q not found", uid)
	}

	sendMsg := func(text string) error {
		return stream.Send(&gamev1.ServerEvent{
			RequestId: requestID,
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{Message: text},
			},
		})
	}

	// Combat guard.
	if sess.Status == int32(gamev1.CombatStatus_COMBAT_STATUS_IN_COMBAT) {
		return sendMsg("You can't rest while in combat.")
	}

	// Job lookup.
	if s.jobRegistry == nil {
		return sendMsg("You rest briefly but have no technologies to rearrange.")
	}
	job, ok := s.jobRegistry.Job(sess.Class)
	if !ok {
		return sendMsg("You rest briefly but have no technologies to rearrange.")
	}

	// Build promptFn from the player's own stream.
	promptFn := func(options []string) (string, error) {
		choices := &ruleset.FeatureChoices{
			Prompt:  "Choose a technology to prepare:",
			Options: options,
			Key:     "tech_choice",
		}
		return s.promptFeatureChoice(stream, "tech_choice", choices)
	}

	if err := RearrangePreparedTechs(context.Background(), sess, sess.CharacterID,
		job, s.techRegistry, promptFn, s.preparedTechRepo,
	); err != nil {
		s.logger.Warn("handleRest: RearrangePreparedTechs failed",
			zap.String("uid", uid),
			zap.Error(err))
		return sendMsg("Something went wrong preparing your technologies.")
	}

	return sendMsg("You finish your rest and your technologies are prepared.")
}
```

Add the pre-dispatch check in the `Session` function. Find the block that checks for `handleStatus` (around line 1131):

```go
if _, ok := msg.Payload.(*gamev1.ClientMessage_Status); ok {
    if err := s.handleStatus(uid, msg.RequestId, stream); err != nil {
```

Add a parallel check immediately before or after it for `RestRequest`:

```go
if _, ok := msg.Payload.(*gamev1.ClientMessage_Rest); ok {
    if err := s.handleRest(uid, msg.RequestId, stream); err != nil {
        errEvt := &gamev1.ServerEvent{
            RequestId: msg.RequestId,
            Payload: &gamev1.ServerEvent_Error{
                Error: &gamev1.ErrorEvent{Message: err.Error()},
            },
        }
        if sendErr := stream.Send(errEvt); sendErr != nil {
            return fmt.Errorf("sending error: %w", sendErr)
        }
    }
    continue
}
```

- [ ] **Step 9: Build**

```bash
cd /home/cjohannsen/src/mud && go build ./internal/gameserver/... 2>&1
```

Expected: no errors.

- [ ] **Step 10: Run the integration tests**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleRest" -v 2>&1 | tail -15
```

Expected: both PASS.

- [ ] **Step 11: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | tail -20
```

Expected: all packages PASS including `TestAllCommandHandlersAreWired`.

- [ ] **Step 12: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, find:

```
        - [ ] Can be rearranged when resting
```

Mark complete:

```
        - [x] Can be rearranged when resting — `rest` command; `RearrangePreparedTechs` aggregates creation + level-up grants, clears and re-fills slots interactively
```

- [ ] **Step 13: Commit**

```bash
git add \
  internal/game/command/commands.go \
  internal/game/command/rest.go \
  api/proto/game/v1/game.proto \
  internal/gameserver/gamev1/game.pb.go \
  internal/gameserver/gamev1/game_grpc.pb.go \
  internal/frontend/handlers/bridge_handlers.go \
  internal/gameserver/grpc_service.go \
  internal/gameserver/grpc_service_rest_test.go \
  docs/requirements/FEATURES.md
git commit -m "feat(gameserver): rest command — rearrange prepared tech slots; combat guard; pre-dispatch stream handler (REQ-RAR6–RAR7, CMD-1–7)"
```

---

## Requirements checklist

| REQ | Task | Description |
|-----|------|-------------|
| REQ-RAR1 | Task 1 | Property: chosen techs come from aggregated pool/fixed |
| REQ-RAR2 | Task 1 | Fixed entries at indices 0..n-1 |
| REQ-RAR3 | Task 1 | LevelUpGrants above sess.Level excluded |
| REQ-RAR4 | Task 1 | Empty PreparedTechs → no-op, no DeleteAll |
| REQ-RAR5 | Task 1 | Auto-assign when pool == open slots |
| REQ-RAR6 | Task 2 | Combat guard: "can't rest" message |
| REQ-RAR7 | Task 2 | Not in combat: PreparedTechs updated, confirmation sent |
| CMD-1–7 | Task 2 | `rest` command fully wired end-to-end |
