# Skill Advancement Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Players advance skill proficiency ranks (untrained→trained→expert→master→legendary) by spending skill increases awarded at every even character level.

**Architecture:** A `pending_skill_increases` column on `characters` tracks unspent increases; the XP service computes and persists new increases at level-up (even levels); a new `trainskill` command (full CMD-1–7 pipeline) lets players spend increases, with level gates enforced server-side. Mirrors the existing ability boost pattern exactly.

**Tech Stack:** Go, PostgreSQL (pgx/v5), protobuf, pgregory.net/rapid (property-based tests)

---

## Background — Key Existing Patterns

Before starting, read these files to understand the patterns you must mirror:

- **Ability boosts** (the model to copy): `internal/storage/postgres/character_progress.go`, `internal/game/xp/xp.go`, `internal/game/xp/service.go`, `internal/gameserver/grpc_service.go:3208-3284` (`handleLevelUp`)
- **Skills repo**: `internal/storage/postgres/character_skills.go`
- **Bridge pattern**: `internal/frontend/handlers/bridge_handlers.go:709-723` (`bridgeLevelUp`)
- **Command handler pattern**: `internal/game/command/levelup.go`

## Rank constants and level gates

```
untrained → trained   (no level gate)
trained   → expert    (character level ≥ 15)
expert    → master    (character level ≥ 35)
master    → legendary (character level ≥ 75)
```

---

### Task 1: Migration + Progress Repo

**Files:**
- Create: `migrations/021_pending_skill_increases.up.sql`
- Create: `migrations/021_pending_skill_increases.down.sql`
- Modify: `internal/storage/postgres/character_progress.go`
- Modify: `internal/storage/postgres/character_progress_test.go`
- Modify: `internal/storage/postgres/main_test.go`

**Step 1: Create migration files**

`migrations/021_pending_skill_increases.up.sql`:
```sql
ALTER TABLE characters ADD COLUMN IF NOT EXISTS pending_skill_increases INTEGER NOT NULL DEFAULT 0;
```

`migrations/021_pending_skill_increases.down.sql`:
```sql
ALTER TABLE characters DROP COLUMN IF EXISTS pending_skill_increases;
```

**Step 2: Add the column to `applyAllMigrations` in the test helper**

