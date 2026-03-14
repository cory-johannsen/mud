# Hero Point System Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a hero point resource to each player session that can be spent to reroll a recent check or stabilize from dying; awarded at level-up and via GM grant command.

**Architecture:** Four fields added to PlayerSession (HeroPoints + LastCheck* + Dead); new SaveHeroPoints persister; heropoint command wired end-to-end via CMD-1–7 pipeline; grant command extended; character sheet updated.

**Tech Stack:** Go, protobuf/gRPC, pgregory.net/rapid (property tests), PostgreSQL.

---

## Task 1: Data Model — New Fields on PlayerSession + Persistence

**Files touched:**
- `internal/game/session/manager.go`
- `internal/gameserver/grpc_service.go` (CharacterSaver interface + session load)
- `internal/storage/postgres/character.go`

### Step 1.1 — Failing tests first (TDD)

- [ ] Create `internal/storage/postgres/character_hero_points_test.go`:

```go
package postgres_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestSaveAndLoadHeroPoints verifies round-trip persistence of hero points.
//
// Precondition: a test character exists with ID from testCharID().
// Postcondition: SaveHeroPoints followed by LoadHeroPoints returns the same value.
func TestSaveAndLoadHeroPoints(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping postgres integration test")
    }
    ctx := context.Background()
    repo := newTestCharacterRepository(t)
    charID := testCharID(t, repo)

    err := repo.SaveHeroPoints(ctx, charID, 3)
    require.NoError(t, err)

    got, err := repo.LoadHeroPoints(ctx, charID)
    require.NoError(t, err)
    assert.Equal(t, 3, got, "loaded hero points must equal saved value")
}

// TestLoadHeroPoints_DefaultZero verifies that a character without a persisted
// hero point count returns 0.
//
// Precondition: test character has never had SaveHeroPoints called.
// Postcondition: LoadHeroPoints returns 0 with no error.
func TestLoadHeroPoints_DefaultZero(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping postgres integration test")
    }
    ctx := context.Background()
    repo := newTestCharacterRepository(t)
    charID := testCharID(t, repo)

    got, err := repo.LoadHeroPoints(ctx, charID)
    require.NoError(t, err)
    assert.Equal(t, 0, got)
}

// TestSaveHeroPoints_InvalidID verifies that id <= 0 returns an error.
//
// Precondition: id = 0.
// Postcondition: error is returned without touching the database.
func TestSaveHeroPoints_InvalidID(t *testing.T) {
    repo := newTestCharacterRepository(t)
    err := repo.SaveHeroPoints(context.Background(), 0, 1)
    require.Error(t, err)
}
```

- [ ] Run `go test ./internal/storage/postgres/... -run TestSaveAndLoadHeroPoints -count=1` — **must fail** (method does not exist yet).

### Step 1.2 — Add `hero_points` column to characters table

- [ ] Create migration `internal/storage/postgres/testdata/hero_points_migration.sql` (for test bootstrapping reference only — production migration managed separately):

```sql
ALTER TABLE characters ADD COLUMN IF NOT EXISTS hero_points INTEGER NOT NULL DEFAULT 0;
```

Note: the production database migration must be applied before deploying. Add the migration to the project's migration runner or apply it manually using the DB password from `.claude/rules/.env`.

### Step 1.3 — Implement SaveHeroPoints and LoadHeroPoints in CharacterRepository

- [ ] Add to `internal/storage/postgres/character.go` after `LoadCurrency`:

```go
// SaveHeroPoints persists the player's current hero point count to the characters table.
//
// Precondition: id > 0; heroPoints >= 0.
// Postcondition: characters.hero_points column is updated.
func (r *CharacterRepository) SaveHeroPoints(ctx context.Context, id int64, heroPoints int) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	_, err := r.db.Exec(ctx, `UPDATE characters SET hero_points = $2 WHERE id = $1`, id, heroPoints)
	return err
}

// LoadHeroPoints returns the stored hero point count for the given character.
// Returns 0 for characters without a persisted count.
//
// Precondition: id > 0.
// Postcondition: returns (heroPoints, nil) on success; heroPoints is 0 if never set.
func (r *CharacterRepository) LoadHeroPoints(ctx context.Context, id int64) (int, error) {
	if id <= 0 {
		return 0, fmt.Errorf("characterID must be > 0, got %d", id)
	}
	var heroPoints int
	err := r.db.QueryRow(ctx, `SELECT COALESCE(hero_points, 0) FROM characters WHERE id = $1`, id).Scan(&heroPoints)
	return heroPoints, err
}
```

### Step 1.4 — Add SaveHeroPoints + LoadHeroPoints to CharacterSaver interface

- [ ] In `internal/gameserver/grpc_service.go`, add to the `CharacterSaver` interface (after `SaveGender`):

```go
SaveHeroPoints(ctx context.Context, characterID int64, heroPoints int) error
LoadHeroPoints(ctx context.Context, characterID int64) (int, error)
```

### Step 1.5 — Add four fields to PlayerSession

- [ ] In `internal/game/session/manager.go`, add the following fields to `PlayerSession` after `Gender`:

```go
// HeroPoints is the number of hero points available to the player (persisted).
HeroPoints int
// LastCheckRoll is the dice result of the most recent ability check (session-only; 0 = none recorded).
LastCheckRoll int
// LastCheckDC is the DC of the most recent ability check (session-only).
LastCheckDC int
// LastCheckName is the display name of the most recent check type (session-only; e.g., "Grapple").
LastCheckName string
// Dead is true when the character is currently dying and eligible for stabilize (session-only).
Dead bool
```

Note: `Dead`, `LastCheckRoll`, `LastCheckDC`, and `LastCheckName` are NOT initialized in `AddPlayerOptions` and are NOT persisted. `HeroPoints` is persisted and loaded at session start.

### Step 1.6 — Load HeroPoints at session start

- [ ] In `internal/gameserver/grpc_service.go`, in the session setup block after the `LoadCurrency` call (around line 578), add:

```go
// Load persisted hero points.
if savedHP, hpErr := s.charSaver.LoadHeroPoints(stream.Context(), characterID); hpErr != nil {
    s.logger.Warn("failed to load hero points on login",
        zap.String("uid", uid),
        zap.Int64("character_id", characterID),
        zap.Error(hpErr),
    )
} else {
    sess.HeroPoints = savedHP
}
```

### Step 1.7 — Verify tests pass

- [ ] Run `go test ./internal/storage/postgres/... -run TestSaveAndLoadHeroPoints -count=1` — **must pass**.
- [ ] Run `go test ./... -count=1` — **must pass** (no regressions).

### Step 1.8 — Commit

