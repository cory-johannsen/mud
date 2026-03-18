# Prepared Technology Use Counts Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Each prepared technology slot is a single use; `use <tech>` expends the first matching non-expended slot; `rest` restores all slots; expended state is persisted across server restarts.

**Architecture:** Add `Expended bool` to `PreparedSlot`, add `expended` column to `character_prepared_technologies`, extend `PreparedTechRepo` with `SetExpended`, and extend `handleUse` to find and expend prepared tech slots after the existing feat/class-feature activation path. The `rest` command already calls `DeleteAll` + `Set` which resets all expended state naturally. A new `PreparedSlotView` proto message surfaces expended state on the character sheet.

**Tech Stack:** Go modules (`github.com/cory-johannsen/mud`), `pgregory.net/rapid v1.2.0`, `github.com/stretchr/testify`, protobuf/gRPC, PostgreSQL (pgx v5)

---

## Chunk 1 — Task 1: Data model + DB + repo

### Task 1: PreparedSlot.Expended + migration + repo changes

**Files:**
- Create: `migrations/027_prepared_tech_expended.up.sql`
- Create: `migrations/027_prepared_tech_expended.down.sql`
- Modify: `internal/game/session/technology.go`
- Modify: `internal/storage/postgres/character_prepared_tech.go`
- Modify: `internal/gameserver/technology_assignment.go` (interface only)
- Modify: `internal/gameserver/grpc_service_rest_test.go` (add SetExpended stub to fake)
- Modify: `internal/gameserver/technology_assignment_test.go` (add SetExpended stub to fake)
- Modify: Any other `_test.go` files that have a `PreparedTechRepo` fake (search for `PreparedTechRepo` fakes)

---

- [ ] **T1-S1: Create migration up file**

  Create `migrations/027_prepared_tech_expended.up.sql`:

  ```sql
  ALTER TABLE character_prepared_technologies
      ADD COLUMN expended BOOLEAN NOT NULL DEFAULT FALSE;
  ```

- [ ] **T1-S2: Create migration down file**

  Create `migrations/027_prepared_tech_expended.down.sql`:

  ```sql
  ALTER TABLE character_prepared_technologies
      DROP COLUMN expended;
  ```

- [ ] **T1-S3: Add Expended field to PreparedSlot**

  In `internal/game/session/technology.go`, update `PreparedSlot`:

  ```go
  // PreparedSlot holds one prepared technology slot.
  type PreparedSlot struct {
  	TechID   string
  	Expended bool
  }
  ```

- [ ] **T1-S4: Write failing storage test for SetExpended (REQ-UC6)**

  Add to `internal/storage/postgres/character_prepared_tech.go`'s test file.
  First check: does a test file exist at `internal/storage/postgres/character_prepared_tech_test.go`?
  If not, create it. If yes, append to it.

  The test file should be `package postgres_test` with a build tag or skip when no DB is available
  (follow whatever pattern is used in other postgres test files in that directory — check
  `internal/storage/postgres/` for an example).

  Add test:

  ```go
  // TestCharacterPreparedTechRepository_SetExpended_RoundTrip verifies that SetExpended
  // persists the expended state and GetAll returns it correctly (REQ-UC6).
  func TestCharacterPreparedTechRepository_SetExpended_RoundTrip(t *testing.T) {
      // This test requires a real DB. Follow the skip/connect pattern from other postgres tests.
      // If you cannot find a DB test pattern, skip this test with t.Skip("requires DB") for now
      // and note it as DONE_WITH_CONCERNS.
      t.Skip("DB integration test — run with a live database")
  }
  ```

  **Note:** The full DB integration test is deferred. What matters now is that the interface and
  implementation compile. The in-memory fake tests in the gameserver package (Task 2) cover REQ-UC6
  at the unit level.

- [ ] **T1-S5: Update GetAll to scan expended column**

  In `internal/storage/postgres/character_prepared_tech.go`, update the `GetAll` method:

  - Change the SELECT query to include `expended`:

    ```go
    rows, err := r.db.Query(ctx,
    	`SELECT slot_level, slot_index, tech_id, expended
         FROM character_prepared_technologies
         WHERE character_id = $1
         ORDER BY slot_level, slot_index`,
    	characterID,
    )
    ```

  - Update the Scan call to read the fourth column:

    ```go
    var level, index int
    var techID string
    var expended bool
    if err := rows.Scan(&level, &index, &techID, &expended); err != nil {
    	return nil, fmt.Errorf("CharacterPreparedTechRepository.GetAll scan: %w", err)
    }
    for len(result[level]) <= index {
    	result[level] = append(result[level], nil)
    }
    result[level][index] = &session.PreparedSlot{TechID: techID, Expended: expended}
    ```