In `internal/storage/postgres/main_test.go`, find the `applyAllMigrations` function (it's a large SQL block applied in `TestMain`). Add to the end:

```sql
ALTER TABLE characters ADD COLUMN IF NOT EXISTS pending_skill_increases INTEGER NOT NULL DEFAULT 0;
```

**Step 3: Write failing tests**

In `internal/storage/postgres/character_progress_test.go`, add:

```go
func TestIncrementPendingSkillIncreases_RoundTrip(t *testing.T) {
    pool := sharedPool(t)
    ch := setupCharReposShared(t, pool)
    repo := NewCharacterProgressRepository(pool)
    ctx := context.Background()

    require.NoError(t, repo.IncrementPendingSkillIncreases(ctx, ch.ID, 3))
    n, err := repo.GetPendingSkillIncreases(ctx, ch.ID)
    require.NoError(t, err)
    assert.Equal(t, 3, n)
}

func TestConsumePendingSkillIncrease_Decrements(t *testing.T) {
    pool := sharedPool(t)
    ch := setupCharReposShared(t, pool)
    repo := NewCharacterProgressRepository(pool)
    ctx := context.Background()

    require.NoError(t, repo.IncrementPendingSkillIncreases(ctx, ch.ID, 2))
    require.NoError(t, repo.ConsumePendingSkillIncrease(ctx, ch.ID))
    n, err := repo.GetPendingSkillIncreases(ctx, ch.ID)
    require.NoError(t, err)
    assert.Equal(t, 1, n)
}

func TestConsumePendingSkillIncrease_NoneAvailable_ReturnsError(t *testing.T) {
    pool := sharedPool(t)
    ch := setupCharReposShared(t, pool)
    repo := NewCharacterProgressRepository(pool)
    ctx := context.Background()

    err := repo.ConsumePendingSkillIncrease(ctx, ch.ID)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "no pending skill increases")
}

func TestPropertyIncrementPendingSkillIncreases(t *testing.T) {
    pool := sharedPool(t)
    repo := NewCharacterProgressRepository(pool)
    ctx := context.Background()

    rapid.Check(t, func(rt *rapid.T) {
        ch := setupCharReposShared(rt, pool)
        n := rapid.IntRange(1, 10).Draw(rt, "n")
        require.NoError(rt, repo.IncrementPendingSkillIncreases(ctx, ch.ID, n))
        got, err := repo.GetPendingSkillIncreases(ctx, ch.ID)
        require.NoError(rt, err)
        if got != n {
            rt.Fatalf("expected %d, got %d", n, got)
        }
    })
}
```

Run: `go test ./internal/storage/postgres/... -run TestIncrementPendingSkillIncreases -v -timeout 300s`
Expected: FAIL (method not defined)

**Step 4: Implement the three methods**

Add to `internal/storage/postgres/character_progress.go`:

```go
// GetPendingSkillIncreases returns the number of unspent skill increases for a character.
//
// Precondition: id > 0.
// Postcondition: Returns the current pending_skill_increases value (0 if character not found).
func (r *CharacterProgressRepository) GetPendingSkillIncreases(ctx context.Context, id int64) (int, error) {
    if id <= 0 {
        return 0, fmt.Errorf("characterID must be > 0, got %d", id)
    }
    var n int
    err := r.pool.QueryRow(ctx,
        `SELECT pending_skill_increases FROM characters WHERE id = $1`, id,
    ).Scan(&n)
    if err != nil {
        return 0, fmt.Errorf("GetPendingSkillIncreases: %w", err)
    }
    return n, nil
}

// IncrementPendingSkillIncreases adds n to the character's pending skill increases.
//
// Precondition: id > 0; n >= 1.
// Postcondition: pending_skill_increases increased by n.
func (r *CharacterProgressRepository) IncrementPendingSkillIncreases(ctx context.Context, id int64, n int) error {
    if id <= 0 {
        return fmt.Errorf("characterID must be > 0, got %d", id)
    }
    if n < 1 {
        return fmt.Errorf("n must be >= 1, got %d", n)
    }
    _, err := r.pool.Exec(ctx,
        `UPDATE characters SET pending_skill_increases = pending_skill_increases + $2 WHERE id = $1`,
        id, n,
    )
    if err != nil {
        return fmt.Errorf("IncrementPendingSkillIncreases: %w", err)
    }
    return nil
}

// ConsumePendingSkillIncrease decrements pending_skill_increases by 1.
// Returns an error containing "no pending skill increases" if none are available.
//
// Precondition: id > 0.
// Postcondition: pending_skill_increases decremented by 1, or error returned.
func (r *CharacterProgressRepository) ConsumePendingSkillIncrease(ctx context.Context, id int64) error {
    if id <= 0 {
        return fmt.Errorf("characterID must be > 0, got %d", id)
    }
    tag, err := r.pool.Exec(ctx,
        `UPDATE characters SET pending_skill_increases = pending_skill_increases - 1
         WHERE id = $1 AND pending_skill_increases > 0`,
        id,
    )
    if err != nil {
        return fmt.Errorf("ConsumePendingSkillIncrease: %w", err)
    }
    if tag.RowsAffected() == 0 {
        return errors.New("no pending skill increases available for character")
    }
    return nil
}
```

**Step 5: Run tests**

```
go test ./internal/storage/postgres/... -timeout 300s -count=1
```
Expected: all pass.

**Step 6: Commit**

```bash
git add migrations/021_pending_skill_increases.up.sql migrations/021_pending_skill_increases.down.sql \
    internal/storage/postgres/character_progress.go \
    internal/storage/postgres/character_progress_test.go \
    internal/storage/postgres/main_test.go
git commit -m "feat: migration 021 and pending skill increases repo methods"
```

---

### Task 2: UpgradeSkill Repository Method

**Files:**
- Modify: `internal/storage/postgres/character_skills.go`
- Modify: `internal/storage/postgres/character_skills_test.go`

**Step 1: Write failing test**

In `internal/storage/postgres/character_skills_test.go`:

```go
func TestUpgradeSkill_SetsRank(t *testing.T) {
    pool := sharedPool(t)
    ch := setupCharReposShared(t, pool)
    repo := NewCharacterSkillsRepository(pool)
    ctx := context.Background()

    // Set initial rank
    require.NoError(t, repo.SetAll(ctx, ch.ID, map[string]string{"parkour": "untrained"}))

    // Upgrade
    require.NoError(t, repo.UpgradeSkill(ctx, ch.ID, "parkour", "trained"))

    skills, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    assert.Equal(t, "trained", skills["parkour"])
}

func TestUpgradeSkill_InsertsIfMissing(t *testing.T) {
    pool := sharedPool(t)
    ch := setupCharReposShared(t, pool)
    repo := NewCharacterSkillsRepository(pool)
    ctx := context.Background()

    // No existing row
    require.NoError(t, repo.UpgradeSkill(ctx, ch.ID, "muscle", "trained"))

    skills, err := repo.GetAll(ctx, ch.ID)
    require.NoError(t, err)
    assert.Equal(t, "trained", skills["muscle"])
}

func TestPropertyUpgradeSkill_RoundTrip(t *testing.T) {
    pool := sharedPool(t)
    repo := NewCharacterSkillsRepository(pool)
    ctx := context.Background()
    ranks := []string{"untrained", "trained", "expert", "master", "legendary"}

    rapid.Check(t, func(rt *rapid.T) {
        ch := setupCharReposShared(rt, pool)
        rank := rapid.SampledFrom(ranks).Draw(rt, "rank")
        require.NoError(rt, repo.UpgradeSkill(ctx, ch.ID, "parkour", rank))
        skills, err := repo.GetAll(ctx, ch.ID)
        require.NoError(rt, err)
        if skills["parkour"] != rank {
            rt.Fatalf("expected %q got %q", rank, skills["parkour"])
        }
    })
}
```

Run: `go test ./internal/storage/postgres/... -run TestUpgradeSkill -v -timeout 300s`
Expected: FAIL (method not defined)

**Step 2: Implement UpgradeSkill**

Add to `internal/storage/postgres/character_skills.go`:

```go
// UpgradeSkill upserts a single skill rank for a character.
//
// Precondition: characterID > 0; skillID and newRank must be non-empty.
// Postcondition: character_skills row for (characterID, skillID) exists with proficiency = newRank.
func (r *CharacterSkillsRepository) UpgradeSkill(ctx context.Context, characterID int64, skillID, newRank string) error {
    if characterID <= 0 {
        return fmt.Errorf("characterID must be > 0, got %d", characterID)
    }
    if skillID == "" || newRank == "" {
        return fmt.Errorf("skillID and newRank must be non-empty")
    }
    _, err := r.db.Exec(ctx, `
        INSERT INTO character_skills (character_id, skill_id, proficiency)
        VALUES ($1, $2, $3)
        ON CONFLICT (character_id, skill_id) DO UPDATE SET proficiency = EXCLUDED.proficiency
    `, characterID, skillID, newRank)
    if err != nil {
        return fmt.Errorf("UpgradeSkill: %w", err)
    }
    return nil
}
```

**Step 3: Run tests**

```
go test ./internal/storage/postgres/... -timeout 300s -count=1
```
Expected: all pass.

**Step 4: Commit**

```bash
git add internal/storage/postgres/character_skills.go internal/storage/postgres/character_skills_test.go
git commit -m "feat: UpgradeSkill repository method"
```

---

### Task 3: XP Service — NewSkillIncreases

**Files:**
- Modify: `internal/game/xp/xp.go`
- Modify: `internal/game/xp/xp_test.go`
- Modify: `internal/game/xp/service.go`
- Modify: `internal/game/xp/service_test.go`

**Step 1: Write failing tests for Award**

In `internal/game/xp/xp_test.go`, add:

```go
func TestAward_NewSkillIncreases_EvenLevels(t *testing.T) {
    cfg := &XPConfig{BaseXP: 100, LevelCap: 100, HPPerLevel: 5, BoostInterval: 5}
    // Level 1→2: one even level crossed → 1 skill increase
    result := Award(1, 0, 400, cfg) // XPToLevel(2,100)=400
    assert.Equal(t, 1, result.NewSkillIncreases, "crossing level 2 (even) should grant 1 increase")

    // Level 1→3: levels 2 and 3 crossed → only level 2 is even → 1 increase
    result = Award(1, 0, 900, cfg) // XPToLevel(3,100)=900
    assert.Equal(t, 1, result.NewSkillIncreases)

    // Level 1→4: levels 2,3,4 → even: 2,4 → 2 increases
    result = Award(1, 0, 1600, cfg)
    assert.Equal(t, 2, result.NewSkillIncreases)

    // No level-up → 0 increases
    result = Award(5, 2500, 1, cfg)
    assert.Equal(t, 0, result.NewSkillIncreases)
}

func TestProperty_NewSkillIncreases_EvenCount(t *testing.T) {
    cfg := &XPConfig{BaseXP: 100, LevelCap: 100, HPPerLevel: 5, BoostInterval: 5}
    rapid.Check(t, func(rt *rapid.T) {
        level := rapid.IntRange(1, 50).Draw(rt, "level")
        xp := XPToLevel(level, cfg.BaseXP)
        levelsToGain := rapid.IntRange(1, 5).Draw(rt, "gain")
        targetLevel := level + levelsToGain
        if targetLevel > cfg.LevelCap {
            return
        }
        awardXP := XPToLevel(targetLevel, cfg.BaseXP) - xp + 1
        result := Award(level, xp, awardXP, cfg)

        expected := 0
        for l := level + 1; l <= result.NewLevel; l++ {
            if l%2 == 0 {
                expected++
            }
        }
        if result.NewSkillIncreases != expected {
            rt.Fatalf("level %d→%d: expected %d skill increases, got %d",
                level, result.NewLevel, expected, result.NewSkillIncreases)
        }
    })
}
```

Run: `go test ./internal/game/xp/... -run TestAward_NewSkillIncreases -v`
Expected: FAIL (field NewSkillIncreases not defined)

**Step 2: Add NewSkillIncreases to AwardResult and Award**

In `internal/game/xp/xp.go`:

Add `NewSkillIncreases int` field to `AwardResult`:
```go
// NewSkillIncreases is the number of new skill increases earned this award (one per even level gained).
NewSkillIncreases int
```

In `Award()`, after the `newBoosts` calculation block:
```go
newSkillIncreases := 0
for l := level + 1; l <= newLevel; l++ {
    if l%2 == 0 {
        newSkillIncreases++
    }
}
```

Add to the return struct:
```go
NewSkillIncreases: newSkillIncreases,
```

**Step 3: Run tests**

```
go test ./internal/game/xp/... -timeout 60s -count=1
```
Expected: all pass.

**Step 4: Write failing service test**

In `internal/game/xp/service_test.go`, add tests that assert `sess.PendingSkillIncreases` is incremented on even level-up. Look at existing tests in this file for the pattern to follow (mock `ProgressSaver`).

```go
type mockSkillIncreaseSaver struct {
    calls []int // n values passed
}

func (m *mockSkillIncreaseSaver) IncrementPendingSkillIncreases(ctx context.Context, id int64, n int) error {
    m.calls = append(m.calls, n)
    return nil
}

func TestAwardKill_IncrementsSkillIncreasesOnEvenLevel(t *testing.T) {
    cfg := defaultTestConfig() // use whatever helper exists in this test file
    saver := &mockProgressSaver{}
    skillSaver := &mockSkillIncreaseSaver{}
    svc := NewService(cfg, saver)
    svc.SetSkillIncreaseSaver(skillSaver)

    sess := &session.PlayerSession{Level: 1, Experience: 0, MaxHP: 10, CurrentHP: 10, PendingSkillIncreases: 0}
    // Award enough XP to reach level 2 (even)
    xpNeeded := XPToLevel(2, cfg.BaseXP)
    _, err := svc.AwardKill(context.Background(), sess, xpNeeded, 99)
    require.NoError(t, err)
    assert.Equal(t, 1, sess.PendingSkillIncreases)
    assert.Len(t, skillSaver.calls, 1)
    assert.Equal(t, 1, skillSaver.calls[0])
}

func TestAwardKill_NoSkillIncreasesOnOddLevel(t *testing.T) {
    cfg := defaultTestConfig()
    saver := &mockProgressSaver{}
    skillSaver := &mockSkillIncreaseSaver{}
    svc := NewService(cfg, saver)
    svc.SetSkillIncreaseSaver(skillSaver)

    sess := &session.PlayerSession{Level: 2, Experience: XPToLevel(2, cfg.BaseXP), MaxHP: 10, CurrentHP: 10}
    // Award enough XP to reach level 3 (odd)
    xpNeeded := XPToLevel(3, cfg.BaseXP) - sess.Experience
    _, err := svc.AwardKill(context.Background(), sess, xpNeeded/cfg.Awards.KillXPPerNPCLevel+1, 99)
    require.NoError(t, err)
    // Level 3 is odd, no new skill increase
    assert.Equal(t, 0, sess.PendingSkillIncreases)
    assert.Empty(t, skillSaver.calls)
}
```

Run: `go test ./internal/game/xp/... -run TestAwardKill_Increments -v`
Expected: FAIL

**Step 5: Add SkillIncreaseSaver interface and wire into Service**

In `internal/game/xp/service.go`:

Add interface (after `ProgressSaver`):
```go
// SkillIncreaseSaver persists pending skill increases after a level-up.
type SkillIncreaseSaver interface {
    IncrementPendingSkillIncreases(ctx context.Context, id int64, n int) error
}
```

Add field to `Service`:
```go
type Service struct {
    cfg         *XPConfig
    saver       ProgressSaver
    skillSaver  SkillIncreaseSaver // optional; nil means no persistence for skill increases
}
```

Add setter:
```go
// SetSkillIncreaseSaver registers the saver for pending skill increases.
//
// Postcondition: skill increases will be persisted via saver on each level-up.
func (s *Service) SetSkillIncreaseSaver(saver SkillIncreaseSaver) {
    s.skillSaver = saver
}
```

In `service.award()`, after `sess.PendingBoosts += result.NewBoosts`, add:
```go
sess.PendingSkillIncreases += result.NewSkillIncreases
```

In the messages block, after the `result.NewBoosts > 0` message, add:
```go
if result.NewSkillIncreases > 0 {
    msgs = append(msgs, "You have a pending skill increase! Type 'trainskill <skill>' to advance a skill.")
}
```

In the `characterID > 0` block, after `s.saver.SaveProgress(...)`, add:
```go
if result.NewSkillIncreases > 0 && s.skillSaver != nil {
    if err := s.skillSaver.IncrementPendingSkillIncreases(ctx, characterID, result.NewSkillIncreases); err != nil {
        return msgs, fmt.Errorf("saving skill increases after level-up: %w", err)
    }
}
```

Note: `sess` must have `PendingSkillIncreases int` field — this is added in Task 4. Add it now to avoid compile errors; it will be zero-valued until Task 4 wires the login load.

**Step 6: Run tests**

```
go test ./internal/game/xp/... -timeout 60s -count=1
```
Expected: all pass.

**Step 7: Commit**

```bash
git add internal/game/xp/xp.go internal/game/xp/xp_test.go internal/game/xp/service.go internal/game/xp/service_test.go
git commit -m "feat: XP service computes and persists NewSkillIncreases on even level-up"
```

---

### Task 4: Session Field + Proto + Login Wiring

**Files:**
- Modify: `internal/game/session/manager.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/frontend/handlers/text_renderer.go`

**Step 1: Add PendingSkillIncreases to PlayerSession**

In `internal/game/session/manager.go`, in `PlayerSession` struct, after `PendingBoosts int`:
```go
// PendingSkillIncreases is the number of unspent skill rank increases.
PendingSkillIncreases int
```

**Step 2: Add pending_skill_increases to proto**

In `api/proto/game/v1/game.proto`, in `CharacterSheetView`, after field 31 (`pending_boosts`):
```proto
int32 pending_skill_increases = 32; // number of unassigned skill increases
```

Run: `make proto`
Verify: `go build ./...` (no errors)

**Step 3: Wire SkillIncreaseSaver in grpc_service.go**

In `internal/gameserver/grpc_service.go`, find where `xpSvc` is set up (near `SetXPService`). The `CharacterProgressRepository` already implements `IncrementPendingSkillIncreases` (added in Task 1). Wire it:

Find the `SetXPService` method and the place where `progressRepo` is configured. After `s.xpSvc.SetXPService(svc)` or wherever the service is initialized, add:
```go
svc.SetSkillIncreaseSaver(s.progressRepo)
```

(Check the exact location by searching for `SetXPService` in `cmd/gameserver/main.go` — that's where the wiring happens.)

**Step 4: Load PendingSkillIncreases at login**

In `grpc_service.go` around line 398, find the `GetProgress` call:
```go
dbLevel, dbExperience, dbMaxHP, boosts, progressErr := s.progressRepo.GetProgress(stream.Context(), characterID)
```

After populating `sess.PendingBoosts = boosts`, add:
```go
if skillIncreases, siErr := s.progressRepo.GetPendingSkillIncreases(stream.Context(), characterID); siErr == nil {
    sess.PendingSkillIncreases = skillIncreases
}
```

**Step 5: Backfill at login**

In the same login block (after loading `sess.PendingSkillIncreases`), add the backfill:
```go
// Backfill: give existing characters their earned skill increases if they have none recorded.
earnedIncreases := sess.Level / 2
if sess.PendingSkillIncreases == 0 && earnedIncreases > 0 {
    if bfErr := s.progressRepo.IncrementPendingSkillIncreases(stream.Context(), characterID, earnedIncreases); bfErr == nil {
        sess.PendingSkillIncreases = earnedIncreases
    } else {
        s.logger.Warn("backfill skill increases failed", zap.Error(bfErr))
    }
}
```

**Step 6: Include pending_skill_increases in CharacterSheetView**

Search for where `CharacterSheetView` is populated in `grpc_service.go` (grep for `view.PendingBoosts`). After that line, add:
```go
view.PendingSkillIncreases = int32(sess.PendingSkillIncreases)
```

**Step 7: Run tests**

```
go test ./... -timeout 300s -count=1
```
Expected: all pass.

**Step 8: Commit**

```bash
git add internal/game/session/manager.go api/proto/game/v1/game.proto \
    internal/gameserver/gamev1/game.pb.go internal/gameserver/grpc_service.go
git commit -m "feat: PendingSkillIncreases on session, proto field, login wiring and backfill"
```

---

### Task 5: trainskill Command (CMD-1 through CMD-7)

**Files:**
- Modify: `internal/game/command/commands.go`
- Create: `internal/game/command/trainskill.go`
- Create: `internal/game/command/trainskill_test.go`
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/frontend/handlers/bridge_handlers.go`
- Modify: `internal/gameserver/grpc_service.go`

#### CMD-1 & CMD-2

In `internal/game/command/commands.go`:

Add constant (with other Handler constants):
```go
HandlerTrainSkill = "trainskill"
```

Add to `BuiltinCommands()` (in `CategoryCharacter` or similar category):
```go
{Handler: HandlerTrainSkill, Usage: "trainskill <skill>", Description: "Advance a skill proficiency rank using a pending skill increase"},
```

#### CMD-3: Handler with TDD

**Step 1: Write failing tests**

Create `internal/game/command/trainskill_test.go`:

```go
package command_test

import (
    "testing"

    "github.com/cory-johannsen/mud/internal/game/command"
    "github.com/stretchr/testify/assert"
    "pgregory.net/rapid"
)

func TestHandleTrainSkill_NoArgs_ReturnsUsage(t *testing.T) {
    result, err := command.HandleTrainSkill(nil)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "usage")
}

func TestHandleTrainSkill_UnknownSkill_ReturnsError(t *testing.T) {
    _, err := command.HandleTrainSkill([]string{"notaskill"})
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "unknown skill")
}