```bash
cd /home/cjohannsen/src/mud && git add internal/game/session/manager.go internal/gameserver/grpc_service.go internal/storage/postgres/character.go internal/storage/postgres/character_hero_points_test.go && git commit -m "feat(session): add HeroPoints, Dead, LastCheck* fields; add SaveHeroPoints/LoadHeroPoints"
```

---

## Task 2: Award Hero Point on Level-Up

**Files touched:**
- `internal/gameserver/grpc_service.go` (handleGrant's xp case)

### Step 2.1 — Failing test first (TDD)

- [ ] Create `internal/gameserver/grpc_service_hero_point_levelup_test.go`:

```go
package gameserver

import (
    "context"
    "testing"

    "github.com/cory-johannsen/mud/internal/game/command"
    "github.com/cory-johannsen/mud/internal/game/session"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/zap/zaptest"
)

// mockCharSaverWithHeroPoints is a test double that records SaveHeroPoints calls.
type mockCharSaverWithHeroPoints struct {
    mockCharSaver // embed existing mock
    savedHeroPoints map[int64]int
    heroPointsSaved int
}

func (m *mockCharSaverWithHeroPoints) SaveHeroPoints(_ context.Context, characterID int64, hp int) error {
    if m.savedHeroPoints == nil {
        m.savedHeroPoints = make(map[int64]int)
    }
    m.savedHeroPoints[characterID] = hp
    m.heroPointsSaved++
    return nil
}

func (m *mockCharSaverWithHeroPoints) LoadHeroPoints(_ context.Context, _ int64) (int, error) {
    return 0, nil
}

// TestHandleGrant_XP_LevelUp_AwardsHeroPoint verifies that when granting XP causes
// a level-up, exactly 1 hero point is awarded to the target and SaveHeroPoints is called.
//
// Precondition: target is at level 1 with 0 XP; granting 400 XP causes level-up.
// Postcondition: target.HeroPoints == 1; SaveHeroPoints called once.
func TestHandleGrant_XP_LevelUp_AwardsHeroPoint(t *testing.T) {
    _, sessMgr := testWorldAndSession(t)
    logger := zaptest.NewLogger(t)

    grantorSess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "editor_uid", Username: "Editor", CharName: "Editor",
        RoomID: "room_a", Role: "editor",
    })
    require.NoError(t, err)
    _ = grantorSess

    targetSess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "target_uid", Username: "Player", CharName: "Target",
        RoomID: "room_a", Role: "player",
    })
    require.NoError(t, err)
    targetSess.CharacterID = 42
    targetSess.Level = 1
    targetSess.Experience = 0
    targetSess.HeroPoints = 0

    mockSaver := &mockCharSaverWithHeroPoints{}
    svc := NewGameServiceServer(
        nil, sessMgr,
        command.DefaultRegistry(),
        nil, nil,
        logger,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
        nil, nil, nil,
        nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        mockSaver, testXPSvc(t),
    )

    event, err := svc.handleGrant("editor_uid", &gamev1.GrantRequest{
        GrantType: "xp",
        CharName:  "Target",
        Amount:    400, // enough to reach level 2
    })
    require.NoError(t, err)
    require.NotNil(t, event)
    assert.Equal(t, 1, targetSess.HeroPoints, "level-up must award exactly 1 hero point")
    assert.Equal(t, 1, mockSaver.heroPointsSaved, "SaveHeroPoints must be called exactly once on level-up")
}
```

- [ ] Run `go test ./internal/gameserver/... -run TestHandleGrant_XP_LevelUp_AwardsHeroPoint -count=1` — **must fail**.

### Step 2.2 — Implement level-up hero point award in handleGrant

- [ ] In `internal/gameserver/grpc_service.go`, in `handleGrant`, within the `case "xp":` block, after the `if result.LeveledUp {` block (around line 5709), add hero point award logic:

```go
if result.LeveledUp {
    // ... existing levelMsgs code ...

    // Award 1 hero point on level-up (REQ-AWARD1).
    target.HeroPoints++
    if target.CharacterID > 0 && s.charSaver != nil {
        if hpErr := s.charSaver.SaveHeroPoints(ctx, target.CharacterID, target.HeroPoints); hpErr != nil {
            s.logger.Warn("handleGrant: SaveHeroPoints failed on level-up", zap.Error(hpErr))
        }
    }
    levelMsgs = append(levelMsgs, "You earned 1 hero point!")
}
```

The exact insertion point is after the existing `if result.NewSkillIncreases > 0` message line within the `if result.LeveledUp` block, before the block closes.

### Step 2.3 — Verify tests pass

- [ ] Run `go test ./internal/gameserver/... -run TestHandleGrant_XP_LevelUp_AwardsHeroPoint -count=1` — **must pass**.
- [ ] Run `go test ./... -count=1` — **must pass**.

### Step 2.4 — Commit

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_hero_point_levelup_test.go && git commit -m "feat(xp): award 1 hero point on level-up; persist via SaveHeroPoints"
```

---

## Task 3: `heropoint` Command — Full CMD-1 Through CMD-7 Pipeline

**Files touched:**
- `internal/game/command/commands.go` (CMD-1, CMD-2)
- `internal/game/command/heropoint.go` (CMD-3, new file)
- `internal/game/command/heropoint_test.go` (CMD-3 tests)
- `api/proto/game/v1/game.proto` (CMD-4)
- `internal/frontend/handlers/bridge_handlers.go` (CMD-5)
- `internal/gameserver/grpc_service.go` (CMD-6)
- `internal/gameserver/grpc_service_hero_point_test.go` (CMD-6 tests)

### Step 3.1 — CMD-1: Add HandlerHeroPoint constant

- [ ] In `internal/game/command/commands.go`, add to the handler constant block:

```go
HandlerHeroPoint = "heropoint"
```

Place it after `HandlerCalm`.

### Step 3.2 — CMD-2: Add Command entry to BuiltinCommands()

- [ ] In `internal/game/command/commands.go`, add to `BuiltinCommands()` (after the `calm` entry):

```go
{Name: "heropoint", Aliases: []string{"hp"}, Help: "Spend a hero point (heropoint reroll | heropoint stabilize)", Category: CategoryCharacter, Handler: HandlerHeroPoint},
```

### Step 3.3 — CMD-3: Failing unit tests for HandleHeroPoint

- [ ] Create `internal/game/command/heropoint_test.go`:

```go
package command_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/command"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// TestHandleHeroPoint_NoArgs verifies that invoking heropoint with no args
// returns a usage error.
//
// Precondition: args is empty.
// Postcondition: error containing "Usage:" is returned.
func TestHandleHeroPoint_NoArgs(t *testing.T) {
    result, err := command.HandleHeroPoint([]string{})
    require.NoError(t, err)
    assert.Contains(t, result.Error, "Usage:")
}

// TestHandleHeroPoint_InvalidSubcommand verifies that an unknown subcommand returns an error.
//
// Precondition: args[0] = "attack".
// Postcondition: error containing "unknown subcommand" is returned.
func TestHandleHeroPoint_InvalidSubcommand(t *testing.T) {
    result, err := command.HandleHeroPoint([]string{"attack"})
    require.NoError(t, err)
    assert.Contains(t, result.Error, "unknown subcommand")
}

// TestHandleHeroPoint_Reroll verifies that args[0]="reroll" produces subcommand="reroll".
//
// Precondition: args = ["reroll"].
// Postcondition: result.Subcommand == "reroll"; no error.
func TestHandleHeroPoint_Reroll(t *testing.T) {
    result, err := command.HandleHeroPoint([]string{"reroll"})
    require.NoError(t, err)
    assert.Empty(t, result.Error)
    assert.Equal(t, "reroll", result.Subcommand)
}

// TestHandleHeroPoint_Stabilize verifies that args[0]="stabilize" produces subcommand="stabilize".
//
// Precondition: args = ["stabilize"].
// Postcondition: result.Subcommand == "stabilize"; no error.
func TestHandleHeroPoint_Stabilize(t *testing.T) {
    result, err := command.HandleHeroPoint([]string{"stabilize"})
    require.NoError(t, err)
    assert.Empty(t, result.Error)
    assert.Equal(t, "stabilize", result.Subcommand)
}
```

- [ ] Run `go test ./internal/game/command/... -run TestHandleHeroPoint -count=1` — **must fail** (file does not exist).

### Step 3.4 — CMD-3: Implement HandleHeroPoint

- [ ] Create `internal/game/command/heropoint.go`:

```go
package command

// HeroPointResult is the parsed output of HandleHeroPoint.
//
// Exactly one of Subcommand or Error is non-empty.
type HeroPointResult struct {
    // Subcommand is "reroll" or "stabilize" when parsing succeeds.
    Subcommand string
    // Error is a non-empty user-facing error message when parsing fails.
    Error string
}

// HandleHeroPoint parses the heropoint command arguments.
//
// Precondition: args is the slice of words following "heropoint".
// Postcondition: returns a HeroPointResult; the returned error is always nil
// (parse errors are reported in HeroPointResult.Error).
func HandleHeroPoint(args []string) (HeroPointResult, error) {
    if len(args) == 0 {
        return HeroPointResult{Error: "Usage: heropoint <reroll|stabilize>"}, nil
    }
    switch args[0] {
    case "reroll":
        return HeroPointResult{Subcommand: "reroll"}, nil
    case "stabilize":
        return HeroPointResult{Subcommand: "stabilize"}, nil
    default:
        return HeroPointResult{Error: "unknown subcommand '" + args[0] + "': use 'reroll' or 'stabilize'"}, nil
    }
}
```

- [ ] Run `go test ./internal/game/command/... -run TestHandleHeroPoint -count=1` — **must pass**.

### Step 3.5 — CMD-4: Add HeroPointRequest proto message and wire into ClientMessage oneof

- [ ] In `api/proto/game/v1/game.proto`, add `HeroPointRequest` to the `ClientMessage` oneof after field 70 (calm):

```proto
HeroPointRequest hero_point = 71;
```

- [ ] Add the `HeroPointRequest` message definition (near the other request messages, after `CalmRequest`):

```proto
// HeroPointRequest asks the server to execute a hero point subcommand.
message HeroPointRequest {
  string subcommand = 1; // "reroll" or "stabilize"
}
```

- [ ] Run `make proto` from `/home/cjohannsen/src/mud` to regenerate Go bindings.
- [ ] Verify `internal/gameserver/gamev1/game.pb.go` was updated and the project compiles: `go build ./...`.

### Step 3.6 — CMD-5: Add bridgeHeroPoint to bridge_handlers.go

- [ ] In `internal/frontend/handlers/bridge_handlers.go`, add to `bridgeHandlerMap`:

```go
command.HandlerHeroPoint: bridgeHeroPoint,
```

- [ ] Add the implementation function (near other bridge functions, after `bridgeCalm`):

```go
// bridgeHeroPoint builds a HeroPointRequest from the parsed subcommand.
//
// Precondition: bctx must be non-nil with a valid parsed command.
// Postcondition: Returns a non-nil msg containing a HeroPointRequest, or writes an error and returns done=true.
func bridgeHeroPoint(bctx *bridgeContext) (bridgeResult, error) {
    result, err := command.HandleHeroPoint(bctx.parsed.Args)
    if err != nil {
        return bridgeResult{}, err
    }
    if result.Error != "" {
        return writeErrorPrompt(bctx, result.Error)
    }
    return bridgeResult{
        msg: &gamev1.ClientMessage{
            RequestId: bctx.reqID,
            Payload: &gamev1.ClientMessage_HeroPoint{
                HeroPoint: &gamev1.HeroPointRequest{
                    Subcommand: result.Subcommand,
                },
            },
        },
    }, nil
}
```

- [ ] Run `go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -count=1` — **must pass**.

### Step 3.7 — CMD-6: Implement handleHeroPoint in grpc_service.go

#### Step 3.7a — Failing tests first

- [ ] Create `internal/gameserver/grpc_service_hero_point_test.go`:

```go
package gameserver

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/command"
    "github.com/cory-johannsen/mud/internal/game/dice"
    "github.com/cory-johannsen/mud/internal/game/session"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/zap/zaptest"
    "pgregory.net/rapid"
)

