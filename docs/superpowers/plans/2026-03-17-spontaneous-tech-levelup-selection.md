# Spontaneous Tech Level-Up Selection Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two new Neural level-1 spontaneous techs, wire Influencer to grant one known tech at levels 3 and 5 via interactive selection, and verify the deferral mechanism end-to-end with tests.

**Architecture:** No mechanism changes — the existing `PartitionTechGrants` / `fillFromSpontaneousPool` / `ResolvePendingTechGrants` pipeline already handles deferred selection. This plan adds content YAML files, updates the archetype config, and adds tests that verify the end-to-end behavior through `handleSelectTech` and `LevelUpTechnologies`.

**Tech Stack:** Go 1.22, `pgregory.net/rapid` (property tests), protobuf/gRPC, YAML content files.

---

## Spec reference

`docs/superpowers/specs/2026-03-17-spontaneous-tech-levelup-selection-design.md`

---

## File Map

| Action | Path | Purpose |
|--------|------|---------|
| Create | `content/technologies/neural/neural_static.yaml` | New Neural level-1 tech |
| Create | `content/technologies/neural/synaptic_surge.yaml` | New Neural level-1 tech |
| Modify | `content/archetypes/influencer.yaml` | Add `known_by_level` + pool at levels 3 and 5 |
| Create | `internal/gameserver/grpc_service_spontaneous_selection_test.go` | grpc-level end-to-end tests (TEST-SSL1/2/3) |
| Modify | `internal/gameserver/technology_assignment_test.go` | Add TEST-SSL4 property test |
| Modify | `docs/requirements/FEATURES.md` | Mark Sub-project B complete |

---

## Chunk 1: Content Files

### Task 1: Create `neural_static.yaml`

**Files:**
- Create: `content/technologies/neural/neural_static.yaml`

- [ ] **Step 1: Create the file**

```yaml
id: neural_static
name: Neural Static
description: Floods a target's sensory nerves with dissonant white-noise, slowing their reactions.
tradition: neural
level: 1
usage_type: spontaneous
action_cost: 2
range: ranged
targets: single
duration: instant
save_type: reflex
save_dc: 15
effects:
  - type: condition
    condition_id: slowed
    duration: rounds:1
amped_level: 3
amped_effects:
  - type: condition
    condition_id: slowed
    duration: rounds:2
```

- [ ] **Step 2: Verify the file loads**

Run: `go test ./internal/game/technology/... -v 2>&1 | tail -10`
Expected: PASS — zero failures

---

### Task 2: Create `synaptic_surge.yaml`

**Files:**
- Create: `content/technologies/neural/synaptic_surge.yaml`

- [ ] **Step 1: Create the file**

```yaml
id: synaptic_surge
name: Synaptic Surge
description: Overwhelms a target's nervous system with a burst of pain signals.
tradition: neural
level: 1
usage_type: spontaneous
action_cost: 2
range: ranged
targets: single
duration: instant
save_type: will
save_dc: 15
effects:
  - type: damage
    dice: 2d4
    damage_type: neural
  - type: condition
    condition_id: frightened
    value: 1
    duration: rounds:1
amped_level: 3
amped_effects:
  - type: damage
    dice: 4d4
    damage_type: neural
  - type: condition
    condition_id: frightened
    value: 2
    duration: rounds:1
```

- [ ] **Step 2: Verify the file is present**

```bash
grep -r "synaptic_surge" content/technologies/
```
Expected: file path listed

---

### Task 3: Update `influencer.yaml`

**Files:**
- Modify: `content/archetypes/influencer.yaml`

The current `level_up_grants` block has only `uses_by_level` at each level. We add `known_by_level` and `pool` at levels 3 and 5. Pool size (3) > open slots (1) ensures `PartitionTechGrants` always defers to interactive selection.

- [ ] **Step 1: Update the file to the following complete content**

Only `level_up_grants` changes; all top-level fields are preserved unchanged:

