# Exploration Challenges — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `Challenge` content type that composes with the existing `SkillChecks` / `Triggers` / `Outcomes` / `Hazards` infrastructure. Authoring lives in `content/exploration/challenges/<id>.yaml`. Activation builds a per-character session, prompts a menu (telnet numbered list / web modal), resolves the chosen option through a shared DoS resolver (with #252's `skillaction.DoS`), applies effects (existing `damage`/`condition`/`deny`/`reveal` + new `grant_item`/`grant_credits`/`grant_xp`/`advance_stage`/`unlock_exit`/`complete_challenge`), and persists one-shot completion to a per-character ledger so non-repeatable challenges do not re-fire. Existing `SkillChecks` triggers stay untouched.

**Spec:** [docs/superpowers/specs/2026-04-25-exploration-challenges.md](https://github.com/cory-johannsen/mud/blob/main/docs/superpowers/specs/2026-04-25-exploration-challenges.md) (PR [#282](https://github.com/cory-johannsen/mud/pull/282))

**Architecture:** Three layers. (1) Content + loader (`internal/game/exploration/challenge/def.go`) — YAML schemas validated at startup against the item registry, condition catalog, and Mud skill set. (2) Service (`service.go`) — `Activate`/`Resolve`/`Cancel` with deterministic session IDs scoped per character per challenge; `Resolve` reuses #252's DoS computation (extract to a shared package if not already shared) and dispatches effects through both the existing skillcheck-effect pipeline and a new effect-set introduced here. (3) Persistence (`store.go` + a new `migrations/NNN_character_completed_challenges.up.sql`/`down.sql` pair) — composite-keyed `(character_id, challenge_id)` records keep the one-shot ledger. UX is a numbered menu on telnet (`challenge_handler.go`) and an in-flow modal on web (`ChallengeModal.tsx`). The `dc.ForZoneTier(tier, difficulty)` helper lives in the existing zone-difficulty package and is consulted at activation time when an option declares `dc.kind: zone_tier`.

**Tech Stack:** Go (`internal/game/exploration/challenge/`, `internal/gameserver/`), `pgregory.net/rapid` (property tests for the DoS edges + ledger round-trip), protobuf (`api/proto/game/v1/game.proto`), Postgres migrations, telnet handlers, React/TypeScript (`cmd/webclient/ui/src/game/exploration/`).

**Prerequisite:** None hard. #252 is a soft dep — the DoS computation is shared. If #252 has not landed when implementation begins, this plan defines the DoS function in `internal/game/skillaction/dos.go` (single shared function used by both ticket families). The 2026-04-19 zone-difficulty-scaling spec is a soft dep — `dc.ForZoneTier` is added in that package; if it does not exist yet, this plan creates it.

**Note on spec PR**: The spec for this issue is on PR #282 and is NOT yet merged to `main`. The plan PR depends on the spec PR landing first. When PR #282 is revised, re-validate the plan's task list against the latest spec before implementation begins.

---

## File Map

| Action | Path |
|--------|------|
| Create | `internal/game/exploration/challenge/def.go` |
| Create | `internal/game/exploration/challenge/def_test.go` |
| Create | `internal/game/exploration/challenge/service.go` |
| Create | `internal/game/exploration/challenge/service_test.go` |
| Create | `internal/game/exploration/challenge/store.go` |
| Create | `internal/game/exploration/challenge/store_test.go` |
| Create | `internal/game/exploration/challenge/effect.go` |
| Create | `internal/game/exploration/challenge/effect_test.go` |
| Create | `internal/game/skillaction/dos.go` (shared DoS function — only if #252 has not landed) |
| Modify | `internal/game/zone/difficulty.go` (`dc.ForZoneTier` helper) |
| Modify | `internal/game/zone/difficulty_test.go` |
| Modify | `internal/gameserver/grpc_service.go` (`ListChallenges`, `ActivateChallenge`, `ResolveChallenge` RPCs; activate-on-room-enter wiring) |
| Create | `internal/gameserver/grpc_service_explore.go` |
| Create | `internal/gameserver/grpc_service_explore_test.go` |
| Modify | `api/proto/game/v1/game.proto` (`ChallengeView`, `ChallengeStageView`, `ChallengeOptionView`, `ChallengeOutcomeView`, `ActivateChallengeRequest`, `ResolveChallengeRequest`) |
| Create | `migrations/NNN_character_completed_challenges.up.sql` |
| Create | `migrations/NNN_character_completed_challenges.down.sql` |
| Create | `migrations/NNN_character_unlocked_exits.up.sql` (only if EXC-29 persistence ships) |
| Create | `migrations/NNN_character_unlocked_exits.down.sql` |
| Create | `internal/frontend/telnet/challenge_handler.go` |
| Create | `internal/frontend/telnet/challenge_handler_test.go` |
| Create | `cmd/webclient/ui/src/game/exploration/ChallengeModal.tsx` |
| Create | `cmd/webclient/ui/src/game/exploration/ChallengeModal.test.tsx` |
| Create | `content/exploration/challenges/` (3 exemplar YAML files) |
| Modify | `docs/architecture/exploration.md` (or new `docs/architecture/challenges.md` — section per EXC-31) |

---

### Task 1: Content schema + YAML loader

**Files:**
- Create: `internal/game/exploration/challenge/def.go`
- Create: `internal/game/exploration/challenge/def_test.go`

- [ ] **Step 1: Failing tests** for the schema (EXC-1, EXC-2, EXC-3, EXC-5):

```go
func TestLoadChallenge_TwoStageWithRewards(t *testing.T) {
    def, err := challenge.Load([]byte(twoStageYAML))
    require.NoError(t, err)
    require.Equal(t, "warehouse_locks", def.ID)
    require.False(t, def.Repeatable)
    require.Len(t, def.Stages, 2)
    s0 := def.Stages[0]
    require.Len(t, s0.Options, 3, "Athletics OR Acrobatics OR Engineering Lore")
    require.Equal(t, challenge.OutcomeCritSuccess, def.Stages[0].Outcomes[0].DoS)
    require.IsType(t, challenge.GrantItem{}, def.Stages[1].Outcomes[0].Effects[0])
}

func TestLoadChallenge_RejectsUnknownAdvanceStageRef(t *testing.T) {
    _, err := challenge.Load([]byte(`
id: bogus
stages:
  - id: stage_one
    options:
      - { id: o, label: x, skill: muscle, dc: { kind: fixed, value: 15 } }
    outcomes:
      success:
        - advance_stage: { stage_id: nope }
`))
    require.Error(t, err)
    require.Contains(t, err.Error(), "advance_stage references unknown stage 'nope'")
}

func TestLoadChallenge_RejectsUnknownItem(t *testing.T) {
    _, err := challenge.Load(yamlWithGrantItem("not_a_real_item"))
    require.Error(t, err)
    require.Contains(t, err.Error(), "not_a_real_item")
}

func TestLoadChallenge_RejectsUnknownSkill(t *testing.T) {
    _, err := challenge.Load(yamlWithSkill("definitely_not_a_skill"))
    require.Error(t, err)
}

func TestLoadChallenge_ImplicitTerminationWhenNoAdvanceOrComplete(t *testing.T) {
    def, err := challenge.Load([]byte(oneStageNoTerminator))
    require.NoError(t, err)
    require.True(t, def.Stages[0].Outcomes[challenge.Success].ImplicitTerminate, "EXC-4 implicit termination")
}
```

- [ ] **Step 2: Implement** the schema:

```go
type Definition struct {
    ID          string
    DisplayName string
    Description string
    Trigger     Trigger
    Repeatable  bool
    Stages      []*Stage
}

type Stage struct {
    ID          string
    Narrative   string
    AutoResolve bool
    Options     []*Option
    Outcomes    map[DoS]*OutcomeBlock
}

type Option struct {
    ID         string
    Label      string
    Skill      string
    DC         DC      // shared shape with skillaction.DC; reuse the type
    Difficulty Difficulty
    APCost     int
}

type OutcomeBlock struct {
    Effects           []Effect
    ImplicitTerminate bool
}

type Effect interface{ effect() }

type Damage      struct{ Expr string }
type Condition   struct{ ID string; DurationRounds int }
type Deny        struct{ Reason string }
type Reveal      struct{ Count int }
type GrantItem   struct{ ItemID string; Quantity int }
type GrantCredits struct{ Amount int }
type GrantXP     struct{ Amount int }
type AdvanceStage struct{ StageID string }
type UnlockExit  struct{ Direction string; Persist bool }
type CompleteChallenge struct{}
```

- [ ] **Step 3: Loader validation** per EXC-5: every `advance_stage.stage_id` resolves; every `unlock_exit.direction` resolves to a real room exit (looked up at content-load against room metadata); every `grant_item.item_id` resolves; every `skill` resolves.

- [ ] **Step 4: Loader entry point** reads `content/exploration/challenges/*.yaml` and returns `map[string]*Definition`. Cached at startup.

---

### Task 2: Per-character completion ledger + migration

**Files:**
- Create: `migrations/NNN_character_completed_challenges.up.sql`
- Create: `migrations/NNN_character_completed_challenges.down.sql`
- Create: `internal/game/exploration/challenge/store.go`
- Create: `internal/game/exploration/challenge/store_test.go`

- [ ] **Step 1: Determine the next migration number** by listing `migrations/` and incrementing past the current head.

- [ ] **Step 2: Author the migration** (EXC-20, EXC-21):

```sql
-- migrations/NNN_character_completed_challenges.up.sql
CREATE TABLE character_completed_challenges (
    character_id TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    challenge_id TEXT NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, challenge_id)
);
CREATE INDEX character_completed_challenges_by_char ON character_completed_challenges(character_id);
```

```sql
-- migrations/NNN_character_completed_challenges.down.sql
DROP TABLE IF EXISTS character_completed_challenges;
```

- [ ] **Step 3: Failing tests** for the repository (EXC-22, EXC-23):

```go
func TestStore_RoundTrip(t *testing.T) {
    s := newPGStore(t)
    require.False(t, mustBool(s.IsCompleted("c1", "warehouse_locks")))
    require.NoError(t, s.MarkCompleted("c1", "warehouse_locks"))
    require.True(t, mustBool(s.IsCompleted("c1", "warehouse_locks")))
}

func TestStore_IsCompletedScopedPerCharacter(t *testing.T) {
    s := newPGStore(t)
    require.NoError(t, s.MarkCompleted("c1", "warehouse_locks"))
    require.False(t, mustBool(s.IsCompleted("c2", "warehouse_locks")))
}

func TestStore_MarkCompletedIdempotent(t *testing.T) {
    s := newPGStore(t)
    require.NoError(t, s.MarkCompleted("c1", "x"))
    require.NoError(t, s.MarkCompleted("c1", "x"), "duplicate write must not error")
}
```

- [ ] **Step 4: Implement** the store:

```go
type Store interface {
    IsCompleted(charID, challengeID string) (bool, error)
    MarkCompleted(charID, challengeID string) error
}

type PGStore struct{ db *sql.DB }

func (s *PGStore) IsCompleted(charID, challengeID string) (bool, error) {
    var n int
    err := s.db.QueryRow(`SELECT 1 FROM character_completed_challenges WHERE character_id=$1 AND challenge_id=$2`, charID, challengeID).Scan(&n)
    if err == sql.ErrNoRows { return false, nil }
    return err == nil, err
}

func (s *PGStore) MarkCompleted(charID, challengeID string) error {
    _, err := s.db.Exec(`INSERT INTO character_completed_challenges (character_id, challenge_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, charID, challengeID)
    return err
}
```

- [ ] **Step 5:** Migration applies and rolls back cleanly on a fresh test database (acceptance bullet).

---

### Task 3: Zone-tier DC helper

**Files:**
- Modify: `internal/game/zone/difficulty.go`
- Modify: `internal/game/zone/difficulty_test.go`

- [ ] **Step 1: Failing tests** (EXC-24..26):

```go
func TestForZoneTier_Standard(t *testing.T) {
    require.Equal(t, 14, dc.ForZoneTier(1, dc.Standard))
    require.Equal(t, 19, dc.ForZoneTier(5, dc.Standard))
}

func TestForZoneTier_DifficultyOffsets(t *testing.T) {
    base := dc.ForZoneTier(5, dc.Standard)
    require.Equal(t, base-2, dc.ForZoneTier(5, dc.Easy))
    require.Equal(t, base+2, dc.ForZoneTier(5, dc.Hard))
    require.Equal(t, base+5, dc.ForZoneTier(5, dc.Severe))
    require.Equal(t, base+10, dc.ForZoneTier(5, dc.Extreme))
}
```

- [ ] **Step 2: Implement** with the PF2E "DCs by Level" mapping table baked in. Tier 0 → DC 14; tier 1 → DC 15; … per the published table. Difficulty offsets per spec.

- [ ] **Step 3: Document** the table inline (one comment block) so future readers can see the source row.

---

### Task 4: `Resolve` + `Activate` + `Cancel` service

**Files:**
- Create: `internal/game/exploration/challenge/service.go`
- Create: `internal/game/exploration/challenge/service_test.go`
- Create: `internal/game/skillaction/dos.go` (only if not yet shared; otherwise import)

- [ ] **Step 1: Decide DoS source.** If `internal/game/skillaction/dos.go` does not yet exist, this task creates it (the same function used by spec #252). Otherwise import.

```go
// internal/game/skillaction/dos.go
package skillaction

func DoS(roll, bonus, dc int) DegreeOfSuccess { /* ±10 band + nat 1/20 step */ }
```

- [ ] **Step 2: Failing service tests** (EXC-7, EXC-8, EXC-9, EXC-10, EXC-11):

```go
func TestActivate_AutoResolveSilentlyRolls(t *testing.T) {
    svc := newSvc(t, withChallenge(autoResolveOneOption))
    sess, err := svc.Activate(ctx, "char-1", "auto_one")
    require.NoError(t, err)
    require.True(t, sess.AutoResolved, "auto_resolve stages must skip the menu (EXC-9)")
    require.NotNil(t, sess.Outcome)
}

func TestResolve_FailingRollAppliesEffectsAndTerminates(t *testing.T) {
    svc := newSvc(t, withChallenge(simpleAthleticsClimb))
    sess, _ := svc.Activate(ctx, "char-1", "fence")
    out, _ := svc.Resolve(ctx, sess.ID, "athletics", withRoll(2))
    require.Equal(t, skillaction.Failure, out.DoS)
    require.True(t, sess.Terminated, "implicit termination on outcome with no advance_stage (EXC-4)")
}

func TestResolve_AdvanceStageMovesToNamedStage(t *testing.T) {
    svc := newSvc(t, withChallenge(twoStageWithAdvance))
    sess, _ := svc.Activate(ctx, "char-1", "warehouse_locks")
    svc.Resolve(ctx, sess.ID, "athletics", withRoll(20))
    require.Equal(t, "stage_two", sess.CurrentStageID)
}

func TestResolve_NonRepeatableMarksLedgerOnComplete(t *testing.T) {
    svc, store := newSvcWithStore(t, withChallenge(oneShotChallenge))
    sess, _ := svc.Activate(ctx, "char-1", "one_shot")
    svc.Resolve(ctx, sess.ID, "athletics", withRoll(20))
    completed, _ := store.IsCompleted("char-1", "one_shot")
    require.True(t, completed, "non-repeatable challenge writes ledger on complete (EXC-11)")
}

func TestActivate_SkipsCompletedNonRepeatable(t *testing.T) {
    svc, store := newSvcWithStore(t, withChallenge(oneShotChallenge))
    store.MarkCompleted("char-1", "one_shot")
    _, err := svc.Activate(ctx, "char-1", "one_shot")
    require.ErrorIs(t, err, challenge.ErrAlreadyCompleted, "EXC-23 skip when ledger says completed")
}

func TestCancel_RemovesSessionAndApplesNoEffects(t *testing.T) {
    svc := newSvc(t, withChallenge(simpleAthleticsClimb))
    sess, _ := svc.Activate(ctx, "char-1", "fence")
    require.NoError(t, svc.Cancel(ctx, sess.ID))
    _, err := svc.Resolve(ctx, sess.ID, "athletics", withRoll(20))
    require.ErrorIs(t, err, challenge.ErrSessionNotFound)
}

func TestActivate_RepeatableNeverConsultsLedger(t *testing.T) {
    svc, store := newSvcWithStore(t, withChallenge(repeatableHaggle))
    store.MarkCompleted("char-1", "haggle") // sneaky write
    sess, err := svc.Activate(ctx, "char-1", "haggle")
    require.NoError(t, err, "repeatable: true ignores the ledger")
    require.NotNil(t, sess)
}
```

- [ ] **Step 3: Implement** the service. Sessions live in an in-memory map keyed by a deterministic ID (`fmt.Sprintf("%s:%s", charID, challengeID)` — single session per char-challenge at a time). The service composes:

```go
type Service struct {
    defs        map[string]*Definition
    dice        dice.Roller
    xpSvc       XPService
    creditSvc   CreditService
    items       ItemRegistry
    conditions  ConditionCatalog
    completions Store
    sessions    map[string]*Session
    mu          sync.Mutex
}

func (s *Service) Activate(ctx context.Context, charID, challengeID string) (*Session, error) { ... }
func (s *Service) Resolve(ctx context.Context, sessionID, optionID string, dice dice.Roller) (Outcome, error) { ... }
func (s *Service) Cancel(ctx context.Context, sessionID string) error { ... }
```

`Session` carries `ID`, `CharID`, `ChallengeID`, `CurrentStageID`, `AutoResolved bool`, `Terminated bool`, `Outcome *Outcome`.

- [ ] **Step 4: Auto-cancel on combat / disconnect / room exit (EXC-10).** The service exposes a `CancelAllForCharacter(charID)` method called by:
  - `internal/gameserver/combat_handler.go` on combat start.
  - `handleMove` after the player leaves the room.
  - the disconnection handler.

---

### Task 5: Effect dispatch — new effect types

**Files:**
- Create: `internal/game/exploration/challenge/effect.go`
- Create: `internal/game/exploration/challenge/effect_test.go`

- [ ] **Step 1: Failing tests** (EXC-3 effect types, EXC-27 unlocks, EXC-29 persist flag):

```go
func TestEffect_GrantItemDispatchesToInventory(t *testing.T) {
    e := captureEffectsApplied(t, &challenge.GrantItem{ItemID: "lockpick", Quantity: 2})
    require.Equal(t, []invSvcCall{{ItemID: "lockpick", Qty: 2}}, e)
}

func TestEffect_GrantCreditsAddsToWallet(t *testing.T) {
    e := captureCreditsApplied(t, &challenge.GrantCredits{Amount: 50})
    require.Equal(t, 50, e)
}

func TestEffect_GrantXPOverridesImplicitAward(t *testing.T) {
    e := captureXPApplied(t, &challenge.GrantXP{Amount: 100})
    require.Equal(t, 100, e)
}

func TestEffect_UnlockExitSessionScopedByDefault(t *testing.T) {
    sess := newSession()
    challenge.ApplyEffect(sess, &challenge.UnlockExit{Direction: "north"})
    require.Contains(t, sess.UnlockedExits, "north")
    require.False(t, sess.PersistedUnlocks["north"], "session-scoped by default (EXC-29)")
}

func TestEffect_UnlockExitWithPersistTrueWritesToTable(t *testing.T) {
    persists := captureUnlockPersists(t, &challenge.UnlockExit{Direction: "north", Persist: true})
    require.Equal(t, []unlockPersist{{Direction: "north"}}, persists)
}
```

- [ ] **Step 2: Implement** an `ApplyEffect(sess, effect Effect, ctx ResolveContext)` dispatch that switches on effect type. Existing effect types route to the existing skillcheck-effect dispatch (`damage`, `condition`, `deny`, `reveal`); new types route to the dedicated services.

- [ ] **Step 3: Implement `unlocked_exits` session state** and `character_unlocked_exits` migration if EXC-29 persistence ships:

```sql
-- migrations/NNN_character_unlocked_exits.up.sql
CREATE TABLE character_unlocked_exits (
    character_id TEXT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    room_id      TEXT NOT NULL,
    direction    TEXT NOT NULL,
    unlocked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (character_id, room_id, direction)
);
```

- [ ] **Step 4: Movement validation** — `handleMove` consults both session state and `character_unlocked_exits` when checking a normally-locked exit:

```go
func canTraverse(sess *Session, charID, roomID, direction string) bool {
    if sess.UnlockedExits[direction] { return true }
    return store.IsExitUnlocked(charID, roomID, direction)
}
```

Tests cover both session and persist paths.

---

### Task 6: gRPC RPCs + activation pipeline

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Create: `internal/gameserver/grpc_service_explore.go`
- Modify: `internal/gameserver/grpc_service.go` (room-enter wiring)
- Create: `internal/gameserver/grpc_service_explore_test.go`

- [ ] **Step 1: Add proto messages**:

```proto
message ChallengeView {
  string id = 1;
  string display_name = 2;
  string description = 3;
  ChallengeStageView current_stage = 4;
  bool auto_resolved = 5;
  ChallengeOutcomeView terminal_outcome = 6;
}

message ChallengeStageView {
  string id = 1;
  string narrative = 2;
  repeated ChallengeOptionView options = 3;
}

message ChallengeOptionView {
  string id = 1;
  string label = 2;
  string skill = 3;
  string dc_summary = 4;
  int32  ap_cost = 5;
}

message ChallengeOutcomeView {
  int32 roll = 1;
  int32 bonus = 2;
  int32 dc = 3;
  string dos = 4;
  repeated string narrative_lines = 5;
}

message ActivateChallengeRequest { string character_id = 1; string challenge_id = 2; }
message ActivateChallengeResponse { ChallengeView view = 1; }

message ResolveChallengeRequest { string session_id = 1; string option_id = 2; }
message ResolveChallengeResponse { ChallengeView view = 1; }
```

- [ ] **Step 2: Wire room-enter activation** (EXC-30 keeps `applyRoomSkillChecks` unchanged):

```go
func (s *Service) onEnterRoom(ctx context.Context, char *Character, room *Room) {
    s.applyRoomSkillChecks(ctx, char, room) // existing
    s.resolveRoomChallenges(ctx, char, room)  // NEW
}

func (s *Service) resolveRoomChallenges(ctx context.Context, char *Character, room *Room) {
    for _, ch := range s.challenges.ForRoom(room.ID) {
        sess, err := s.challengeSvc.Activate(ctx, char.ID, ch.ID)
        if errors.Is(err, challenge.ErrAlreadyCompleted) {
            continue // EXC-23
        }
        if err != nil {
            log.Error().Err(err).Msg("challenge activate failed")
            continue
        }
        if sess.AutoResolved {
            continue // outcome already applied
        }
        s.broadcastChallengeView(char, sess)
    }
}
```

- [ ] **Step 3: Failing handler tests**:

```go
func TestActivateChallenge_ReturnsCurrentStageView(t *testing.T) { ... }
func TestResolveChallenge_AppliesEffectsAndAdvances(t *testing.T) { ... }
func TestRoomEnter_QueuesChallengesByYAMLDeclarationOrder(t *testing.T) {
    // EXC-Q2 — strict serial ordering.
    ...
}
```

- [ ] **Step 4: Two-challenge queueing.** When the same room has two challenges, the second `Activate` MUST wait until the first session is resolved or cancelled. Implement via a per-character pending queue.

---

### Task 7: Telnet UX — numbered menu + 60s timeout

**Files:**
- Create: `internal/frontend/telnet/challenge_handler.go`
- Create: `internal/frontend/telnet/challenge_handler_test.go`

- [ ] **Step 1: Failing tests** (EXC-12..15):

```go
func TestChallengeMenu_RendersNumberedOptions(t *testing.T) {
    h := newHandlerWithChallenge(t, twoOptionChallenge)
    out := h.SimulateActivation()
    require.Contains(t, out, "Challenge: Warehouse Locks")
    require.Contains(t, out, "1) Climb the fence (Athletics)")
    require.Contains(t, out, "2) Hack the lock (Engineering Lore)")
    require.Contains(t, out, "0) walk away")
}

func TestChallengeMenu_AcceptsNumberOrID(t *testing.T) {
    h := newHandlerWithChallenge(t, twoOptionChallenge)
    h.SimulateActivation()
    h.Run("1")
    require.Equal(t, "athletics", h.LastResolvedOption())
    h.SimulateActivation()
    h.Run("engineering")
    require.Equal(t, "engineering_lore", h.LastResolvedOption())
}

func TestChallengeMenu_CancelOnZeroOrCancel(t *testing.T) {
    for _, in := range []string{"0", "cancel"} {
        h := newHandlerWithChallenge(t, twoOptionChallenge)
        h.SimulateActivation()
        h.Run(in)
        require.True(t, h.SessionCancelled())
    }
}

func TestChallengeMenu_TimeoutCancels(t *testing.T) {
    h := newHandlerWithChallenge(t, twoOptionChallenge)
    h.SimulateActivation()
    h.AdvanceTime(61 * time.Second)
    require.True(t, h.SessionCancelled())
    require.Contains(t, h.LastOutput(), "You hesitate too long.")
}

func TestChallengeMenu_OutcomeIncludesRollDetails(t *testing.T) {
    h := newHandlerWithChallenge(t, twoOptionChallenge, withDeterministicDice(17))
    h.SimulateActivation()
    h.Run("1")
    require.Regexp(t, `Roll 17 \+ \d+ = \d+ vs DC \d+ → (success|critical success|failure|critical failure)`, h.LastOutput())
}
```

- [ ] **Step 2: Implement** the per-session menu state machine. The 60-second timer is reset on each Activate; combat/move during the wait dispatches `CancelAllForCharacter`.

---

### Task 8: Web UX — `ChallengeModal`

**Files:**
- Create: `cmd/webclient/ui/src/game/exploration/ChallengeModal.tsx`
- Create: `cmd/webclient/ui/src/game/exploration/ChallengeModal.test.tsx`

- [ ] **Step 1: Failing component tests** (EXC-16..19):

```ts
test("modal renders banner, narrative, option buttons", () => {
  render(<ChallengeModal view={twoOptionView} />);
  expect(screen.getByText("Warehouse Locks")).toBeVisible();
  expect(screen.getByRole("button", { name: /Climb the fence/i })).toBeVisible();
  expect(screen.getByRole("button", { name: /Hack the lock/i })).toBeVisible();
});

test("option button shows DC summary and AP-cost badge", () => {
  render(<ChallengeModal view={withAPCostView} />);
  expect(screen.getByText("Athletics DC 18")).toBeVisible();
  expect(screen.getByText("AP 1")).toBeVisible();
});

test("cancel button always present and dispatches cancel", () => {
  const onCancel = jest.fn();
  render(<ChallengeModal view={twoOptionView} onCancel={onCancel} />);
  fireEvent.click(screen.getByRole("button", { name: /walk away/i }));
  expect(onCancel).toHaveBeenCalled();
});

test("show roll toggle reveals roll detail panel", () => {
  render(<ChallengeModal view={resolvedView} />);
  expect(screen.queryByText("Roll detail")).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: /Show roll/i }));
  expect(screen.getByText(/Roll 17/)).toBeVisible();
});