func newHeroPointSvc(t *testing.T, roller *dice.Roller) (*GameServiceServer, *session.Manager) {
    t.Helper()
    worldMgr, sessMgr := testWorldAndSession(t)
    logger := zaptest.NewLogger(t)
    svc := NewGameServiceServer(
        worldMgr, sessMgr,
        command.DefaultRegistry(),
        NewWorldHandler(worldMgr, sessMgr, nil, nil, nil, nil),
        NewChatHandler(sessMgr),
        logger,
        nil, roller, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
        nil, nil, nil,
        nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        nil, nil,
    )
    return svc, sessMgr
}

// TestHandleHeroPoint_NoSession verifies that handleHeroPoint returns an error when
// the player session does not exist.
//
// Precondition: uid "unknown_hp_uid" has no session.
// Postcondition: error is returned; event is nil.
func TestHandleHeroPoint_NoSession(t *testing.T) {
    svc, _ := newHeroPointSvc(t, nil)
    event, err := svc.handleHeroPoint("unknown_hp_uid", &gamev1.HeroPointRequest{Subcommand: "reroll"})
    require.Error(t, err)
    assert.Nil(t, event)
}

// TestHandleHeroPointReroll_NoPoints verifies that reroll with 0 hero points
// returns an error event and does NOT decrement HeroPoints.
//
// Precondition: sess.HeroPoints == 0; sess.LastCheckRoll == 10.
// Postcondition: error event; HeroPoints remains 0.
func TestHandleHeroPointReroll_NoPoints(t *testing.T) {
    svc, sessMgr := newHeroPointSvc(t, nil)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "u_hp_np", Username: "P", CharName: "P",
        RoomID: "room_a", Role: "player",
    })
    require.NoError(t, err)
    sess.HeroPoints = 0
    sess.LastCheckRoll = 10

    event, err := svc.handleHeroPoint("u_hp_np", &gamev1.HeroPointRequest{Subcommand: "reroll"})
    require.NoError(t, err)
    require.NotNil(t, event)
    errEvt := event.GetError()
    require.NotNil(t, errEvt)
    assert.Contains(t, errEvt.Message, "hero point")
    assert.Equal(t, 0, sess.HeroPoints)
}

