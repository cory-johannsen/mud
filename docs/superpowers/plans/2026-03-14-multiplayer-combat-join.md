# Multiplayer Combat Join Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a player enters a room with active combat they receive a yes/no prompt; on accept they join as a combatant with fresh initiative; XP, currency, and items split equally among all living participants.

**Architecture:** Three independent layers built bottom-up: (1) data model additions (`PendingCombatJoin`, `Participants`, `AddCombatant`, `AwardXPAmount`); (2) join/decline command pipeline (CMD-1–7); (3) runtime wiring (move trigger, combat-end cleanup, split distribution). Each task produces committed, green tests before the next begins.

**Tech Stack:** Go 1.23, `pgregory.net/rapid` (property-based tests), protobuf v3, `go test ./...`

**Module path:** `github.com/cory-johannsen/mud` (used in all import paths)

---

## Chunk 1: Data Model and Engine Methods

### Task 1: PlayerSession.PendingCombatJoin + Combat.Participants

**Spec ref:** Feature 1 — Data Model
**Files:**
- Modify: `internal/game/session/manager.go` (PlayerSession struct)
- Modify: `internal/game/combat/engine.go` (Combat struct + StartCombat)
- Modify: `internal/game/combat/engine_test.go` (add Participants tests)

The existing `engine_test.go` is `package combat_test`. All combat types must be referenced with the `combat.` qualifier. The existing helper `makeTwoCombatantCombat` sets `Initiative` fields directly on `Combatant` literals (no `RollInitiative` call needed for unit tests that don't test initiative rolling).

- [ ] **Step 1: Write failing tests for Participants population**

Add `"fmt"` to the import block in `internal/game/combat/engine_test.go` (it is not currently imported).

Add to the bottom of `internal/game/combat/engine_test.go`:

```go
func TestStartCombat_PopulatesParticipants_OnePlayer(t *testing.T) {
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, Initiative: 15},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 8},
	}
	cbt, err := eng.StartCombat("room1", combatants, makeTestRegistry(), nil, "")
	require.NoError(t, err)
	require.Len(t, cbt.Participants, 1, "expected 1 participant (player only)")
	assert.Equal(t, "p1", cbt.Participants[0])
}

func TestStartCombat_PopulatesParticipants_TwoPlayers(t *testing.T) {
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, Initiative: 15},
		{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 20, CurrentHP: 20, AC: 13, Level: 1, Initiative: 12},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 8},
	}
	cbt, err := eng.StartCombat("room1", combatants, makeTestRegistry(), nil, "")
	require.NoError(t, err)
	assert.Len(t, cbt.Participants, 2, "expected 2 participants (both players)")
}

func TestStartCombat_NPCNotInParticipants(t *testing.T) {
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, Initiative: 15},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 8},
	}
	cbt, err := eng.StartCombat("room1", combatants, makeTestRegistry(), nil, "")
	require.NoError(t, err)
	for _, uid := range cbt.Participants {
		assert.NotEqual(t, "n1", uid, "NPC should not appear in Participants")
	}
}

// REQ-T-PROP (property): for any mix of combatants, len(Participants) == number of KindPlayer combatants.
func TestProperty_StartCombat_ParticipantsCountEqualsPlayerCount(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 8).Draw(rt, "n")
		var combatants []*combat.Combatant
		wantCount := 0
		for i := 0; i < n; i++ {
			kind := rapid.SampledFrom([]combat.Kind{combat.KindPlayer, combat.KindNPC}).Draw(rt, fmt.Sprintf("kind_%d", i))
			id := fmt.Sprintf("c%d", i)
			combatants = append(combatants, &combat.Combatant{
				ID:        id,
				Kind:      kind,
				Name:      id,
				MaxHP:     20,
				CurrentHP: 20,
				AC:        12,
				Level:     1,
				Initiative: rapid.IntRange(1, 20).Draw(rt, fmt.Sprintf("init_%d", i)),
			})
			if kind == combat.KindPlayer {
				wantCount++
			}
		}
		eng := combat.NewEngine()
		cbt, err := eng.StartCombat("room1", combatants, makeTestRegistry(), nil, "")
		require.NoError(rt, err)
		require.Equal(rt, wantCount, len(cbt.Participants), "Participants count must equal player count")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestStartCombat_Populates|TestStartCombat_NPC|TestProperty_StartCombat_Participants" -v
```

Expected: FAIL — `cbt.Participants` field does not exist (or `combat.CombatantKind` type unknown).

- [ ] **Step 3: Add `Participants []string` to `Combat` struct**

In `internal/game/combat/engine.go`, find the `Combat` struct (around line 14). Add after the `Over bool` or last existing field:

```go
// Participants is the ordered list of player UIDs who were ever active combatants
// in this encounter. Used for XP and loot distribution. Never shrunk after join.
Participants []string
```

- [ ] **Step 4: Update `StartCombat` to populate `Participants`**

In `StartCombat`, after `cbt.Combatants` is assigned (after `sortByInitiativeDesc`) and before `return cbt, nil`, add:

```go
// Populate Participants with player UIDs from the initial combatant list.
// nil-slice append is valid Go; no explicit initialization required.
for _, c := range combatants {
	if c.Kind == KindPlayer {
		cbt.Participants = append(cbt.Participants, c.ID)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestStartCombat_Populates|TestStartCombat_NPC|TestProperty_StartCombat_Participants" -v
```

Expected: PASS

- [ ] **Step 6: Add `PendingCombatJoin` to `PlayerSession`**

In `internal/game/session/manager.go`, find the `PlayerSession` struct. Add after `BankedAP int` (the last field):

```go
// PendingCombatJoin holds the RoomID of a combat the player has been invited to join.
// Empty string means no pending join offer. Protected by combatMu in the gameserver.
PendingCombatJoin string
```

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all packages PASS.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/session/manager.go internal/game/combat/engine.go internal/game/combat/engine_test.go && git commit -m "feat(combat): add PendingCombatJoin to session and Participants to Combat; populate in StartCombat"
```

---

### Task 2: AddCombatant engine method

**Spec ref:** Feature 1 — Engine new method
**Files:**
- Modify: `internal/game/combat/engine.go` (new method)
- Create: `internal/game/combat/engine_add_combatant_test.go`

**Key codebase facts:**
- `combat.Source` interface (in `resolver.go`) requires `Intn(n int) int`. Use `rand.New(rand.NewSource(seed))` which returns `*rand.Rand` implementing `Intn`.
- `cbt.CurrentTurn()` returns the `*Combatant` whose turn it is (skips dead combatants).
- `cbt.AdvanceTurn()` increments `turnIndex` modulo `len(Combatants)`.
- `cbt.Conditions` is `map[string]*condition.ActiveSet`.
- `condition.NewActiveSet()` — verify this constructor name in `internal/game/condition/`; see how `StartCombat` initializes it.

- [ ] **Step 1: Verify `condition.NewActiveSet` constructor**

```bash
grep -n "func New\|func.*ActiveSet\|Conditions\[" /home/cjohannsen/src/mud/internal/game/condition/*.go /home/cjohannsen/src/mud/internal/game/combat/engine.go | head -20
```

Note the exact constructor used in `StartCombat` for `cbt.Conditions[c.ID] = ...`. Use the same in `AddCombatant`.

- [ ] **Step 2: Write failing tests**

Create `internal/game/combat/engine_add_combatant_test.go`:

```go
package combat_test

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/combat"
)

// combatWithKnownInitiatives creates a combat where initiatives are pre-set (not rolled),
// so tests are deterministic.
func combatWithKnownInitiatives(t *testing.T, roomID string) (*combat.Engine, *combat.Combat) {
	t.Helper()
	eng := combat.NewEngine()
	combatants := []*combat.Combatant{
		{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, AC: 14, Level: 1, Initiative: 15},
		{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 5},
	}
	// Pass makeTestRegistry() (defined in engine_test.go in the same package) to avoid
	// nil-pointer panics in StartRound's condition-tick code.
	cbt, err := eng.StartCombat(roomID, combatants, makeTestRegistry(), nil, "")
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	return eng, cbt
}

// REQ-T15: AddCombatant on non-existent roomID returns error.
func TestAddCombatant_NonExistentRoom_ReturnsError(t *testing.T) {
	eng := combat.NewEngine()
	c := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 10, CurrentHP: 10, Initiative: 8}
	err := eng.AddCombatant("no-room", c)
	if err == nil {
		t.Fatal("expected error for non-existent room, got nil")
	}
}

// REQ-T5 (example): AddCombatant inserts in correct initiative-sorted position.
func TestAddCombatant_InsertsInInitiativeOrder(t *testing.T) {
	eng, cbt := combatWithKnownInitiatives(t, "room1")

	// Insert p2 with initiative 10 — should go between p1(15) and n1(5).
	p2 := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 10, CurrentHP: 10, Initiative: 10}
	if err := eng.AddCombatant("room1", p2); err != nil {
		t.Fatal(err)
	}

	// Order: p1(15), p2(10), n1(5)
	if len(cbt.Combatants) != 3 {
		t.Fatalf("want 3 combatants, got %d", len(cbt.Combatants))
	}
	if cbt.Combatants[0].ID != "p1" {
		t.Errorf("pos 0: want p1, got %s", cbt.Combatants[0].ID)
	}
	if cbt.Combatants[1].ID != "p2" {
		t.Errorf("pos 1: want p2, got %s", cbt.Combatants[1].ID)
	}
	if cbt.Combatants[2].ID != "n1" {
		t.Errorf("pos 2: want n1, got %s", cbt.Combatants[2].ID)
	}
}

// REQ-T14 (example): AddCombatant appends to Participants for KindPlayer.
func TestAddCombatant_AppendsToParticipants_Player(t *testing.T) {
	eng, cbt := combatWithKnownInitiatives(t, "room1")
	initialLen := len(cbt.Participants)

	p2 := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 10, CurrentHP: 10, Initiative: 8}
	if err := eng.AddCombatant("room1", p2); err != nil {
		t.Fatal(err)
	}

	if len(cbt.Participants) != initialLen+1 {
		t.Errorf("Participants: want %d, got %d", initialLen+1, len(cbt.Participants))
	}
	found := false
	for _, uid := range cbt.Participants {
		if uid == "p2" {
			found = true
		}
	}
	if !found {
		t.Errorf("p2 not in Participants: %v", cbt.Participants)
	}
}

// REQ-T14 (example): NPCs are NOT added to Participants.
func TestAddCombatant_NPC_NotAddedToParticipants(t *testing.T) {
	eng, cbt := combatWithKnownInitiatives(t, "room1")
	initialLen := len(cbt.Participants)

	npc2 := &combat.Combatant{ID: "n2", Kind: combat.KindNPC, Name: "Ganger2", MaxHP: 10, CurrentHP: 10, Initiative: 3}
	if err := eng.AddCombatant("room1", npc2); err != nil {
		t.Fatal(err)
	}
	if len(cbt.Participants) != initialLen {
		t.Errorf("Participants grew for NPC: want %d, got %d", initialLen, len(cbt.Participants))
	}
}

// REQ-T14 (example): Conditions initialized for new combatant after AddCombatant.
func TestAddCombatant_InitializesConditions(t *testing.T) {
	eng, cbt := combatWithKnownInitiatives(t, "room1")

	p2 := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 10, CurrentHP: 10, Initiative: 8}
	if err := eng.AddCombatant("room1", p2); err != nil {
		t.Fatal(err)
	}

	if cbt.Conditions["p2"] == nil {
		t.Error("Conditions[p2] not initialized after AddCombatant")
	}
}

// REQ-T22 (example): turnIndex adjusted when new combatant inserts before current actor.
// Setup: p1(15), n1(5). Advance turn so n1 (index 1) is current actor (turnIndex=1).
// Insert p2(10) at index 1 — insertion position (1) <= turnIndex (1), so turnIndex becomes 2.
// After adjustment, CurrentTurn() should still return n1.
func TestAddCombatant_AdjustsTurnIndex_WhenInsertingBeforeCurrentActor(t *testing.T) {
	eng, cbt := combatWithKnownInitiatives(t, "room1")

	// Advance so n1 is the current actor (turnIndex goes 0→1).
	cbt.AdvanceTurn()

	// Verify n1 is current before insertion.
	if cur := cbt.CurrentTurn(); cur == nil || cur.ID != "n1" {
		t.Fatalf("before AddCombatant: current actor should be n1, got %v", cur)
	}

	// Insert p2(10) at index 1 (between p1=15 and n1=5).
	p2 := &combat.Combatant{ID: "p2", Kind: combat.KindPlayer, Name: "Bob", MaxHP: 10, CurrentHP: 10, Initiative: 10}
	if err := eng.AddCombatant("room1", p2); err != nil {
		t.Fatal(err)
	}

	// After insertion, current actor must still be n1 (now at index 2).
	if cur := cbt.CurrentTurn(); cur == nil || cur.ID != "n1" {
		id := "<nil>"
		if cur != nil {
			id = cur.ID
		}
		t.Errorf("after AddCombatant: current actor should be n1, got %q", id)
	}
}

// REQ-T5 (property): AddCombatant always produces a slice sorted by initiative descending.
func TestProperty_AddCombatant_MaintainsSortOrder(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		eng := combat.NewEngine()
		p1 := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, Initiative: 15}
		n1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 10, CurrentHP: 10, Initiative: 5}
		if _, err := eng.StartCombat("room1", []*combat.Combatant{p1, n1}, makeTestRegistry(), nil, ""); err != nil {
			rt.Fatal(err)
		}

		joinerInit := rapid.IntRange(-5, 25).Draw(rt, "joinerInitiative")
		joiner := &combat.Combatant{
			ID:         "p2",
			Kind:       combat.KindPlayer,
			Name:       "Bob",
			MaxHP:      10,
			CurrentHP:  10,
			Initiative: joinerInit,
		}
		if err := eng.AddCombatant("room1", joiner); err != nil {
			rt.Fatal(err)
		}

		cbt, _ := eng.GetCombat("room1")
		for i := 1; i < len(cbt.Combatants); i++ {
			if cbt.Combatants[i].Initiative > cbt.Combatants[i-1].Initiative {
				rt.Fatalf("slice not sorted at index %d: %d > %d",
					i, cbt.Combatants[i].Initiative, cbt.Combatants[i-1].Initiative)
			}
		}
	})
}

// REQ-T14 (property): Participants grows monotonically; length equals player combatants added.
func TestProperty_AddCombatant_ParticipantsGrowsMonotonically(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		eng := combat.NewEngine()
		p1 := &combat.Combatant{ID: "p1", Kind: combat.KindPlayer, Name: "Alice", MaxHP: 20, CurrentHP: 20, Initiative: 15}
		n1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Ganger", MaxHP: 10, CurrentHP: 10, Initiative: 5}
		if _, err := eng.StartCombat("room1", []*combat.Combatant{p1, n1}, makeTestRegistry(), nil, ""); err != nil {
			rt.Fatal(err)
		}

		expectedParticipants := 1 // p1 from StartCombat
		n := rapid.IntRange(0, 5).Draw(rt, "numJoiners")
		for i := 0; i < n; i++ {
			init := rapid.IntRange(1, 20).Draw(rt, fmt.Sprintf("initiative_%d", i))
			joiner := &combat.Combatant{
				ID:         fmt.Sprintf("p%d", i+2),
				Kind:       combat.KindPlayer,
				MaxHP:      10,
				CurrentHP:  10,
				Initiative: init,
			}
			if err := eng.AddCombatant("room1", joiner); err != nil {
				rt.Fatal(err)
			}
			expectedParticipants++
		}

		cbt, _ := eng.GetCombat("room1")
		if len(cbt.Participants) != expectedParticipants {
			rt.Fatalf("Participants: want %d, got %d", expectedParticipants, len(cbt.Participants))
		}
	})
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestAddCombatant|TestProperty_AddCombatant" -v 2>&1 | head -30
```

Expected: compile error or FAIL — `AddCombatant` not defined.

- [ ] **Step 4: Confirm conditions constructor (no action required)**

`condition.NewActiveSet()` takes zero arguments. `StartCombat` in `engine.go` uses this pattern:
```go
set := condition.NewActiveSet()
if scriptMgr != nil {
    set.SetScripting(scriptMgr, zoneID)
}
cbt.Conditions[c.ID] = set
```
The implementation below replicates this exactly using `cbt.scriptMgr` and `cbt.zoneID`.

- [ ] **Step 5: Implement `AddCombatant` on `Engine`**

Add to `internal/game/combat/engine.go`, after `EndCombat`:

```go
// AddCombatant inserts c into the combat for roomID in initiative order.
//
// Precondition: a Combat for roomID exists; c.Initiative has already been rolled.
// Postcondition: c appears in cbt.Combatants sorted by initiative descending;
//   c.ID appended to cbt.Participants if c.Kind == KindPlayer;
//   cbt.Conditions[c.ID] initialized to match the pattern used in StartCombat.
//   ActionQueues is NOT populated — StartRound rebuilds it each round.
//
// Locking: acquires e.mu (write lock) for the full duration. Caller must NOT hold e.mu.
//
// turnIndex: If insertion position i <= cbt.turnIndex, cbt.turnIndex is incremented
//   to preserve the identity of the current actor.
func (e *Engine) AddCombatant(roomID string, c *Combatant) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cbt, ok := e.combats[roomID]
	if !ok {
		return fmt.Errorf("AddCombatant: no combat in room %q", roomID)
	}

	// Find insertion index: first position where existing combatant has strictly lower initiative.
	insertAt := len(cbt.Combatants)
	for i, existing := range cbt.Combatants {
		if c.Initiative > existing.Initiative {
			insertAt = i
			break
		}
	}

	// Positional insert (single-position slice expansion, not re-sort).
	cbt.Combatants = append(cbt.Combatants, nil)
	copy(cbt.Combatants[insertAt+1:], cbt.Combatants[insertAt:])
	cbt.Combatants[insertAt] = c

	// Adjust turnIndex to preserve the current actor's identity.
	if insertAt <= cbt.turnIndex {
		cbt.turnIndex++
	}

	// Track player combatants in Participants.
	if c.Kind == KindPlayer {
		cbt.Participants = append(cbt.Participants, c.ID)
	}

	// Initialize conditions for the new combatant (same pattern as StartCombat).
	set := condition.NewActiveSet()
	if cbt.scriptMgr != nil {
		set.SetScripting(cbt.scriptMgr, cbt.zoneID)
	}
	cbt.Conditions[c.ID] = set

	return nil
}
```

Also add `"fmt"` to the imports if not already present.

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/combat/... -run "TestAddCombatant|TestProperty_AddCombatant" -v
```

Expected: PASS

- [ ] **Step 7: Run full suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/combat/engine.go internal/game/combat/engine_add_combatant_test.go && git commit -m "feat(combat): implement Engine.AddCombatant with initiative-order insertion and turnIndex correction"
```

---

### Task 3: AwardXPAmount on xp.Service

**Spec ref:** Feature 3 — XP split new method
**Files:**
- Modify: `internal/game/xp/service.go`
- Modify: `internal/game/xp/service_test.go`

**Key codebase facts (from existing `service_test.go`):**
- Package: `package xp_test`
- Mock saver type: `fakeProgressSaver` (already defined in the test file)
- Config helper: `testCfg()` returns `*xp.XPConfig`
- Session helper: `testSess(level, currentXP, maxHP int)` returns `*session.PlayerSession`
- Uses `require.NoError`, `assert.Equal` from `testify`

- [ ] **Step 1: Write failing tests**

Add to `internal/game/xp/service_test.go`:

```go
func TestService_AwardXPAmount_AwardsCorrectXP(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 0, 10)

	_, err := svc.AwardXPAmount(context.Background(), sess, 0, 25)
	require.NoError(t, err)
	assert.Equal(t, 25, sess.Experience, "expected 25 XP awarded")
}

func TestService_AwardXPAmount_ZeroXP_NoChange(t *testing.T) {
	saver := &fakeProgressSaver{}
	svc := xp.NewService(testCfg(), saver)
	sess := testSess(1, 50, 10)

	_, err := svc.AwardXPAmount(context.Background(), sess, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 50, sess.Experience, "0 XP award should not change experience")
}

func TestService_AwardXPAmount_SameAsAwardKill_WhenFullAmount(t *testing.T) {
	// Verify AwardXPAmount with the full kill amount equals AwardKill behavior.
	cfg := testCfg()
	saver1 := &fakeProgressSaver{}
	svc1 := xp.NewService(cfg, saver1)
	sess1 := testSess(1, 0, 10)
	_, err := svc1.AwardKill(context.Background(), sess1, 1, 0)
	require.NoError(t, err)

	saver2 := &fakeProgressSaver{}
	svc2 := xp.NewService(cfg, saver2)
	sess2 := testSess(1, 0, 10)
	fullXP := cfg.Awards.KillXPPerNPCLevel * 1 // npcLevel=1
	_, err = svc2.AwardXPAmount(context.Background(), sess2, 0, fullXP)
	require.NoError(t, err)

	assert.Equal(t, sess1.Experience, sess2.Experience, "AwardXPAmount(fullXP) should equal AwardKill")
}

// REQ-T-PROP-A (property, SWENG-5a): AwardXPAmount(0) never changes Experience.
func TestProperty_AwardXPAmount_ZeroAwardNoChange(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		saver := &fakeProgressSaver{}
		svc := xp.NewService(testCfg(), saver)
		startXP := rapid.IntRange(0, 1000).Draw(rt, "startXP")
		sess := testSess(1, startXP, 10)

		_, err := svc.AwardXPAmount(context.Background(), sess, 0, 0)
		require.NoError(rt, err)
		require.Equal(rt, startXP, sess.Experience, "zero XP award must not change Experience")
	})
}

// REQ-T-PROP-B (property, SWENG-5a): AwardXPAmount(n) increases Experience by exactly n
// when no level-up boundary is crossed (startXP=0, xpAmount well below level threshold).
func TestProperty_AwardXPAmount_PositiveAwardIncreasesExperience(t *testing.T) {
	cfg := testCfg()
	rapid.Check(t, func(rt *rapid.T) {
		saver := &fakeProgressSaver{}
		svc := xp.NewService(cfg, saver)
		// xpAmount capped at 100 to stay safely below any level threshold.
		xpAmount := rapid.IntRange(1, 100).Draw(rt, "xpAmount")
		sess := testSess(1, 0, 10) // start with 0 XP

		_, err := svc.AwardXPAmount(context.Background(), sess, 0, xpAmount)
		require.NoError(rt, err)
		require.Equal(rt, xpAmount, sess.Experience, "Experience must increase by exactly xpAmount")
	})
}
```

**Note on return type:** The spec's `AwardXPAmount` signature shows `error` only, but the implementation returns `([]string, error)` to pass level-up announcement messages to callers — consistent with `AwardKill` and `award`. The plan's `([]string, error)` return is authoritative; the spec's return type is an omission.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/xp/... -run "TestService_AwardXPAmount" -v
```

Expected: FAIL — `AwardXPAmount` method does not exist.

- [ ] **Step 3: Implement `AwardXPAmount`**

Add to `internal/game/xp/service.go`, after `AwardKill`:

```go
// AwardXPAmount awards a pre-computed XP amount directly to a player.
// Use this instead of AwardKill when splitting a kill reward across multiple
// participants, to avoid re-multiplying by KillXPPerNPCLevel.
//
// Precondition: sess non-nil; xpAmount >= 0.
// Postcondition: same as AwardKill — XP, level, HP updated in-place; persisted.
func (s *Service) AwardXPAmount(ctx context.Context, sess *session.PlayerSession, characterID int64, xpAmount int) ([]string, error) {
	return s.award(ctx, sess, characterID, xpAmount)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/xp/... -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/xp/service.go internal/game/xp/service_test.go && git commit -m "feat(xp): add AwardXPAmount for pre-divided kill XP in multiplayer splits"
```

---

## Chunk 2: join and decline Commands (CMD-1–7)

### Task 4: join and decline commands — proto + bridge + constants

**Spec ref:** Feature 2 — join/decline commands (CMD-1, CMD-2, CMD-4, CMD-5)
**Files:**
- Modify: `internal/game/command/commands.go`
- Modify: `api/proto/game/v1/game.proto`
- Run: `make proto` (regenerates `internal/gameserver/gamev1/game.pb.go`)
- Modify: `internal/frontend/handlers/bridge_handlers.go`

- [ ] **Step 1: Add handler constants and BuiltinCommands entries (CMD-1, CMD-2)**

In `internal/game/command/commands.go`, add constants (find the block with other Handler constants):

```go
HandlerJoin    = "join"
HandlerDecline = "decline"
```

In `BuiltinCommands()`, append two entries:

```go
{Name: "join", Help: "Join active combat in the current room.", Category: CategoryCombat, Handler: HandlerJoin},
{Name: "decline", Help: "Decline to join active combat.", Category: CategoryCombat, Handler: HandlerDecline},
```

- [ ] **Step 2: Add proto messages (CMD-4)**

In `api/proto/game/v1/game.proto`, add two new message types (at the top-level, not inside another message):

```protobuf
message JoinRequest {}
message DeclineRequest {}
```

In the `ClientMessage` oneof `payload`, after `DelayRequest delay = 72`:

```protobuf
JoinRequest join = 73;
DeclineRequest decline = 74;
```

- [ ] **Step 3: Run `make proto`**

```bash
cd /home/cjohannsen/src/mud && make proto
```

Expected: regenerates `internal/gameserver/gamev1/game.pb.go` — `ClientMessage_Join` and `ClientMessage_Decline` wrapper types now exist.

- [ ] **Step 4: Confirm `TestAllCommandHandlersAreWired` fails first (TDD red — CMD-5)**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/... -run "TestAllCommandHandlersAreWired" -v
```

Expected: FAIL — `join` and `decline` not yet wired.

- [ ] **Step 5: Add bridge functions (CMD-5)**

In `internal/frontend/handlers/bridge_handlers.go`, add after the last existing bridge function:

```go
func bridgeJoin(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Join{Join: &gamev1.JoinRequest{}},
	}}, nil
}

func bridgeDecline(bctx *bridgeContext) (bridgeResult, error) {
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_Decline{Decline: &gamev1.DeclineRequest{}},
	}}, nil
}
```

Register both in `bridgeHandlerMap`:

```go
command.HandlerJoin:    bridgeJoin,
command.HandlerDecline: bridgeDecline,
```

- [ ] **Step 6: Verify `TestAllCommandHandlersAreWired` passes (TDD green — CMD-5)**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/frontend/... -run "TestAllCommandHandlersAreWired" -v
```

Expected: PASS.

- [ ] **Step 7: Run full suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/command/commands.go api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go internal/frontend/handlers/bridge_handlers.go && git commit -m "feat(combat): add join/decline commands — proto messages, constants, bridge handlers (CMD-1,2,4,5)"
```

---

### Task 5: handleJoin and handleDecline (CMD-3, CMD-6, CMD-7)

**Spec ref:** Feature 2 — join/decline commands full pipeline
**Files:**
- Create: `internal/game/command/join.go`
- Create: `internal/game/command/decline.go`
- Create: `internal/gameserver/grpc_service_join_test.go`
- Modify: `internal/gameserver/grpc_service.go`

**Before writing tests:** Read `internal/gameserver/grpc_service_delay_test.go` and `internal/gameserver/grpc_service_grapple_test.go` for the exact test helper function signatures, test service constructor, and how to set up a player session with specific Status values. Use those patterns exactly.

- [ ] **Step 1: Write failing CMD-3 unit tests for HandleJoin and HandleDecline**

The command-layer `HandleX` functions follow the pattern in `internal/game/command/calm.go` and `grapple.go`: they take `args []string` and return `(*XRequest, error)`. Tests live in `package command` (internal package, same as the source file).

Create `internal/game/command/join_test.go`:

```go
package command

import (
	"testing"

	"pgregory.net/rapid"
)

func TestHandleJoin_ReturnsNonNilRequest(t *testing.T) {
	req, err := HandleJoin(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil JoinRequest")
	}
}

func TestHandleJoin_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := HandleJoin(args)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			rt.Fatal("expected non-nil JoinRequest")
		}
	})
}
```

Create `internal/game/command/decline_test.go`:

```go
package command

import (
	"testing"

	"pgregory.net/rapid"
)

func TestHandleDecline_ReturnsNonNilRequest(t *testing.T) {
	req, err := HandleDecline(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil DeclineRequest")
	}
}

func TestHandleDecline_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := HandleDecline(args)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			rt.Fatal("expected non-nil DeclineRequest")
		}
	})
}
```

- [ ] **Step 2: Run CMD-3 tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/command/... -run "TestHandleJoin|TestHandleDecline" -v 2>&1 | head -10
```

Expected: FAIL — `HandleJoin`/`HandleDecline`/`JoinRequest`/`DeclineRequest` not defined.

- [ ] **Step 3: Create thin command stubs (CMD-3)**

`internal/game/command/join.go`:

```go
package command

// JoinRequest is the parsed form of the join command.
type JoinRequest struct{}

// HandleJoin parses the join command. Arguments are ignored.
//
// Postcondition: Always returns a non-nil *JoinRequest and nil error.
func HandleJoin(_ []string) (*JoinRequest, error) {
	return &JoinRequest{}, nil
}
```

`internal/game/command/decline.go`:

```go
package command

// DeclineRequest is the parsed form of the decline command.
type DeclineRequest struct{}

// HandleDecline parses the decline command. Arguments are ignored.
//
// Postcondition: Always returns a non-nil *DeclineRequest and nil error.
func HandleDecline(_ []string) (*DeclineRequest, error) {
	return &DeclineRequest{}, nil
}
```

- [ ] **Step 4: Run CMD-3 tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/command/... -run "TestHandleJoin|TestHandleDecline" -v
```

Expected: PASS.

- [ ] **Step 5: Write failing gameserver tests**

Read `internal/gameserver/grpc_service_grapple_test.go` in full to understand:
- The package name (`package gameserver` — NOT `gameserver_test`)
- The `newGrappleSvc` / `newGrappleSvcWithCombat` helper pattern
- How player sessions are registered and retrieved
- How `CombatHandler` is constructed with `NewCombatHandler`

Then create `internal/gameserver/grpc_service_join_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/command"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"pgregory.net/rapid"
)

// newJoinSvc mirrors newGrappleSvc — a minimal GameServiceServer for join/decline tests.
func newJoinSvc(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	worldMgr, sessMgr := testWorldAndSession(t)
	logger := zaptest.NewLogger(t)
	npcMgr := npc.NewManager()
	roller := dice.NewRoller(dice.NewSource(42))
	combatHandler := NewCombatHandler(
		combat.NewEngine(), npcMgr, sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
	svc := NewGameServiceServer(
		worldMgr, sessMgr,
		command.DefaultRegistry(),
		NewWorldHandler(worldMgr, sessMgr, npcMgr, nil, nil, nil),
		NewChatHandler(sessMgr),
		logger,
		nil, roller, nil, npcMgr, combatHandler, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, "",
		nil, nil, nil,
		nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil,
		nil, nil,
	)
	return svc, sessMgr
}

// REQ-T6: handleJoin when PendingCombatJoin == "" returns "No combat to join."
func TestHandleJoin_NoPendingCombatJoin_ReturnsNoJoinMessage(t *testing.T) {
	svc, sessMgr := newJoinSvc(t)
	sess := sessMgr.NewSession()
	require.NotNil(t, sess)
	// PendingCombatJoin is "" by default.

	resp, err := svc.handleJoin(sess.UID, &gamev1.JoinRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, responseText(resp), "No combat to join")
}

// REQ-T3: handleDecline when PendingCombatJoin == "" returns "Nothing to decline."
func TestHandleDecline_NoPendingCombatJoin_ReturnsNothingToDecline(t *testing.T) {
	svc, sessMgr := newJoinSvc(t)
	sess := sessMgr.NewSession()
	require.NotNil(t, sess)

	resp, err := svc.handleDecline(sess.UID, &gamev1.DeclineRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, responseText(resp), "Nothing to decline")
}

// REQ-T3 (full): handleDecline when PendingCombatJoin != "" clears it and returns watch message.
func TestHandleDecline_WithPendingJoin_ClearsAndReturnsWatchMessage(t *testing.T) {
	svc, sessMgr := newJoinSvc(t)
	sess := sessMgr.NewSession()
	require.NotNil(t, sess)
	sess.PendingCombatJoin = "room-1"

	resp, err := svc.handleDecline(sess.UID, &gamev1.DeclineRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Contains(t, responseText(resp), "stay back")
	assert.Equal(t, "", sess.PendingCombatJoin, "PendingCombatJoin must be cleared")
}

// REQ-T2: handleJoin with valid PendingCombatJoin joins combat, sets status, clears pending.
func TestHandleJoin_WithPendingJoin_JoinsCombatAndClearsField(t *testing.T) {
	svc, sessMgr := newJoinSvc(t)
	sess := sessMgr.NewSession()
	require.NotNil(t, sess)

	// Start a combat in "room-1" with one NPC.
	npc1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 8}
	_, err := svc.combatH.engine.StartCombat("room-1", []*combat.Combatant{npc1},
		makeTestConditionRegistry(), nil, "")
	require.NoError(t, err)

	sess.PendingCombatJoin = "room-1"
	sess.CurrentHP = 20

	resp, err := svc.handleJoin(sess.UID, &gamev1.JoinRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "", sess.PendingCombatJoin, "PendingCombatJoin must be cleared after join")
	assert.Equal(t, statusInCombat, sess.Status, "Status must be statusInCombat after join")
}

// REQ-T-PROP (property, SWENG-5a): handleDecline always clears PendingCombatJoin regardless
// of the room ID string value.
func TestProperty_HandleDecline_AlwaysClearsPending(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sessMgr := newJoinSvc(t)
		sess := sessMgr.NewSession()
		if sess == nil {
			rt.Fatal("nil session")
		}
		roomID := rapid.StringMatching(`[a-z0-9-]+`).Draw(rt, "roomID")
		sess.PendingCombatJoin = roomID

		_, err := svc.handleDecline(sess.UID, &gamev1.DeclineRequest{})
		require.NoError(rt, err)
		require.Equal(rt, "", sess.PendingCombatJoin,
			"PendingCombatJoin must be empty after decline for any room ID")
	})
}
```

**Note:** `responseText` is a helper that extracts the text from a `*gamev1.ServerEvent`. Check `grpc_service_grapple_test.go` for whether this helper already exists; if not, add it in this file:
```go
func responseText(ev *gamev1.ServerEvent) string {
	if ev == nil {
		return ""
	}
	if msg, ok := ev.Payload.(*gamev1.ServerEvent_Message); ok {
		return msg.Message
	}
	return ""
}
```

**IMPORTANT:** Replace the `NewGameServiceServer` argument list in `newJoinSvc` with the exact parameter list from `newGrappleSvc` in `grpc_service_grapple_test.go` — copy it verbatim, do not guess. The constructor signature may differ.

Similarly, replace `sessMgr.NewSession()` with the actual session-creation helper used in existing tests.

- [ ] **Step 6: Run gameserver tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleJoin|TestHandleDecline|TestProperty_HandleDecline" -v 2>&1 | head -20
```

Expected: FAIL — `handleJoin`/`handleDecline` not wired yet.

- [ ] **Step 7: Check message helper names**

```bash
grep -n "func messageEvent\|func errorEvent\|func.*Event\b" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -10
```

Note the helper function names used to return text responses. Both `combatMu` and `broadcastFn` are unexported fields on `CombatHandler`, but since `GameServiceServer` and `CombatHandler` are both in `package gameserver`, `handleJoin` on `GameServiceServer` can access `s.combatH.combatMu` directly — no exported wrapper needed.

- [ ] **Step 8: Implement `handleJoin` in `grpc_service.go`**

```go
func (s *GameServiceServer) handleJoin(uid string, _ *gamev1.JoinRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("Session not found."), nil
	}

	s.combatH.combatMu.Lock()
	defer s.combatH.combatMu.Unlock()

	if sess.PendingCombatJoin == "" {
		return messageEvent("No combat to join."), nil
	}

	roomID := sess.PendingCombatJoin
	_, exists := s.combatH.engine.GetCombat(roomID)
	if !exists {
		sess.PendingCombatJoin = ""
		return messageEvent("The combat has ended."), nil
	}

	// Build player combatant — same pattern as startCombatLocked (lines 1547–1592).
	const dexMod = 1
	var playerAC int
	if s.combatH.invRegistry != nil {
		defStats := sess.Equipment.ComputedDefenses(s.combatH.invRegistry, dexMod)
		playerAC = 10 + defStats.ACBonus + defStats.EffectiveDex
	} else {
		playerAC = 10 + dexMod
	}

	playerCbt := &combat.Combatant{
		ID:        sess.UID,
		Kind:      combat.KindPlayer,
		Name:      sess.CharName,
		MaxHP:     sess.CurrentHP,
		CurrentHP: sess.CurrentHP,
		AC:        playerAC,
		Level:     1,
		StrMod:    2,
		DexMod:    dexMod,
	}

	s.combatH.loadoutsMu.Lock()
	if lo, ok := s.combatH.loadouts[uid]; ok {
		playerCbt.Loadout = lo
	}
	s.combatH.loadoutsMu.Unlock()

	weaponProfRank := "untrained"
	if playerCbt.Loadout != nil && playerCbt.Loadout.MainHand != nil && playerCbt.Loadout.MainHand.Def != nil {
		cat := playerCbt.Loadout.MainHand.Def.ProficiencyCategory
		if r, ok := sess.Proficiencies[cat]; ok {
			weaponProfRank = r
		}
	}
	playerCbt.WeaponProficiencyRank = weaponProfRank

	if playerCbt.Loadout != nil && playerCbt.Loadout.MainHand != nil && playerCbt.Loadout.MainHand.Def != nil {
		playerCbt.WeaponDamageType = playerCbt.Loadout.MainHand.Def.DamageType
	}

	playerCbt.Resistances = sess.Resistances
	playerCbt.Weaknesses = sess.Weaknesses
	playerCbt.GritMod = combat.AbilityMod(sess.Abilities.Grit)
	playerCbt.QuicknessMod = combat.AbilityMod(sess.Abilities.Quickness)
	playerCbt.SavvyMod = combat.AbilityMod(sess.Abilities.Savvy)
	playerCbt.ToughnessRank = combat.DefaultSaveRank(sess.Proficiencies["toughness"])
	playerCbt.HustleRank = combat.DefaultSaveRank(sess.Proficiencies["hustle"])
	playerCbt.CoolRank = combat.DefaultSaveRank(sess.Proficiencies["cool"])

	// Roll initiative. Must not hold engine.mu when calling AddCombatant.
	combat.RollInitiative([]*combat.Combatant{playerCbt}, s.combatH.dice.Src())

	if err := s.combatH.engine.AddCombatant(roomID, playerCbt); err != nil {
		return errorEvent("Failed to join combat."), nil
	}

	sess.Status = statusInCombat
	sess.PendingCombatJoin = ""

	// Broadcast join announcement to room.
	s.combatH.broadcastFn(roomID, []*gamev1.CombatEvent{{
		Type:      gamev1.CombatEventType_COMBAT_EVENT_TYPE_UNSPECIFIED,
		Narrative: fmt.Sprintf("%s joins the fight!", sess.CharName),
	}})

	return messageEvent(fmt.Sprintf("You join the combat (initiative %d).", playerCbt.Initiative)), nil
}

func (s *GameServiceServer) handleDecline(uid string, _ *gamev1.DeclineRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("Session not found."), nil
	}

	s.combatH.combatMu.Lock()
	defer s.combatH.combatMu.Unlock()

	if sess.PendingCombatJoin == "" {
		return messageEvent("Nothing to decline."), nil
	}

	sess.PendingCombatJoin = ""
	return messageEvent("You stay back and watch."), nil
}
```

Verify `messageEvent` exists in `grpc_service.go`. If not, use the existing pattern for returning text (e.g., `errorEvent` may be the only helper; add `messageEvent` if needed or use `errorEvent` for both and adjust test assertions).

- [ ] **Step 9: Wire into dispatch type switch (CMD-6)**

In `grpc_service.go`, in the `dispatch` type switch, add two cases:

```go
case *gamev1.ClientMessage_Join:
	return s.handleJoin(uid, p.Join)
case *gamev1.ClientMessage_Decline:
	return s.handleDecline(uid, p.Decline)
```

- [ ] **Step 10: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleJoin|TestHandleDecline|TestProperty_HandleDecline" -v
```

Expected: PASS

- [ ] **Step 11: Run full suite (CMD-7)**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 12: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/game/command/join.go internal/game/command/join_test.go internal/game/command/decline.go internal/game/command/decline_test.go internal/gameserver/grpc_service.go internal/gameserver/grpc_service_join_test.go && git commit -m "feat(combat): implement handleJoin and handleDecline gRPC handlers (CMD-3,6,7)"
```

---

## Chunk 3: Runtime Wiring and XP/Loot Split

### Task 6: Combat join trigger in handleMove + combat-end cleanup

**Spec ref:** Feature 2 — Trigger, Combat ends while player is pending
**Files:**
- Modify: `internal/gameserver/grpc_service.go` (handleMove, new helper)
- Modify: `internal/gameserver/grpc_service.go` or `combat_handler.go` (combat-end cleanup)
- Create: `internal/gameserver/grpc_service_move_join_test.go`

- [ ] **Step 1: Read how onCombatEndFn is set**

```bash
grep -n "onCombatEndFn\|SetOnCombatEnd\|CombatEnd\|combatEnd" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go | head -15
```

Note the exact line where the `onCombatEndFn` closure is set. The combat-end cleanup will be added to that closure.

- [ ] **Step 2: Read the session manager's iteration method**

```bash
grep -n "func.*Manager.*All\|func.*Manager.*Range\|func.*Manager.*Each\|func.*AllPlayers" /home/cjohannsen/src/mud/internal/game/session/manager.go | head -10
```

Note the exact method for iterating all player sessions. Use it in `clearPendingJoinForRoom`.

- [ ] **Step 3: Write failing tests**

Tests for `notifyCombatJoinIfEligible` call the helper directly — this avoids complex `handleMove` world routing setup while still validating the spec behavior.

Create `internal/gameserver/grpc_service_move_join_test.go`:

```go
package gameserver

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newMoveJoinSvc mirrors newJoinSvc — a minimal service for move/join trigger tests.
func newMoveJoinSvc(t *testing.T) (*GameServiceServer, *session.Manager) {
	t.Helper()
	return newJoinSvc(t) // reuse the same constructor
}

// REQ-T1: notifyCombatJoinIfEligible sets PendingCombatJoin when room has active combat.
func TestNotifyCombatJoinIfEligible_ActiveCombat_SetsPendingJoin(t *testing.T) {
	svc, sessMgr := newMoveJoinSvc(t)
	sess := sessMgr.NewSession()
	require.NotNil(t, sess)

	// Start a combat in "room-1" with one NPC.
	npc1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 8}
	_, err := svc.combatH.engine.StartCombat("room-1", []*combat.Combatant{npc1},
		makeTestConditionRegistry(), nil, "")
	require.NoError(t, err)

	svc.notifyCombatJoinIfEligible(sess, "room-1")

	assert.Equal(t, "room-1", sess.PendingCombatJoin,
		"PendingCombatJoin must be set to the room with active combat")
}

// REQ-T13: Player already in combat → notifyCombatJoinIfEligible does nothing.
func TestNotifyCombatJoinIfEligible_PlayerAlreadyInCombat_NoChange(t *testing.T) {
	svc, sessMgr := newMoveJoinSvc(t)
	sess := sessMgr.NewSession()
	require.NotNil(t, sess)
	sess.Status = statusInCombat // player is already in combat

	npc1 := &combat.Combatant{ID: "n1", Kind: combat.KindNPC, Name: "Ganger",
		MaxHP: 10, CurrentHP: 10, AC: 12, Level: 1, Initiative: 8}
	_, err := svc.combatH.engine.StartCombat("room-2", []*combat.Combatant{npc1},
		makeTestConditionRegistry(), nil, "")
	require.NoError(t, err)

	svc.notifyCombatJoinIfEligible(sess, "room-2")

	assert.Equal(t, "", sess.PendingCombatJoin, "Player already in combat must not receive join prompt")
}

// REQ-T1 (no combat): notifyCombatJoinIfEligible does nothing when room has no active combat.
func TestNotifyCombatJoinIfEligible_NoCombat_NoChange(t *testing.T) {
	svc, sessMgr := newMoveJoinSvc(t)
	sess := sessMgr.NewSession()
	require.NotNil(t, sess)

	svc.notifyCombatJoinIfEligible(sess, "empty-room")

	assert.Equal(t, "", sess.PendingCombatJoin, "No combat in room — PendingCombatJoin must remain empty")
}
```

Also add a property-based test (SWENG-5a):

```go
// REQ-T-PROP (property, SWENG-5a): clearPendingJoinForRoom always leaves no session
// with PendingCombatJoin == roomID, regardless of how many sessions are pending.
func TestProperty_ClearPendingJoin_AlwaysClearsMatchingRoom(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		svc, sessMgr := newJoinSvc(t)
		roomID := rapid.StringMatching(`[a-z0-9-]+`).Draw(rt, "roomID")
		otherRoom := rapid.StringMatching(`[A-Z0-9]+`).Draw(rt, "otherRoom") // distinct from roomID
		n := rapid.IntRange(0, 5).Draw(rt, "n")
		for i := 0; i < n; i++ {
			sess := sessMgr.NewSession()
			if sess == nil {
				continue
			}
			// Alternate: some pending for roomID, some for otherRoom.
			if i%2 == 0 {
				sess.PendingCombatJoin = roomID
			} else {
				sess.PendingCombatJoin = otherRoom
			}
		}

		svc.clearPendingJoinForRoom(roomID)

		// After clear, no session should have PendingCombatJoin == roomID.
		for _, sess := range sessMgr.AllPlayers() {
			require.NotEqual(rt, roomID, sess.PendingCombatJoin,
				"clearPendingJoinForRoom must clear all sessions pending for this room")
		}
	})
}
```

**Note:** Replace `sessMgr.AllPlayers()` with the actual iteration method discovered in Step 2.

- [ ] **Step 4: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestNotifyCombatJoin|TestProperty_ClearPendingJoin" -v 2>&1 | head -20
```

Expected: FAIL.

- [ ] **Step 5: Add `notifyCombatJoinIfEligible` helper to `grpc_service.go`**

**Locking note:** The spec describes the trigger as running "while `combatMu` is held," but to avoid deadlock (handleMove may already hold other locks), `notifyCombatJoinIfEligible` acquires `combatMu` itself. Both `GameServiceServer` and `CombatHandler` are in `package gameserver`, so unexported fields are accessible directly.

```go
// notifyCombatJoinIfEligible sets PendingCombatJoin and sends a join prompt if
// the player has just entered a room with active combat and is eligible to join.
//
// Precondition: sess != nil; called after a successful move; combatH.combatMu NOT held.
// Note: acquires combatH.combatMu itself (intentional deviation from spec's "held by caller"
// to avoid deadlock in handleMove's call chain).
func (s *GameServiceServer) notifyCombatJoinIfEligible(sess *session.PlayerSession, newRoomID string) {
	if sess.Status == statusInCombat {
		return
	}

	s.combatH.combatMu.Lock()
	defer s.combatH.combatMu.Unlock()

	cbt, exists := s.combatH.engine.GetCombat(newRoomID)
	if !exists {
		return
	}

	// Check player is not already a combatant.
	for _, c := range cbt.Combatants {
		if c.ID == sess.UID {
			return
		}
	}

	// Overwrite any previous pending join (no notification for overridden offer — REQ-T20).
	sess.PendingCombatJoin = newRoomID

	// Push join prompt to player's entity stream.
	evt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: "Active combat in progress. Join the fight? (join / decline)",
			},
		},
	}
	data, err := proto.Marshal(evt)
	if err != nil {
		return
	}
	if pushErr := sess.Entity.Push(data); pushErr != nil && s.logger != nil {
		s.logger.Warn("pushing combat join prompt", zap.String("uid", sess.UID), zap.Error(pushErr))
	}
}
```

- [ ] **Step 6: Call `notifyCombatJoinIfEligible` in `handleMove`**

In `handleMove`, just before the final `return &gamev1.ServerEvent{...}, nil` (around line 1444), add:

```go
// Combat join trigger — prompt the player if entering a room with active combat.
s.notifyCombatJoinIfEligible(sess, result.View.RoomId)
```

- [ ] **Step 7: Add `clearPendingJoinForRoom` and wire into combat-end closure**

Add to `grpc_service.go`:

```go
// clearPendingJoinForRoom notifies and clears PendingCombatJoin for all players
// waiting to join the combat in roomID (called when that combat ends).
func (s *GameServiceServer) clearPendingJoinForRoom(roomID string) {
	// Use the session manager's iteration method found in Step 2.
	// Replace AllPlayers() below with the actual method name if different.
	for _, sess := range s.sessions.AllPlayers() {
		if sess.PendingCombatJoin == roomID {
			sess.PendingCombatJoin = ""
			evt := &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_Message{
					Message: &gamev1.MessageEvent{
						Content: "The combat has ended.",
					},
				},
			}
			data, err := proto.Marshal(evt)
			if err == nil {
				_ = sess.Entity.Push(data)
			}
		}
	}
}
```

Find where `onCombatEndFn` is assigned (from Step 1) and extend the closure to call `clearPendingJoinForRoom`. For example, if the existing code is:

```go
s.combatH.SetOnCombatEnd(func(roomID string) {
	// existing logic...
})
```

Extend it to:

```go
s.combatH.SetOnCombatEnd(func(roomID string) {
	// existing logic...
	s.clearPendingJoinForRoom(roomID)
})
```

- [ ] **Step 8: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestNotifyCombatJoin|TestProperty_ClearPendingJoin" -v
```

Expected: PASS

- [ ] **Step 9: Run full suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 10: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_move_join_test.go && git commit -m "feat(combat): add combat join trigger in handleMove; clear pending joins on combat end"
```

---

### Task 7: XP/currency/item split in removeDeadNPCsLocked

**Spec ref:** Feature 3 — XP split, Currency split, Item split
**Files:**
- Modify: `internal/gameserver/combat_handler.go`
- Create: `internal/gameserver/combat_handler_split_test.go`

**Before starting:** Read `internal/gameserver/combat_handler.go` lines 2138–2270 (`removeDeadNPCsLocked`) and the existing XP broadcast code carefully. Note:
- The existing currency award uses `firstLivingPlayer(cbt)` — these calls will be replaced.
- The existing XP award uses `h.xpSvc.AwardKill(...)` — this will be replaced with `h.xpSvc.AwardXPAmount(...)`.
- `inst.Name()` returns the NPC's display name for messages.
- XP messages are pushed via `sess.Entity.Push(...)` not `broadcastFn` — check lines 2209–2240 for the exact push pattern and replicate it.

- [ ] **Step 1: Write failing tests**

Read `internal/gameserver/grpc_service_grapple_test.go` for the full combat setup pattern, then create `internal/gameserver/combat_handler_split_test.go`:

```go
package gameserver

import (
	"context"
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/xp"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newMinimalCombatHandler builds a CombatHandler with no savers (saves skipped when nil).
func newMinimalCombatHandler(t *testing.T) *CombatHandler {
	t.Helper()
	_, sessMgr := testWorldAndSession(t)
	roller := dice.NewRoller(dice.NewSource(42))
	return NewCombatHandler(
		combat.NewEngine(), npc.NewManager(), sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
}

// REQ-T12 (example): Single participant receives full currency — no split dilution.
func TestSplit_SingleParticipant_CurrencyUnchanged(t *testing.T) {
	h := newMinimalCombatHandler(t)
	p1 := &session.PlayerSession{UID: "p1", Currency: 0}

	h.distributeCurrencyLocked(context.Background(), []*session.PlayerSession{p1}, 42)

	require.Equal(t, 42, p1.Currency, "single participant must receive all currency")
}

// REQ-T7 (example): Two players; NPC level 2 at KillXPPerNPCLevel=50 → totalXP=100; each gets 50.
// Uses AwardXPAmount directly (the production code path for split awards).
func TestSplit_TwoPlayers_XPSplit(t *testing.T) {
	xpSvc := xp.NewService(testXPConfig(), &grantXPProgressSaver{})
	p1 := &session.PlayerSession{UID: "p1", Experience: 0, Level: 1, MaxHP: 20, CurrentHP: 20}
	p2 := &session.PlayerSession{UID: "p2", Experience: 0, Level: 1, MaxHP: 20, CurrentHP: 20}

	cfg := xpSvc.Config()
	totalXP := 2 * cfg.Awards.KillXPPerNPCLevel // npcLevel=2
	share := totalXP / 2

	_, err := xpSvc.AwardXPAmount(context.Background(), p1, 0, share)
	require.NoError(t, err)
	_, err = xpSvc.AwardXPAmount(context.Background(), p2, 0, share)
	require.NoError(t, err)

	require.Equal(t, share, p1.Experience, "p1 must receive floor(totalXP/2)")
	require.Equal(t, share, p2.Experience, "p2 must receive floor(totalXP/2)")
}

// REQ-T8 (example): Two players, 10 currency → each receives 5.
func TestSplit_TwoPlayers_CurrencySplit(t *testing.T) {
	h := newMinimalCombatHandler(t)
	p1 := &session.PlayerSession{UID: "p1", Currency: 0}
	p2 := &session.PlayerSession{UID: "p2", Currency: 0}

	h.distributeCurrencyLocked(context.Background(), []*session.PlayerSession{p1, p2}, 10)

	require.Equal(t, 5, p1.Currency, "p1 must receive floor(10/2)=5")
	require.Equal(t, 5, p2.Currency, "p2 must receive floor(10/2)=5")
}

// REQ-T18 (example): 3 players, 2 currency → share=0 fallback: first gets 1, others get 0.
func TestSplit_Currency_ShareZeroFallback(t *testing.T) {
	h := newMinimalCombatHandler(t)
	p1 := &session.PlayerSession{UID: "p1", Currency: 0}
	p2 := &session.PlayerSession{UID: "p2", Currency: 0}
	p3 := &session.PlayerSession{UID: "p3", Currency: 0}

	h.distributeCurrencyLocked(context.Background(), []*session.PlayerSession{p1, p2, p3}, 2)

	require.Equal(t, 1, p1.Currency, "first participant gets 1 in share=0 fallback")
	require.Equal(t, 0, p2.Currency, "second participant gets 0 in share=0 fallback")
	require.Equal(t, 0, p3.Currency, "third participant gets 0 in share=0 fallback")
}

// REQ-T10 (property): Each share is floor(totalCurrency/n); no one gets more than share.
// Tests distributeCurrencyLocked directly with synthetic sessions.
func TestProperty_Split_Currency_EqualShare(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		totalCurrency := rapid.IntRange(0, 10000).Draw(rt, "totalCurrency")
		expectedShare := totalCurrency / n

		// Build n synthetic player sessions with zero currency.
		participants := make([]*session.PlayerSession, n)
		for i := range participants {
			participants[i] = &session.PlayerSession{UID: fmt.Sprintf("p%d", i), Currency: 0}
		}

		// Build a minimal CombatHandler (no saver — currency save skipped when nil).
		_, sessMgr := testWorldAndSession(t)
		roller := dice.NewRoller(dice.NewSource(42))
		h := NewCombatHandler(
			combat.NewEngine(), npc.NewManager(), sessMgr, roller,
			func(_ string, _ []*gamev1.CombatEvent) {},
			testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
		)

		h.distributeCurrencyLocked(context.Background(), participants, totalCurrency)

		if totalCurrency == 0 {
			for _, p := range participants {
				require.Equal(rt, 0, p.Currency, "zero totalCurrency: no participant gains anything")
			}
			return
		}

		if expectedShare == 0 {
			// Only first participant gets 1; rest stay at 0.
			require.Equal(rt, 1, participants[0].Currency,
				"share=0 fallback: first participant gets 1")
			for _, p := range participants[1:] {
				require.Equal(rt, 0, p.Currency, "share=0 fallback: other participants get 0")
			}
			return
		}

		for _, p := range participants {
			require.Equal(rt, expectedShare, p.Currency,
				"each participant must receive exactly floor(totalCurrency/n)")
		}
	})
}

// REQ-T17 (property): Each participant receives exactly floor(totalXP/n) XP via AwardXPAmount.
// When share==0 and totalXP>0, first participant gets 1; others get 0.
func TestProperty_Split_XP_EqualShare(t *testing.T) {
	cfg := testXPConfig()
	rapid.Check(t, func(rt *rapid.T) {
		npcLevel := rapid.IntRange(1, 5).Draw(rt, "npcLevel")
		n := rapid.IntRange(1, 5).Draw(rt, "n")

		totalXP := npcLevel * cfg.Awards.KillXPPerNPCLevel
		share := totalXP / n

		// Build n sessions starting at 0 XP.
		participants := make([]*session.PlayerSession, n)
		for i := range participants {
			participants[i] = &session.PlayerSession{
				UID: fmt.Sprintf("p%d", i), Experience: 0, Level: 1, MaxHP: 20, CurrentHP: 20,
			}
		}

		xpSvc := xp.NewService(cfg, &grantXPProgressSaver{})

		if share == 0 && totalXP > 0 {
			// Award 1 XP to first participant only.
			_, err := xpSvc.AwardXPAmount(context.Background(), participants[0], 0, 1)
			require.NoError(rt, err)
			require.Equal(rt, 1, participants[0].Experience,
				"first participant must receive 1 XP in share=0 fallback")
			for _, p := range participants[1:] {
				require.Equal(rt, 0, p.Experience,
					"other participants must receive 0 XP in share=0 fallback")
			}
		} else {
			for _, p := range participants {
				_, err := xpSvc.AwardXPAmount(context.Background(), p, 0, share)
				require.NoError(rt, err)
			}
			for _, p := range participants {
				require.Equal(rt, share, p.Experience,
					"each participant must receive exactly floor(totalXP/n) XP")
			}
		}
	})
}
```

**IMPORTANT:** Replace all comment-only bodies with real test implementations using the package helpers.

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestSplit|TestProperty_Split" -v 2>&1 | head -20
```

Expected: FAIL — split logic not implemented.

- [ ] **Step 3: Add `livingParticipantSessions` to `combat_handler.go`**

```go
// livingParticipantSessions returns []*session.PlayerSession for all combat participants
// whose Dead field is false, in initiative-descending order (same as cbt.Combatants).
//
// Dead player combatants remain in cbt.Combatants with Dead==true (only NPCs are removed
// by removeDeadNPCsLocked). The Dead flag filter correctly excludes them.
// A player with CurrentHP==0 but Dead==false (dying state) IS included.
//
// Caller must hold combatMu.
func (h *CombatHandler) livingParticipantSessions(cbt *combat.Combat) []*session.PlayerSession {
	participantSet := make(map[string]bool, len(cbt.Participants))
	for _, uid := range cbt.Participants {
		participantSet[uid] = true
	}
	var result []*session.PlayerSession
	for _, c := range cbt.Combatants {
		if !participantSet[c.ID] || c.Dead {
			continue
		}
		if sess, ok := h.sessions.GetPlayer(c.ID); ok {
			result = append(result, sess)
		}
	}
	return result
}
```

- [ ] **Step 4: Add `distributeCurrencyLocked` to `combat_handler.go`**

```go
// distributeCurrencyLocked distributes totalCurrency equally among livingParticipants.
// When share == 0 (more participants than currency units), only the first participant
// receives 1 unit — no currency is created from nothing (REQ-CURR3).
// SaveCurrency errors are logged as warnings and do not propagate.
//
// Caller must hold combatMu.
func (h *CombatHandler) distributeCurrencyLocked(ctx context.Context, livingParticipants []*session.PlayerSession, totalCurrency int) {
	if totalCurrency == 0 || len(livingParticipants) == 0 {
		return
	}
	share := totalCurrency / len(livingParticipants)
	if share == 0 {
		// Award 1 unit to first participant only.
		livingParticipants[0].Currency++
		if h.currencySaver != nil {
			if err := h.currencySaver.SaveCurrency(ctx, livingParticipants[0].CharacterID, livingParticipants[0].Currency); err != nil && h.logger != nil {
				h.logger.Warn("SaveCurrency failed (share=0 fallback)",
					zap.String("uid", livingParticipants[0].UID),
					zap.Error(err),
				)
			}
		}
		return
	}
	for _, p := range livingParticipants {
		p.Currency += share
		if h.currencySaver != nil {
			if err := h.currencySaver.SaveCurrency(ctx, p.CharacterID, p.Currency); err != nil && h.logger != nil {
				h.logger.Warn("SaveCurrency failed",
					zap.String("uid", p.UID),
					zap.Error(err),
				)
			}
		}
	}
}
```

- [ ] **Step 5: Rework XP award block in `removeDeadNPCsLocked`**

Find the existing XP block (around lines 2204–2240) and replace the `if h.xpSvc != nil { ... }` block with:

```go
// Award split XP to all living participants (REQ-XP4: use AwardXPAmount, NOT AwardKill).
if h.xpSvc != nil {
	cfg := h.xpSvc.Config()
	livingParticipants := h.livingParticipantSessions(cbt)
	if len(livingParticipants) > 0 {
		totalXP := inst.Level * cfg.Awards.KillXPPerNPCLevel
		share := totalXP / len(livingParticipants)
		if share == 0 && totalXP > 0 {
			// Award 1 XP to first participant only; no XP created from nothing.
			xpMsgs, xpErr := h.xpSvc.AwardXPAmount(context.Background(), livingParticipants[0], livingParticipants[0].CharacterID, 1)
			if xpErr != nil && h.logger != nil {
				h.logger.Warn("AwardXPAmount failed", zap.Error(xpErr))
			}
			if xpErr == nil {
				// Push XP messages to player — replicate the existing push pattern from lines 2215–2235.
				h.pushXPMessages(livingParticipants[0], xpMsgs, 1, inst.Name())
			}
		} else {
			for _, p := range livingParticipants {
				xpMsgs, xpErr := h.xpSvc.AwardXPAmount(context.Background(), p, p.CharacterID, share)
				if xpErr != nil && h.logger != nil {
					h.logger.Warn("AwardXPAmount failed", zap.Error(xpErr))
				}
				if xpErr == nil {
					h.pushXPMessages(p, xpMsgs, share, inst.Name())
				}
			}
		}
	}
}
```

Also remove the now-dead `xpAmount` local variable at the old line 2208 (it was `xpAmount := inst.Level * cfg.Awards.KillXPPerNPCLevel`).

Add `pushXPMessages` helper (replicates the existing single-player XP push pattern exactly):

```go
// pushXPMessages sends XP narrative messages to sess after an AwardXPAmount call.
// Mirrors the XP push block in removeDeadNPCsLocked: grant announcement + level-up msgs
// + CharacterInfo update when a level-up occurred.
func (h *CombatHandler) pushXPMessages(sess *session.PlayerSession, levelMsgs []string, xpAmount int, npcName string) {
	xpGrantEvt := &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_Message{
			Message: &gamev1.MessageEvent{
				Content: fmt.Sprintf("You gain %d XP for killing %s.", xpAmount, npcName),
				Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
			},
		},
	}
	if data, marshalErr := proto.Marshal(xpGrantEvt); marshalErr == nil {
		_ = sess.Entity.Push(data)
	}
	for _, msg := range levelMsgs {
		xpEvt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Message{
				Message: &gamev1.MessageEvent{
					Content: msg,
					Type:    gamev1.MessageType_MESSAGE_TYPE_UNSPECIFIED,
				},
			},
		}
		if data, marshalErr := proto.Marshal(xpEvt); marshalErr == nil {
			_ = sess.Entity.Push(data)
		}
	}
	if len(levelMsgs) > 0 {
		ciEvt := &gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_CharacterInfo{
				CharacterInfo: &gamev1.CharacterInfo{
					CurrentHp: int32(sess.CurrentHP),
					MaxHp:     int32(sess.MaxHP),
				},
			},
		}
		if data, marshalErr := proto.Marshal(ciEvt); marshalErr == nil {
			_ = sess.Entity.Push(data)
		}
	}
}
```

- [ ] **Step 6: Rework currency award blocks in `removeDeadNPCsLocked`**

**Loot-table path** — find the block that starts `if inst.Loot != nil {` and replace the currency section:

```go
// Split currency among living participants.
livingParticipants := h.livingParticipantSessions(cbt)
totalCurrency := result.Currency + inst.Currency
inst.Currency = 0
h.distributeCurrencyLocked(context.Background(), livingParticipants, totalCurrency)
```

Remove the old `if totalCurrency > 0 { if killer := h.firstLivingPlayer(cbt); ... }` block.

**No-loot-table path** — find the `else if inst.Currency > 0 {` block and replace:

```go
livingParticipants := h.livingParticipantSessions(cbt)
totalCurrency := inst.Currency
inst.Currency = 0
h.distributeCurrencyLocked(context.Background(), livingParticipants, totalCurrency)
```

- [ ] **Step 7: Rework item distribution in `removeDeadNPCsLocked`**

The existing code floor-drops all items via `h.floorMgr.Drop` — there is no per-player backpack grant mechanism. For multiplayer, items continue to be floor-dropped (all participants can pick them up). No change to the item loop is required; the only change is that `livingParticipants` is now computed above for currency, so the existing floor-drop code can remain as-is.

Verify the existing drop code is still present after your currency edits:

```bash
grep -n "floorMgr.Drop\|result.Items" /home/cjohannsen/src/mud/internal/gameserver/combat_handler.go | head -10
```

If it was accidentally removed during currency edits, restore it:

```go
// Drop items on the room floor (all participants can pick them up).
if h.floorMgr != nil {
	for _, lootItem := range result.Items {
		h.floorMgr.Drop(roomID, inventory.ItemInstance{
			InstanceID: lootItem.InstanceID,
			ItemDefID:  lootItem.ItemDefID,
			Quantity:   lootItem.Quantity,
		})
	}
}
```

- [ ] **Step 8: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestSplit|TestProperty_Split" -v
```

Expected: PASS.

- [ ] **Step 9: Run full suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | tail -10
```

Expected: all packages PASS.

- [ ] **Step 10: Commit**

```bash
cd /home/cjohannsen/src/mud && git add internal/gameserver/combat_handler.go internal/gameserver/combat_handler_split_test.go && git commit -m "feat(combat): split XP/currency/items among living participants; add livingParticipantSessions and distributeCurrencyLocked"
```

---

## Final Verification

- [ ] **Run full test suite one last time**

```bash
cd /home/cjohannsen/src/mud && go test ./... -count=1 2>&1 | tail -20
```

Expected: all packages PASS, zero failures.

- [ ] **Verify CMD-1–7 completeness for both join and decline**

```bash
# CMD-1,2: constants + BuiltinCommands
grep -n "HandlerJoin\|HandlerDecline" /home/cjohannsen/src/mud/internal/game/command/commands.go

# CMD-3: handle functions exist
ls /home/cjohannsen/src/mud/internal/game/command/join.go /home/cjohannsen/src/mud/internal/game/command/decline.go

# CMD-4: proto messages wired
grep -n "JoinRequest\|DeclineRequest" /home/cjohannsen/src/mud/api/proto/game/v1/game.proto

# CMD-5: bridge handlers registered
grep -n "bridgeJoin\|bridgeDecline" /home/cjohannsen/src/mud/internal/frontend/handlers/bridge_handlers.go

# CMD-6,7: gRPC handlers wired
grep -n "handleJoin\|handleDecline\|ClientMessage_Join\|ClientMessage_Decline" /home/cjohannsen/src/mud/internal/gameserver/grpc_service.go
```

All expected to return matches.
