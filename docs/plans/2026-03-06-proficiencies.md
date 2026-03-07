# Proficiencies Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Add armor and weapon proficiency tracking per character, apply PF2E strict proficiency bonus (+0 untrained / level+2 trained) to attack rolls and AC, and expose a `proficiencies` command and character sheet section.

**Architecture:** Pure pipeline addition — new `proficiency_category` field in weapon/armor YAML, new `character_proficiencies` DB table backfilled at login from job YAML (same pattern as `character_skills`), proficiency rank stored on `Combatant` and used in both resolvers, new proto messages and command following the established CMD-1..CMD-7 wiring pattern.

**Tech Stack:** YAML content files, PostgreSQL migration (SQL), Go (`pgx/v5`, `pgregory.net/rapid`), protobuf (`make proto`), Telnet split-screen renderer.

---

### Task 1: Add `proficiency_category` to all weapon YAMLs and validate in WeaponDef

**Files:**
- Modify: `internal/game/inventory/weapon.go` — add `ProficiencyCategory` field to `WeaponDef`, add validation
- Modify: all 41 files in `content/weapons/` — add `proficiency_category:` line
- Modify: `internal/game/inventory/weapon_test.go` (or create if absent) — add content validation test

**Valid weapon proficiency categories:** `simple_weapons`, `simple_ranged`, `martial_weapons`, `martial_ranged`, `martial_melee`, `unarmed`, `specialized`

**Weapon → category assignments:**
| File | proficiency_category |
|------|---------------------|
| anti_materiel_rifle | martial_ranged |
| assault_rifle | martial_ranged |
| battle_rifle | martial_ranged |
| bayonet | martial_melee |
| ceramic_shiv | simple_weapons |
| chainsaw | martial_weapons |
| cheap_blade | simple_weapons |
| cleaver | simple_weapons |
| combat_knife | martial_melee |
| combat_shotgun | martial_ranged |
| corp_security_smg | martial_ranged |
| cyberlinked_smg | martial_ranged |
| dock_hook | simple_weapons |
| emp_pistol | specialized |
| flamethrower | specialized |
| flechette_pistol | martial_ranged |
| flechette_spreader | martial_ranged |
| ganger_pistol | simple_ranged |
| golf_club | simple_weapons |
| grapple_gun | specialized |
| harvest_sickle | simple_weapons |
| heavy_revolver | martial_ranged |
| holdout_derringer | simple_ranged |
| laser_rifle | specialized |
| mono_wire_whip | specialized |
| net_launcher | specialized |
| railgun_carbine | specialized |
| rebar_club | simple_weapons |
| riot_shotgun | simple_ranged |
| sawn_off | simple_ranged |
| smartgun_pistol | martial_ranged |
| sniper_rifle | martial_ranged |
| sonic_disruptor | specialized |
| spiked_knuckles | unarmed |
| steel_pipe | simple_weapons |
| street_sweeper_smg | simple_ranged |
| stun_baton | simple_weapons |
| suburban_machete | simple_weapons |
| suppressed_smg | martial_ranged |
| tactical_hatchet | martial_melee |
| vibroblade | martial_melee |

**Step 1: Add `ProficiencyCategory` to `WeaponDef` in `internal/game/inventory/weapon.go`**

In the `WeaponDef` struct (after the `Group` field at line ~50), add:
```go
ProficiencyCategory string `yaml:"proficiency_category"` // e.g. "simple_weapons", "martial_ranged"
```

In `WeaponDef.Validate()`, add after the existing group validation:
```go
validWeaponProfCategories := map[string]bool{
    "simple_weapons": true, "simple_ranged": true, "martial_weapons": true,
    "martial_ranged": true, "martial_melee": true, "unarmed": true, "specialized": true,
}
if !validWeaponProfCategories[w.ProficiencyCategory] {
    errs = append(errs, fmt.Errorf("proficiency_category %q is not valid", w.ProficiencyCategory))
}
```

**Step 2: Add `proficiency_category:` to each of the 41 weapon YAML files**

For each file in `content/weapons/`, add the line `proficiency_category: <value>` directly after the `group:` line (or `team_affinity:` if `group` is absent). Use the table above.

Example for `content/weapons/assault_rifle.yaml`:
```yaml
group: firearm
proficiency_category: martial_ranged
```

**Step 3: Write failing content validation test**

In `internal/game/inventory/weapon_test.go`, add:
```go
func TestAllWeaponsHaveProficiencyCategory(t *testing.T) {
    weapons, err := LoadWeapons("../../../content/weapons")
    require.NoError(t, err)
    require.NotEmpty(t, weapons)
    for _, w := range weapons {
        assert.NotEmpty(t, w.ProficiencyCategory, "weapon %s missing proficiency_category", w.ID)
        err := w.Validate()
        assert.NoError(t, err, "weapon %s failed validation", w.ID)
    }
}
```

**Step 4: Run the test to verify it fails (before adding YAML fields)**

```bash
go test ./internal/game/inventory/... -run TestAllWeaponsHaveProficiencyCategory -v
```
Expected: FAIL — weapons missing proficiency_category.