// TestHandleHeroPointReroll_NoRecentCheck verifies that reroll with LastCheckRoll == 0
// returns an error event and does NOT decrement HeroPoints.
//
// Precondition: sess.HeroPoints == 2; sess.LastCheckRoll == 0.
// Postcondition: error event; HeroPoints remains 2.
func TestHandleHeroPointReroll_NoRecentCheck(t *testing.T) {
    svc, sessMgr := newHeroPointSvc(t, nil)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "u_hp_nrc", Username: "P", CharName: "P",
        RoomID: "room_a", Role: "player",
    })
    require.NoError(t, err)
    sess.HeroPoints = 2
    sess.LastCheckRoll = 0

    event, err := svc.handleHeroPoint("u_hp_nrc", &gamev1.HeroPointRequest{Subcommand: "reroll"})
    require.NoError(t, err)
    require.NotNil(t, event)
    errEvt := event.GetError()
    require.NotNil(t, errEvt)
    assert.Contains(t, errEvt.Message, "no recent check")
    assert.Equal(t, 2, sess.HeroPoints)
}

// TestHandleHeroPointReroll_NewRollWins verifies that when the new roll is higher,
// LastCheckRoll is updated to the new roll and HeroPoints is decremented.
//
// Precondition: sess.HeroPoints == 1; sess.LastCheckRoll == 8; dice always returns 15.
// Postcondition: message contains "keeping 15"; LastCheckRoll == 15; HeroPoints == 0.
func TestHandleHeroPointReroll_NewRollWins(t *testing.T) {
    logger := zaptest.NewLogger(t)
    src := &fixedDiceSource{val: 14} // val+1 = 15
    roller := dice.NewLoggedRoller(src, logger)
    svc, sessMgr := newHeroPointSvc(t, roller)

    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "u_hp_nrw", Username: "P", CharName: "P",
        RoomID: "room_a", Role: "player",
    })
    require.NoError(t, err)
    sess.HeroPoints = 1
    sess.LastCheckRoll = 8

    event, err := svc.handleHeroPoint("u_hp_nrw", &gamev1.HeroPointRequest{Subcommand: "reroll"})
    require.NoError(t, err)
    require.NotNil(t, event)
    msgEvt := event.GetMessage()
    require.NotNil(t, msgEvt)
    assert.Contains(t, msgEvt.Content, "keeping 15")
    assert.Equal(t, 15, sess.LastCheckRoll)
    assert.Equal(t, 0, sess.HeroPoints)
}

// TestHandleHeroPointReroll_OldRollWins verifies that when the original roll is higher,
// LastCheckRoll stays at the original value and HeroPoints is still decremented.
//
// Precondition: sess.HeroPoints == 1; sess.LastCheckRoll == 15; dice always returns 8.
// Postcondition: message contains "keeping 15"; LastCheckRoll == 15; HeroPoints == 0.
func TestHandleHeroPointReroll_OldRollWins(t *testing.T) {
    logger := zaptest.NewLogger(t)
    src := &fixedDiceSource{val: 7} // val+1 = 8
    roller := dice.NewLoggedRoller(src, logger)
    svc, sessMgr := newHeroPointSvc(t, roller)

    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "u_hp_orw", Username: "P", CharName: "P",
        RoomID: "room_a", Role: "player",
    })
    require.NoError(t, err)
    sess.HeroPoints = 1
    sess.LastCheckRoll = 15

    event, err := svc.handleHeroPoint("u_hp_orw", &gamev1.HeroPointRequest{Subcommand: "reroll"})
    require.NoError(t, err)
    require.NotNil(t, event)
    msgEvt := event.GetMessage()
    require.NotNil(t, msgEvt)
    assert.Contains(t, msgEvt.Content, "keeping 15")
    assert.Equal(t, 15, sess.LastCheckRoll)
    assert.Equal(t, 0, sess.HeroPoints)
}

// TestHandleHeroPointStabilize_NotDying verifies that stabilize when Dead==false
// returns an error and does NOT decrement HeroPoints.
//
// Precondition: sess.Dead == false; sess.HeroPoints == 1.
// Postcondition: error event; HeroPoints remains 1.
func TestHandleHeroPointStabilize_NotDying(t *testing.T) {
    svc, sessMgr := newHeroPointSvc(t, nil)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "u_hp_nd", Username: "P", CharName: "P",
        RoomID: "room_a", Role: "player",
    })
    require.NoError(t, err)
    sess.HeroPoints = 1
    sess.Dead = false

    event, err := svc.handleHeroPoint("u_hp_nd", &gamev1.HeroPointRequest{Subcommand: "stabilize"})
    require.NoError(t, err)
    require.NotNil(t, event)
    errEvt := event.GetError()
    require.NotNil(t, errEvt)
    assert.Contains(t, errEvt.Message, "not dying")
    assert.Equal(t, 1, sess.HeroPoints)
}

// TestHandleHeroPointStabilize_Success verifies that stabilize when dying sets
// Dead=false, CurrentHP=0, and decrements HeroPoints.
//
// Precondition: sess.Dead == true; sess.HeroPoints == 1; sess.CurrentHP == -5.
// Postcondition: Dead==false; CurrentHP==0; HeroPoints==0; message matches spec.
func TestHandleHeroPointStabilize_Success(t *testing.T) {
    svc, sessMgr := newHeroPointSvc(t, nil)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "u_hp_stab", Username: "P", CharName: "P",
        RoomID: "room_a", Role: "player",
    })
    require.NoError(t, err)
    sess.HeroPoints = 1
    sess.Dead = true
    sess.CurrentHP = -5

    event, err := svc.handleHeroPoint("u_hp_stab", &gamev1.HeroPointRequest{Subcommand: "stabilize"})
    require.NoError(t, err)
    require.NotNil(t, event)
    msgEvt := event.GetMessage()
    require.NotNil(t, msgEvt)
    assert.Contains(t, msgEvt.Content, "stabilize at 0 HP")
    assert.False(t, sess.Dead)
    assert.Equal(t, 0, sess.CurrentHP)
    assert.Equal(t, 0, sess.HeroPoints)
}