func TestHandleTrainSkill_ValidSkill_ReturnsNormalizedID(t *testing.T) {
    id, err := command.HandleTrainSkill([]string{"Parkour"})
    assert.NoError(t, err)
    assert.Equal(t, "parkour", id)
}

func TestHandleTrainSkill_AllValidSkills(t *testing.T) {
    for _, skillID := range command.ValidSkillIDs {
        id, err := command.HandleTrainSkill([]string{skillID})
        assert.NoError(t, err)
        assert.Equal(t, skillID, id)
    }
}

func TestPropertyHandleTrainSkill_ValidAlwaysSucceeds(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        skill := rapid.SampledFrom(command.ValidSkillIDs).Draw(rt, "skill")
        id, err := command.HandleTrainSkill([]string{skill})
        if err != nil {
            rt.Fatalf("unexpected error for valid skill %q: %v", skill, err)
        }
        if id != skill {
            rt.Fatalf("expected %q got %q", skill, id)
        }
    })
}
```

Run: `go test ./internal/game/command/... -run TestHandleTrainSkill -v`
Expected: FAIL

**Step 2: Write minimal implementation**

Create `internal/game/command/trainskill.go`:

```go
package command

import (
    "fmt"
    "strings"
)

// ValidSkillIDs is the set of skill IDs accepted by HandleTrainSkill.
// These correspond to the skill IDs defined in content/skills.yaml.
var ValidSkillIDs = []string{
    "parkour", "ghosting", "grift", "muscle",
    "tech_lore", "rigging", "conspiracy", "factions", "intel",
    "patch_job", "wasteland", "gang_codes", "scavenging",
    "hustle", "smooth_talk", "hard_look", "rep",
}