**Step 5: Add `proficiency_category` to all 41 weapon YAMLs (Step 2 above)**

**Step 6: Run test to verify it passes**

```bash
go test ./internal/game/inventory/... -run TestAllWeaponsHaveProficiencyCategory -v
```
Expected: PASS.

**Step 7: Run full inventory test suite**

```bash
go test ./internal/game/inventory/... -count=1 -v 2>&1 | tail -10
```
Expected: all PASS.

**Step 8: Commit**

```bash
git add internal/game/inventory/weapon.go content/weapons/ internal/game/inventory/weapon_test.go
git commit -m "feat: add proficiency_category to all weapon YAMLs and WeaponDef"
```

---

### Task 2: Add `proficiency_category` to all armor YAMLs and validate in ArmorDef

**Files:**
- Modify: `internal/game/inventory/armor.go` — add `ProficiencyCategory` field to `ArmorDef`, add validation
- Modify: all 35 files in `content/armor/` — add `proficiency_category:` line
- Modify: `internal/game/inventory/armor_test.go` (or create if absent)

**Valid armor proficiency categories:** `unarmored`, `light_armor`, `medium_armor`, `heavy_armor`

**Armor → category assignments (by group):**
- `group: leather` → `light_armor`
- `group: chain` → `medium_armor`
- `group: composite` → `medium_armor`
- `group: plate` → `heavy_armor`

Files and their assignments:
| File | proficiency_category |
|------|---------------------|
| armored_gauntlets | heavy_armor |
| ballistic_cap | light_armor |
| combat_helmet | medium_armor |
| corp_security_helm | heavy_armor |
| corp_suit_liner | medium_armor |
| exo_frame_feet | heavy_armor |
| exo_frame_torso | heavy_armor |
| fingerless_gloves | light_armor |
| kevlar_vest | medium_armor |
| leather_jacket | light_armor |
| left_arm_guards | light_armor |
| left_ballistic_leggings | medium_armor |
| left_ballistic_sleeve | medium_armor |
| left_exo_frame_arm | heavy_armor |
| left_exo_frame_leg | heavy_armor |
| left_leg_guards | light_armor |
| left_tactical_greaves | medium_armor |
| left_tactical_vambrace | medium_armor |
| mag_boots | medium_armor |
| military_plate | heavy_armor |
| neural_interface_helm | medium_armor |
| right_arm_guards | light_armor |
| right_ballistic_leggings | medium_armor |
| right_ballistic_sleeve | medium_armor |
| right_exo_frame_arm | heavy_armor |
| right_exo_frame_leg | heavy_armor |
| right_leg_guards | light_armor |
| right_tactical_greaves | medium_armor |
| right_tactical_vambrace | medium_armor |
| riot_visor | medium_armor |
| shock_gloves | medium_armor |
| street_boots | light_armor |
| tactical_boots | medium_armor |
| tactical_gloves | light_armor |
| tactical_vest | medium_armor |

**Step 1: Add `ProficiencyCategory` to `ArmorDef` in `internal/game/inventory/armor.go`**

In the `ArmorDef` struct (after the `Group` field), add:
```go
ProficiencyCategory string `yaml:"proficiency_category"` // "unarmored","light_armor","medium_armor","heavy_armor"
```

In `ArmorDef.Validate()`, add:
```go
validArmorProfCategories := map[string]bool{
    "unarmored": true, "light_armor": true, "medium_armor": true, "heavy_armor": true,
}
if !validArmorProfCategories[a.ProficiencyCategory] {
    errs = append(errs, fmt.Errorf("proficiency_category %q is not valid", a.ProficiencyCategory))
}
```

**Step 2: Write failing test**

In `internal/game/inventory/armor_test.go`, add:
```go
func TestAllArmorHasProficiencyCategory(t *testing.T) {
    armors, err := LoadArmors("../../../content/armor")
    require.NoError(t, err)
    require.NotEmpty(t, armors)
    for _, a := range armors {
        assert.NotEmpty(t, a.ProficiencyCategory, "armor %s missing proficiency_category", a.ID)
        err := a.Validate()
        assert.NoError(t, err, "armor %s failed validation", a.ID)
    }
}
```

**Step 3: Run to verify FAIL, add YAML fields, run to verify PASS**

```bash
go test ./internal/game/inventory/... -run TestAllArmorHasProficiencyCategory -v
```
After adding all `proficiency_category:` lines to the 35 armor files:
```bash
go test ./internal/game/inventory/... -count=1 -v 2>&1 | tail -10
```
Expected: all PASS.

**Step 4: Commit**

```bash
git add internal/game/inventory/armor.go content/armor/ internal/game/inventory/armor_test.go
git commit -m "feat: add proficiency_category to all armor YAMLs and ArmorDef"
```

---

### Task 3: DB migration and CharacterProficienciesRepository

**Files:**
- Create: `migrations/018_character_proficiencies.up.sql`
- Create: `migrations/018_character_proficiencies.down.sql`
- Create: `internal/storage/postgres/character_proficiencies.go`
- Create: `internal/storage/postgres/character_proficiencies_test.go`

**Step 1: Create migration files**