test("modal closes after terminal outcome", async () => {
  const { rerender } = render(<ChallengeModal view={twoOptionView} />);
  rerender(<ChallengeModal view={terminalOutcomeView} />);
  await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
});
```

- [ ] **Step 2: Implement** the modal with a Redux/Zustand slice for `activeChallengeSessionID`. The outcome panel collapses by default; the auto-close timer fires 1 second after the terminal outcome lands.

- [ ] **Step 3:** Accessible focus management — `aria-modal`, `role="dialog"`, escape key dispatches cancel, focus trapped on the option buttons until resolution.

---

### Task 9: Exemplar content + telemetry + docs

**Files:**
- Create: `content/exploration/challenges/warehouse_locks.yaml` (two-stage one-shot)
- Create: `content/exploration/challenges/cracked_pipe.yaml` (one-stage two-options repeatable)
- Create: `content/exploration/challenges/whisper_drop.yaml` (single stage with branching outcomes)
- Modify: `internal/game/exploration/challenge/service.go` (telemetry hook per EXC-32)
- Modify: `docs/architecture/exploration.md` (or new `docs/architecture/challenges.md`)

- [ ] **Step 1: Author the three exemplars** per EXC-6:

```yaml
# content/exploration/challenges/warehouse_locks.yaml
id: warehouse_locks
display_name: Warehouse Locks
description: A pair of stubborn padlocks bar the side door.
trigger:
  room_id: industrial_warehouse_entry
  event: on_enter