// TestHandleHeroPointStabilize_NoPoints verifies that stabilize with 0 hero points
// returns an error and leaves state unchanged.
//
// Precondition: sess.Dead == true; sess.HeroPoints == 0.
// Postcondition: error event; Dead remains true; HeroPoints remains 0.
func TestHandleHeroPointStabilize_NoPoints(t *testing.T) {
    svc, sessMgr := newHeroPointSvc(t, nil)
    sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "u_hp_snp", Username: "P", CharName: "P",
        RoomID: "room_a", Role: "player",
    })
    require.NoError(t, err)
    sess.HeroPoints = 0
    sess.Dead = true

    event, err := svc.handleHeroPoint("u_hp_snp", &gamev1.HeroPointRequest{Subcommand: "stabilize"})
    require.NoError(t, err)
    require.NotNil(t, event)
    errEvt := event.GetError()
    require.NotNil(t, errEvt)
    assert.Equal(t, 0, sess.HeroPoints)
    assert.True(t, sess.Dead)
}

// TestProperty_HeroPointReroll_Invariants is a property-based test verifying the
// invariants for a successful reroll:
//   - HeroPoints decrements by exactly 1
//   - Winning roll is >= original LastCheckRoll
//   - No error is returned
//
// REQ-T10: for any HeroPoints in [0,10] and LastCheckRoll in [1,20].
func TestProperty_HeroPointReroll_Invariants(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        heroPoints := rapid.IntRange(1, 10).Draw(rt, "heroPoints") // must be >= 1 for valid precondition
        lastRoll := rapid.IntRange(1, 20).Draw(rt, "lastCheckRoll")
        newRollVal := rapid.IntRange(1, 20).Draw(rt, "newRoll")

        logger := zaptest.NewLogger(t)
        src := &fixedDiceSource{val: newRollVal - 1} // fixedDiceSource adds 1
        roller := dice.NewLoggedRoller(src, logger)
        svc, sessMgr := newHeroPointSvc(t, roller)

        uid := rapid.StringMatching(`[a-z]{4}_prop`).Draw(rt, "uid")
        sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
            UID: uid, Username: "P", CharName: "P",
            RoomID: "room_prop", Role: "player",
        })
        require.NoError(rt, err)
        sess.HeroPoints = heroPoints
        sess.LastCheckRoll = lastRoll

        event, err := svc.handleHeroPoint(uid, &gamev1.HeroPointRequest{Subcommand: "reroll"})
        require.NoError(rt, err)
        require.NotNil(rt, event)
        msgEvt := event.GetMessage()
        require.NotNil(rt, msgEvt, "successful reroll must return a message event")

        assert.Equal(rt, heroPoints-1, sess.HeroPoints, "HeroPoints must decrement by exactly 1")
        winner := lastRoll
        if newRollVal > lastRoll {
            winner = newRollVal
        }
        assert.Equal(rt, winner, sess.LastCheckRoll, "LastCheckRoll must be max(old, new)")
        assert.GreaterOrEqual(rt, sess.LastCheckRoll, lastRoll, "winning roll must be >= original")
    })
}
```

- [ ] Run `go test ./internal/gameserver/... -run TestHandleHeroPoint -count=1` — **must fail**.

#### Step 3.7b — Implement handleHeroPoint, handleHeroPointReroll, handleHeroPointStabilize

- [ ] In `internal/gameserver/grpc_service.go`, add the dispatch case to the type switch (after `case *gamev1.ClientMessage_Calm:`):

```go
case *gamev1.ClientMessage_HeroPoint:
    return s.handleHeroPoint(uid, p.HeroPoint)
```

- [ ] Add the implementation functions at the end of `grpc_service.go` (before or after `handleCalm`):

```go
// handleHeroPoint dispatches heropoint subcommands: reroll and stabilize.
//
// Precondition: uid identifies an active session; req is non-nil with a valid subcommand.
// Postcondition: returns a MessageEvent on success or an ErrorEvent on failure; never returns a non-nil error.
func (s *GameServiceServer) handleHeroPoint(uid string, req *gamev1.HeroPointRequest) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }
    switch req.Subcommand {
    case "reroll":
        return s.handleHeroPointReroll(sess)
    case "stabilize":
        return s.handleHeroPointStabilize(sess)
    default:
        return errorEvent("unknown heropoint subcommand: use 'reroll' or 'stabilize'"), nil
    }
}

// handleHeroPointReroll spends 1 hero point to reroll the most recent ability check,
// keeping the higher result.
//
// Precondition: sess.HeroPoints >= 1; sess.LastCheckRoll != 0.
// Postcondition: on success, HeroPoints decremented, LastCheckRoll updated to winner,
// SaveHeroPoints persisted; returns MessageEvent with outcome.
func (s *GameServiceServer) handleHeroPointReroll(sess *session.PlayerSession) (*gamev1.ServerEvent, error) {
    if sess.HeroPoints < 1 {
        return errorEvent("You have no hero points to spend."), nil
    }
    if sess.LastCheckRoll == 0 {
        return errorEvent("You have no recent check to reroll."), nil
    }

    oldRoll := sess.LastCheckRoll
    var newRoll int
    if s.roller != nil {
        newRoll = s.roller.Roll(20)
    } else {
        newRoll = rand.Intn(20) + 1
    }
    winner := oldRoll
    if newRoll > oldRoll {
        winner = newRoll
    }

    sess.LastCheckRoll = winner
    sess.HeroPoints--

    ctx := context.Background()
    if sess.CharacterID > 0 && s.charSaver != nil {
        if err := s.charSaver.SaveHeroPoints(ctx, sess.CharacterID, sess.HeroPoints); err != nil {
            s.logger.Warn("handleHeroPointReroll: SaveHeroPoints failed", zap.Error(err))
        }
    }

    msg := fmt.Sprintf("You spend a hero point. Original roll: %d, New roll: %d — keeping %d.", oldRoll, newRoll, winner)
    return messageEvent(msg), nil
}