```yaml
id: influencer
name: Influencer
description: "Pay attention to me. Influencers shape the world through personality, persuasion, and the power of narrative."
key_ability: flair
hit_points_per_level: 8

ability_boosts:
  fixed: [flair, savvy]
  free: 2

technology_grants:
  spontaneous:
    known_by_level:
      1: 2
    uses_by_level:
      1: 2
level_up_grants:
  2:
    spontaneous:
      uses_by_level:
        1: 1
  3:
    spontaneous:
      uses_by_level:
        1: 1
      known_by_level:
        1: 1
      pool:
        - id: mind_spike
          level: 1
        - id: neural_static
          level: 1
        - id: synaptic_surge
          level: 1
  4:
    spontaneous:
      uses_by_level:
        1: 1
  5:
    spontaneous:
      uses_by_level:
        1: 1
      known_by_level:
        1: 1
      pool:
        - id: mind_spike
          level: 1
        - id: neural_static
          level: 1
        - id: synaptic_surge
          level: 1
```

- [ ] **Step 2: Run full test suite to verify no regressions**

Run: `go test ./... 2>&1 | tail -20`
Expected: all PASS, no FAIL

- [ ] **Step 3: Commit content files**

```bash
git add content/technologies/neural/neural_static.yaml \
        content/technologies/neural/synaptic_surge.yaml \
        content/archetypes/influencer.yaml
git commit -m "feat(content): add neural_static, synaptic_surge; wire influencer levels 3+5 known tech selection"
```

---

## Chunk 2: Tests

### Task 4: Add TEST-SSL4 property test to `technology_assignment_test.go`

**Files:**
- Modify: `internal/gameserver/technology_assignment_test.go`
- Package: `gameserver_test`

TEST-SSL4 verifies that `LevelUpTechnologies` calls `promptFn` exactly N times when pool size > N open slots, with no duplicates, all chosen IDs from the pool, and exactly N entries in `sess.SpontaneousTechs[level]` after completion.

The grant has no `Fixed` entries (ensures only prompt-chosen techs appear in the session).

- [ ] **Step 1: Write the failing test**

Append to `internal/gameserver/technology_assignment_test.go`:

```go
// REQ-SSL4 (property): LevelUpTechnologies calls promptFn exactly N times when pool > open slots.
// All selected IDs come from the pool; no duplicates; session has exactly N entries at the level.
func TestPropertyLevelUpTechnologies_SpontaneousPromptCount(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate pool size 2-6 and open slots strictly less than pool size.
		nPool := rapid.IntRange(2, 6).Draw(rt, "nPool")
		nOpen := rapid.IntRange(1, nPool-1).Draw(rt, "nOpen")

		pool := make([]ruleset.SpontaneousEntry, nPool)
		for i := range pool {
			pool[i] = ruleset.SpontaneousEntry{ID: fmt.Sprintf("prop_tech_%d", i), Level: 1}
		}

		grants := &ruleset.TechnologyGrants{
			Spontaneous: &ruleset.SpontaneousGrants{
				KnownByLevel: map[int]int{1: nOpen},
				// No Fixed entries — only prompt-chosen techs populate SpontaneousTechs[1].
				Pool: pool,
			},
		}

		sess := &session.PlayerSession{Level: 5}
		spont := &fakeSpontaneousRepo{}
		hw := &fakeHardwiredRepo{}
		prep := &fakePreparedRepo{}
		inn := &fakeInnateRepo{}

		promptCallCount := 0
		promptFn := func(options []string) (string, error) {
			promptCallCount++
			// Return the first option each time (greedy selection).
			if len(options) == 0 {
				return "", nil
			}
			return options[0], nil
		}

		ctx := context.Background()
		err := gameserver.LevelUpTechnologies(ctx, sess, 1, grants, nil, promptFn, hw, prep, spont, inn, nil)
		if err != nil {
			rt.Fatalf("LevelUpTechnologies: %v", err)
		}

		// Invariant 1: promptFn called exactly nOpen times.
		if promptCallCount != nOpen {
			rt.Fatalf("expected promptFn called %d times, got %d", nOpen, promptCallCount)
		}

		chosen := sess.SpontaneousTechs[1]

		// Invariant 2: exactly nOpen entries in session.
		if len(chosen) != nOpen {
			rt.Fatalf("expected %d entries in SpontaneousTechs[1], got %d", nOpen, len(chosen))
		}

		// Invariant 3: all IDs are from the pool.
		validIDs := make(map[string]bool, nPool)
		for _, e := range pool {
			validIDs[e.ID] = true
		}
		for _, id := range chosen {
			if !validIDs[id] {
				rt.Fatalf("chosen tech %q not in pool", id)
			}
		}

		// Invariant 4: no duplicates.
		seen := make(map[string]bool, len(chosen))
		for _, id := range chosen {
			if seen[id] {
				rt.Fatalf("duplicate tech ID %q in SpontaneousTechs[1]", id)
			}
			seen[id] = true
		}
	})
}
```