`migrations/018_character_proficiencies.up.sql`:
```sql
CREATE TABLE character_proficiencies (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    category     TEXT   NOT NULL,
    rank         TEXT   NOT NULL DEFAULT 'untrained',
    PRIMARY KEY (character_id, category)
);
```

`migrations/018_character_proficiencies.down.sql`:
```sql
DROP TABLE IF EXISTS character_proficiencies;
```

**Step 2: Write failing tests for the repository**

`internal/storage/postgres/character_proficiencies_test.go`:
```go
package postgres_test

// These tests require a real Postgres connection and are skipped if DB_DSN is unset.
// Pattern identical to character_skills_test.go.

import (
    "context"
    "os"
    "testing"

    "github.com/cory-johannsen/mud/internal/storage/postgres"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    pgregory "pgregory.net/rapid"
)

func newProfTestPool(t *testing.T) *pgxpool.Pool {
    t.Helper()
    dsn := os.Getenv("DB_DSN")
    if dsn == "" {
        t.Skip("DB_DSN not set")
    }
    pool, err := pgxpool.New(context.Background(), dsn)
    require.NoError(t, err)
    t.Cleanup(pool.Close)
    return pool
}

func TestCharacterProficienciesRepository_GetAll_Empty(t *testing.T) {
    pool := newProfTestPool(t)
    repo := postgres.NewCharacterProficienciesRepository(pool)
    // Use a character ID that cannot exist (negative).
    profs, err := repo.GetAll(context.Background(), -1)
    require.NoError(t, err)
    assert.Empty(t, profs)
}

func TestCharacterProficienciesRepository_Upsert_Idempotent(t *testing.T) {
    pgregory.Run(t, func(rt *pgregory.T) {
        pool := newProfTestPool(t)
        repo := postgres.NewCharacterProficienciesRepository(pool)
        ctx := context.Background()
        // Insert twice — must not error or duplicate.
        err := repo.Upsert(ctx, 1, "light_armor", "trained")
        assert.NoError(rt, err)
        err = repo.Upsert(ctx, 1, "light_armor", "trained")
        assert.NoError(rt, err)
        profs, err := repo.GetAll(ctx, 1)
        require.NoError(rt, err)
        assert.Equal(rt, "trained", profs["light_armor"])
    })
}
```

**Step 3: Implement `CharacterProficienciesRepository`**

`internal/storage/postgres/character_proficiencies.go`:
```go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

// CharacterProficienciesRepository persists per-character armor/weapon proficiency ranks.
type CharacterProficienciesRepository struct {
    db *pgxpool.Pool
}

// NewCharacterProficienciesRepository creates a repository backed by the given pool.
//
// Precondition: db must be a valid, open connection pool.
func NewCharacterProficienciesRepository(db *pgxpool.Pool) *CharacterProficienciesRepository {
    return &CharacterProficienciesRepository{db: db}
}

// GetAll returns all proficiency ranks for a character as a category→rank map.
//
// Precondition: characterID > 0.
// Postcondition: Returns an empty map (not nil) if no rows exist.
func (r *CharacterProficienciesRepository) GetAll(ctx context.Context, characterID int64) (map[string]string, error) {
    if characterID <= 0 {
        return make(map[string]string), nil
    }
    rows, err := r.db.Query(ctx,
        `SELECT category, rank FROM character_proficiencies WHERE character_id = $1`, characterID,
    )
    if err != nil {
        return nil, fmt.Errorf("GetAll proficiencies: %w", err)
    }
    defer rows.Close()
    out := make(map[string]string)
    for rows.Next() {
        var cat, rank string
        if err := rows.Scan(&cat, &rank); err != nil {
            return nil, fmt.Errorf("scanning proficiency row: %w", err)
        }
        out[cat] = rank
    }
    return out, rows.Err()
}

// Upsert inserts or updates a single proficiency rank for a character.
// If the (character_id, category) row already exists, the rank is updated.
//
// Precondition: characterID > 0; category and rank must not be empty.
// Postcondition: Exactly one row exists for (characterID, category).
func (r *CharacterProficienciesRepository) Upsert(ctx context.Context, characterID int64, category, rank string) error {
    if characterID <= 0 {
        return fmt.Errorf("Upsert proficiency: characterID must be > 0")
    }
    if category == "" {
        return fmt.Errorf("Upsert proficiency: category must not be empty")
    }
    if rank == "" {
        return fmt.Errorf("Upsert proficiency: rank must not be empty")
    }
    _, err := r.db.Exec(ctx,
        `INSERT INTO character_proficiencies (character_id, category, rank) VALUES ($1, $2, $3)
         ON CONFLICT (character_id, category) DO UPDATE SET rank = EXCLUDED.rank`,
        characterID, category, rank,
    )
    if err != nil {
        return fmt.Errorf("Upsert proficiency %s: %w", category, err)
    }
    return nil
}
```

**Step 4: Run migration locally**

```bash
make migrate CONFIG=configs/dev.yaml
```
Expected: `migrated up to version=18 dirty=false [...]`

**Step 5: Run repository tests (skipped if no DB_DSN)**

