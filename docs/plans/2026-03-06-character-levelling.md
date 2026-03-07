# Character Levelling Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add XP-based character levelling with three XP sources (combat kills, room discovery, skill checks), a geometric curve, automatic level-up, deferred ability boost via `levelup` command, and configurable values.

**Architecture:** A pure `internal/game/xp` package handles all XP math (no DB calls); `internal/storage/postgres` gets a `character_pending_boosts` table and `SaveProgress` method; the gameserver wires three XP award call sites and implements the `levelup` command end-to-end (CMD-1 through CMD-7). All values are configurable via `content/xp_config.yaml`.

**Tech Stack:** Go, `pgregory.net/rapid` (property-based tests), `gopkg.in/yaml.v3`, existing `internal/game/xp` package (new), existing postgres testutil shared-container pattern.

---

### Task 1: XP config file and loader

**Files:**
- Create: `content/xp_config.yaml`
- Create: `internal/game/xp/config.go`
- Create: `internal/game/xp/config_test.go`

**Step 1: Create `content/xp_config.yaml`**

```yaml
base_xp: 100
hp_per_level: 5
boost_interval: 5
level_cap: 100
job_level_cap: 20

awards:
  kill_xp_per_npc_level: 50
  new_room_xp: 10
  skill_check_success_xp: 10
  skill_check_crit_success_xp: 25
  skill_check_dc_multiplier: 2
```

**Step 2: Write failing test**

Create `internal/game/xp/config_test.go`:

```go
package xp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/xp"
)

func TestLoadXPConfig_ParsesAllFields(t *testing.T) {
	yaml := `
base_xp: 100
hp_per_level: 5
boost_interval: 5
level_cap: 100
job_level_cap: 20
awards:
  kill_xp_per_npc_level: 50
  new_room_xp: 10
  skill_check_success_xp: 10
  skill_check_crit_success_xp: 25
  skill_check_dc_multiplier: 2
`
	tmp := filepath.Join(t.TempDir(), "xp_config.yaml")
	require.NoError(t, os.WriteFile(tmp, []byte(yaml), 0644))

	cfg, err := xp.LoadXPConfig(tmp)
	require.NoError(t, err)
	assert.Equal(t, 100, cfg.BaseXP)
	assert.Equal(t, 5, cfg.HPPerLevel)
	assert.Equal(t, 5, cfg.BoostInterval)
	assert.Equal(t, 100, cfg.LevelCap)
	assert.Equal(t, 50, cfg.Awards.KillXPPerNPCLevel)
	assert.Equal(t, 10, cfg.Awards.NewRoomXP)
	assert.Equal(t, 10, cfg.Awards.SkillCheckSuccessXP)
	assert.Equal(t, 25, cfg.Awards.SkillCheckCritSuccessXP)
	assert.Equal(t, 2, cfg.Awards.SkillCheckDCMultiplier)
}

func TestLoadXPConfig_MissingFile(t *testing.T) {
	_, err := xp.LoadXPConfig("/nonexistent/xp_config.yaml")
	assert.Error(t, err)
}
```

Run: `go test ./internal/game/xp/... -run TestLoadXPConfig -v`
Expected: FAIL (package does not exist)

**Step 3: Implement `internal/game/xp/config.go`**

```go
// Package xp provides XP award logic and level-up calculations.
package xp

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Awards holds the configurable XP values for each award source.
type Awards struct {
	// KillXPPerNPCLevel is the XP multiplier per NPC level for combat kills.
	KillXPPerNPCLevel int `yaml:"kill_xp_per_npc_level"`
	// NewRoomXP is the flat XP award for discovering a previously unseen room.
	NewRoomXP int `yaml:"new_room_xp"`
	// SkillCheckSuccessXP is the base XP for a success outcome on a skill check.
	SkillCheckSuccessXP int `yaml:"skill_check_success_xp"`
	// SkillCheckCritSuccessXP is the base XP for a crit_success outcome on a skill check.
	SkillCheckCritSuccessXP int `yaml:"skill_check_crit_success_xp"`
	// SkillCheckDCMultiplier is multiplied by DC and added to skill check XP awards.
	SkillCheckDCMultiplier int `yaml:"skill_check_dc_multiplier"`
}

// XPConfig holds all configurable parameters for the levelling system.
type XPConfig struct {
	// BaseXP is the coefficient in the formula: xp_to_reach_level(n) = n² × BaseXP.
	BaseXP int `yaml:"base_xp"`
	// HPPerLevel is the max HP increase granted on each level-up.
	HPPerLevel int `yaml:"hp_per_level"`
	// BoostInterval is the level interval at which an ability boost is granted.
	BoostInterval int `yaml:"boost_interval"`
	// LevelCap is the maximum character level.
	LevelCap int `yaml:"level_cap"`
	// JobLevelCap is the maximum level for any single job (reserved for future use).
	JobLevelCap int `yaml:"job_level_cap"`
	// Awards holds per-source XP values.
	Awards Awards `yaml:"awards"`
}

// LoadXPConfig reads and parses the XP configuration from the given YAML file.
//
// Precondition: path must refer to a readable YAML file matching XPConfig.
// Postcondition: Returns a non-nil *XPConfig on success, or a non-nil error.
func LoadXPConfig(path string) (*XPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading xp config %q: %w", path, err)
	}
	var cfg XPConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing xp config %q: %w", path, err)
	}
	return &cfg, nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/game/xp/... -run TestLoadXPConfig -v`
Expected: PASS

**Step 5: Commit**

```bash
git add content/xp_config.yaml internal/game/xp/config.go internal/game/xp/config_test.go
git commit -m "feat: add XP config file and loader"
```

---

### Task 2: Pure XP math functions

**Files:**
- Create: `internal/game/xp/xp.go`
- Create: `internal/game/xp/xp_test.go`

**Step 1: Write failing tests**

Create `internal/game/xp/xp_test.go`:

```go
package xp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/xp"
)

func TestXPToLevel_Formula(t *testing.T) {
	// xp_to_reach_level(n) = n² × baseXP
	assert.Equal(t, 100, xp.XPToLevel(1, 100))   // 1² × 100
	assert.Equal(t, 400, xp.XPToLevel(2, 100))   // 2² × 100
	assert.Equal(t, 900, xp.XPToLevel(3, 100))   // 3² × 100
	assert.Equal(t, 10000, xp.XPToLevel(10, 100))
	assert.Equal(t, 1000000, xp.XPToLevel(100, 100))
}

func TestLevelForXP_Boundaries(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100}
	// 0 XP → level 1
	assert.Equal(t, 1, xp.LevelForXP(0, cfg))
	// exactly enough for level 2 (400 XP)
	assert.Equal(t, 2, xp.LevelForXP(400, cfg))
	// one short of level 2
	assert.Equal(t, 1, xp.LevelForXP(399, cfg))
	// exactly enough for level 3 (900 XP)
	assert.Equal(t, 3, xp.LevelForXP(900, cfg))
	// massive XP → capped at LevelCap
	assert.Equal(t, 100, xp.LevelForXP(999_999_999, cfg))
}

func TestProperty_LevelForXP_Inverse(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100}
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 100).Draw(rt, "level")
		threshold := xp.XPToLevel(level, cfg.BaseXP)
		got := xp.LevelForXP(threshold, cfg)
		if got != level {
			rt.Fatalf("LevelForXP(XPToLevel(%d)) = %d, want %d", level, got, level)
		}
	})
}

func TestProperty_LevelForXP_NeverExceedsCap(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 50}
	rapid.Check(t, func(rt *rapid.T) {
		xpVal := rapid.IntRange(0, 10_000_000).Draw(rt, "xp")
		got := xp.LevelForXP(xpVal, cfg)
		if got > cfg.LevelCap {
			rt.Fatalf("level %d exceeds cap %d for xp=%d", got, cfg.LevelCap, xpVal)
		}
		if got < 1 {
			rt.Fatalf("level %d < 1 for xp=%d", got, xpVal)
		}
	})
}

func TestAward_NoLevelUp(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100, HPPerLevel: 5, BoostInterval: 5}
	result := xp.Award(1, 0, 50, cfg) // level=1, currentXP=0, award 50
	assert.Equal(t, 50, result.NewXP)
	assert.Equal(t, 1, result.NewLevel)
	assert.Equal(t, 0, result.HPGained)
	assert.Equal(t, 0, result.NewBoosts)
	assert.False(t, result.LeveledUp)
}

func TestAward_LevelUp(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100, HPPerLevel: 5, BoostInterval: 5}
	// 400 XP needed for level 2; award 400 at XP=0
	result := xp.Award(1, 0, 400, cfg)
	assert.Equal(t, 400, result.NewXP)
	assert.Equal(t, 2, result.NewLevel)
	assert.Equal(t, 5, result.HPGained)
	assert.True(t, result.LeveledUp)
	assert.Equal(t, 0, result.NewBoosts) // boost at level 5, not 2
}

func TestAward_BoostAtInterval(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100, HPPerLevel: 5, BoostInterval: 5}
	// 2500 XP needed for level 5
	result := xp.Award(4, xp.XPToLevel(4, 100)-1, 3000, cfg)
	assert.Equal(t, 5, result.NewLevel)
	assert.Equal(t, 1, result.NewBoosts) // level 5 is a boost level
}

func TestAward_AtLevelCap(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 3, HPPerLevel: 5, BoostInterval: 5}
	// Already at cap, extra XP should not level up
	result := xp.Award(3, 900, 10000, cfg)
	assert.Equal(t, 3, result.NewLevel)
	assert.False(t, result.LeveledUp)
}

func TestProperty_Award_NeverSkipsLevel(t *testing.T) {
	cfg := &xp.XPConfig{BaseXP: 100, LevelCap: 100, HPPerLevel: 5, BoostInterval: 5}
	rapid.Check(t, func(rt *rapid.T) {
		level := rapid.IntRange(1, 99).Draw(rt, "level")
		currentXP := xp.XPToLevel(level, cfg.BaseXP)
		award := rapid.IntRange(0, 10_000_000).Draw(rt, "award")
		result := xp.Award(level, currentXP, award, cfg)
		// Level should increase by at most the correct amount (no skipping)
		if result.NewLevel > cfg.LevelCap {
			rt.Fatalf("level %d exceeded cap %d", result.NewLevel, cfg.LevelCap)
		}
		if result.NewLevel < level {
			rt.Fatalf("level went backward: %d → %d", level, result.NewLevel)
		}
	})
}
```