repeatable: false
stages:
  - id: stage_one
    narrative: The first lock. Heavy and old — could be rust, could be magic.
    options:
      - { id: athletics,    label: Force it open (Athletics),         skill: muscle,     dc: { kind: zone_tier }, difficulty: standard }
      - { id: engineering,  label: Pick the lock (Engineering Lore),  skill: hustle,     dc: { kind: zone_tier }, difficulty: hard }
      - { id: stealth_back, label: Slip around back (Stealth),        skill: smooth_talk, dc: { kind: zone_tier }, difficulty: hard }
    outcomes:
      success:
        - advance_stage: { stage_id: stage_two }
      crit_success:
        - advance_stage: { stage_id: stage_two }
        - grant_item: { item_id: scavenged_padlock, quantity: 1 }
      failure:
        - condition: { id: clumsy, duration_rounds: 3 }
      crit_failure:
        - damage: { expr: "1d4" }
        - condition: { id: clumsy, duration_rounds: 5 }
  - id: stage_two
    narrative: The second lock. Newer. The mechanism rattles.
    options:
      - { id: athletics,   label: Snap the chain (Athletics), skill: muscle, dc: { kind: zone_tier }, difficulty: hard }
      - { id: engineering, label: Pick the lock (Engineering Lore), skill: hustle, dc: { kind: zone_tier }, difficulty: standard }
    outcomes:
      success:
        - unlock_exit: { direction: side }
        - grant_credits: { amount: 50 }
        - complete_challenge
      crit_success:
        - unlock_exit: { direction: side, persist: true }
        - grant_credits: { amount: 100 }
        - grant_xp: { amount: 20 }
        - complete_challenge
      failure:
        - condition: { id: frustrated, duration_rounds: 3 }
      crit_failure:
        - damage: { expr: "1d6" }