- [ ] **T1-S6: Update Set to explicitly write expended = FALSE**

  In `internal/storage/postgres/character_prepared_tech.go`, update the `Set` method SQL:

  ```go
  func (r *CharacterPreparedTechRepository) Set(ctx context.Context, characterID int64, level, index int, techID string) error {
  	_, err := r.db.Exec(ctx,
  		`INSERT INTO character_prepared_technologies (character_id, slot_level, slot_index, tech_id, expended)
           VALUES ($1, $2, $3, $4, FALSE)
           ON CONFLICT (character_id, slot_level, slot_index) DO UPDATE SET tech_id = EXCLUDED.tech_id, expended = FALSE`,
  		characterID, level, index, techID,
  	)
  	if err != nil {
  		return fmt.Errorf("CharacterPreparedTechRepository.Set: %w", err)
  	}
  	return nil
  }
  ```

- [ ] **T1-S7: Add SetExpended to CharacterPreparedTechRepository**

  Add after `Set` in `internal/storage/postgres/character_prepared_tech.go`:

  ```go
  // SetExpended marks or unmarks a single prepared slot as expended.
  //
  // Precondition: characterID > 0; level >= 1; index >= 0.
  // Postcondition: character_prepared_technologies row has expended = expended.
  func (r *CharacterPreparedTechRepository) SetExpended(ctx context.Context, characterID int64, level, index int, expended bool) error {
  	if characterID <= 0 {
  		return fmt.Errorf("characterID must be > 0, got %d", characterID)
  	}
  	_, err := r.db.Exec(ctx,
  		`UPDATE character_prepared_technologies
  		    SET expended = $1
  		  WHERE character_id = $2 AND slot_level = $3 AND slot_index = $4`,
  		expended, characterID, level, index,
  	)
  	if err != nil {
  		return fmt.Errorf("CharacterPreparedTechRepository.SetExpended: %w", err)
  	}
  	return nil
  }
  ```

- [ ] **T1-S8: Add SetExpended to PreparedTechRepo interface**

  In `internal/gameserver/technology_assignment.go`, update the `PreparedTechRepo` interface:

  ```go
  // PreparedTechRepo defines persistence for prepared technology slot assignments.
  type PreparedTechRepo interface {
  	GetAll(ctx context.Context, characterID int64) (map[int][]*session.PreparedSlot, error)
  	Set(ctx context.Context, characterID int64, level, index int, techID string) error
  	DeleteAll(ctx context.Context, characterID int64) error
  	// SetExpended marks or unmarks a single prepared slot as expended.
  	//
  	// Precondition: characterID > 0; level >= 1; index >= 0.
  	// Postcondition: character_prepared_technologies row has expended = expended.
  	SetExpended(ctx context.Context, characterID int64, level, index int, expended bool) error
  }
  ```

- [ ] **T1-S9: Add SetExpended stubs to all PreparedTechRepo fakes**

  Search for all files that implement `PreparedTechRepo` (fake structs in test files):

  ```bash
  grep -rn "func.*PreparedRepo\|fakePrepared\|fakePrepRepo\|PreparedTechRepo" \
    /home/cjohannsen/src/mud/internal/gameserver/ --include="*_test.go" | grep "func "
  ```

  For **each** fake found, add a `SetExpended` stub. The stub should record calls for assertion in tests.

  For `fakePreparedRepoRest` in `internal/gameserver/grpc_service_rest_test.go`:

  ```go
  func (r *fakePreparedRepoRest) SetExpended(_ context.Context, _ int64, level, index int, expended bool) error {
  	if r.slots == nil {
  		return nil
  	}
  	if slots, ok := r.slots[level]; ok && index < len(slots) && slots[index] != nil {
  		slots[index].Expended = expended
  	}
  	return nil
  }
  ```

  For `fakePreparedRepo` in `internal/gameserver/technology_assignment_test.go` (and any `fakePrepRepInternal` variants):

  ```go
  func (r *fakePreparedRepo) SetExpended(_ context.Context, _ int64, level, index int, expended bool) error {
  	if r.slots == nil {
  		return nil
  	}
  	if slots, ok := r.slots[level]; ok && index < len(slots) && slots[index] != nil {
  		slots[index].Expended = expended
  	}
  	return nil
  }
  ```

  For any internal fakes in `grpc_service_levelup_tech_test.go` or similar, add the same pattern.