Run: `go test ./internal/game/xp/... -run TestXPToLevel -v`
Expected: FAIL (xp.go doesn't exist)

**Step 2: Implement `internal/game/xp/xp.go`**

```go
package xp

// AwardResult holds the outcome of an XP award calculation.
type AwardResult struct {
	// NewXP is the character's total XP after the award.
	NewXP int
	// NewLevel is the character's level after the award.
	NewLevel int
	// HPGained is the total max HP increase from level-ups this award.
	HPGained int
	// NewBoosts is the number of new ability boosts earned this award.
	NewBoosts int
	// LeveledUp is true if the character gained at least one level.
	LeveledUp bool
}

// XPToLevel returns the total XP required to reach the given level.
//
// Precondition: level >= 1; baseXP > 0.
// Postcondition: Returns level² × baseXP.
func XPToLevel(level, baseXP int) int {
	return level * level * baseXP
}

// LevelForXP returns the level a character with the given XP should be at.
//
// Precondition: cfg must be non-nil; cfg.LevelCap >= 1; cfg.BaseXP > 0.
// Postcondition: Returns a value in [1, cfg.LevelCap].
func LevelForXP(xp int, cfg *XPConfig) int {
	level := 1
	for level < cfg.LevelCap {
		next := level + 1
		if xp < XPToLevel(next, cfg.BaseXP) {
			break
		}
		level = next
	}
	return level
}

// Award calculates the result of adding awardXP to a character at the given
// level with the given currentXP.
//
// Precondition: level >= 1; currentXP >= 0; awardXP >= 0; cfg must be non-nil.
// Postcondition: Returns an AwardResult with the updated level, XP, HP gain, and boost count.
// Level never exceeds cfg.LevelCap. At most one level-up is processed if the
// award spans multiple thresholds — callers may call Award again for remaining XP.
func Award(level, currentXP, awardXP int, cfg *XPConfig) AwardResult {
	newXP := currentXP + awardXP
	newLevel := LevelForXP(newXP, cfg)
	if newLevel > cfg.LevelCap {
		newLevel = cfg.LevelCap
	}

	levelsGained := newLevel - level
	hpGained := levelsGained * cfg.HPPerLevel

	newBoosts := 0
	if cfg.BoostInterval > 0 {
		for l := level + 1; l <= newLevel; l++ {
			if l%cfg.BoostInterval == 0 {
				newBoosts++
			}
		}
	}

	return AwardResult{
		NewXP:     newXP,
		NewLevel:  newLevel,
		HPGained:  hpGained,
		NewBoosts: newBoosts,
		LeveledUp: newLevel > level,
	}
}
```

**Step 3: Run tests**

Run: `go test ./internal/game/xp/... -timeout 60s -v 2>&1 | tail -20`
Expected: all PASS

**Step 4: Commit**

```bash
git add internal/game/xp/xp.go internal/game/xp/xp_test.go
git commit -m "feat: add XP math functions (XPToLevel, LevelForXP, Award)"
```

---

### Task 3: DB migration and SaveProgress

**Files:**
- Modify: `internal/testutil/postgres.go`
- Create: `internal/storage/postgres/character_progress.go`
- Create: `internal/storage/postgres/character_progress_test.go`
- Modify: `internal/storage/postgres/main_test.go` (add migration to applyAllMigrations)
- Modify: `internal/gameserver/grpc_service.go` (add SaveProgress to CharacterSaver interface)

**Step 1: Add pending boosts migration to testutil and main_test.go**

In `internal/testutil/postgres.go`, add a new method after `ApplyProficienciesMigration`:

```go
// ApplyPendingBoostsMigration adds the character_pending_boosts table for tests.
//
// Precondition: Pool connected; characters table exists.
// Postcondition: character_pending_boosts table exists in the test database.
func (pc *PostgresContainer) ApplyPendingBoostsMigration(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	start := time.Now()
	_, err := pc.RawPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS character_pending_boosts (
			character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
			count        INT    NOT NULL DEFAULT 0,
			PRIMARY KEY (character_id)
		);
	`)
	if err != nil {
		t.Fatalf("applying pending boosts migration: %v", err)
	}
	t.Logf("pending boosts migration applied [%s]", time.Since(start))
}
```

In `internal/storage/postgres/main_test.go`, add to `applyAllMigrations` (before the final closing backtick):

```sql
CREATE TABLE IF NOT EXISTS character_pending_boosts (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    count        INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id)
);
```

Also add the real DB migration file. Create `migrations/019_character_pending_boosts.sql`:

```sql
CREATE TABLE IF NOT EXISTS character_pending_boosts (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    count        INT    NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id)
);
```

Check the migrations directory first: `ls /home/cjohannsen/src/mud/migrations/` to find the correct next migration number.

**Step 2: Write failing tests**

Create `internal/storage/postgres/character_progress_test.go`:

```go
package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestSaveProgress_RoundTrip(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	err := progressRepo.SaveProgress(ctx, ch.ID, 5, 2500, 35, 1)
	require.NoError(t, err)

	level, xp, maxHP, boosts, err := progressRepo.GetProgress(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, level)
	assert.Equal(t, 2500, xp)
	assert.Equal(t, 35, maxHP)
	assert.Equal(t, 1, boosts)
}

func TestSaveProgress_UpdatesExisting(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	require.NoError(t, progressRepo.SaveProgress(ctx, ch.ID, 2, 400, 15, 0))
	require.NoError(t, progressRepo.SaveProgress(ctx, ch.ID, 3, 900, 20, 0))

	level, xp, _, _, err := progressRepo.GetProgress(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, level)
	assert.Equal(t, 900, xp)
}

func TestGetProgress_DefaultsForNew(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	level, xp, maxHP, boosts, err := progressRepo.GetProgress(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, ch.Level, level)
	assert.Equal(t, ch.Experience, xp)
	assert.Equal(t, ch.MaxHP, maxHP)
	assert.Equal(t, 0, boosts)
}

func TestConsumePendingBoost_DecrementsCount(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	require.NoError(t, progressRepo.SaveProgress(ctx, ch.ID, 5, 2500, 35, 2))
	require.NoError(t, progressRepo.ConsumePendingBoost(ctx, ch.ID))

	_, _, _, boosts, err := progressRepo.GetProgress(ctx, ch.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, boosts)
}

func TestConsumePendingBoost_NoneAvailableReturnsError(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)
	ch := createTestCharacter(t, charRepo, ctx)

	err := progressRepo.ConsumePendingBoost(ctx, ch.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pending boosts")
}