// handleHeroPointStabilize spends 1 hero point to pull the character back from dying.
//
// Precondition: sess.HeroPoints >= 1; sess.Dead == true.
// Postcondition: on success, HeroPoints decremented, Dead set to false, CurrentHP set to 0,
// SaveHeroPoints persisted; returns MessageEvent.
func (s *GameServiceServer) handleHeroPointStabilize(sess *session.PlayerSession) (*gamev1.ServerEvent, error) {
    if sess.HeroPoints < 1 {
        return errorEvent("You have no hero points to spend."), nil
    }
    if !sess.Dead {
        return errorEvent("You are not dying."), nil
    }

    sess.Dead = false
    sess.CurrentHP = 0
    sess.HeroPoints--

    ctx := context.Background()
    if sess.CharacterID > 0 && s.charSaver != nil {
        if err := s.charSaver.SaveHeroPoints(ctx, sess.CharacterID, sess.HeroPoints); err != nil {
            s.logger.Warn("handleHeroPointStabilize: SaveHeroPoints failed", zap.Error(err))
        }
    }

    return messageEvent("You spend a hero point, pulling back from the brink. You stabilize at 0 HP."), nil
}
```

**Important note on `s.roller`:** The field name used by `GameServiceServer` for the dice roller must be verified. Search for `roller` field references in `grpc_service.go` to confirm the field name (e.g., `s.roller` or `s.dice`). If the field uses a different accessor pattern (e.g., `s.combatHandler.roller`), adjust accordingly. Use `s.roller.Roll(20)` with fallback to `rand.Intn(20) + 1` when nil.

### Step 3.8 — Record LastCheckRoll in skill check handlers

The `heropoint reroll` precondition requires `sess.LastCheckRoll != 0`. This means skill check handlers that roll dice must record their roll into `sess.LastCheckRoll`, `sess.LastCheckDC`, and `sess.LastCheckName`.

**Handlers to update** (search for `Roll(20)` or `skillRoll` in grpc_service.go to find all check sites):
- `handleGrapple` — after computing `roll := s.roller.Roll(20) + bonus`, add:
  ```go
  sess.LastCheckRoll = roll
  sess.LastCheckDC   = dc
  sess.LastCheckName = "Grapple"
  ```
- `handleTrip` — same pattern with `LastCheckName = "Trip"`
- `handleFeint` — `LastCheckName = "Feint"`
- `handleDemoralize` — `LastCheckName = "Demoralize"`
- `handleDisarm` — `LastCheckName = "Disarm"`
- `handleHide` — `LastCheckName = "Hide"`
- `handleSneak` — `LastCheckName = "Sneak"`
- `handleFirstAid` — `LastCheckName = "First Aid"`
- `handleEscape` — `LastCheckName = "Escape"`
- `handleMotive` — `LastCheckName = "Motive"`
- `handleSeek` — `LastCheckName = "Seek"`
- `handleClimb` — `LastCheckName = "Climb"`
- `handleSwim` — `LastCheckName = "Swim"`
- `handleCalm` — `LastCheckName = "Calm"`

For each handler: locate the line where the skill roll total is computed (the `roll` variable that is compared against `dc`), then immediately after that line, before the pass/fail branch, set the three `sess.LastCheck*` fields.

The `sess` pointer must be retrieved from `s.sessions.GetPlayer(uid)` at the top of each handler (most already do this). Example pattern from grapple:

```go
// After computing: roll := baseRoll + bonus
sess.LastCheckRoll = roll   // raw total including bonus
sess.LastCheckDC   = dc
sess.LastCheckName = "Grapple"
```

### Step 3.9 — Verify all tests pass

- [ ] Run `go test ./internal/gameserver/... -run TestHandleHeroPoint -count=1` — **must pass**.
- [ ] Run `go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -count=1` — **must pass**.
- [ ] Run `go test ./... -count=1` — **must pass**.

### Step 3.10 — Commit

```bash
cd /home/cjohannsen/src/mud && git add \
  internal/game/command/commands.go \
  internal/game/command/heropoint.go \
  internal/game/command/heropoint_test.go \
  api/proto/game/v1/game.proto \
  internal/gameserver/gamev1/game.pb.go \
  internal/frontend/handlers/bridge_handlers.go \
  internal/gameserver/grpc_service.go \
  internal/gameserver/grpc_service_hero_point_test.go \
  && git commit -m "feat(heropoint): implement heropoint command end-to-end (CMD-1 through CMD-7); record LastCheck* in skill handlers"
```

---

## Task 4: GM `grant heropoint` Subcommand

**Files touched:**
- `api/proto/game/v1/game.proto` (extend GrantRequest with heropoint type)
- `internal/frontend/handlers/bridge_handlers.go` (extend bridgeGrant)
- `internal/gameserver/grpc_service.go` (extend handleGrant)

### Step 4.1 — Failing test first

- [ ] Create `internal/gameserver/grpc_service_grant_heropoint_test.go`:

```go
package gameserver

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/session"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.uber.org/zap/zaptest"
)

// TestHandleGrant_HeroPoint_Awards verifies that 'grant heropoint <player>' awards
// exactly 1 hero point to the target, persists it, notifies the target, and confirms to the GM.
//
// Precondition: editor grants heropoint to online player "Target".
// Postcondition: target.HeroPoints == 1; SaveHeroPoints called once; GM gets confirmation message.
func TestHandleGrant_HeroPoint_Awards(t *testing.T) {
    _, sessMgr := testWorldAndSession(t)
    logger := zaptest.NewLogger(t)

    _, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "editor_ghp", Username: "Ed", CharName: "Editor",
        RoomID: "room_a", Role: "editor",
    })
    require.NoError(t, err)

    targetSess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "target_ghp", Username: "T", CharName: "Target",
        RoomID: "room_a", Role: "player",
    })
    require.NoError(t, err)
    targetSess.CharacterID = 99
    targetSess.HeroPoints = 0

    mockSaver := &mockCharSaverWithHeroPoints{}
    svc := NewGameServiceServer(
        nil, sessMgr,
        nil, nil, nil,
        logger,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
        nil, nil, nil,
        nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        mockSaver, nil,
    )

    event, err := svc.handleGrant("editor_ghp", &gamev1.GrantRequest{
        GrantType: "heropoint",
        CharName:  "Target",
        Amount:    1,
    })
    require.NoError(t, err)
    require.NotNil(t, event)
    msgEvt := event.GetMessage()
    require.NotNil(t, msgEvt)
    assert.Contains(t, msgEvt.Content, "hero point")
    assert.Equal(t, 1, targetSess.HeroPoints)
    assert.Equal(t, 1, mockSaver.heroPointsSaved)
}