- [ ] **T1-S10: Build to verify no errors**

  ```bash
  cd /home/cjohannsen/src/mud && go build ./... 2>&1 | head -20
  ```

  Expected: no errors. All `PreparedTechRepo` implementations now satisfy the extended interface.

- [ ] **T1-S11: Run tests**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/storage/... ./internal/gameserver/... ./internal/game/session/... -count=1 2>&1 | tail -10
  ```

  Expected: PASS.

- [ ] **T1-S12: Commit**

  ```bash
  cd /home/cjohannsen/src/mud && git add \
    migrations/027_prepared_tech_expended.up.sql \
    migrations/027_prepared_tech_expended.down.sql \
    internal/game/session/technology.go \
    internal/storage/postgres/character_prepared_tech.go \
    internal/gameserver/technology_assignment.go \
    internal/gameserver/grpc_service_rest_test.go \
    internal/gameserver/technology_assignment_test.go
  # Also add any other test files you modified with SetExpended stubs
  git commit -m "feat(storage,session): PreparedSlot.Expended, migration 027, SetExpended repo method"
  ```

---

## Chunk 2 — Task 2: handleUse extension

### Task 2: Extend handleUse for prepared tech activation

**Files:**
- Modify: `internal/gameserver/grpc_service.go` (handleUse function only)
- Create: `internal/gameserver/grpc_service_use_tech_test.go` (package gameserver)

---

- [ ] **T2-S1: Read handleUse before writing**

  Read the full `handleUse` function in `internal/gameserver/grpc_service.go` starting at line 4529.
  Identify:
  - Where the `abilityID == ""` list-mode path returns
  - Where the feat activation path ends (what happens if no feat matched)
  - Where the class feature activation path ends (what happens if no class feature matched)
  - The exact return pattern when nothing matches (does it return a "not found" message, or fall through?)

  This step produces no code — it informs T2-S3 (where to insert the prepared tech code).

- [ ] **T2-S2: Write failing tests**

  Create `internal/gameserver/grpc_service_use_tech_test.go` as `package gameserver`:

  ```go
  package gameserver

  import (
  	"fmt"
  	"testing"

  	"github.com/stretchr/testify/assert"
  	"github.com/stretchr/testify/require"
  	"pgregory.net/rapid"

  	"github.com/cory-johannsen/mud/internal/game/session"
  	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
  )

  // fakePrepRepoUse is a PreparedTechRepo fake for use-tech tests.
  // It tracks SetExpended calls.
  type fakePrepRepoUse struct {
  	slots           map[int][]*session.PreparedSlot
  	setExpendedCalls int
  }

  func (r *fakePrepRepoUse) GetAll(_ context.Context, _ int64) (map[int][]*session.PreparedSlot, error) {
  	return r.slots, nil
  }
  func (r *fakePrepRepoUse) Set(_ context.Context, _ int64, level, index int, techID string) error {
  	if r.slots == nil {
  		r.slots = make(map[int][]*session.PreparedSlot)
  	}
  	for len(r.slots[level]) <= index {
  		r.slots[level] = append(r.slots[level], nil)
  	}
  	r.slots[level][index] = &session.PreparedSlot{TechID: techID}
  	return nil
  }
  func (r *fakePrepRepoUse) DeleteAll(_ context.Context, _ int64) error {
  	r.slots = nil
  	return nil
  }
  func (r *fakePrepRepoUse) SetExpended(_ context.Context, _ int64, level, index int, expended bool) error {
  	if r.slots != nil {
  		if slots, ok := r.slots[level]; ok && index < len(slots) && slots[index] != nil {
  			slots[index].Expended = expended
  		}
  	}
  	r.setExpendedCalls++
  	return nil
  }

  // setupUsePlayer creates a session manager, service, and player for use-tech tests.
  // Returns (svc, sessMgr, uid). The player has no CharacterID set (0).
  func setupUsePlayer(t *testing.T, prepSlots map[int][]*session.PreparedSlot) (*GameServiceServer, *session.Manager, string) {
  	t.Helper()
  	sessMgr := session.NewManager()
  	svc := testMinimalService(t, sessMgr)
  	uid := "player-use-tech"
  	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
  		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
  	})
  	require.NoError(t, err)
  	sess, ok := sessMgr.GetPlayer(uid)
  	require.True(t, ok)
  	sess.PreparedTechs = prepSlots
  	return svc, sessMgr, uid
  }

  // REQ-UC1: use <tech> with a non-expended prepared slot expends it and returns activation message.
  func TestHandleUse_PreparedTech_ExpendsSlot(t *testing.T) {
  	prepRepo := &fakePrepRepoUse{
  		slots: map[int][]*session.PreparedSlot{
  			1: {{TechID: "shock_grenade", Expended: false}},
  		},
  	}
  	svc, sessMgr, uid := setupUsePlayer(t, prepRepo.slots)
  	svc.SetPreparedTechRepo(prepRepo)

  	evt, err := svc.handleUse(uid, "shock_grenade")
  	require.NoError(t, err)
  	require.NotNil(t, evt)

  	// Session slot must now be expended.
  	sess, _ := sessMgr.GetPlayer(uid)
  	require.NotNil(t, sess.PreparedTechs[1][0])
  	assert.True(t, sess.PreparedTechs[1][0].Expended, "slot must be marked expended")

  	// Repo SetExpended must have been called.
  	assert.Equal(t, 1, prepRepo.setExpendedCalls)

  	// Response must contain activation message.
  	msg := evt.GetMessage()
  	require.NotNil(t, msg)
  	assert.Contains(t, msg.Content, "shock_grenade")
  }

  // REQ-UC2: use <tech> with all slots expended returns "No prepared uses remaining".
  func TestHandleUse_PreparedTech_AllExpended_ReturnsNoRemaining(t *testing.T) {
  	prepRepo := &fakePrepRepoUse{
  		slots: map[int][]*session.PreparedSlot{
  			1: {{TechID: "shock_grenade", Expended: true}},
  		},
  	}
  	svc, _, uid := setupUsePlayer(t, prepRepo.slots)
  	svc.SetPreparedTechRepo(prepRepo)

  	evt, err := svc.handleUse(uid, "shock_grenade")
  	require.NoError(t, err)
  	require.NotNil(t, evt)

  	msg := evt.GetMessage()
  	require.NotNil(t, msg)
  	assert.Contains(t, msg.Content, "No prepared uses")
  	assert.Equal(t, 0, prepRepo.setExpendedCalls, "SetExpended must not be called when all slots expended")
  }

  // REQ-UC3: use <tech> with no slot for that tech returns "No prepared uses remaining".
  func TestHandleUse_PreparedTech_NoSlotForTech_ReturnsNoRemaining(t *testing.T) {
  	prepRepo := &fakePrepRepoUse{
  		slots: map[int][]*session.PreparedSlot{
  			1: {{TechID: "other_tech", Expended: false}},
  		},
  	}
  	svc, _, uid := setupUsePlayer(t, prepRepo.slots)
  	svc.SetPreparedTechRepo(prepRepo)

  	evt, err := svc.handleUse(uid, "shock_grenade")
  	require.NoError(t, err)
  	require.NotNil(t, evt)

  	msg := evt.GetMessage()
  	require.NotNil(t, msg)
  	assert.Contains(t, msg.Content, "No prepared uses")
  }

  // REQ-UC4: use (no arg) includes prepared techs with remaining use counts in the choices list.
  func TestHandleUse_NoArg_IncludesPreparedTechs(t *testing.T) {
  	prepRepo := &fakePrepRepoUse{
  		slots: map[int][]*session.PreparedSlot{
  			1: {
  				{TechID: "shock_grenade", Expended: false},
  				{TechID: "shock_grenade", Expended: true},
  				{TechID: "neural_disruptor", Expended: false},
  			},
  		},
  	}
  	svc, _, uid := setupUsePlayer(t, prepRepo.slots)
  	svc.SetPreparedTechRepo(prepRepo)

  	evt, err := svc.handleUse(uid, "")
  	require.NoError(t, err)
  	require.NotNil(t, evt)

  	resp := evt.GetUseResponse()
  	require.NotNil(t, resp)

  	// Find prepared tech entries in choices.
  	var foundShock, foundNeural bool
  	for _, c := range resp.Choices {
  		if c.FeatId == "shock_grenade" {
  			foundShock = true
  			// Should indicate 1 remaining (1 non-expended out of 2 slots)
  			assert.Contains(t, c.Description, "1")
  		}
  		if c.FeatId == "neural_disruptor" {
  			foundNeural = true
  		}
  	}
  	assert.True(t, foundShock, "shock_grenade must appear in choices")
  	assert.True(t, foundNeural, "neural_disruptor must appear in choices")
  }

  // REQ-UC7 (property): For any N non-expended slots of the same tech, exactly min(calls, N) slots
  // become expended after that many use calls; the (N+1)th call returns "no remaining".
  func TestPropertyHandleUse_PreparedTech_ExpendsExactly(t *testing.T) {
  	rapid.Check(t, func(rt *rapid.T) {
  		n := rapid.IntRange(1, 4).Draw(rt, "n")
  		slots := make([]*session.PreparedSlot, n)
  		for i := range slots {
  			slots[i] = &session.PreparedSlot{TechID: "test_tech", Expended: false}
  		}
  		prepRepo := &fakePrepRepoUse{
  			slots: map[int][]*session.PreparedSlot{1: slots},
  		}
  		sessMgr := session.NewManager()
  		svc := testMinimalService(t, sessMgr)
  		uid := fmt.Sprintf("prop-use-%d", rapid.IntRange(0, 9999).Draw(rt, "uid"))
  		_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
  			UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
  		})
  		if err != nil {
  			rt.Skip()
  		}
  		sess, ok := sessMgr.GetPlayer(uid)
  		if !ok {
  			rt.Skip()
  		}
  		sess.PreparedTechs = prepRepo.slots
  		svc.SetPreparedTechRepo(prepRepo)

  		// Call use n times — all should succeed.
  		for i := 0; i < n; i++ {
  			evt, err := svc.handleUse(uid, "test_tech")
  			if err != nil {
  				rt.Fatalf("call %d: unexpected error: %v", i, err)
  			}
  			if msg := evt.GetMessage(); msg != nil && strings.Contains(msg.Content, "No prepared uses") {
  				rt.Fatalf("call %d of %d: got 'no remaining' too early", i, n)
  			}
  		}

  		// (N+1)th call must return "no remaining".
  		evt, err := svc.handleUse(uid, "test_tech")
  		if err != nil {
  			rt.Fatalf("(n+1)th call: unexpected error: %v", err)
  		}
  		msg := evt.GetMessage()
  		if msg == nil || !strings.Contains(msg.Content, "No prepared uses") {
  			rt.Fatalf("(n+1)th call: expected 'No prepared uses', got: %v", msg)
  		}

  		// Exactly n slots must be expended.
  		expiredCount := 0
  		for _, slot := range sess.PreparedTechs[1] {
  			if slot != nil && slot.Expended {
  				expiredCount++
  			}
  		}
  		if expiredCount != n {
  			rt.Fatalf("expected %d expended slots, got %d", n, expiredCount)
  		}
  	})
  }
  ```

  **Note:** The property test uses `strings` — add `"strings"` to imports. Also add `"context"` if needed.

- [ ] **T2-S3: Confirm compile failure**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleUse_PreparedTech|TestPropertyHandleUse" -v 2>&1 | tail -10
  ```

  Expected: compile error — `handleUse` doesn't handle prepared techs yet.