func TestProperty_SaveProgress_RoundTrip(t *testing.T) {
	ctx := context.Background()
	charRepo := pgstore.NewCharacterRepository(sharedPool)
	progressRepo := pgstore.NewCharacterProgressRepository(sharedPool)

	rapid.Check(t, func(rt *rapid.T) {
		ch := createTestCharacter(t, charRepo, ctx)
		level := rapid.IntRange(1, 100).Draw(rt, "level")
		xpVal := rapid.IntRange(0, 1_000_000).Draw(rt, "xp")
		maxHP := rapid.IntRange(1, 500).Draw(rt, "maxHP")
		boosts := rapid.IntRange(0, 20).Draw(rt, "boosts")

		if err := progressRepo.SaveProgress(ctx, ch.ID, level, xpVal, maxHP, boosts); err != nil {
			rt.Fatalf("SaveProgress: %v", err)
		}
		gotLevel, gotXP, gotMaxHP, gotBoosts, err := progressRepo.GetProgress(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetProgress: %v", err)
		}
		if gotLevel != level || gotXP != xpVal || gotMaxHP != maxHP || gotBoosts != boosts {
			rt.Fatalf("mismatch: got level=%d xp=%d maxHP=%d boosts=%d, want level=%d xp=%d maxHP=%d boosts=%d",
				gotLevel, gotXP, gotMaxHP, gotBoosts, level, xpVal, maxHP, boosts)
		}
	})
}
```

Run: `go test ./internal/storage/postgres/... -run TestSaveProgress -v`
Expected: FAIL (CharacterProgressRepository does not exist)

**Step 3: Implement `internal/storage/postgres/character_progress.go`**

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CharacterProgressRepository persists and retrieves character level, XP, and
// pending ability boost counts.
type CharacterProgressRepository struct {
	pool *pgxpool.Pool
}

// NewCharacterProgressRepository returns a new CharacterProgressRepository.
//
// Precondition: pool must not be nil.
// Postcondition: Returns a non-nil repository.
func NewCharacterProgressRepository(pool *pgxpool.Pool) *CharacterProgressRepository {
	return &CharacterProgressRepository{pool: pool}
}

// SaveProgress persists level, experience, max_hp, and pending_boosts for a character.
// Updates characters.level, characters.experience, characters.max_hp in one statement,
// and upserts character_pending_boosts.
//
// Precondition: id > 0; level >= 1; experience >= 0; maxHP >= 1; pendingBoosts >= 0.
// Postcondition: All four values are durably persisted before returning.
func (r *CharacterProgressRepository) SaveProgress(ctx context.Context, id int64, level, experience, maxHP, pendingBoosts int) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("SaveProgress begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		UPDATE characters
		SET level = $2, experience = $3, max_hp = $4
		WHERE id = $1
	`, id, level, experience, maxHP)
	if err != nil {
		return fmt.Errorf("SaveProgress update character: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO character_pending_boosts (character_id, count)
		VALUES ($1, $2)
		ON CONFLICT (character_id) DO UPDATE SET count = EXCLUDED.count
	`, id, pendingBoosts)
	if err != nil {
		return fmt.Errorf("SaveProgress upsert pending boosts: %w", err)
	}

	return tx.Commit(ctx)
}

// GetProgress returns level, experience, max_hp, and pending_boosts for a character.
// If no pending_boosts row exists, returns 0 for pending boosts.
//
// Precondition: id > 0.
// Postcondition: Returns (level, experience, maxHP, pendingBoosts, nil) on success.
func (r *CharacterProgressRepository) GetProgress(ctx context.Context, id int64) (level, experience, maxHP, pendingBoosts int, err error) {
	if id <= 0 {
		return 0, 0, 0, 0, fmt.Errorf("characterID must be > 0, got %d", id)
	}
	err = r.pool.QueryRow(ctx, `
		SELECT c.level, c.experience, c.max_hp, COALESCE(b.count, 0)
		FROM characters c
		LEFT JOIN character_pending_boosts b ON b.character_id = c.id
		WHERE c.id = $1
	`, id).Scan(&level, &experience, &maxHP, &pendingBoosts)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("GetProgress: %w", err)
	}
	return level, experience, maxHP, pendingBoosts, nil
}

// ConsumePendingBoost decrements the pending boost count by 1 for a character.
// Returns an error if the character has no pending boosts.
//
// Precondition: id > 0.
// Postcondition: Pending boost count decremented by 1, or error returned if count was 0.
func (r *CharacterProgressRepository) ConsumePendingBoost(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("characterID must be > 0, got %d", id)
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE character_pending_boosts
		SET count = count - 1
		WHERE character_id = $1 AND count > 0
	`, id)
	if err != nil {
		return fmt.Errorf("ConsumePendingBoost: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errors.New("no pending boosts available for character")
	}
	return nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/storage/postgres/... -run TestSaveProgress -timeout 120s -v 2>&1 | tail -20`
Expected: PASS

Run full postgres suite: `go test ./internal/storage/postgres/... -timeout 120s 2>&1`
Expected: `ok github.com/cory-johannsen/mud/internal/storage/postgres`

**Step 5: Add SaveProgress to CharacterSaver interface**

In `internal/gameserver/grpc_service.go`, find the `CharacterSaver` interface (around line 60). Add:

```go
SaveProgress(ctx context.Context, id int64, level, experience, maxHP, pendingBoosts int) error
```

Also add a new interface field and constructor parameter for `CharacterProgressRepository`. Find `NewGameServiceServer` and add:

```go
progressRepo *postgres.CharacterProgressRepository
```

as a field on `GameServiceServer`, passed via constructor.

**Step 6: Commit**

```bash
git add internal/testutil/postgres.go \
    internal/storage/postgres/character_progress.go \
    internal/storage/postgres/character_progress_test.go \
    internal/storage/postgres/main_test.go \
    migrations/0NN_character_pending_boosts.sql \
    internal/gameserver/grpc_service.go
git commit -m "feat: add character_pending_boosts table and CharacterProgressRepository"
```

---

### Task 4: XPService and wire award call sites

**Files:**
- Create: `internal/game/xp/service.go`
- Create: `internal/game/xp/service_test.go`
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `internal/gameserver/grpc_service.go`

**Step 1: Write failing service tests**

Create `internal/game/xp/service_test.go`:

```go
package xp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/xp"
)

type fakeProgressSaver struct {
	savedLevel    int
	savedXP       int
	savedMaxHP    int
	savedBoosts   int
}

func (f *fakeProgressSaver) SaveProgress(_ context.Context, _ int64, level, experience, maxHP, pendingBoosts int) error {
	f.savedLevel = level
	f.savedXP = experience
	f.savedMaxHP = maxHP
	f.savedBoosts = pendingBoosts
	return nil
}

func newTestSvc(t *testing.T) (*xp.Service, *fakeProgressSaver) {
	t.Helper()
	cfg := &xp.XPConfig{
		BaseXP:        100,
		HPPerLevel:    5,
		BoostInterval: 5,
		LevelCap:      100,
		Awards: xp.Awards{
			KillXPPerNPCLevel:       50,
			NewRoomXP:               10,
			SkillCheckSuccessXP:     10,
			SkillCheckCritSuccessXP: 25,
			SkillCheckDCMultiplier:  2,
		},
	}
	saver := &fakeProgressSaver{}
	svc := xp.NewService(cfg, saver)
	return svc, saver
}

func newTestSession(level, currentXP, maxHP int) *session.PlayerSession {
	sess := &session.PlayerSession{}
	sess.Level = level
	sess.Experience = currentXP
	sess.MaxHP = maxHP
	sess.CurrentHP = maxHP
	return sess
}

func TestService_AwardKill_NoLevelUp(t *testing.T) {
	svc, saver := newTestSvc(t)
	sess := newTestSession(1, 0, 10)

	msgs, err := svc.AwardKill(context.Background(), sess, 1, 0 /*charID*/)
	require.NoError(t, err)
	assert.Empty(t, msgs) // no level-up message
	assert.Equal(t, 50, sess.Experience)
	assert.Equal(t, 0, saver.savedLevel) // SaveProgress not called (no level-up)
}

func TestService_AwardKill_LevelUp(t *testing.T) {
	svc, saver := newTestSvc(t)
	sess := newTestSession(1, 350, 10)

	msgs, err := svc.AwardKill(context.Background(), sess, 1, 1 /*charID*/)
	require.NoError(t, err)
	// 350 + 50 = 400 → level 2
	assert.Equal(t, 2, sess.Level)
	assert.Equal(t, 400, sess.Experience)
	assert.Equal(t, 15, sess.MaxHP) // +5 HP
	assert.NotEmpty(t, msgs)
	assert.Contains(t, msgs[0], "level 2")
	assert.Equal(t, 2, saver.savedLevel)
}

func TestService_AwardRoomDiscovery(t *testing.T) {
	svc, _ := newTestSvc(t)
	sess := newTestSession(1, 0, 10)

	_, err := svc.AwardRoomDiscovery(context.Background(), sess, 0)
	require.NoError(t, err)
	assert.Equal(t, 10, sess.Experience)
}

func TestService_AwardSkillCheck_Success(t *testing.T) {
	svc, _ := newTestSvc(t)
	sess := newTestSession(1, 0, 10)

	_, err := svc.AwardSkillCheck(context.Background(), sess, 14, false /*isCrit*/, 0)
	require.NoError(t, err)
	// success: 10 + 14×2 = 38
	assert.Equal(t, 38, sess.Experience)
}

func TestService_AwardSkillCheck_CritSuccess(t *testing.T) {
	svc, _ := newTestSvc(t)
	sess := newTestSession(1, 0, 10)

	_, err := svc.AwardSkillCheck(context.Background(), sess, 14, true /*isCrit*/, 0)
	require.NoError(t, err)
	// crit: 25 + 14×2 = 53
	assert.Equal(t, 53, sess.Experience)
}

func TestService_BoostPending_NotifiedAtInterval(t *testing.T) {
	svc, saver := newTestSvc(t)
	sess := newTestSession(4, xp.XPToLevel(4, 100)-1, 30)

	// Award enough XP to hit level 5
	msgs, err := svc.AwardKill(context.Background(), sess, 5, 1)
	require.NoError(t, err)
	assert.Equal(t, 5, sess.Level)
	assert.Contains(t, msgs[len(msgs)-1], "levelup") // pending boost message
	assert.Equal(t, 1, saver.savedBoosts)
}
```

Run: `go test ./internal/game/xp/... -run TestService -v`
Expected: FAIL (Service does not exist)

**Step 2: Implement `internal/game/xp/service.go`**

```go
package xp

import (
	"context"
	"fmt"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// ProgressSaver persists character level progress.
//
// Precondition: characterID must be > 0.
type ProgressSaver interface {
	SaveProgress(ctx context.Context, id int64, level, experience, maxHP, pendingBoosts int) error
}

// Service orchestrates XP awards and level-up detection.
type Service struct {
	cfg   *XPConfig
	saver ProgressSaver
}

// NewService creates a new XP Service.
//
// Precondition: cfg and saver must be non-nil.
// Postcondition: Returns a non-nil Service ready to award XP.
func NewService(cfg *XPConfig, saver ProgressSaver) *Service {
	return &Service{cfg: cfg, saver: saver}
}

// AwardKill awards XP for killing an NPC of the given level.
// Returns notification messages (level-up announcement, boost prompt if applicable).
//
// Precondition: sess and saver must be non-nil; npcLevel >= 1.
// Postcondition: sess.Experience, sess.Level, sess.MaxHP updated; SaveProgress called on level-up.
func (s *Service) AwardKill(ctx context.Context, sess *session.PlayerSession, npcLevel int, characterID int64) ([]string, error) {
	amount := npcLevel * s.cfg.Awards.KillXPPerNPCLevel
	return s.award(ctx, sess, characterID, amount)
}

// AwardRoomDiscovery awards XP for entering a previously unseen room.
//
// Precondition: sess must be non-nil.
// Postcondition: sess.Experience updated; SaveProgress called on level-up.
func (s *Service) AwardRoomDiscovery(ctx context.Context, sess *session.PlayerSession, characterID int64) ([]string, error) {
	return s.award(ctx, sess, characterID, s.cfg.Awards.NewRoomXP)
}

// AwardSkillCheck awards XP for a successful or crit-successful skill check.
// isCrit should be true for crit_success outcomes.
//
// Precondition: sess must be non-nil; dc >= 0.
// Postcondition: sess.Experience updated; SaveProgress called on level-up.
func (s *Service) AwardSkillCheck(ctx context.Context, sess *session.PlayerSession, dc int, isCrit bool, characterID int64) ([]string, error) {
	base := s.cfg.Awards.SkillCheckSuccessXP
	if isCrit {
		base = s.cfg.Awards.SkillCheckCritSuccessXP
	}
	amount := base + dc*s.cfg.Awards.SkillCheckDCMultiplier
	return s.award(ctx, sess, characterID, amount)
}

// award applies awardXP to sess and handles level-up logic.
func (s *Service) award(ctx context.Context, sess *session.PlayerSession, characterID int64, awardXP int) ([]string, error) {
	result := Award(sess.Level, sess.Experience, awardXP, s.cfg)

	sess.Experience = result.NewXP
	sess.Level = result.NewLevel
	sess.MaxHP += result.HPGained
	if sess.CurrentHP > sess.MaxHP {
		sess.CurrentHP = sess.MaxHP
	}
	sess.PendingBoosts += result.NewBoosts

	if !result.LeveledUp {
		return nil, nil
	}

	var msgs []string
	msgs = append(msgs, fmt.Sprintf("*** You reached level %d! ***", result.NewLevel))
	if result.HPGained > 0 {
		msgs = append(msgs, fmt.Sprintf("Max HP increased by %d (now %d).", result.HPGained, sess.MaxHP))
	}
	if result.NewBoosts > 0 {
		msgs = append(msgs, "You have a pending ability boost! Type 'levelup' to assign it.")
	}

	if characterID > 0 {
		if err := s.saver.SaveProgress(ctx, characterID, sess.Level, sess.Experience, sess.MaxHP, sess.PendingBoosts); err != nil {
			return msgs, fmt.Errorf("saving progress: %w", err)
		}
	}

	return msgs, nil
}
```

**Step 3: Add PendingBoosts field to PlayerSession**

In `internal/game/session/session.go`, find the `PlayerSession` struct and add:

```go
// PendingBoosts is the number of ability boosts the player has not yet assigned.
PendingBoosts int
```

Also add `Level`, `Experience`, `MaxHP`, `CurrentHP` if not already present. Check:

```bash
grep -n "Level\|Experience\|MaxHP\|CurrentHP\|PendingBoosts" /home/cjohannsen/src/mud/internal/game/session/session.go
```

**Step 4: Wire XP service into gameserver**

In `internal/gameserver/grpc_service.go`:

1. Add `xpSvc *xp.Service` field to `GameServiceServer`.
2. In `NewGameServiceServer`, accept and store `xpSvc`.
3. Load `PendingBoosts` from `progressRepo.GetProgress` during session init (after loading skills/feats).

In `internal/gameserver/combat_handler.go`:

In `removeDeadNPCsLocked`, after the existing loot award block, add XP award for the first living player:

```go
// Award kill XP to the first living player.
if h.xpSvc != nil {
    if killer := h.firstLivingPlayer(cbt); killer != nil {
        if charID := killer.CharacterID; charID > 0 {
            msgs, err := h.xpSvc.AwardKill(context.Background(), killer, inst.Level, charID)
            if err != nil {
                // log but don't fail combat
            }
            for _, msg := range msgs {
                h.broadcast(roomID, msg)
            }
        }
    }
}
```

Add `xpSvc *xp.Service` field to `CombatHandler` and pass it via constructor or setter.

In `internal/gameserver/grpc_service.go`, in the room move handler (around line 1067 where `newRoom` is retrieved):

Before calling `applyRoomSkillChecks`, check and award room discovery XP:

```go
// Award room discovery XP if this room is newly discovered.
if sess.AutomapCache[newRoom.ZoneID] == nil || !sess.AutomapCache[newRoom.ZoneID][newRoom.ID] {
    if s.xpSvc != nil {
        if msgs, err := s.xpSvc.AwardRoomDiscovery(stream.Context(), sess, characterID); err == nil {
            for _, msg := range msgs {
                // send as server message
            }
        }
    }
}
```

In `applyRoomSkillChecks` and `applyNPCSkillChecks`, after resolving each skill check, add XP award:

```go
if result.Outcome == skillcheck.CritSuccess || result.Outcome == skillcheck.Success {
    isCrit := result.Outcome == skillcheck.CritSuccess
    if s.xpSvc != nil {
        if sess, ok := s.sessions.GetPlayer(uid); ok {
            s.xpSvc.AwardSkillCheck(ctx, sess, trigger.DC, isCrit, sess.CharacterID) //nolint:errcheck
        }
    }
}
```

**Step 5: Run tests**

Run: `go test ./internal/game/xp/... -timeout 60s 2>&1`
Expected: all PASS

Run: `go test ./internal/gameserver/... -timeout 60s 2>&1 | tail -5`
Fix any compilation errors.

**Step 6: Commit**

```bash
git add internal/game/xp/service.go internal/game/xp/service_test.go \
    internal/game/session/session.go \
    internal/gameserver/grpc_service.go \
    internal/gameserver/combat_handler.go
git commit -m "feat: add XPService and wire kill/room/skill-check XP awards"
```

---

### Task 5: `levelup` command (CMD-1 through CMD-7)

**Files:**
- Modify: `internal/game/command/commands.go` (CMD-1, CMD-2)
- Create: `internal/game/command/levelup.go` (CMD-3)
- Modify: `api/proto/game/v1/game.proto` (CMD-4)
- Modify: `internal/frontend/handlers/bridge_handlers.go` (CMD-5)
- Modify: `internal/gameserver/grpc_service.go` (CMD-6)
- Modify: `internal/frontend/handlers/bridge_handlers_test.go` (CMD-7 wiring test)

**Step 1: CMD-1 + CMD-2 — add handler constant and BuiltinCommands entry**

In `internal/game/command/commands.go`, add after `HandlerProficiencies`:

```go
HandlerLevelUp = "levelup"
```

In `BuiltinCommands()`, add:

```go
{Name: "levelup", Aliases: []string{"lu"}, Help: "Assign a pending ability boost.", Category: CategoryWorld, Handler: HandlerLevelUp},
```

**Step 2: CMD-3 — implement HandleLevelUp**

Create `internal/game/command/levelup.go`:

```go
package command

import "strings"

// validAbilities lists the six ability names accepted by the levelup command.
var validAbilities = []string{"brutality", "quickness", "grit", "reasoning", "savvy", "flair"}

// HandleLevelUp validates and normalises the ability argument for the levelup command.
//
// Precondition: rawArgs is the raw argument string from the player (may be empty).
// Postcondition: Returns the lowercase ability name if valid; returns a Usage string starting with "Usage:" otherwise.
func HandleLevelUp(rawArgs string) string {
	ability := strings.ToLower(strings.TrimSpace(rawArgs))
	if ability == "" {
		return "Usage: levelup <ability>\nChoose one: brutality, quickness, grit, reasoning, savvy, flair"
	}
	for _, valid := range validAbilities {
		if ability == valid {
			return ability
		}
	}
	return "Usage: levelup <ability>\nChoose one: brutality, quickness, grit, reasoning, savvy, flair"
}
```

Write test `internal/game/command/levelup_test.go`:

```go
package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/command"
)

func TestHandleLevelUp_ValidAbilities(t *testing.T) {
	for _, ab := range []string{"brutality", "quickness", "grit", "reasoning", "savvy", "flair"} {
		assert.Equal(t, ab, command.HandleLevelUp(ab))
		assert.Equal(t, ab, command.HandleLevelUp(strings.ToUpper(ab)))
	}
}

func TestHandleLevelUp_EmptyReturnsUsage(t *testing.T) {
	result := command.HandleLevelUp("")
	assert.True(t, strings.HasPrefix(result, "Usage:"))
}

func TestHandleLevelUp_InvalidReturnsUsage(t *testing.T) {
	result := command.HandleLevelUp("charisma")
	assert.True(t, strings.HasPrefix(result, "Usage:"))
}

func TestProperty_HandleLevelUp_OnlyValidAbilitiesPass(t *testing.T) {
	valid := map[string]bool{"brutality": true, "quickness": true, "grit": true, "reasoning": true, "savvy": true, "flair": true}
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.StringMatching(`[a-z]{1,20}`).Draw(rt, "input")
		result := command.HandleLevelUp(input)
		if valid[input] {
			if result != input {
				rt.Fatalf("valid ability %q returned %q", input, result)
			}
		} else {
			if !strings.HasPrefix(result, "Usage:") {
				rt.Fatalf("invalid ability %q returned non-usage %q", input, result)
			}
		}
	})
}
```

Note: add `"strings"` import to the test file.

Run: `go test ./internal/game/command/... -run TestHandleLevelUp -v`
Expected: PASS

**Step 3: CMD-4 — add proto message**

In `api/proto/game/v1/game.proto`, add after the last request message before `ClientMessage`:

```protobuf
// LevelUpRequest asks the server to apply a pending ability boost.
message LevelUpRequest {
  string ability = 1; // one of: brutality, quickness, grit, reasoning, savvy, flair
}
```

In the `ClientMessage` oneof, add:

```protobuf
LevelUpRequest level_up_request = NN; // use next available field number
```

Run: `cd /home/cjohannsen/src/mud && make proto`
Expected: `internal/gameserver/gamev1/game.pb.go` regenerated cleanly.

**Step 4: CMD-5 — bridge handler**

In `internal/frontend/handlers/bridge_handlers.go`, add to `bridgeHandlerMap`:

```go
command.HandlerLevelUp: bridgeLevelUp,
```

Add the handler function:

```go
// bridgeLevelUp sends a LevelUpRequest with the chosen ability.
//
// Precondition: bctx must be non-nil with a valid reqID.
// Postcondition: Returns usage text as done=true if ability is invalid;
// otherwise returns a non-nil msg containing a LevelUpRequest.
func bridgeLevelUp(bctx *bridgeContext) (bridgeResult, error) {
	parsed := command.HandleLevelUp(bctx.parsed.RawArgs)
	if strings.HasPrefix(parsed, "Usage:") {
		return writeErrorPrompt(bctx, parsed)
	}
	return bridgeResult{msg: &gamev1.ClientMessage{
		RequestId: bctx.reqID,
		Payload:   &gamev1.ClientMessage_LevelUpRequest{LevelUpRequest: &gamev1.LevelUpRequest{Ability: parsed}},
	}}, nil
}
```

Run: `go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v`
Expected: PASS

**Step 5: CMD-6 — server handler**

In `internal/gameserver/grpc_service.go`, add to the dispatch switch:

```go
case *gamev1.ClientMessage_LevelUpRequest:
    return s.handleLevelUp(uid, msg.Payload.(*gamev1.ClientMessage_LevelUpRequest).LevelUpRequest.Ability)