- [ ] **Step 2: Run the test to verify it passes (no production code changes needed)**

Run: `go test ./internal/gameserver/... -run TestPropertyLevelUpTechnologies_SpontaneousPromptCount -v`
Expected: PASS

If it fails, debug: the most likely cause is that `parseTechID` strips the option string differently than expected. The `promptFn` returns `options[0]` which is the raw string from `buildSpontaneousOptions`. Check `buildSpontaneousOptions` output format and adjust accordingly. `parseTechID` in `technology_assignment.go` strips everything after `" — "`.

- [ ] **Step 3: Run full gameserver test suite**

Run: `go test ./internal/gameserver/... 2>&1 | tail -20`
Expected: all PASS

---

### Task 5: Write grpc-level tests (TEST-SSL1/2/3)

**Files:**
- Create: `internal/gameserver/grpc_service_spontaneous_selection_test.go`
- Package: `gameserver` (same as `grpc_service_selecttech_test.go`)

**Key setup facts:**
- `handleSelectTech(uid, requestID, stream)` checks `s.jobRegistry != nil` before proceeding
- `ResolvePendingTechGrants` calls `spontaneousTechRepo.Add` and `progressRepo.SetPendingTechLevels`
- Both repos must be non-nil; inject via `svc.SetSpontaneousTechRepo(...)` and `svc.SetProgressRepo(...)`
- `fakeSessionStream` is defined in `grpc_service_rest_test.go` (same package) — pre-populate `stream.recv` with `ClientMessage` values whose `Say.Message` field is the numeric choice string
- `testMinimalService` is defined in `grpc_service_grant_test.go` (same package)

**How `promptFeatureChoice` works:**
1. Sends a `MessageEvent` with numbered list of options
2. Reads a `ClientMessage` from `stream.Recv()`
3. Extracts text from `SayRequest.Message` or `MoveRequest.Direction`
4. Parses as integer 1..N; on invalid input logs warning and sends "Invalid selection. You will be prompted again on next login."

**Pending grants setup:**
```go
sess.PendingTechGrants = map[int]*ruleset.TechnologyGrants{
    3: {
        Spontaneous: &ruleset.SpontaneousGrants{
            KnownByLevel: map[int]int{1: 1},
            Pool: []ruleset.SpontaneousEntry{
                {ID: "mind_spike", Level: 1},
                {ID: "neural_static", Level: 1},
                {ID: "synaptic_surge", Level: 1},
            },
        },
    },
}
```

**Option indices** (1-based, as sent by `promptFeatureChoice`): option 2 selects `neural_static` **only if** `buildSpontaneousOptions` outputs them in pool order. Since `techReg` is nil in `testMinimalService`, option strings are raw IDs: `"mind_spike"`, `"neural_static"`, `"synaptic_surge"`. Selecting "2" returns `"neural_static"`. `parseTechID` strips after `" — "`, so `parseTechID("neural_static")` = `"neural_static"`.

- [ ] **Step 1: Write the test file**