// TestHandleGrant_HeroPoint_PermissionDenied verifies that a player (not editor) cannot grant hero points.
//
// Precondition: caller has role "player".
// Postcondition: error event containing "permission denied".
func TestHandleGrant_HeroPoint_PermissionDenied(t *testing.T) {
    _, sessMgr := testWorldAndSession(t)
    logger := zaptest.NewLogger(t)

    _, err := sessMgr.AddPlayer(session.AddPlayerOptions{
        UID: "player_ghp_pd", Username: "P", CharName: "Player",
        RoomID: "room_a", Role: "player",
    })
    require.NoError(t, err)

    svc := NewGameServiceServer(
        nil, sessMgr,
        nil, nil, nil,
        logger,
        nil, nil, nil, nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
        nil, nil, nil,
        nil, nil, nil,
        nil, nil, nil, nil, nil, nil, nil,
        nil, nil,
    )

    event, err := svc.handleGrant("player_ghp_pd", &gamev1.GrantRequest{
        GrantType: "heropoint",
        CharName:  "SomePlayer",
        Amount:    1,
    })
    require.NoError(t, err)
    errEvt := event.GetError()
    require.NotNil(t, errEvt)
    assert.Contains(t, errEvt.Message, "permission denied")
}
```

- [ ] Run `go test ./internal/gameserver/... -run TestHandleGrant_HeroPoint -count=1` — **must fail**.

### Step 4.2 — Extend GrantRequest in proto (optional — alternative: use existing GrantType string "heropoint")

The existing `GrantRequest` already has `grant_type string`. The implementation uses `req.GrantType == "heropoint"` in the switch statement. **No proto change is required** — the existing `GrantRequest` message is sufficient. The `amount` field will be ignored for heropoint grants (always exactly 1), but the existing `amount > 0` validation must be bypassed for heropoint type.

### Step 4.3 — Extend handleGrant with "heropoint" case

- [ ] In `internal/gameserver/grpc_service.go`, in `handleGrant`, modify the `amount > 0` guard to allow heropoint grants with amount 0 or 1:

Change:
```go
if req.Amount <= 0 {
    return errorEvent("amount must be greater than zero"), nil
}
```

To:
```go
if req.GrantType != "heropoint" && req.Amount <= 0 {
    return errorEvent("amount must be greater than zero"), nil
}
```

- [ ] Add `case "heropoint":` to the switch in `handleGrant` (after `case "money":`, before `default:`):

```go
case "heropoint":
    target.HeroPoints++
    if target.CharacterID > 0 && s.charSaver != nil {
        if hpErr := s.charSaver.SaveHeroPoints(ctx, target.CharacterID, target.HeroPoints); hpErr != nil {
            s.logger.Warn("handleGrant: SaveHeroPoints failed", zap.Error(hpErr))
        }
    }
    // Notify target.
    notif := messageEvent(fmt.Sprintf("You have been granted a hero point by %s. You now have %d.", sess.CharName, target.HeroPoints))
    if data, mErr := proto.Marshal(notif); mErr == nil {
        _ = target.Entity.Push(data)
    }
    return messageEvent(fmt.Sprintf("Granted 1 hero point to %s. They now have %d.", target.CharName, target.HeroPoints)), nil