```bash
go test ./internal/storage/postgres/... -run TestCharacterProficiencies -v 2>&1 | tail -10
```

**Step 6: Commit**

```bash
git add migrations/018_character_proficiencies.up.sql migrations/018_character_proficiencies.down.sql \
    internal/storage/postgres/character_proficiencies.go internal/storage/postgres/character_proficiencies_test.go
git commit -m "feat: add character_proficiencies table and repository"
```

---

### Task 4: PlayerSession.Proficiencies, backfill, and session load

**Files:**
- Modify: `internal/game/session/manager.go` — add `Proficiencies map[string]string` to `PlayerSession` and `SessionOptions`
- Modify: `internal/gameserver/grpc_service.go` — add repo field, backfill block, and load block
- Modify: `cmd/gameserver/main.go` — wire new repo into GameServiceServer

**Step 1: Add `Proficiencies` to `PlayerSession` in `internal/game/session/manager.go`**

In the `PlayerSession` struct (after `Skills map[string]string` at ~line 56), add:
```go
// Proficiencies maps proficiency category to rank for the active character.
// Populated after backfill completes; empty map means all untrained.
Proficiencies map[string]string
```

In `SessionOptions` (after `Skills map[string]string`), add:
```go
Proficiencies map[string]string
```

In `NewPlayerSession` (where abilities/skills are copied), add:
```go
proficiencies := opts.Proficiencies
if proficiencies == nil {
    proficiencies = make(map[string]string)
}
```
And set on the returned session:
```go
Proficiencies: proficiencies,
```

**Step 2: Add repository field and interface to `internal/gameserver/grpc_service.go`**

After the `CharacterSkillsRepository` interface (around line 82), add:
```go
// CharacterProficienciesRepository persists per-character armor/weapon proficiency data.
type CharacterProficienciesRepository interface {
    GetAll(ctx context.Context, characterID int64) (map[string]string, error)
    Upsert(ctx context.Context, characterID int64, category, rank string) error
}
```

In `GameServiceServer` struct, after `characterSkillsRepo`, add:
```go
characterProficienciesRepo CharacterProficienciesRepository
```

In `NewGameServiceServer` (constructor), add the new parameter after `characterSkillsRepo`:
```go
characterProficienciesRepo CharacterProficienciesRepository
```
And assign: `s.characterProficienciesRepo = characterProficienciesRepo`.

**Step 3: Add backfill + load block to `Session()` in `internal/gameserver/grpc_service.go`**

After the skill backfill/load block (around line 513), add a proficiency backfill block:

```go
// Proficiency backfill: assign job proficiencies for characters that have none.
// Always adds `unarmored: trained` (PF2E baseline for all characters).
if characterID > 0 && s.characterProficienciesRepo != nil && s.jobRegistry != nil {
    existing, profCheckErr := s.characterProficienciesRepo.GetAll(stream.Context(), characterID)
    if profCheckErr != nil {
        s.logger.Warn("checking character proficiencies for backfill",
            zap.Int64("character_id", characterID),
            zap.Error(profCheckErr),
        )
    } else {
        // Always ensure unarmored is trained.
        profMap := map[string]string{"unarmored": "trained"}
        if job, ok := s.jobRegistry.Job(sess.Class); ok {
            for cat, rank := range job.Proficiencies {
                profMap[cat] = rank
            }
        }
        for cat, rank := range profMap {
            if _, alreadySet := existing[cat]; !alreadySet {
                if upsertErr := s.characterProficienciesRepo.Upsert(
                    stream.Context(), characterID, cat, rank,
                ); upsertErr != nil {
                    s.logger.Error("upserting proficiency",
                        zap.String("category", cat),
                        zap.Error(upsertErr),
                    )
                }
            }
        }
    }
    // Load proficiencies into session.
    profMap, loadProfErr := s.characterProficienciesRepo.GetAll(stream.Context(), characterID)
    if loadProfErr != nil {
        s.logger.Warn("loading character proficiencies into session",
            zap.Int64("character_id", characterID),
            zap.Error(loadProfErr),
        )
    } else {
        if sess.Proficiencies == nil {
            sess.Proficiencies = make(map[string]string)
        }
        for cat, rank := range profMap {
            sess.Proficiencies[cat] = rank
        }
    }
}
```

**Step 4: Wire the new repo in `cmd/gameserver/main.go`**

Find where `NewGameServiceServer` is called and add `postgres.NewCharacterProficienciesRepository(pool)` as the new argument in the correct position.

**Step 5: Run the compiler to verify no errors**

```bash
go build ./... 2>&1
```
Expected: no errors.

**Step 6: Run tests that exercise session loading**

```bash
go test ./internal/gameserver/... -count=1 -timeout=120s 2>&1 | tail -10
```
Expected: all PASS (any `nil` for the new repo arg in test constructors is handled by the nil-guard).

**Step 7: Commit**

```bash
git add internal/game/session/manager.go internal/gameserver/grpc_service.go cmd/gameserver/main.go
git commit -m "feat: add Proficiencies to PlayerSession; backfill and load from DB on login"
```

---