```

(Plus `cracked_pipe.yaml` and `whisper_drop.yaml` exercising the other two patterns.)

- [ ] **Step 2: Telemetry hook (EXC-32)** — at every `Activate` / option pick / outcome / completion, emit one structured log line:

```go
log.Info().
    Str("character_id", charID).
    Str("challenge_id", challengeID).
    Str("stage_id", stageID).
    Str("option_id", optionID).
    Int("roll", roll).
    Int("dc", dc).
    Str("dos", dos.String()).
    Strs("outcome_effects", effectIDs).
    Msg("challenge.event")
```

- [ ] **Step 3: Architecture doc** (EXC-31) — section explaining when to use `SkillChecks` triggers vs Challenges, the activation flow, the effect type vocabulary, and the migration path. Cross-link the spec, this plan, the three exemplars, and `internal/game/exploration/challenge/`.

---

## Verification

Per SWENG-6, the full test suite MUST pass before commit / PR:

```
go test ./...
( cd cmd/webclient/ui && pnpm test )
make migrate-up && make migrate-down  # both directions clean
```

Additional sanity:

- `go vet ./...` clean.
- `make proto` re-runs cleanly with no diff.
- Telnet smoke test: enter the warehouse-locks room, verify the menu appears; pick option 1 with low dice; verify the failure narrative + condition; re-enter the room with the challenge marked completed; verify it does not re-fire.
- Web smoke test: same scenario; verify the modal renders, AP-cost badge appears on the AP-costing option, escape cancels, the Show Roll toggle reveals the roll detail.
- Migration smoke test: apply on a fresh database; insert a row; roll back; verify the table is gone.

---

## Rollout / Open Questions Resolved at Plan Time

- **EXC-Q1**: Cancel is free in v1. Losing the reward is enough disincentive.
- **EXC-Q2**: Two challenges in one room queue in YAML declaration order. Per-character pending queue.
- **EXC-Q3**: No diminishing returns on repeatable challenges in v1. Authors who want this declare multiple stages.
- **EXC-Q4**: `unlock_exit` is per-character. Mirrors PF2E's "you climbed up but the rope came down with you" convention.
- **EXC-Q5**: "Global" / shared completion deferred. Future ticket.

## Non-Goals Reaffirmed

Per spec §2.2:

- Existing `SkillChecks` triggers stay untouched.
- No branching graphs beyond single-fork-per-stage.
- No real-time / wall-clock timers.
- No multi-character cooperation.
- No combat-mode integration.
- No authoring GUI.