```go
package gameserver

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// --- local fakes (package gameserver, no conflict with gameserver_test fakes) ---

type sslFakeSpontaneousRepo struct {
	techs map[int][]string
}

func (r *sslFakeSpontaneousRepo) GetAll(_ context.Context, _ int64) (map[int][]string, error) {
	if r.techs == nil {
		return make(map[int][]string), nil
	}
	return r.techs, nil
}
func (r *sslFakeSpontaneousRepo) Add(_ context.Context, _ int64, techID string, level int) error {
	if r.techs == nil {
		r.techs = make(map[int][]string)
	}
	r.techs[level] = append(r.techs[level], techID)
	return nil
}
func (r *sslFakeSpontaneousRepo) DeleteAll(_ context.Context, _ int64) error {
	r.techs = nil
	return nil
}

// sslFakeProgressRepo implements ProgressRepository (9 methods).
// Only GetPendingTechLevels and SetPendingTechLevels are exercised by handleSelectTech;
// the rest are no-op stubs to satisfy the interface.
type sslFakeProgressRepo struct{}

func (r *sslFakeProgressRepo) GetProgress(_ context.Context, _ int64) (level, experience, maxHP, pendingBoosts int, err error) {
	return 1, 0, 10, 0, nil
}
func (r *sslFakeProgressRepo) GetPendingSkillIncreases(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (r *sslFakeProgressRepo) IncrementPendingSkillIncreases(_ context.Context, _ int64, _ int) error {
	return nil
}
func (r *sslFakeProgressRepo) ConsumePendingBoost(_ context.Context, _ int64) error { return nil }
func (r *sslFakeProgressRepo) ConsumePendingSkillIncrease(_ context.Context, _ int64) error {
	return nil
}
func (r *sslFakeProgressRepo) IsSkillIncreasesInitialized(_ context.Context, _ int64) (bool, error) {
	return true, nil
}
func (r *sslFakeProgressRepo) MarkSkillIncreasesInitialized(_ context.Context, _ int64) error {
	return nil
}
func (r *sslFakeProgressRepo) GetPendingTechLevels(_ context.Context, _ int64) ([]int, error) {
	return nil, nil
}
func (r *sslFakeProgressRepo) SetPendingTechLevels(_ context.Context, _ int64, _ []int) error {
	return nil
}

// spontaneousSelectionTestService builds a minimal GameServiceServer suitable for
// handleSelectTech tests. Injects a non-nil jobRegistry, spontaneousTechRepo, and
// progressRepo — the three dependencies that handleSelectTech requires to proceed past
// its early-exit guards.
func spontaneousSelectionTestService(t *testing.T) (*GameServiceServer, *session.Manager, *sslFakeSpontaneousRepo) {
	t.Helper()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)

	// jobRegistry must be non-nil for handleSelectTech to proceed past the early-exit check.
	reg := ruleset.NewJobRegistry()
	reg.Register(&ruleset.Job{ID: "influencer", Name: "Influencer"})
	svc.SetJobRegistry(reg)

	spontRepo := &sslFakeSpontaneousRepo{}
	svc.SetSpontaneousTechRepo(spontRepo)
	svc.SetProgressRepo(&sslFakeProgressRepo{})

	return svc, sessMgr, spontRepo
}

// pendingGrant builds a PendingTechGrants map with a single spontaneous grant at level 3
// containing a 3-tech pool at level 1 with 1 open slot.
// This guarantees pool(3) > open(1) so PartitionTechGrants always defers.
func pendingGrant() map[int]*ruleset.TechnologyGrants {
	return map[int]*ruleset.TechnologyGrants{
		3: {
			Spontaneous: &ruleset.SpontaneousGrants{
				KnownByLevel: map[int]int{1: 1},
				Pool: []ruleset.SpontaneousEntry{
					{ID: "mind_spike", Level: 1},
					{ID: "neural_static", Level: 1},
					{ID: "synaptic_surge", Level: 1},
				},
			},
		},
	}
}

// sayMsg builds a ClientMessage with a SayRequest, simulating a player typing a number.
func sayMsg(text string) *gamev1.ClientMessage {
	return &gamev1.ClientMessage{
		Payload: &gamev1.ClientMessage_Say{
			Say: &gamev1.SayRequest{Message: text},
		},
	}
}

// REQ-SSL1: selecttech resolves a deferred spontaneous grant when a valid choice is submitted.
func TestHandleSelectTech_SpontaneousGrant_ValidChoice_ResolvesGrant(t *testing.T) {
	svc, sessMgr, spontRepo := spontaneousSelectionTestService(t)

	uid := "player-ssl1"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a",
		Role: "player", Class: "influencer",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.PendingTechGrants = pendingGrant()

	// Option 2 = "neural_static" (pool order: mind_spike=1, neural_static=2, synaptic_surge=3).
	// techReg is nil so buildSpontaneousOptions returns raw IDs in pool order.
	stream := &fakeSessionStream{
		recv: []*gamev1.ClientMessage{sayMsg("2")},
	}

	err = svc.handleSelectTech(uid, "req1", stream)
	require.NoError(t, err)

	// Grant cleared.
	assert.Empty(t, sess.PendingTechGrants, "PendingTechGrants must be empty after resolution")

	// Tech added to session.
	require.NotNil(t, sess.SpontaneousTechs)
	assert.Contains(t, sess.SpontaneousTechs[1], "neural_static",
		"neural_static must appear in SpontaneousTechs[1]")

	// Repo persisted the choice.
	assert.Contains(t, spontRepo.techs[1], "neural_static",
		"spontaneousTechRepo must have recorded neural_static at level 1")
}

// REQ-SSL2: selecttech sends "Invalid selection" when an out-of-range numeric choice is submitted.
// Known limitation: the pending grant is also cleared (silently lost) on invalid input.
func TestHandleSelectTech_SpontaneousGrant_InvalidChoice_SendsInvalidSelection(t *testing.T) {
	svc, sessMgr, _ := spontaneousSelectionTestService(t)

	uid := "player-ssl2"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a",
		Role: "player", Class: "influencer",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.PendingTechGrants = pendingGrant()

	// "99" is out of range (only 3 options).
	stream := &fakeSessionStream{
		recv: []*gamev1.ClientMessage{sayMsg("99")},
	}

	err = svc.handleSelectTech(uid, "req2", stream)
	require.NoError(t, err)

	// "Invalid selection" must appear in at least one sent message.
	found := false
	for _, evt := range stream.sent {
		if msg := evt.GetMessage(); msg != nil && strings.Contains(msg.Content, "Invalid selection") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected 'Invalid selection' in stream.sent messages")

	// No tech assigned.
	assert.Empty(t, sess.SpontaneousTechs[1],
		"no tech must be assigned after invalid selection")
}

// REQ-SSL3: The prompt sent by selecttech lists all three pool tech options.
func TestHandleSelectTech_SpontaneousGrant_PromptListsAllPoolOptions(t *testing.T) {
	svc, sessMgr, _ := spontaneousSelectionTestService(t)

	uid := "player-ssl3"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a",
		Role: "player", Class: "influencer",
	})
	require.NoError(t, err)

	sess, ok := sessMgr.GetPlayer(uid)
	require.True(t, ok)
	sess.PendingTechGrants = pendingGrant()

	// Pre-queue a valid choice so the stream doesn't EOF before the prompt is sent.
	stream := &fakeSessionStream{
		recv: []*gamev1.ClientMessage{sayMsg("1")},
	}

	err = svc.handleSelectTech(uid, "req3", stream)
	require.NoError(t, err)

	// Collect all message content.
	var allContent strings.Builder
	for _, evt := range stream.sent {
		if msg := evt.GetMessage(); msg != nil {
			allContent.WriteString(msg.Content)
			allContent.WriteString("\n")
		}
	}
	combined := allContent.String()

	assert.Contains(t, combined, "mind_spike", "prompt must list mind_spike")
	assert.Contains(t, combined, "neural_static", "prompt must list neural_static")
	assert.Contains(t, combined, "synaptic_surge", "prompt must list synaptic_surge")
}
```