```

Add the handler function:

```go
// handleLevelUp applies a pending ability boost for the player.
//
// Precondition: uid must identify an active session; ability must be one of the six valid abilities.
// Postcondition: If the player has a pending boost and ability is valid, the boost is applied
// and persisted; otherwise an error message is returned.
func (s *GameServiceServer) handleLevelUp(uid, ability string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return errorEvent("session not found"), nil
	}
	if sess.PendingBoosts <= 0 {
		return s.plainTextEvent("You have no pending ability boosts."), nil
	}

	validAbilities := map[string]bool{
		"brutality": true, "quickness": true, "grit": true,
		"reasoning": true, "savvy": true, "flair": true,
	}
	if !validAbilities[ability] {
		return s.plainTextEvent("Unknown ability. Choose: brutality, quickness, grit, reasoning, savvy, flair"), nil
	}

	applyAbilityBoost(&sess.Abilities, ability, 2)
	sess.PendingBoosts--

	characterID := sess.CharacterID
	ctx := context.Background()
	if err := s.charSaver.SaveAbilities(ctx, characterID, sess.Abilities); err != nil {
		return errorEvent(fmt.Sprintf("saving abilities: %v", err)), nil
	}
	if err := s.progressRepo.ConsumePendingBoost(ctx, characterID); err != nil {
		return errorEvent(fmt.Sprintf("consuming pending boost: %v", err)), nil
	}

	return s.plainTextEvent(fmt.Sprintf("Your %s increased by 2 (now %d)!", ability, abilityScore(&sess.Abilities, ability))), nil
}
```

Also add helper functions `applyAbilityBoost` and `abilityScore` (or reuse existing patterns):

```go
func applyAbilityBoost(abilities *character.AbilityScores, ability string, amount int) {
	switch ability {
	case "brutality":  abilities.Brutality += amount
	case "quickness":  abilities.Quickness += amount
	case "grit":       abilities.Grit += amount
	case "reasoning":  abilities.Reasoning += amount
	case "savvy":      abilities.Savvy += amount
	case "flair":      abilities.Flair += amount
	}
}