var validSkillSet = func() map[string]bool {
    m := make(map[string]bool, len(ValidSkillIDs))
    for _, s := range ValidSkillIDs {
        m[s] = true
    }
    return m
}()

// HandleTrainSkill validates a trainskill command argument.
//
// Precondition: args contains the raw arguments after the command name.
// Postcondition: returns the normalized skill ID on success;
// returns an error with usage hint if args is empty;
// returns an error with "unknown skill" if the skill ID is invalid.
func HandleTrainSkill(args []string) (string, error) {
    if len(args) == 0 {
        return "", fmt.Errorf("usage: trainskill <skill>  (e.g. trainskill parkour)")
    }
    skillID := strings.ToLower(strings.TrimSpace(args[0]))
    if !validSkillSet[skillID] {
        return "", fmt.Errorf("unknown skill %q; valid skills: %s", skillID, strings.Join(ValidSkillIDs, ", "))
    }
    return skillID, nil
}
```

Run: `go test ./internal/game/command/... -timeout 60s -count=1`
Expected: all pass.

#### CMD-4: Proto

In `api/proto/game/v1/game.proto`:

Add message (near other Request messages):
```proto
message TrainSkillRequest {
  string skill_id = 1;
}
```

Find the `ClientMessage` oneof and determine the next available field number (currently 46 is `combat_default`). Add:
```proto
TrainSkillRequest train_skill = 47;
```

Run: `make proto`
Run: `go build ./...`

#### CMD-5: Bridge

In `internal/frontend/handlers/bridge_handlers.go`:

Add to `bridgeHandlerMap`:
```go
command.HandlerTrainSkill: bridgeTrainSkill,
```

Add function:
```go
// bridgeTrainSkill validates and sends a TrainSkillRequest.
//
// Precondition: bctx must be non-nil with a valid conn, reqID, and parsed.Args.
// Postcondition: if HandleTrainSkill returns an error, writes usage error and returns done=true;
// otherwise returns a non-nil msg containing a TrainSkillRequest.
func bridgeTrainSkill(bctx *bridgeContext) (bridgeResult, error) {
    skillID, err := command.HandleTrainSkill(bctx.parsed.Args)
    if err != nil {
        return writeErrorPrompt(bctx, err.Error())
    }
    return bridgeResult{msg: &gamev1.ClientMessage{
        RequestId: bctx.reqID,
        Payload:   &gamev1.ClientMessage_TrainSkill{TrainSkill: &gamev1.TrainSkillRequest{SkillId: skillID}},
    }}, nil
}
```

Verify wiring test passes:
```
go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v
```

#### CMD-6: gRPC Handler

In `internal/gameserver/grpc_service.go`:

Add case in dispatch type switch (find `switch p := in.Payload.(type)`):
```go
case *gamev1.ClientMessage_TrainSkill:
    return s.handleTrainSkill(uid, p.TrainSkill.SkillId)