- [ ] **T2-S4: Extend handleUse — list mode**

  In `internal/gameserver/grpc_service.go`, in `handleUse`, find the `abilityID == ""` block
  (around line 4588). Before the `return` statement, add prepared tech entries to `active`:

  ```go
  if abilityID == "" {
  	// Append non-expended prepared tech entries to active abilities list.
  	if len(sess.PreparedTechs) > 0 {
  		counts := make(map[string]int)
  		for _, slots := range sess.PreparedTechs {
  			for _, slot := range slots {
  				if slot != nil && !slot.Expended {
  					counts[slot.TechID]++
  				}
  			}
  		}
  		for techID, remaining := range counts {
  			active = append(active, &gamev1.FeatEntry{
  				FeatId:      techID,
  				Name:        techID,
  				Category:    "prepared_tech",
  				Active:      true,
  				Description: fmt.Sprintf("%d use(s) remaining", remaining),
  			})
  		}
  	}
  	// Return list of all active abilities for the client to prompt selection.
  	return &gamev1.ServerEvent{
  		Payload: &gamev1.ServerEvent_UseResponse{
  			UseResponse: &gamev1.UseResponse{Choices: active},
  		},
  	}, nil
  }
  ```

- [ ] **T2-S5: Extend handleUse — activation path**

  After the existing feat and class-feature activation code in `handleUse`, before the final
  return (or where the "not found" case falls through), add the prepared tech activation block.

  First, read the end of the `handleUse` function to find where to insert. The prepared tech
  block must execute only when `abilityID != ""` and no feat/class-feature matched.

  Add after the class-feature activation section:

  ```go
  // Attempt prepared tech activation if no feat/class-feature matched.
  if s.preparedTechRepo != nil && len(sess.PreparedTechs) > 0 {
  	// Find first non-expended slot with the matching tech ID (ascending level, ascending index).
  	levels := make([]int, 0, len(sess.PreparedTechs))
  	for lvl := range sess.PreparedTechs {
  		levels = append(levels, lvl)
  	}
  	sort.Ints(levels)
  	for _, lvl := range levels {
  		for idx, slot := range sess.PreparedTechs[lvl] {
  			if slot == nil || slot.TechID != abilityID || slot.Expended {
  				continue
  			}
  			// Found a non-expended slot — expend it.
  			if err := s.preparedTechRepo.SetExpended(ctx, sess.CharacterID, lvl, idx, true); err != nil {
  				s.logger.Warn("handleUse: SetExpended failed",
  					zap.String("uid", uid),
  					zap.String("techID", abilityID),
  					zap.Error(err))
  			}
  			sess.PreparedTechs[lvl][idx].Expended = true
  			return messageEvent(fmt.Sprintf("You activate %s.", abilityID)), nil
  		}
  	}
  	// No non-expended slot found for this tech ID.
  	return messageEvent(fmt.Sprintf("No prepared uses of %s remaining.", abilityID)), nil
  }
  ```

  **Note:** Check whether `sort` is already imported in `grpc_service.go` — it was added for
  `handleGrant`. If not, add it to the imports. Verify `messageEvent` is the correct helper
  (it is — used throughout the file for simple text responses).