### Task 5: Rank-aware proficiency bonus in combat resolvers

**Files:**
- Modify: `internal/game/combat/combat.go` — change `ProficiencyBonus` to rank-aware version
- Modify: `internal/game/combat/resolver.go` — update both resolvers to use rank
- Modify: `internal/game/combat/combat.go` — add `WeaponProficiencyRank` and `ArmorProficiencyRank` to `Combatant`
- Modify: `internal/gameserver/combat_handler.go` — populate ranks when building combatants
- Modify: `internal/game/combat/combat_test.go` — update ProficiencyBonus tests
- Modify: `internal/game/combat/resolver_test.go` — update resolver tests

**Step 1: Update `ProficiencyBonus` in `internal/game/combat/combat.go`**

Change the existing signature (line ~109):
```go
// Before:
func ProficiencyBonus(level int) int {
    return 2 + (level-1)/4
}
```
To (PF2E strict: untrained=0, trained=level+2, scales for future ranks):
```go
// CombatProficiencyBonus returns the PF2E proficiency bonus for an attack or AC calculation.
// Untrained: 0. Trained: level+2. Expert: level+4. Master: level+6. Legendary: level+8.
//
// Precondition: level >= 1; rank is one of "untrained","trained","expert","master","legendary" or "".
// Postcondition: returns 0 for untrained/empty; level+2 for trained; etc.
func CombatProficiencyBonus(level int, rank string) int {
    switch rank {
    case "trained":
        return level + 2
    case "expert":
        return level + 4
    case "master":
        return level + 6
    case "legendary":
        return level + 8
    default:
        return 0
    }
}
```

Keep the old `ProficiencyBonus(level int) int` as a deprecated shim for any other callers:
```go
// ProficiencyBonus returns the combat proficiency bonus assuming trained rank.
// Deprecated: use CombatProficiencyBonus with an explicit rank.
func ProficiencyBonus(level int) int {
    return CombatProficiencyBonus(level, "trained")
}
```

**Step 2: Add rank fields to `Combatant` struct in `internal/game/combat/combat.go`**

After the `NPCType` field, add:
```go
// WeaponProficiencyRank is the character's proficiency rank for their equipped weapon category.
// Empty string or "untrained" → no proficiency bonus on attack rolls.
WeaponProficiencyRank string
// ArmorProficiencyRank is the character's proficiency rank for their equipped armor category.
// Empty string or "untrained" → no proficiency bonus to AC.
ArmorProficiencyRank string
```

**Step 3: Update `ResolveAttack` in `internal/game/combat/resolver.go`**

Change line ~55:
```go
// Before:
atkMod := attacker.StrMod + ProficiencyBonus(attacker.Level)

// After:
atkMod := attacker.StrMod + CombatProficiencyBonus(attacker.Level, attacker.WeaponProficiencyRank)
```

**Step 4: Update `ResolveFirearmAttack` in `internal/game/combat/resolver.go`**

Change line ~100:
```go
// Before:
profBonus := 2 + (attacker.Level-1)/4

// After:
profBonus := CombatProficiencyBonus(attacker.Level, attacker.WeaponProficiencyRank)
```

**Step 5: Update `startCombatLocked` in `internal/gameserver/combat_handler.go`**

Populate the player combatant's proficiency ranks from `sess.Proficiencies`.

First, determine the weapon proficiency category from the equipped weapon:
```go
// Determine weapon proficiency rank.
weaponProfRank := "untrained"
if playerCbt.Loadout != nil {
    if wDef := h.invRegistry.WeaponByID(playerCbt.Loadout.WeaponID); wDef != nil {
        cat := wDef.ProficiencyCategory
        if r, ok := sess.Proficiencies[cat]; ok {
            weaponProfRank = r
        }
    }
}
playerCbt.WeaponProficiencyRank = weaponProfRank

// Determine armor proficiency rank.
armorProfRank := "trained" // unarmored is always trained
defStats := sess.Equipment.ComputedDefenses(h.invRegistry, dexMod)
if defStats.BodyArmorCategory != "" {
    if r, ok := sess.Proficiencies[defStats.BodyArmorCategory]; ok {
        armorProfRank = r
    }
}
playerCbt.ArmorProficiencyRank = armorProfRank

// Update AC to include proficiency bonus.
playerAC = 10 + defStats.ACBonus + defStats.EffectiveDex + combat.CombatProficiencyBonus(1, armorProfRank)
playerCbt.AC = playerAC
```

Note: `defStats.BodyArmorCategory` needs to be added to `EquipmentStats` — see sub-step below.

**Step 5a: Add `BodyArmorCategory` to `EquipmentStats` in `internal/game/inventory/equipment.go`**

In `EquipmentStats` struct (around line 110), add:
```go
BodyArmorCategory string // proficiency_category of the equipped torso armor; "" if unarmored
```

In `ComputedDefenses()` (where it computes ACBonus), when iterating equipped slots and finding torso slot armor, set:
```go
stats.BodyArmorCategory = def.ProficiencyCategory
```

**Step 6: Write failing tests**