```

Add handler function. Study `handleLevelUp` (line 3214) for the exact pattern to follow:

```go
// handleTrainSkill advances a skill proficiency rank for the player.
//
// Precondition: uid must identify an active session; skillID must be a valid skill ID.
// Postcondition: if no pending skill increases, returns error message event;
// if level gate not met, returns error message event;
// otherwise upgrades the skill rank, decrements pending count, updates session, and returns confirmation.
func (s *GameServiceServer) handleTrainSkill(uid, skillID string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessions.GetPlayer(uid)
    if !ok {
        return nil, fmt.Errorf("player %q not found", uid)
    }
    if sess.PendingSkillIncreases <= 0 {
        return &gamev1.ServerEvent{
            Payload: &gamev1.ServerEvent_Message{
                Message: &gamev1.MessageEvent{Content: "You have no pending skill increases."},
            },
        }, nil
    }

    currentRank := sess.Skills[skillID]
    if currentRank == "" {
        currentRank = "untrained"
    }

    // Compute next rank and check level gate.
    nextRank, gateLevel, err := nextSkillRank(currentRank, sess.Level)
    if err != nil {
        return &gamev1.ServerEvent{
            Payload: &gamev1.ServerEvent_Message{
                Message: &gamev1.MessageEvent{Content: err.Error()},
            },
        }, nil
    }
    if gateLevel > 0 {
        return &gamev1.ServerEvent{
            Payload: &gamev1.ServerEvent_Message{
                Message: &gamev1.MessageEvent{
                    Content: fmt.Sprintf("You must be level %d to advance %s to %s.", gateLevel, skillID, nextRank),
                },
            },
        }, nil
    }

    ctx := context.Background()
    // Persist rank change first, then consume — mirrors handleLevelUp pattern.
    if s.skillsRepo != nil {
        if err := s.skillsRepo.UpgradeSkill(ctx, sess.CharacterID, skillID, nextRank); err != nil {
            s.logger.Warn("handleTrainSkill: UpgradeSkill failed", zap.Error(err))
            return &gamev1.ServerEvent{
                Payload: &gamev1.ServerEvent_Message{
                    Message: &gamev1.MessageEvent{Content: "Failed to upgrade skill. Please try again."},
                },
            }, nil
        }
    }
    if s.progressRepo != nil {
        if err := s.progressRepo.ConsumePendingSkillIncrease(ctx, sess.CharacterID); err != nil {
            s.logger.Warn("handleTrainSkill: ConsumePendingSkillIncrease failed", zap.Error(err))
            return &gamev1.ServerEvent{
                Payload: &gamev1.ServerEvent_Message{
                    Message: &gamev1.MessageEvent{Content: "Failed to consume skill increase. Please try again."},
                },
            }, nil
        }
    }

    // Both persistence calls succeeded — mutate session.
    if sess.Skills == nil {
        sess.Skills = make(map[string]string)
    }
    sess.Skills[skillID] = nextRank
    sess.PendingSkillIncreases--

    return &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_Message{
            Message: &gamev1.MessageEvent{
                Content: fmt.Sprintf("You advanced %s from %s to %s. Pending skill increases remaining: %d.",
                    skillID, currentRank, nextRank, sess.PendingSkillIncreases),
            },
        },
    }, nil
}