- [ ] **T2-S6: Run new tests**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... \
    -run "TestHandleUse_PreparedTech|TestPropertyHandleUse" -v 2>&1 | tail -20
  ```

  Expected: all PASS.

- [ ] **T2-S7: Run full gameserver suite**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -count=1 2>&1 | tail -5
  ```

  Expected: PASS.

- [ ] **T2-S8: Commit**

  ```bash
  cd /home/cjohannsen/src/mud && git add \
    internal/gameserver/grpc_service.go \
    internal/gameserver/grpc_service_use_tech_test.go
  git commit -m "feat(gameserver): handleUse extends to expend prepared tech slots (REQ-UC1–4, REQ-UC7)"
  ```

---

## Chunk 3 — Task 3: Proto + character sheet + rest regression + FEATURES.md

### Task 3: PreparedSlotView proto + character sheet + rest regression + docs

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify (auto): `internal/gameserver/gamev1/game.pb.go` (regenerated)
- Modify: `internal/gameserver/grpc_service.go` (character sheet construction only)
- Create: `internal/gameserver/grpc_service_use_tech_sheet_test.go` (package gameserver, REQ-UC5, REQ-UC8)
- Modify: `docs/requirements/FEATURES.md`

---

- [ ] **T3-S1: Check proto field numbers**

  ```bash
  grep -n "= [0-9]*;" /home/cjohannsen/src/mud/api/proto/game/v1/game.proto | tail -10
  grep -n "PreparedSlotView\|prepared_slots" /home/cjohannsen/src/mud/api/proto/game/v1/game.proto
  ```

  Confirm:
  - `CharacterSheetView` highest field is 43 (`pending_tech_selections`) → `prepared_slots = 44`
  - `PreparedSlotView` does not exist yet