- [ ] **Step 2: Run the failing tests first**

Run: `go test ./internal/gameserver/... -run "TestHandleSelectTech_Spontaneous" -v 2>&1 | head -40`
Expected: compilation success, tests run

- [ ] **Step 3: Fix any compilation errors**

Common issues:
- Import path wrong: use `github.com/cory-johannsen/mud/internal/game/ruleset` and `session` (check existing imports in nearby test files)
- `io` import unused if not needed (remove it)
- `fakeSessionStream` has `recv` field of `[]*gamev1.ClientMessage` and `ClientMessage_Say` oneof wrapper — verify proto field names match `game.proto`

To check the proto oneof wrapper name for `SayRequest`:
```bash
grep -n "ClientMessage_Say\|Say.*SayRequest" internal/gameserver/gamev1/game.pb.go | head -5
```

- [ ] **Step 4: Run full test suite**

Run: `go test ./internal/gameserver/... 2>&1 | tail -20`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/grpc_service_spontaneous_selection_test.go \
        internal/gameserver/technology_assignment_test.go
git commit -m "test(gameserver): add end-to-end spontaneous tech selection tests (REQ-SSL1-4)"
```

---

## Chunk 3: Docs

### Task 6: Update FEATURES.md

**Files:**
- Modify: `docs/requirements/FEATURES.md`

- [ ] **Step 1: Mark Sub-project B items complete**

Find the spontaneous tech level-up selection section (search for "Sub-project B" or "known tech" or "level-up selection") and mark the relevant items as complete (`[x]`). Add annotation such as `<!-- complete 2026-03-17 -->`.

```bash
grep -n "Sub-project B\|spontaneous.*level\|known.*tech.*level" docs/requirements/FEATURES.md | head -10
```

- [ ] **Step 2: Commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark spontaneous tech level-up selection (Sub-project B) complete"
```

---

## Final verification

- [ ] Run full test suite one last time:

```bash
go test ./... 2>&1 | tail -30
```

Expected: all PASS, zero failures.