// nextSkillRank returns (nextRank, gateLevel, err).
// If the skill is already at max rank, err is non-nil.
// If the next rank requires a higher level than sess.Level, gateLevel > 0 and nextRank is still set.
// gateLevel == 0 means the advancement is allowed at the current level.
//
// Precondition: currentRank must be one of the five rank strings; level >= 1.
func nextSkillRank(currentRank string, level int) (nextRank string, gateLevel int, err error) {
    type step struct {
        next string
        gate int // minimum level required
    }
    progression := map[string]step{
        "untrained": {"trained", 0},
        "trained":   {"expert", 15},
        "expert":    {"master", 35},
        "master":    {"legendary", 75},
    }
    s, ok := progression[currentRank]
    if !ok {
        return "", 0, fmt.Errorf("skill is already at maximum rank (legendary)")
    }
    if s.gate > 0 && level < s.gate {
        return s.next, s.gate, nil
    }
    return s.next, 0, nil
}
```

Note: `s.skillsRepo` is a `*postgres.CharacterSkillsRepository` (or interface). Check how existing repos are stored on `GameServiceServer` and follow the pattern. You may need to add a `skillsRepo` field — look at how `charRepo`, `charSaver`, `progressRepo` are declared and injected.

**Step 7: Run all tests**

```
go test ./... -timeout 300s -count=1
```
Expected: all pass.

**Step 8: Commit**

```bash
git add -A
git commit -m "feat: trainskill command (CMD-1 through CMD-7)"
```

---

### Task 6: Add Tests for handleTrainSkill

**Files:**
- Create: `internal/gameserver/grpc_service_trainskill_test.go`

Follow the pattern of `internal/gameserver/grpc_service_combat_default_test.go`. Add tests:

1. No pending skill increases → error message event, no DB calls
2. Skill already at legendary → error message event
3. Level gate not met (e.g., trying expert at level 5) → error message event
4. UpgradeSkill failure → error message event, ConsumePendingSkillIncrease NOT called, session unchanged
5. ConsumePendingSkillIncrease failure → error message event, session unchanged
6. Happy path: skill advances, session updated, confirmation message
7. Property test: all 17 valid skills can be advanced from untrained→trained (no gate)

Run: `go test ./internal/gameserver/... -timeout 120s -count=1`
Expected: all pass.

**Commit:**
```bash
git add internal/gameserver/grpc_service_trainskill_test.go
git commit -m "test: handleTrainSkill server-side tests"
```

---

### Task 7: Character Sheet Display

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/text_renderer_test.go`