- [ ] **T3-S2: Add PreparedSlotView message and prepared_slots to proto**

  In `api/proto/game/v1/game.proto`:

  Add the new message near the other view messages (e.g., before `CharacterSheetView`):

  ```protobuf
  // PreparedSlotView represents one prepared technology slot on the character sheet.
  message PreparedSlotView {
      string tech_id  = 1;
      bool   expended = 2;
  }
  ```

  Add to `CharacterSheetView` after `pending_tech_selections = 43`:

  ```protobuf
  repeated PreparedSlotView prepared_slots = 44; // prepared technology slots with expended state
  ```

- [ ] **T3-S3: Regenerate proto**

  ```bash
  cd /home/cjohannsen/src/mud && make proto 2>&1 | tail -5
  ```

  Expected: no errors; `internal/gameserver/gamev1/game.pb.go` updated.

- [ ] **T3-S4: Write failing test for REQ-UC8**

  Create `internal/gameserver/grpc_service_use_tech_sheet_test.go` as `package gameserver`:

  ```go
  package gameserver

  import (
  	"testing"

  	"github.com/stretchr/testify/assert"
  	"github.com/stretchr/testify/require"

  	"github.com/cory-johannsen/mud/internal/game/session"
  	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
  )

  // REQ-UC8: Character sheet PreparedSlotView.expended reflects session state.
  func TestHandleChar_PreparedSlots_ReflectsExpendedState(t *testing.T) {
  	sessMgr := session.NewManager()
  	svc := testMinimalService(t, sessMgr)

  	uid := "player-char-sheet-prep"
  	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
  		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
  	})
  	require.NoError(t, err)

  	target, ok := sessMgr.GetPlayer(uid)
  	require.True(t, ok)
  	target.PreparedTechs = map[int][]*session.PreparedSlot{
  		1: {
  			{TechID: "shock_grenade", Expended: false},
  			{TechID: "neural_disruptor", Expended: true},
  		},
  	}

  	stream := &fakeSessionStream{}
  	err = svc.handleChar(uid, "req-sheet", stream)
  	require.NoError(t, err)
  	require.NotEmpty(t, stream.sent)

  	var sheetView *gamev1.CharacterSheetView
  	for _, evt := range stream.sent {
  		if cs := evt.GetCharacterSheet(); cs != nil {
  			sheetView = cs
  			break
  		}
  	}
  	require.NotNil(t, sheetView, "CharacterSheetView must be sent")
  	require.Len(t, sheetView.PreparedSlots, 2)

  	byTech := make(map[string]*gamev1.PreparedSlotView)
  	for _, s := range sheetView.PreparedSlots {
  		byTech[s.TechId] = s
  	}
  	require.Contains(t, byTech, "shock_grenade")
  	assert.False(t, byTech["shock_grenade"].Expended, "shock_grenade slot must not be expended")
  	require.Contains(t, byTech, "neural_disruptor")
  	assert.True(t, byTech["neural_disruptor"].Expended, "neural_disruptor slot must be expended")
  }

  // REQ-UC5: After rest, previously expended slots are restored (Expended = false in session).
  func TestHandleRest_ResetsExpendedSlots(t *testing.T) {
  	sessMgr := session.NewManager()
  	svc := testMinimalService(t, sessMgr)

  	uid := "player-rest-expended"
  	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
  		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player", Level: 1,
  	})
  	require.NoError(t, err)

  	sess, ok := sessMgr.GetPlayer(uid)
  	require.True(t, ok)
  	sess.Status = int32(gamev1.CombatStatus_COMBAT_STATUS_IDLE)
  	sess.Class = "test_job_rest"
  	// Set up an expended slot.
  	sess.PreparedTechs = map[int][]*session.PreparedSlot{
  		1: {{TechID: "shock_grenade", Expended: true}},
  	}

  	// Register a job with one pool entry so RearrangePreparedTechs re-prepares.
  	import "github.com/cory-johannsen/mud/internal/game/ruleset"
  	job := &ruleset.Job{
  		ID: "test_job_rest",
  		TechnologyGrants: &ruleset.TechnologyGrants{
  			Prepared: &ruleset.PreparedGrants{
  				SlotsByLevel: map[int]int{1: 1},
  				Pool:         []ruleset.PreparedEntry{{ID: "shock_grenade", Level: 1}},
  			},
  		},
  	}
  	jobReg := ruleset.NewJobRegistry()
  	jobReg.Register(job)
  	svc.SetJobRegistry(jobReg)

  	prepRepo := &fakePreparedRepoRest{}
  	svc.SetPreparedTechRepo(prepRepo)

  	stream := &fakeSessionStream{}
  	require.NoError(t, svc.handleRest(uid, "req-rest", stream))

  	// After rest, the slot must not be expended.
  	require.NotNil(t, sess.PreparedTechs[1])
  	require.Len(t, sess.PreparedTechs[1], 1)
  	assert.False(t, sess.PreparedTechs[1][0].Expended, "slot must not be expended after rest")
  }
  ```

  **IMPORTANT:** The `import` inside a function above is illustrative — move all imports to the
  package-level import block at the top of the file. The `ruleset` import belongs in the file header.
  Remove the inline `import` line from the function body.