```

### Step 4.4 — Extend bridgeGrant to accept "heropoint" type

- [ ] In `internal/frontend/handlers/bridge_handlers.go`, in `bridgeGrant`, update the grant type validation to include "heropoint":

Change:
```go
if grantType != "xp" && grantType != "money" {
```

To:
```go
if grantType != "xp" && grantType != "money" && grantType != "heropoint" {
```

And update both the prompt text and the error message:

Change:
```go
_ = bctx.conn.WritePrompt(telnet.Colorize(telnet.White, "Grant type (xp/money): "))
```

To:
```go
_ = bctx.conn.WritePrompt(telnet.Colorize(telnet.White, "Grant type (xp/money/heropoint): "))
```

Change the error:
```go
return writeErrorPrompt(bctx, "Grant type must be 'xp' or 'money'.")
```

To:
```go
return writeErrorPrompt(bctx, "Grant type must be 'xp', 'money', or 'heropoint'.")
```

For the heropoint grant type, `amount` is not needed. Update bridgeGrant to skip the amount prompt when grantType is "heropoint":

In the amount resolution section, wrap the amount prompt logic:
```go
var amount int32 = 1
if grantType != "heropoint" {
    // ... existing amount resolution logic ...
    amount = int32(n)
}
```

Then pass `Amount: amount` to GrantRequest (which will be 1 for heropoint, or the resolved value for xp/money).

### Step 4.5 — Verify tests pass

- [ ] Run `go test ./internal/gameserver/... -run TestHandleGrant_HeroPoint -count=1` — **must pass**.
- [ ] Run `go test ./... -count=1` — **must pass**.

### Step 4.6 — Commit

```bash
cd /home/cjohannsen/src/mud && git add \
  internal/gameserver/grpc_service.go \
  internal/frontend/handlers/bridge_handlers.go \
  internal/gameserver/grpc_service_grant_heropoint_test.go \
  && git commit -m "feat(grant): add 'grant heropoint <player>' subcommand; notify target and persist"
```

---

## Task 5: Character Sheet Display + FEATURES.md

**Files touched:**
- `api/proto/game/v1/game.proto` (add hero_points field to CharacterSheetView)
- `internal/gameserver/grpc_service.go` (populate hero_points in handleChar)
- `internal/frontend/handlers/text_renderer.go` (render Hero Points line)
- `internal/frontend/handlers/text_renderer_test.go` or new test file
- `docs/FEATURES.md`

### Step 5.1 — Failing test first for character sheet render

- [ ] Create or append to `internal/frontend/handlers/text_renderer_hero_points_test.go`:

```go
package handlers_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/frontend/handlers"
    gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
    "github.com/stretchr/testify/assert"
)

// TestRenderCharacterSheet_ShowsHeroPoints verifies that RenderCharacterSheet includes
// a "Hero Points: N" line when HeroPoints is set.
//
// Precondition: csv.HeroPoints == 3.
// Postcondition: rendered string contains "Hero Points: 3".
func TestRenderCharacterSheet_ShowsHeroPoints(t *testing.T) {
    csv := &gamev1.CharacterSheetView{
        Name:       "TestChar",
        Job:        "Ranger",
        Archetype:  "Hunter",
        Level:      5,
        CurrentHp:  30,
        MaxHp:      40,
        HeroPoints: 3,
    }
    rendered := handlers.RenderCharacterSheet(csv, 80)
    assert.Contains(t, rendered, "Hero Points: 3")
}

// TestRenderCharacterSheet_ZeroHeroPoints verifies that "Hero Points: 0" is shown
// when the character has no hero points.
//
// Precondition: csv.HeroPoints == 0.
// Postcondition: rendered string contains "Hero Points: 0".
func TestRenderCharacterSheet_ZeroHeroPoints(t *testing.T) {
    csv := &gamev1.CharacterSheetView{
        Name:       "TestChar",
        HeroPoints: 0,
    }
    rendered := handlers.RenderCharacterSheet(csv, 80)
    assert.Contains(t, rendered, "Hero Points: 0")
}
```

- [ ] Run `go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_ShowsHeroPoints -count=1` — **must fail** (field does not exist in proto yet).

### Step 5.2 — Add hero_points to CharacterSheetView proto

- [ ] In `api/proto/game/v1/game.proto`, add to `CharacterSheetView` (use next available field number, 42):

```proto
int32 hero_points = 42; // current hero point count
```

- [ ] Run `make proto` to regenerate Go bindings.
- [ ] Verify `go build ./...` succeeds.

### Step 5.3 — Populate hero_points in handleChar

- [ ] In `internal/gameserver/grpc_service.go`, find the `handleChar` function (or wherever `CharacterSheetView` is constructed). Locate the struct literal that builds `&gamev1.CharacterSheetView{...}` and add:

```go
HeroPoints: int32(sess.HeroPoints),
```

### Step 5.4 — Render Hero Points in RenderCharacterSheet

- [ ] In `internal/frontend/handlers/text_renderer.go`, in `RenderCharacterSheet`, add after the `HP:` line (after `left = append(left, slPlain(fmt.Sprintf("HP: %d / %d", ...)))`):

```go
left = append(left, slPlain(fmt.Sprintf("Hero Points: %d", csv.GetHeroPoints())))
```

### Step 5.5 — Update FEATURES.md

- [ ] Locate `docs/FEATURES.md` and add the following:
  - Mark hero point items as in-scope/complete (using `[x]` where implemented).
  - Under the Bosses section, add: `[ ] Award 1 hero point on boss kill`

Example additions to make in FEATURES.md:

```markdown
## Hero Points
- [x] HeroPoints field on PlayerSession (persisted)
- [x] Award 1 hero point on level-up
- [x] `heropoint reroll` — spend 1 HP to reroll most recent check (keep higher)
- [x] `heropoint stabilize` — spend 1 HP to recover from dying (0 HP, Dead=false)
- [x] `grant heropoint <player>` — GM awards 1 hero point
- [x] Hero Points displayed on character sheet
```

Under the Bosses section:
```markdown
- [ ] Award 1 hero point on boss kill
```

### Step 5.6 — Verify tests pass

- [ ] Run `go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_ShowsHeroPoints -count=1` — **must pass**.
- [ ] Run `go test ./... -count=1` — **must pass**.

### Step 5.7 — Commit

```bash
cd /home/cjohannsen/src/mud && git add \
  api/proto/game/v1/game.proto \
  internal/gameserver/gamev1/game.pb.go \
  internal/gameserver/grpc_service.go \
  internal/frontend/handlers/text_renderer.go \
  internal/frontend/handlers/text_renderer_hero_points_test.go \
  docs/FEATURES.md \
  && git commit -m "feat(display): add Hero Points to CharacterSheetView proto and character sheet render; update FEATURES.md"
```

---

## Final Verification

After all tasks are complete:

- [ ] Run the full test suite: `go test ./... -count=1`
- [ ] Verify `TestAllCommandHandlersAreWired` passes.
- [ ] Verify all REQ-T1 through REQ-T10 tests are present and green.
- [ ] Confirm no TODOs, placeholders, or stub bodies exist in new code.
- [ ] Deploy: `make k8s-redeploy` (DB migration for `hero_points` column must be applied first).

---

## Appendix: Key File Locations

| Concern | File |
|---|---|
| PlayerSession struct | `internal/game/session/manager.go` |
| CharacterSaver interface | `internal/gameserver/grpc_service.go` |
| Postgres persistence | `internal/storage/postgres/character.go` |
| Session load (login) | `internal/gameserver/grpc_service.go` ~line 570 |
| Handler constants | `internal/game/command/commands.go` |
| BuiltinCommands() | `internal/game/command/commands.go` |
| HandleHeroPoint (pure parse) | `internal/game/command/heropoint.go` (new) |
| Proto definitions | `api/proto/game/v1/game.proto` |
| Bridge map | `internal/frontend/handlers/bridge_handlers.go` |
| gRPC dispatch switch | `internal/gameserver/grpc_service.go` ~line 1180 |
| handleGrant | `internal/gameserver/grpc_service.go` ~line 5660 |
| handleLevelUp | `internal/gameserver/grpc_service.go` ~line 3585 |
| Character sheet renderer | `internal/frontend/handlers/text_renderer.go` |
| Skill check handlers | `internal/gameserver/grpc_service.go` (handleGrapple, handleTrip, etc.) |
| Dice roller field | `s.roller` in GameServiceServer (verify field name) |
| xp.Award result | `internal/game/xp/xp.go` — `AwardResult.LeveledUp bool` |

## Appendix: Critical Implementation Notes

**Dead field vs. Status field:** The spec's `sess.Dead == true` maps to a new `Dead bool` field on `PlayerSession` (Task 1). This is distinct from `sess.Status` (which tracks combat states: Idle/InCombat/Resting/Unconscious). The `Dead` field specifically tracks dying-and-requiring-stabilization state. When the combat engine kills a player (HP drops to 0), it must set `sess.Dead = true`. Search for where player HP goes to 0 in combat resolution and add `sess.Dead = true` there.

**Dice roller field name:** Before writing `s.roller.Roll(20)`, confirm the field name with `grep -n "roller\|dice\|Roller" internal/gameserver/grpc_service.go | head -30`. The `CombatHandler` constructor receives `roller *dice.Roller`; the `GameServiceServer` likely stores it as `s.roller`.

**Proto field numbers:** The next available field number in `ClientMessage.payload` oneof is 71 (after `calm = 70`). The next available field in `CharacterSheetView` is 42 (after `awareness = 41`). Verify by checking the proto file before adding.

**mockCharSaver:** The test files reference `mockCharSaver` (embedded). Verify its definition in existing test helpers (likely `internal/gameserver/grpc_service_test.go` or a `test_helpers_test.go`). If `mockCharSaver` does not implement `SaveHeroPoints`/`LoadHeroPoints`, the embedding approach in `mockCharSaverWithHeroPoints` will fail to compile until both methods are added to the embedded type or the `CharacterSaver` interface. Add stub implementations to the base `mockCharSaver` that return nil, then override in `mockCharSaverWithHeroPoints`.

**testXPSvc:** The level-up test references `testXPSvc(t)`. Check if this helper exists; if not, construct a minimal `*xp.Service` using `xp.NewService(xpConfig)` with a default config sufficient to trigger level-up at 400 XP from level 1.