**Step 1: Write failing test**

In `text_renderer_test.go`, find the test for the Progress section (grep for `TestRenderCharacterSheet_Progress` or similar). Add a test case asserting `pending_skill_increases` appears:

```go
func TestRenderCharacterSheet_PendingSkillIncreases_Shown(t *testing.T) {
    csv := &gamev1.CharacterSheetView{
        Name:                  "Test",
        Level:                 4,
        Experience:            1600,
        XpToNext:              900,
        PendingBoosts:         0,
        PendingSkillIncreases: 2,
    }
    result := RenderCharacterSheet(csv, 120)
    assert.Contains(t, result, "Pending Skill Increases: 2")
    assert.Contains(t, result, "trainskill")
}
```

Run: `go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_PendingSkillIncreases -v`
Expected: FAIL

**Step 2: Update renderer**

In `internal/frontend/handlers/text_renderer.go`, find the Progress section of `RenderCharacterSheet` (look for where `PendingBoosts` is rendered). After the pending boosts line, add:

```go
if csv.GetPendingSkillIncreases() > 0 {
    left = append(left, sl(fmt.Sprintf("  Pending Skill Increases: %d", csv.GetPendingSkillIncreases())))
    left = append(left, sl(telnet.Colorf(telnet.BrightYellow, "  (type 'trainskill <skill>' to assign)")))
}
```

**Step 3: Run tests**

```
go test ./internal/frontend/handlers/... -timeout 60s -count=1
```
Expected: all pass.

**Step 4: Commit**

```bash
git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat: display pending skill increases on character sheet"
```

---

### Task 8: Full Test Suite + FEATURES.md + Deploy

**Step 1: Run full test suite**

```
cd /home/cjohannsen/src/mud && go test ./... -timeout 300s -count=1
```
All tests must pass.

**Step 2: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, find `- [ ] Skill advancement` and replace with:

```markdown
- [x] Skill advancement
  - [x] `pending_skill_increases` column on `characters` table (migration 021)
  - [x] Skill increases awarded at every even character level (L2, L4, L6...)
  - [x] Rank gates: expert (L15+), master (L35+), legendary (L75+)
  - [x] `trainskill <skill>` command to spend pending skill increases
  - [x] Level gate enforcement at assignment time
  - [x] Backfill: existing characters receive `floor(level/2)` pending increases at login
  - [x] Pending skill increases displayed on character sheet
```

**Step 3: Deploy**

```bash
make k8s-redeploy
```

Wait for completion.

**Step 4: Commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark skill advancement complete in FEATURES.md"
git push
```