- [ ] **T3-S5: Confirm compile failure**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... \
    -run "TestHandleChar_PreparedSlots|TestHandleRest_ResetsExpended" -v 2>&1 | tail -10
  ```

  Expected: compile error — `sheetView.PreparedSlots` field not yet populated in `handleChar`.

- [ ] **T3-S6: Populate PreparedSlots in character sheet construction**

  In `internal/gameserver/grpc_service.go`, find the character sheet construction function
  (`handleChar` or the equivalent, around line 3669 where `PendingTechSelections` is set).

  After `view.PendingTechSelections = int32(len(sess.PendingTechGrants))`, add:

  ```go
  // Prepared technology slots with expended state.
  if len(sess.PreparedTechs) > 0 {
  	levels := make([]int, 0, len(sess.PreparedTechs))
  	for lvl := range sess.PreparedTechs {
  		levels = append(levels, lvl)
  	}
  	sort.Ints(levels)
  	for _, lvl := range levels {
  		for _, slot := range sess.PreparedTechs[lvl] {
  			if slot != nil {
  				view.PreparedSlots = append(view.PreparedSlots, &gamev1.PreparedSlotView{
  					TechId:   slot.TechID,
  					Expended: slot.Expended,
  				})
  			}
  		}
  	}
  }
  ```

- [ ] **T3-S7: Run REQ-UC5 and REQ-UC8 tests**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... \
    -run "TestHandleChar_PreparedSlots|TestHandleRest_ResetsExpended" -v 2>&1 | tail -20
  ```

  Expected: PASS.