func abilityScore(abilities *character.AbilityScores, ability string) int {
	switch ability {
	case "brutality":  return abilities.Brutality
	case "quickness":  return abilities.Quickness
	case "grit":       return abilities.Grit
	case "reasoning":  return abilities.Reasoning
	case "savvy":      return abilities.Savvy
	case "flair":      return abilities.Flair
	default:           return 0
	}
}
```

**Step 6: Run all tests**

Run: `go test ./... -timeout 180s 2>&1 | grep -E "^(ok|FAIL)"`
Expected: all ok (postgres may take up to 120s).

**Step 7: Commit**

```bash
git add internal/game/command/commands.go internal/game/command/levelup.go \
    internal/game/command/levelup_test.go \
    api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go \
    internal/frontend/handlers/bridge_handlers.go \
    internal/gameserver/grpc_service.go
git commit -m "feat: add levelup command end-to-end (CMD-1 through CMD-7)"
```

---

### Task 6: Add XP and pending boosts to character sheet

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/grpc_service.go` (handleCharSheet)
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/text_renderer_test.go`

**Step 1: Add proto fields to CharacterSheetView**

In `api/proto/game/v1/game.proto`, in `CharacterSheetView`, add after `cool_save = 28`:

```protobuf
int32  experience     = 29;
int32  xp_to_next     = 30; // XP needed to reach next level (0 if at cap)
int32  pending_boosts = 31;
```

Run: `make proto`

**Step 2: Populate in handleCharSheet**

In `internal/gameserver/grpc_service.go`, in `handleCharSheet` (around line 2300), after `view.Level = int32(sess.Level)`, add:

```go
view.Experience = int32(sess.Experience)
if sess.Level < s.xpCfg.LevelCap {
	view.XpToNext = int32(xp.XPToLevel(sess.Level+1, s.xpCfg.BaseXP))
}
view.PendingBoosts = int32(sess.PendingBoosts)
```

**Step 3: Write failing renderer test**

In `internal/frontend/handlers/text_renderer_test.go`, add:

```go
func TestRenderCharacterSheet_XPSection(t *testing.T) {
	view := &gamev1.CharacterSheetView{
		Name:          "Hero",
		Level:         5,
		Experience:    2500,
		XpToNext:      3600,
		PendingBoosts: 1,
	}
	result := RenderCharacterSheet(view, 80)
	assert.Contains(t, result, "XP:")
	assert.Contains(t, result, "2500")
	assert.Contains(t, result, "3600")
	assert.Contains(t, result, "Boost")
}
```

Run: `go test ./internal/frontend/handlers/... -run TestRenderCharacterSheet_XPSection -v`
Expected: FAIL

**Step 4: Add XP section to RenderCharacterSheet**

In `internal/frontend/handlers/text_renderer.go`, in `RenderCharacterSheet`, after the `--- Saves ---` section, add to the `left` column:

```go
left = append(left, slPlain(""))
left = append(left, sl(telnet.Colorize(telnet.BrightCyan, "--- Progress ---")))
xpLine := fmt.Sprintf("XP: %d", csv.GetExperience())
if csv.GetXpToNext() > 0 {
    xpLine += fmt.Sprintf(" / %d", csv.GetXpToNext())
}
left = append(left, slPlain(xpLine))
if csv.GetPendingBoosts() > 0 {
    left = append(left, slPlain(fmt.Sprintf("Ability Boost Pending: %d  (type 'levelup')", csv.GetPendingBoosts())))
}
```

**Step 5: Run tests**

Run: `go test ./internal/frontend/handlers/... -timeout 120s 2>&1 | tail -5`
Expected: PASS

**Step 6: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go \
    internal/gameserver/grpc_service.go \
    internal/frontend/handlers/text_renderer.go \
    internal/frontend/handlers/text_renderer_test.go
git commit -m "feat: add XP and pending boosts to character sheet"
```

---

### Task 7: Update FEATURES.md, run full test suite, deploy

**Step 1: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, find `- [ ] Character levelling` and mark sub-items complete:

```markdown
- [x] Character levelling
    - [x] Max character level limit is 100
    - [x] Max job level for any job is 20 (reserved; cap enforced)
    - [x] XP awarded for combat kills, room discovery, skill checks
    - [x] Geometric XP curve: level² × base_xp (configurable via content/xp_config.yaml)
    - [x] Automatic level-up: HP increase + proficiency bonus scales automatically
    - [x] Ability boost every 5 levels via `levelup` command
    - [x] XP and pending boosts shown on character sheet
```

**Step 2: Run full test suite**

```bash
go test ./... -timeout 180s 2>&1 | grep -E "^(ok|FAIL)"
```

Expected: all ok (postgres ~103s, everything else fast).

**Step 3: Commit and deploy**

```bash
git add docs/requirements/FEATURES.md
git commit -m "feat: character levelling complete — XP, level-up, ability boosts"
make k8s-redeploy 2>&1 | tail -8
```