In `internal/game/combat/combat_test.go`, add property-based tests:
```go
func TestCombatProficiencyBonus_UntrainedAlwaysZero(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        level := rapid.IntRange(1, 20).Draw(rt, "level")
        got := combat.CombatProficiencyBonus(level, "untrained")
        assert.Equal(rt, 0, got)
    })
}

func TestCombatProficiencyBonus_TrainedIsLevelPlusTwo(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        level := rapid.IntRange(1, 20).Draw(rt, "level")
        got := combat.CombatProficiencyBonus(level, "trained")
        assert.Equal(rt, level+2, got)
    })
}

func TestCombatProficiencyBonus_ExpertGreaterThanTrained(t *testing.T) {
    rapid.Check(t, func(rt *rapid.T) {
        level := rapid.IntRange(1, 20).Draw(rt, "level")
        expert := combat.CombatProficiencyBonus(level, "expert")
        trained := combat.CombatProficiencyBonus(level, "trained")
        assert.Greater(rt, expert, trained)
    })
}
```

**Step 7: Run tests to verify fail, implement, verify pass**

```bash
go test ./internal/game/combat/... -run "TestCombatProficiencyBonus" -v
```
Expected before implementation: FAIL (function doesn't exist).
After implementation: PASS.

```bash
go test ./internal/game/combat/... -count=1 -timeout=60s 2>&1 | tail -10
```
Expected: all PASS.

**Step 8: Commit**

```bash
git add internal/game/combat/combat.go internal/game/combat/resolver.go \
    internal/gameserver/combat_handler.go internal/game/inventory/equipment.go \
    internal/game/combat/combat_test.go
git commit -m "feat: rank-aware combat proficiency bonus; wire weapon/armor rank into Combatant"
```

---

### Task 6: `proficiencies` command — proto, bridge, handler, grpc dispatch, text render

**Files:**
- Modify: `api/proto/game/v1/game.proto` — add `ProficienciesRequest`, `ProficiencyEntry`, `ProficienciesResponse`
- Run: `make proto`
- Modify: `internal/game/command/commands.go` — add `HandlerProficiencies` constant and `Command` entry
- Create: `internal/game/command/proficiencies.go` — `HandleProficiencies` (client-side only, sends request)
- Modify: `internal/frontend/handlers/bridge_handlers.go` — add `bridgeProficiencies`
- Modify: `internal/gameserver/grpc_service.go` — add `handleProficiencies`; wire into dispatch
- Modify: `internal/frontend/handlers/game_bridge.go` — render `ProficienciesResponse`

**Step 1: Add proto messages to `api/proto/game/v1/game.proto`**

In the `ClientMessage` oneof (after `SummonItemRequest summon_item = 43;`), add:
```proto
ProficienciesRequest proficiencies_request = 44;
```

In the `ServerEvent` oneof (after `ClassFeaturesResponse class_features_response = 23;`), add:
```proto
ProficienciesResponse proficiencies_response = 24;
```

Add new message definitions (after `SkillsResponse`):
```proto
// ProficienciesRequest is sent by the client to view armor/weapon proficiencies.
message ProficienciesRequest {}

// ProficiencyEntry represents one proficiency category with its rank and bonus.
message ProficiencyEntry {
  string category   = 1; // e.g. "light_armor", "simple_weapons"
  string name       = 2; // e.g. "Light Armor", "Simple Weapons"
  string rank       = 3; // "untrained", "trained", etc.
  int32  bonus      = 4; // CombatProficiencyBonus(level, rank)
  string kind       = 5; // "armor" or "weapon"
}

// ProficienciesResponse contains all proficiency entries for the character.
message ProficienciesResponse {
  repeated ProficiencyEntry proficiencies = 1;
}
```

Also add to `CharacterSheetView` (after `class_features = 24;`):
```proto
repeated ProficiencyEntry proficiencies = 25;
```

**Step 2: Regenerate proto**

```bash
make proto
```
Expected: `internal/gameserver/gamev1/game.pb.go` updated with no errors.

**Step 3: Add `HandlerProficiencies` to `internal/game/command/commands.go`**

After `HandlerClassFeatures = "class_features"` (line ~57), add:
```go
HandlerProficiencies = "proficiencies"
```

In `BuiltinCommands()`, after the `class_features` entry, add:
```go
{Name: "proficiencies", Aliases: []string{"prof"}, Help: "Display your armor and weapon proficiencies.", Category: CategoryCharacter, Handler: HandlerProficiencies},
```

**Step 4: Create `internal/game/command/proficiencies.go`**

```go
package command

// HandleProficiencies is a client-side no-op; the bridge sends a ProficienciesRequest
// and the server returns a ProficienciesResponse rendered by the frontend.
// This function is intentionally empty — all logic lives in bridgeProficiencies.
func HandleProficiencies() string { return "" }
```

**Step 5: Add `bridgeProficiencies` to `internal/frontend/handlers/bridge_handlers.go`**

After `bridgeSkills` (around line 608), add:
```go
// bridgeProficiencies builds a ProficienciesRequest to retrieve armor/weapon proficiencies.
//
// Precondition: bctx must contain a valid stream.
// Postcondition: sends ProficienciesRequest over the stream; returns empty bridgeResult (server responds async).
func bridgeProficiencies(bctx *bridgeContext) (bridgeResult, error) {
    return bridgeResult{}, bctx.stream.Send(&gamev1.ClientMessage{
        Payload: &gamev1.ClientMessage_ProficienciesRequest{
            ProficienciesRequest: &gamev1.ProficienciesRequest{},
        },
    })
}
```

Register it in `bridgeHandlerMap` (in `init()` or the map literal), after `HandlerSkills`:
```go
command.HandlerProficiencies: bridgeProficiencies,
```

**Step 6: Add `handleProficiencies` to `internal/gameserver/grpc_service.go`**

After `handleSkills` (around line 2857), add:
```go
// handleProficiencies returns all armor/weapon proficiency entries for the player's character.
//
// Precondition: uid must be a valid logged-in session UID.
// Postcondition: returns a ProficienciesResponse with one entry per proficiency category.
func (s *GameServiceServer) handleProficiencies(uid string) (*gamev1.ServerEvent, error) {
    sess, ok := s.sessionManager.GetSession(uid)
    if !ok {
        return nil, fmt.Errorf("session not found: %s", uid)
    }

    displayNames := map[string]string{
        "unarmored":       "Unarmored",
        "light_armor":     "Light Armor",
        "medium_armor":    "Medium Armor",
        "heavy_armor":     "Heavy Armor",
        "simple_weapons":  "Simple Weapons",
        "simple_ranged":   "Simple Ranged",
        "martial_weapons": "Martial Weapons",
        "martial_ranged":  "Martial Ranged",
        "martial_melee":   "Martial Melee",
        "unarmed":         "Unarmed",
        "specialized":     "Specialized",
    }
    armorCats := []string{"unarmored", "light_armor", "medium_armor", "heavy_armor"}
    weaponCats := []string{"simple_weapons", "simple_ranged", "martial_weapons", "martial_ranged", "martial_melee", "unarmed", "specialized"}

    level := 1 // TODO: use sess.Level when available
    var entries []*gamev1.ProficiencyEntry

    for _, cat := range armorCats {
        rank := sess.Proficiencies[cat]
        if rank == "" {
            rank = "untrained"
        }
        entries = append(entries, &gamev1.ProficiencyEntry{
            Category: cat,
            Name:     displayNames[cat],
            Rank:     rank,
            Bonus:    int32(combat.CombatProficiencyBonus(level, rank)),
            Kind:     "armor",
        })
    }
    for _, cat := range weaponCats {
        rank := sess.Proficiencies[cat]
        if rank == "" {
            rank = "untrained"
        }
        entries = append(entries, &gamev1.ProficiencyEntry{
            Category: cat,
            Name:     displayNames[cat],
            Rank:     rank,
            Bonus:    int32(combat.CombatProficiencyBonus(level, rank)),
            Kind:     "weapon",
        })
    }

    return &gamev1.ServerEvent{
        Payload: &gamev1.ServerEvent_ProficienciesResponse{
            ProficienciesResponse: &gamev1.ProficienciesResponse{
                Proficiencies: entries,
            },
        },
    }, nil
}
```

Wire it into the dispatch `switch` (find the `handleSkills` case and add after):
```go
case *gamev1.ClientMessage_ProficienciesRequest:
    event, err = s.handleProficiencies(uid)
```

**Step 7: Render `ProficienciesResponse` in `internal/frontend/handlers/game_bridge.go`**

In the server event handler switch (where `SkillsResponse` is rendered), add:
```go
case *gamev1.ServerEvent_ProficienciesResponse:
    text = RenderProficiencies(p.ProficienciesResponse)
```

**Step 8: Add `RenderProficiencies` to `internal/frontend/handlers/text_renderer.go`**

```go
// RenderProficiencies formats a ProficienciesResponse as a Telnet proficiencies list.
//
// Precondition: pr must not be nil.
// Postcondition: returns a multi-line string suitable for WriteConsole.
func RenderProficiencies(pr *gamev1.ProficienciesResponse) string {
    var b strings.Builder
    b.WriteString("Armor Proficiencies\r\n")
    for _, e := range pr.Proficiencies {
        if e.Kind != "armor" {
            continue
        }
        rankLabel := fmt.Sprintf("[%s]", e.Rank)
        b.WriteString(fmt.Sprintf("  %-18s %-12s +%d\r\n", e.Name, rankLabel, e.Bonus))
    }
    b.WriteString("\r\nWeapon Proficiencies\r\n")
    for _, e := range pr.Proficiencies {
        if e.Kind != "weapon" {
            continue
        }
        rankLabel := fmt.Sprintf("[%s]", e.Rank)
        b.WriteString(fmt.Sprintf("  %-18s %-12s +%d\r\n", e.Name, rankLabel, e.Bonus))
    }
    return b.String()
}
```

**Step 9: Verify `TestAllCommandHandlersAreWired` passes**

```bash
go test ./internal/frontend/handlers/... -run TestAllCommandHandlersAreWired -v
```
Expected: PASS.

**Step 10: Run full frontend tests**

```bash
go test ./internal/frontend/... -count=1 -timeout=120s 2>&1 | tail -10
```
Expected: all PASS.

**Step 11: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go \
    internal/game/command/commands.go internal/game/command/proficiencies.go \
    internal/frontend/handlers/bridge_handlers.go internal/gameserver/grpc_service.go \
    internal/frontend/handlers/game_bridge.go internal/frontend/handlers/text_renderer.go
git commit -m "feat: proficiencies command — proto, bridge, handler, render"
```

---

### Task 7: Add proficiencies to character sheet

**Files:**
- Modify: `internal/gameserver/grpc_service.go` — populate `proficiencies` in `CharacterSheetView`
- Modify: `internal/frontend/handlers/text_renderer.go` — render proficiencies in `RenderCharacterSheet`

**Step 1: Populate `proficiencies` in `handleCharSheet` in `internal/gameserver/grpc_service.go`**

Find the `handleCharSheet` function (search for `CharacterSheetView`). After populating `class_features`, add:
```go
// Proficiencies
if sess != nil {
    profEntries := buildProficiencyEntries(sess.Proficiencies, sess.Level)
    view.Proficiencies = profEntries
}
```

Extract the entry-building logic into a shared helper (to avoid duplication with `handleProficiencies`):
```go
func buildProficiencyEntries(profs map[string]string, level int) []*gamev1.ProficiencyEntry {
    displayNames := map[string]string{
        "unarmored": "Unarmored", "light_armor": "Light Armor",
        "medium_armor": "Medium Armor", "heavy_armor": "Heavy Armor",
        "simple_weapons": "Simple Weapons", "simple_ranged": "Simple Ranged",
        "martial_weapons": "Martial Weapons", "martial_ranged": "Martial Ranged",
        "martial_melee": "Martial Melee", "unarmed": "Unarmed", "specialized": "Specialized",
    }
    order := []struct{ cat, kind string }{
        {"unarmored", "armor"}, {"light_armor", "armor"}, {"medium_armor", "armor"}, {"heavy_armor", "armor"},
        {"simple_weapons", "weapon"}, {"simple_ranged", "weapon"}, {"martial_weapons", "weapon"},
        {"martial_ranged", "weapon"}, {"martial_melee", "weapon"}, {"unarmed", "weapon"}, {"specialized", "weapon"},
    }
    var entries []*gamev1.ProficiencyEntry
    for _, o := range order {
        rank := profs[o.cat]
        if rank == "" {
            rank = "untrained"
        }
        entries = append(entries, &gamev1.ProficiencyEntry{
            Category: o.cat,
            Name:     displayNames[o.cat],
            Rank:     rank,
            Bonus:    int32(combat.CombatProficiencyBonus(level, rank)),
            Kind:     o.kind,
        })
    }
    return entries
}
```

Also update `handleProficiencies` to use this helper.

**Step 2: Render proficiencies in `RenderCharacterSheet` in `internal/frontend/handlers/text_renderer.go`**

At the end of `RenderCharacterSheet` (after class features), add a Proficiencies section:
```go
if len(csv.Proficiencies) > 0 {
    b.WriteString("\r\nProficiencies\r\n")
    for _, e := range csv.Proficiencies {
        rankLabel := fmt.Sprintf("[%s]", e.Rank)
        b.WriteString(fmt.Sprintf("  %-18s %-12s +%d\r\n", e.Name, rankLabel, e.Bonus))
    }
}
```

**Step 3: Run full test suite**

```bash
go test -race -count=1 -timeout=300s $(go list ./... | grep -v 'github.com/cory-johannsen/mud/internal/storage/postgres') 2>&1 | tail -20
```
Expected: all PASS (pre-existing race in `TestSession_FavoredTargetPromptedWhenMissing_InvalidInput` is known and unrelated).

**Step 4: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/frontend/handlers/text_renderer.go
git commit -m "feat: add proficiencies section to character sheet"
```

---

### Task 8: Update FEATURES.md, run migration, deploy

**Files:**
- Modify: `docs/requirements/FEATURES.md` — mark Proficiencies done
- Run migration in production

**Step 1: Mark FEATURES.md done**

Change:
```
- [ ] Proficiencies
  - [ ] Armor
      - [ ] Unarmored
      - [ ] Light
      - [ ] Medium
      - [ ] Heavy
  - [ ] Weapons
    - [ ] Unarmed
    - [ ] Simple
    - [ ] Martial
    - [ ] Specialized
```
To:
```
- [x] Proficiencies
  - [x] Armor
      - [x] Unarmored
      - [x] Light
      - [x] Medium
      - [x] Heavy
  - [x] Weapons
    - [x] Unarmed
    - [x] Simple
    - [x] Martial
    - [x] Specialized
```

**Step 2: Commit**

```bash
git add docs/requirements/FEATURES.md
git commit -m "docs: mark Proficiencies as complete"
```

**Step 3: Deploy**

```bash
make k8s-redeploy
```
Expected: `Release "mud" has been upgraded. Happy Helming!`

The Helm post-upgrade job runs migration 018 automatically.

**Step 4: Push**

```bash
git push
```