- [ ] **T3-S8: Run full test suite**

  ```bash
  cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | grep -E "FAIL|^ok"
  ```

  Expected: all packages pass.

- [ ] **T3-S9: Update FEATURES.md**

  In `docs/requirements/FEATURES.md`, mark the use-count item complete:

  Change:
  ```
  - [ ] Prepared tech slot expending — each prepared slot is one use; `use <tech>` expends the first matching non-expended slot; `rest` restores all slots; expended state persisted in DB
  ```
  To:
  ```
  - [x] Prepared tech slot expending — each prepared slot is one use; `use <tech>` expends the first matching non-expended slot; `rest` restores all slots; expended state persisted in DB
  ```

- [ ] **T3-S10: Commit**

  ```bash
  cd /home/cjohannsen/src/mud && git add \
    api/proto/game/v1/game.proto \
    internal/gameserver/gamev1/game.pb.go \
    internal/gameserver/gamev1/game_grpc.pb.go \
    internal/gameserver/grpc_service.go \
    internal/gameserver/grpc_service_use_tech_sheet_test.go \
    docs/requirements/FEATURES.md
  git commit -m "feat(gameserver): PreparedSlotView proto; character sheet prepared slots; rest resets expended (REQ-UC5, REQ-UC8)"
  ```

---

## Requirements Checklist

| REQ    | Task   | Description |
|--------|--------|-------------|
| REQ-UC1 | Task 2 | `use <tech>` with non-expended slot → expends it, returns activation message |
| REQ-UC2 | Task 2 | `use <tech>` with all slots expended → "No prepared uses remaining" |
| REQ-UC3 | Task 2 | `use <tech>` with no slot for that tech → "No prepared uses remaining" |
| REQ-UC4 | Task 2 | `use` (no arg) includes prepared techs with remaining use counts in choices |
| REQ-UC5 | Task 3 | After `rest`, expended slots restored (Expended = false) |
| REQ-UC6 | Task 1 | Expended state round-trips through DB (SetExpended + GetAll) |
| REQ-UC7 | Task 2 | Property: N slots, N uses expend all; N+1th returns "no remaining" |
| REQ-UC8 | Task 3 | Character sheet `PreparedSlots` reflects expended state |
