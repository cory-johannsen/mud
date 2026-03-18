# Innate Technologies Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-tech daily use tracking to innate technologies, wire region innate grants into character creation, implement activation/rest/character-sheet, and populate all 11 regions with innate tech YAML content.

**Architecture:** Extend the existing `InnateSlot`/`InnateTechRepo`/`character_innate_technologies` stack with a `uses_remaining` column and two new repo methods (`Decrement`/`RestoreAll`). Add `InnateTechnologies []InnateGrant` to `Region`, pass the region into `AssignTechnologies`, extend `handleUse`/`handleRest`/`handleChar`, and create 11 innate tech YAML files plus update all 11 region YAMLs.

**Tech Stack:** Go 1.23, pgx v5, protobuf/grpc, `pgregory.net/rapid` (property tests), `github.com/stretchr/testify`

---

## Chunk 1: Data Model + DB Migration

### Task 1: InnateSlot UsesRemaining + DB migration

**Files:**
- Modify: `internal/game/session/technology.go`
- Create: `migrations/029_innate_uses_remaining.up.sql`
- Create: `migrations/029_innate_uses_remaining.down.sql`
- Modify: `internal/storage/postgres/main_test.go`

- [ ] **Step 1: Add `UsesRemaining` to `InnateSlot`**

Replace the existing `InnateSlot` struct in `internal/game/session/technology.go`:

```go
// InnateSlot tracks an innate technology granted by a region or archetype.
// MaxUses == 0 means unlimited.
type InnateSlot struct {
	MaxUses       int
	UsesRemaining int
}
```

- [ ] **Step 2: Create migration 029 up**

Create `migrations/029_innate_uses_remaining.up.sql`:

```sql
ALTER TABLE character_innate_technologies
    ADD COLUMN uses_remaining INT NOT NULL DEFAULT 0;
```

- [ ] **Step 3: Create migration 029 down**

Create `migrations/029_innate_uses_remaining.down.sql`:

```sql
ALTER TABLE character_innate_technologies
    DROP COLUMN uses_remaining;
```

- [ ] **Step 4: Update `main_test.go` applyAllMigrations**

In `internal/storage/postgres/main_test.go`, find the `character_innate_technologies` CREATE TABLE statement inside `applyAllMigrations`. Add `uses_remaining INT NOT NULL DEFAULT 0` as a column:

```sql
CREATE TABLE character_innate_technologies (
    character_id BIGINT NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    tech_id      TEXT   NOT NULL,
    max_uses     INT    NOT NULL DEFAULT 0,
    uses_remaining INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (character_id, tech_id)
);
```

- [ ] **Step 5: Run tests to confirm nothing broken**

```bash
cd /home/cjohannsen/src/mud && go build ./... 2>&1
```

Expected: no compilation errors.

- [ ] **Step 6: Commit**

```bash
git add internal/game/session/technology.go migrations/029_innate_uses_remaining.up.sql migrations/029_innate_uses_remaining.down.sql internal/storage/postgres/main_test.go
git commit -m "feat(innate): add UsesRemaining to InnateSlot; migration 029 adds uses_remaining column"
```

---

### Task 2: InnateTechRepo extension + DB repo + DB tests

**Files:**
- Modify: `internal/gameserver/technology_assignment.go` (interface only)
- Modify: `internal/storage/postgres/character_innate_tech.go`
- Create: `internal/storage/postgres/character_innate_tech_test.go`
- Modify: `internal/gameserver/grpc_service_levelup_tech_test.go` (add stubs to fake)

- [ ] **Step 1: Write failing DB tests (REQ-INN9)**

Create `internal/storage/postgres/character_innate_tech_test.go`:

```go
package postgres_test

import (
	"context"
	"testing"

	"pgregory.net/rapid"

	pgstore "github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestInnateUsesRemaining_RoundTrip(t *testing.T) {
	ctx := context.Background()
	pool := testDB(t)
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterInnateTechRepository(pool)

	if err := repo.Set(ctx, ch.ID, "acid_spit", 3); err != nil {
		t.Fatalf("Set: %v", err)
	}

	slots, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	s, ok := slots["acid_spit"]
	if !ok {
		t.Fatalf("expected acid_spit slot, got none")
	}
	if s.MaxUses != 3 {
		t.Errorf("MaxUses: want 3, got %d", s.MaxUses)
	}
	if s.UsesRemaining != 3 {
		t.Errorf("UsesRemaining after Set: want 3, got %d", s.UsesRemaining)
	}

	if err := repo.Decrement(ctx, ch.ID, "acid_spit"); err != nil {
		t.Fatalf("Decrement: %v", err)
	}

	slots2, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll after Decrement: %v", err)
	}
	s2 := slots2["acid_spit"]
	if s2.UsesRemaining != 2 {
		t.Errorf("UsesRemaining after Decrement: want 2, got %d", s2.UsesRemaining)
	}
	if s2.MaxUses != 3 {
		t.Errorf("MaxUses after Decrement: want 3, got %d", s2.MaxUses)
	}

	if err := repo.RestoreAll(ctx, ch.ID); err != nil {
		t.Fatalf("RestoreAll: %v", err)
	}

	slots3, err := repo.GetAll(ctx, ch.ID)
	if err != nil {
		t.Fatalf("GetAll after RestoreAll: %v", err)
	}
	s3 := slots3["acid_spit"]
	if s3.UsesRemaining != 3 {
		t.Errorf("UsesRemaining after RestoreAll: want 3, got %d", s3.UsesRemaining)
	}
}

func TestInnateDecrement_NeverBelowZero_Property(t *testing.T) {
	ctx := context.Background()
	pool := testDB(t)
	charRepo := pgstore.NewCharacterRepository(pool)
	ch := createTestCharacter(t, charRepo, ctx)
	repo := pgstore.NewCharacterInnateTechRepository(pool)

	rapid.Check(t, func(rt *rapid.T) {
		if err := repo.DeleteAll(ctx, ch.ID); err != nil {
			rt.Fatalf("DeleteAll: %v", err)
		}
		n := rapid.IntRange(1, 5).Draw(rt, "maxUses")
		calls := rapid.IntRange(0, 8).Draw(rt, "decrementCalls")

		if err := repo.Set(ctx, ch.ID, "acid_spit", n); err != nil {
			rt.Fatalf("Set: %v", err)
		}
		for i := 0; i < calls; i++ {
			if err := repo.Decrement(ctx, ch.ID, "acid_spit"); err != nil {
				rt.Fatalf("Decrement %d: %v", i, err)
			}
		}
		slots, err := repo.GetAll(ctx, ch.ID)
		if err != nil {
			rt.Fatalf("GetAll: %v", err)
		}
		s := slots["acid_spit"]
		expected := n - calls
		if expected < 0 {
			expected = 0
		}
		if s.UsesRemaining != expected {
			rt.Errorf("UsesRemaining: want %d, got %d (n=%d, calls=%d)", expected, s.UsesRemaining, n, calls)
		}
		if s.UsesRemaining < 0 {
			rt.Errorf("UsesRemaining went below zero: %d", s.UsesRemaining)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/storage/postgres/... -run TestInnateUsesRemaining -v 2>&1 | head -30
```

Expected: compilation error (Decrement/RestoreAll not defined on interface/repo).

- [ ] **Step 3: Extend `InnateTechRepo` interface**

In `internal/gameserver/technology_assignment.go`, replace the existing `InnateTechRepo` interface with:

```go
// InnateTechRepo defines persistence for innate technology assignments.
type InnateTechRepo interface {
	// GetAll returns all innate slots for the character.
	GetAll(ctx context.Context, characterID int64) (map[string]*session.InnateSlot, error)

	// Set initializes or overwrites an innate slot entry.
	// Postcondition: row (characterID, techID) has max_uses=maxUses, uses_remaining=maxUses.
	// Precondition: only called at character creation or full re-assignment, never at login load.
	Set(ctx context.Context, characterID int64, techID string, maxUses int) error

	// DeleteAll removes all innate tech rows for the character.
	DeleteAll(ctx context.Context, characterID int64) error

	// Decrement atomically decrements uses_remaining by 1 if > 0.
	// Precondition: caller has verified UsesRemaining > 0 in session before calling.
	// Postcondition: uses_remaining = max(0, uses_remaining - 1).
	Decrement(ctx context.Context, characterID int64, techID string) error

	// RestoreAll sets uses_remaining = max_uses for all rows of this character.
	// Postcondition: all innate slots are at maximum uses.
	RestoreAll(ctx context.Context, characterID int64) error
}
```

- [ ] **Step 4: Update `CharacterInnateTechRepository.GetAll` to scan `uses_remaining`**

In `internal/storage/postgres/character_innate_tech.go`, replace the `GetAll` method:

```go
func (r *CharacterInnateTechRepository) GetAll(ctx context.Context, characterID int64) (map[string]*session.InnateSlot, error) {
	rows, err := r.db.Query(ctx,
		`SELECT tech_id, max_uses, uses_remaining
         FROM character_innate_technologies
         WHERE character_id = $1`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("CharacterInnateTechRepository.GetAll: %w", err)
	}
	defer rows.Close()
	result := make(map[string]*session.InnateSlot)
	for rows.Next() {
		var techID string
		var maxUses, usesRemaining int
		if err := rows.Scan(&techID, &maxUses, &usesRemaining); err != nil {
			return nil, fmt.Errorf("CharacterInnateTechRepository.GetAll scan: %w", err)
		}
		result[techID] = &session.InnateSlot{MaxUses: maxUses, UsesRemaining: usesRemaining}
	}
	return result, rows.Err()
}
```

- [ ] **Step 5: Update `CharacterInnateTechRepository.Set` to write `uses_remaining`**

Replace the `Set` method:

```go
func (r *CharacterInnateTechRepository) Set(ctx context.Context, characterID int64, techID string, maxUses int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO character_innate_technologies (character_id, tech_id, max_uses, uses_remaining)
         VALUES ($1, $2, $3, $3)
         ON CONFLICT (character_id, tech_id)
         DO UPDATE SET max_uses = EXCLUDED.max_uses, uses_remaining = EXCLUDED.uses_remaining`,
		characterID, techID, maxUses,
	)
	if err != nil {
		return fmt.Errorf("CharacterInnateTechRepository.Set: %w", err)
	}
	return nil
}
```

Note: `$3` appears twice — both `max_uses` and `uses_remaining` are set to `maxUses`.

- [ ] **Step 6: Add `Decrement` and `RestoreAll` to `CharacterInnateTechRepository`**

Append after `DeleteAll`:

```go
// Decrement atomically decrements uses_remaining by 1 if > 0.
func (r *CharacterInnateTechRepository) Decrement(ctx context.Context, characterID int64, techID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE character_innate_technologies
            SET uses_remaining = GREATEST(0, uses_remaining - 1)
          WHERE character_id = $1 AND tech_id = $2`,
		characterID, techID)
	if err != nil {
		return fmt.Errorf("CharacterInnateTechRepository.Decrement: %w", err)
	}
	return nil
}

// RestoreAll sets uses_remaining = max_uses for all rows of this character.
func (r *CharacterInnateTechRepository) RestoreAll(ctx context.Context, characterID int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE character_innate_technologies
            SET uses_remaining = max_uses
          WHERE character_id = $1`,
		characterID)
	if err != nil {
		return fmt.Errorf("CharacterInnateTechRepository.RestoreAll: %w", err)
	}
	return nil
}
```

- [ ] **Step 7: Add stubs to `innateRepoInternal` in `grpc_service_levelup_tech_test.go`**

Find `innateRepoInternal` in `internal/gameserver/grpc_service_levelup_tech_test.go` and add two stub methods:

```go
func (r *innateRepoInternal) Decrement(_ context.Context, _ int64, _ string) error { return nil }
func (r *innateRepoInternal) RestoreAll(_ context.Context, _ int64) error           { return nil }
```

- [ ] **Step 8: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/storage/postgres/... -run "TestInnate" -v -count=1 2>&1
```

Expected: all tests PASS.

- [ ] **Step 9: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS (or pre-existing failures only).

- [ ] **Step 10: Commit**

```bash
git add internal/gameserver/technology_assignment.go internal/storage/postgres/character_innate_tech.go internal/storage/postgres/character_innate_tech_test.go internal/gameserver/grpc_service_levelup_tech_test.go
git commit -m "feat(innate): extend InnateTechRepo with Decrement/RestoreAll; update GetAll/Set for uses_remaining; DB round-trip tests"
```

---

## Chunk 2: Region Content

### Task 3: Region struct + innate tech YAML files + region YAML updates

**Files:**
- Modify: `internal/game/ruleset/region.go`
- Create: `content/technologies/innate/` (11 files)
- Modify: all 11 `content/regions/*.yaml`
- Modify: `internal/game/ruleset/loader_test.go`

- [ ] **Step 1: Add `InnateTechnologies` field to `Region` struct**

In `internal/game/ruleset/region.go`, add the field to `Region`:

```go
type Region struct {
	ID                 string             `yaml:"id"`
	Name               string             `yaml:"name"`
	Article            string             `yaml:"article"`
	Description        string             `yaml:"description"`
	Modifiers          map[string]int     `yaml:"modifiers"`
	Traits             []string           `yaml:"traits"`
	AbilityBoosts      *AbilityBoostGrant `yaml:"ability_boosts"`
	InnateTechnologies []InnateGrant      `yaml:"innate_technologies,omitempty"`
}
```

`InnateGrant` is in the same `ruleset` package (`internal/game/ruleset/technology_grants.go`), so no import needed.

- [ ] **Step 2: Write failing loader tests (REQ-CONTENT1, REQ-CONTENT2)**

**Test 1 (REQ-CONTENT2):** Append to `internal/game/ruleset/loader_test.go` (package `ruleset_test`, existing file):

```go
func TestLoadRegions_AllHaveInnateGrant(t *testing.T) {
	regions, err := ruleset.LoadRegions("../../../content/regions")
	require.NoError(t, err)
	require.NotEmpty(t, regions, "expected regions to be loaded from content/regions")
	for _, r := range regions {
		assert.Len(t, r.InnateTechnologies, 1, "region %q: expected exactly 1 innate grant", r.ID)
	}
}
```

**Test 2 (REQ-CONTENT1):** Create `internal/game/technology/registry_innate_test.go` (package `technology_test`):

```go
package technology_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/technology"
)

func TestLoad_InnateSubdirLoads(t *testing.T) {
	reg, err := technology.Load("../../../content/technologies")
	require.NoError(t, err)
	innate := reg.ByUsageType(technology.UsageInnate)
	assert.Equal(t, 11, len(innate), "expected 11 innate tech files loaded")
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... ./internal/game/technology/... -run "TestLoadRegions_AllHaveInnateGrant|TestLoad_InnateSubdirLoads" -v 2>&1
```

Expected: FAIL (no innate grants in YAMLs yet).

- [ ] **Step 4: Create innate tech YAML directory and files**

Create directory `content/technologies/innate/` and all 11 files:

**Valid ranges:** `self`, `melee`, `ranged`, `zone`. **Valid targets:** `single`, `all_enemies`, `all_allies`, `zone`. Map PF2E-style values: `emanation→zone`, `close→ranged`, `touch→melee`, `area→zone`. All files include `effects: [{type: utility}]` — `utility` is a valid effect type requiring no additional fields; effect resolution is out of scope.

**`content/technologies/innate/blackout_pulse.yaml`**
```yaml
id: blackout_pulse
name: Blackout Pulse
description: Emits a localized EM burst that kills electronic lighting and blinds optical sensors in a small radius.
tradition: technical
level: 1
usage_type: innate
action_cost: 1
range: zone
targets: zone
duration: rounds:1
effects:
  - type: utility
```

**`content/technologies/innate/arc_lights.yaml`**
```yaml
id: arc_lights
name: Arc Lights
description: Projects three hovering electromagnetic arc-light drones that illuminate and disorient.
tradition: technical
level: 1
usage_type: innate
action_cost: 1
range: ranged
targets: zone
duration: minutes:1
effects:
  - type: utility
```

**`content/technologies/innate/pressure_burst.yaml`**
```yaml
id: pressure_burst
name: Pressure Burst
description: A pneumatic compression rig vents a focused blast that shoves targets and shatters brittle obstacles.
tradition: technical
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: single
duration: instant
effects:
  - type: utility
```

**`content/technologies/innate/nanite_infusion.yaml`**
```yaml
id: nanite_infusion
name: Nanite Infusion
description: Releases a cloud of salvaged medical nanites that accelerate tissue repair in a touched target.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 2
range: melee
targets: single
duration: instant
effects:
  - type: utility
```

**`content/technologies/innate/atmospheric_surge.yaml`**
```yaml
id: atmospheric_surge
name: Atmospheric Surge
description: A wrist-mounted atmospheric compressor discharges a powerful wind blast that scatters enemies.
tradition: technical
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: zone
duration: instant
effects:
  - type: utility
```

**`content/technologies/innate/viscous_spray.yaml`**
```yaml
id: viscous_spray
name: Viscous Spray
description: A bio-synthetic adhesive secretion coats a target's joints and limbs, restraining movement.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: single
duration: rounds:1
effects:
  - type: utility
```

**`content/technologies/innate/chrome_reflex.yaml`**
```yaml
id: chrome_reflex
name: Chrome Reflex
description: A neural-augmented reflex burst that overrides the nervous system and forces a second attempt at a failed saving throw.
tradition: neural
level: 1
usage_type: innate
action_cost: 1
range: self
targets: single
duration: instant
effects:
  - type: utility
```

**`content/technologies/innate/seismic_sense.yaml`**
```yaml
id: seismic_sense
name: Seismic Sense
description: Bone-conduction implants detect ground vibrations, revealing the movement of creatures through floors and walls.
tradition: technical
level: 1
usage_type: innate
action_cost: 1
range: zone
targets: zone
duration: rounds:1
effects:
  - type: utility
```

**`content/technologies/innate/moisture_reclaim.yaml`**
```yaml
id: moisture_reclaim
name: Moisture Reclaim
description: Atmospheric condensation filters extract potable water from ambient humidity.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 1
range: self
targets: single
duration: instant
effects:
  - type: utility
```

**`content/technologies/innate/terror_broadcast.yaml`**
```yaml
id: terror_broadcast
name: Terror Broadcast
description: A subdermal transmitter floods nearby targets with a fear-inducing neural frequency.
tradition: neural
level: 1
usage_type: innate
action_cost: 2
range: zone
targets: zone
duration: rounds:1
effects:
  - type: utility
```

**`content/technologies/innate/acid_spit.yaml`**
```yaml
id: acid_spit
name: Acid Spit
description: A bio-synthetic gland secretes pressurized corrosive fluid at a single target.
tradition: bio_synthetic
level: 1
usage_type: innate
action_cost: 2
range: ranged
targets: single
duration: instant
effects:
  - type: utility
```

- [ ] **Step 5: Update all 11 region YAML files with `innate_technologies:` blocks**

Add to `content/regions/old_town.yaml` (append at end):
```yaml
innate_technologies:
  - id: blackout_pulse
    uses_per_day: 0
```

Add to `content/regions/northeast.yaml`:
```yaml
innate_technologies:
  - id: arc_lights
    uses_per_day: 0
```

Add to `content/regions/pearl_district.yaml`:
```yaml
innate_technologies:
  - id: pressure_burst
    uses_per_day: 1
```

Add to `content/regions/southeast_portland.yaml`:
```yaml
innate_technologies:
  - id: nanite_infusion
    uses_per_day: 1
```

Add to `content/regions/pacific_northwest.yaml`:
```yaml
innate_technologies:
  - id: atmospheric_surge
    uses_per_day: 1
```

Add to `content/regions/south.yaml`:
```yaml
innate_technologies:
  - id: viscous_spray
    uses_per_day: 1
```

Add to `content/regions/southern_california.yaml`:
```yaml
innate_technologies:
  - id: chrome_reflex
    uses_per_day: 1
```

Add to `content/regions/mountain.yaml`:
```yaml
innate_technologies:
  - id: seismic_sense
    uses_per_day: 0
```

Add to `content/regions/midwest.yaml`:
```yaml
innate_technologies:
  - id: moisture_reclaim
    uses_per_day: 0
```

Add to `content/regions/north_portland.yaml`:
```yaml
innate_technologies:
  - id: terror_broadcast
    uses_per_day: 1
```

Add to `content/regions/gresham_outskirts.yaml`:
```yaml
innate_technologies:
  - id: acid_spit
    uses_per_day: 1
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/game/ruleset/... ./internal/game/technology/... -run "TestLoadRegions_AllHaveInnateGrant|TestLoad_InnateSubdirLoads" -v 2>&1
```

Expected: PASS.

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/game/ruleset/region.go content/technologies/innate/ content/regions/ internal/game/ruleset/loader_test.go internal/game/technology/registry_innate_test.go
git commit -m "feat(innate): Region.InnateTechnologies field; 11 innate tech YAMLs; all 11 region innate grants"
```

---

## Chunk 3: Assignment + Activation + Rest

### Task 4: AssignTechnologies region parameter

**Files:**
- Modify: `internal/gameserver/technology_assignment.go`
- Modify: `internal/gameserver/grpc_service.go` (call site only)
- Modify: `internal/gameserver/technology_assignment_test.go`

- [ ] **Step 1: Write failing test (REQ-INN7)**

The existing `AssignTechnologies` signature (from the test file pattern) is:
```go
gameserver.AssignTechnologies(ctx, sess, characterID, job, archetype, techReg, promptFn, hwRepo, prepRepo, spontRepo, innateRepo, usePoolRepo)
```
After this task adds `region`, it will be:
```go
gameserver.AssignTechnologies(ctx, sess, characterID, job, archetype, techReg, promptFn, hwRepo, prepRepo, spontRepo, innateRepo, usePoolRepo, region)
```
(region added at end)

In `internal/gameserver/technology_assignment_test.go`, append:

```go
func TestAssignTechnologies_RegionInnateGrant(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}

	region := &ruleset.Region{
		ID: "gresham_outskirts",
		InnateTechnologies: []ruleset.InnateGrant{
			{ID: "acid_spit", UsesPerDay: 1},
		},
	}

	hw := &fakeHardwiredRepo{}
	prep := &fakePreparedRepo{}
	spont := &fakeSpontaneousRepo{}
	inn := &fakeInnateRepo{}

	err := gameserver.AssignTechnologies(ctx, sess, 1, nil, nil, nil, noPrompt, hw, prep, spont, inn, nil, region)
	require.NoError(t, err)

	slot, ok := sess.InnateTechs["acid_spit"]
	require.True(t, ok, "expected acid_spit in session InnateTechs")
	assert.Equal(t, 1, slot.MaxUses)
	assert.Equal(t, 1, slot.UsesRemaining)

	repoSlot, repoOk := inn.slots["acid_spit"]
	require.True(t, repoOk, "expected acid_spit persisted to repo")
	assert.Equal(t, 1, repoSlot.MaxUses)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestAssignTechnologies_RegionInnateGrant -v 2>&1 | head -30
```

Expected: compile error (wrong arg count to AssignTechnologies) or FAIL.

- [ ] **Step 3: Add `region` parameter to `AssignTechnologies` and update innate block**

In `internal/gameserver/technology_assignment.go`, find `AssignTechnologies`. Add `region *ruleset.Region` as the **last parameter** (after `usePoolRepo SpontaneousUsePoolRepo`). This minimizes changes to existing call sites — they just append `nil` or the actual region. Then replace the innate assignment section:

```go
// Innate: initialize map once before both archetype and region blocks
if sess.InnateTechs == nil {
    sess.InnateTechs = make(map[string]*session.InnateSlot)
}

// Innate (from archetype)
if archetype != nil {
    for _, grant := range archetype.InnateTechnologies {
        sess.InnateTechs[grant.ID] = &session.InnateSlot{
            MaxUses:       grant.UsesPerDay,
            UsesRemaining: grant.UsesPerDay,
        }
        if err := innateRepo.Set(ctx, characterID, grant.ID, grant.UsesPerDay); err != nil {
            return fmt.Errorf("AssignTechnologies innate (archetype) %s: %w", grant.ID, err)
        }
    }
}

// Innate (from region)
if region != nil {
    for _, grant := range region.InnateTechnologies {
        sess.InnateTechs[grant.ID] = &session.InnateSlot{
            MaxUses:       grant.UsesPerDay,
            UsesRemaining: grant.UsesPerDay,
        }
        if err := innateRepo.Set(ctx, characterID, grant.ID, grant.UsesPerDay); err != nil {
            return fmt.Errorf("AssignTechnologies innate (region) %s: %w", grant.ID, err)
        }
    }
}
```

Note: the old code had `sess.InnateTechs = make(...)` inside the archetype conditional — **remove** that inner `make` call; it is replaced by the nil guard above.

- [ ] **Step 4: Update `AssignTechnologies` call site in `grpc_service.go`**

Search `grpc_service.go` for the call to `AssignTechnologies`. Since `region` is the last parameter, append `s.regions[dbChar.Region]` at the end of the existing call:

```go
err = AssignTechnologies(ctx, sess, characterID, job, archetype, techReg, promptFn,
    s.hardwiredTechRepo, s.preparedTechRepo, s.spontaneousTechRepo,
    s.innateTechRepo, s.spontaneousUsePoolRepo,
    s.regions[dbChar.Region])
```

Match the exact existing argument names — only add `s.regions[dbChar.Region]` at the end.

- [ ] **Step 5: Update `fakeInnateRepo` in `technology_assignment_test.go` if needed**

Ensure `fakeInnateRepo` (or whatever the innate repo fake is named in that file) implements `Decrement` and `RestoreAll`. Add stubs if missing:

```go
func (r *fakeInnateRepo) Decrement(_ context.Context, _ int64, _ string) error { return nil }
func (r *fakeInnateRepo) RestoreAll(_ context.Context, _ int64) error           { return nil }
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestAssignTechnologies_RegionInnateGrant -v 2>&1
```

Expected: PASS.

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/gameserver/technology_assignment.go internal/gameserver/grpc_service.go internal/gameserver/technology_assignment_test.go
git commit -m "feat(innate): AssignTechnologies gains region param; nil guard before both innate blocks"
```

---

### Task 5: `handleUse` innate path + `handleRest` innate restoration

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Create: `internal/gameserver/grpc_service_innate_test.go`

- [ ] **Step 1: Write failing tests (REQ-INN1–INN6)**

Create `internal/gameserver/grpc_service_innate_test.go`:

```go
package gameserver

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/session"
)

// innateRepoForGrpcTest implements InnateTechRepo for innate tech grpc tests.
// Named distinctly from fakeInnateRepo in technology_assignment_test.go (different package).
type innateRepoForGrpcTest struct {
	slots            map[string]*session.InnateSlot
	decremented      []string
	restoreAllCalled int
}

func (r *innateRepoForGrpcTest) GetAll(_ context.Context, _ int64) (map[string]*session.InnateSlot, error) {
	out := make(map[string]*session.InnateSlot)
	for k, v := range r.slots {
		cp := *v
		out[k] = &cp
	}
	return out, nil
}
func (r *innateRepoForGrpcTest) Set(_ context.Context, _ int64, techID string, maxUses int) error {
	if r.slots == nil {
		r.slots = make(map[string]*session.InnateSlot)
	}
	r.slots[techID] = &session.InnateSlot{MaxUses: maxUses, UsesRemaining: maxUses}
	return nil
}
func (r *innateRepoForGrpcTest) DeleteAll(_ context.Context, _ int64) error {
	r.slots = nil
	return nil
}
func (r *innateRepoForGrpcTest) Decrement(_ context.Context, _ int64, techID string) error {
	r.decremented = append(r.decremented, techID)
	if r.slots != nil {
		if s, ok := r.slots[techID]; ok && s.UsesRemaining > 0 {
			s.UsesRemaining--
		}
	}
	return nil
}
func (r *innateRepoForGrpcTest) RestoreAll(_ context.Context, _ int64) error {
	r.restoreAllCalled++
	for _, s := range r.slots {
		s.UsesRemaining = s.MaxUses
	}
	return nil
}

// innateTestService sets up a minimal service with innateTechRepo wired.
// handleUse does not use a stream — it returns (*gamev1.ServerEvent, error).
func innateTestService(t *testing.T, innateTechRepo *innateRepoForGrpcTest) (*GameServiceServer, string) {
	t.Helper()
	sessMgr := session.NewManager()
	svc := testMinimalService(t, sessMgr)
	svc.SetInnateTechRepo(innateTechRepo)

	uid := "player-innate-test"
	_, err := sessMgr.AddPlayer(session.AddPlayerOptions{
		UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
	})
	require.NoError(t, err)
	return svc, uid
}

// REQ-INN1: use <tech> with uses remaining → activation message; DB decremented.
func TestHandleUse_InnateActivation_DecrementsCalled(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 1},
	}}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessionManager.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 1},
	}

	evt, err := svc.handleUse(uid, "acid_spit")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.True(t, strings.Contains(msg, "acid_spit"), "expected activation message containing acid_spit, got: %q", msg)
	assert.Contains(t, repo.decremented, "acid_spit", "expected Decrement called for acid_spit")
	assert.Equal(t, 0, sess.InnateTechs["acid_spit"].UsesRemaining)
}

// REQ-INN2: use <tech> with 0 uses → "No uses of <tech> remaining."
func TestHandleUse_InnateExhausted_ReturnsNoUsesMessage(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 0},
	}}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessionManager.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 0},
	}

	evt, err := svc.handleUse(uid, "acid_spit")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "No uses of acid_spit remaining", "expected exhausted message")
	assert.Empty(t, repo.decremented, "Decrement must not be called when exhausted")
}

// REQ-INN3: use <tech> not in innate techs → "You don't have innate tech <tech>."
func TestHandleUse_InnateNotKnown_ReturnsNotKnownMessage(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{}}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessionManager.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{}

	evt, err := svc.handleUse(uid, "acid_spit")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.Contains(t, msg, "don't have innate tech acid_spit")
}

// REQ-INN4: use <tech> unlimited (MaxUses=0) → activation message; Decrement NOT called.
func TestHandleUse_InnateUnlimited_NoDecrement(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"blackout_pulse": {MaxUses: 0, UsesRemaining: 0},
	}}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessionManager.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"blackout_pulse": {MaxUses: 0, UsesRemaining: 0},
	}

	evt, err := svc.handleUse(uid, "blackout_pulse")
	require.NoError(t, err)
	require.NotNil(t, evt)

	msg := evt.GetMessage().GetContent()
	assert.True(t, strings.Contains(msg, "blackout_pulse"), "expected activation message for blackout_pulse, got: %q", msg)
	assert.Empty(t, repo.decremented, "Decrement must NOT be called for unlimited tech")
}

// REQ-INN5: use (no-arg) lists innate techs; unlimited shown as (unlimited); exhausted omitted.
func TestHandleUse_NoArg_ListsInnateTechs(t *testing.T) {
	repo := &innateRepoForGrpcTest{}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessionManager.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"blackout_pulse": {MaxUses: 0, UsesRemaining: 0}, // unlimited — include in choices
		"acid_spit":      {MaxUses: 1, UsesRemaining: 1}, // 1 remaining — include
		"pressure_burst": {MaxUses: 1, UsesRemaining: 0}, // exhausted — omit
	}

	evt, err := svc.handleUse(uid, "")
	require.NoError(t, err)
	require.NotNil(t, evt)

	choices := evt.GetUseResponse().GetChoices()
	var descriptions []string
	for _, c := range choices {
		descriptions = append(descriptions, c.GetDescription())
	}
	joined := strings.Join(descriptions, "\n")

	assert.Contains(t, joined, "blackout_pulse")
	assert.Contains(t, joined, "unlimited")
	assert.Contains(t, joined, "acid_spit")
	assert.NotContains(t, joined, "pressure_burst", "exhausted innate tech must not appear in list")
}

// REQ-INN6: After rest, limited innate slots restored to max (session + DB RestoreAll called).
func TestHandleRest_RestoresInnateSlots(t *testing.T) {
	repo := &innateRepoForGrpcTest{slots: map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 0},
	}}
	svc, uid := innateTestService(t, repo)

	sess, ok := svc.sessionManager.GetPlayer(uid)
	require.True(t, ok)
	sess.InnateTechs = map[string]*session.InnateSlot{
		"acid_spit": {MaxUses: 1, UsesRemaining: 0},
	}

	stream := &fakeSessionStream{}
	err := svc.handleRest(uid, "req1", stream)
	require.NoError(t, err)

	assert.Equal(t, 1, repo.restoreAllCalled, "RestoreAll must be called once on rest")
	assert.Equal(t, 1, sess.InnateTechs["acid_spit"].UsesRemaining, "session slot must be restored")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleUse_Innate|TestHandleRest_Restore" -v 2>&1 | head -40
```

Expected: FAIL (innate path not implemented in handleUse/handleRest).

- [ ] **Step 3: Implement innate path in `handleUse` (no-arg list mode)**

In `internal/gameserver/grpc_service.go`, find `handleUse`. The no-arg path builds a slice of `*gamev1.FeatEntry` called `active` (inspect the existing code for the exact variable name), then returns a `UseResponse{Choices: active}` event. After the spontaneous tech listing block (the section that appends spontaneous tech entries when `abilityID == ""`), add innate tech entries before the final `return`:

```go
// Innate techs (no-arg list mode)
if len(sess.InnateTechs) > 0 {
    innateIDs := make([]string, 0, len(sess.InnateTechs))
    for id := range sess.InnateTechs {
        innateIDs = append(innateIDs, id)
    }
    sort.Strings(innateIDs)
    for _, id := range innateIDs {
        slot := sess.InnateTechs[id]
        var desc string
        if slot.MaxUses == 0 {
            desc = fmt.Sprintf("%s (unlimited)", id)
        } else if slot.UsesRemaining > 0 {
            desc = fmt.Sprintf("%s (%d uses remaining)", id, slot.UsesRemaining)
        } else {
            continue // exhausted — omit
        }
        active = append(active, &gamev1.FeatEntry{
            FeatId:      id,
            Name:        id,
            Category:    "innate_tech",
            Active:      true,
            Description: desc,
        })
    }
}
```

Note: `active` is the `[]*gamev1.FeatEntry` slice built in the existing no-arg path. This block must appear **before** the `return &gamev1.ServerEvent{Payload: &gamev1.ServerEvent_UseResponse{...}}` statement. `sort` is already imported.

- [ ] **Step 4: Implement innate activation path in `handleUse`**

After the spontaneous tech activation block (when `abilityID != ""`), add the innate path before the final fallthrough. `handleUse` returns `(*gamev1.ServerEvent, error)` — use `messageEvent(text)` (the existing helper) to build response events:

```go
// Innate tech activation
if slot, ok := sess.InnateTechs[abilityID]; ok {
    if slot.MaxUses != 0 && slot.UsesRemaining <= 0 {
        return messageEvent(fmt.Sprintf("No uses of %s remaining.", abilityID)), nil
    }
    if slot.MaxUses != 0 {
        if err := s.innateTechRepo.Decrement(ctx, sess.CharacterID, abilityID); err != nil {
            return nil, fmt.Errorf("handleUse: decrement innate %s: %w", abilityID, err)
        }
        slot.UsesRemaining--
        sess.InnateTechs[abilityID] = slot
        return messageEvent(fmt.Sprintf("You activate %s. (%d uses remaining.)", abilityID, slot.UsesRemaining)), nil
    }
    return messageEvent(fmt.Sprintf("You activate %s.", abilityID)), nil
}
return messageEvent(fmt.Sprintf("You don't have innate tech %s.", abilityID)), nil
```

Note: `messageEvent(text)` is defined in `grpc_service.go` as a package-level helper that returns `*gamev1.ServerEvent`. `ctx` is available in scope. `sess.InnateTechs` is `map[string]*session.InnateSlot` — the slot is a pointer; update the pointer's field and re-assign.

- [ ] **Step 5: Implement innate restoration in `handleRest`**

In `handleRest`, after the spontaneous use pool restoration block, add:

```go
if err := s.innateTechRepo.RestoreAll(ctx, sess.CharacterID); err != nil {
    return fmt.Errorf("handleRest: restore innate slots: %w", err)
}
innates, err := s.innateTechRepo.GetAll(ctx, sess.CharacterID)
if err != nil {
    return fmt.Errorf("handleRest: reload innate slots: %w", err)
}
sess.InnateTechs = innates
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run "TestHandleUse_Innate|TestHandleRest_Restore" -v 2>&1
```

Expected: all PASS.

- [ ] **Step 7: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/grpc_service_innate_test.go
git commit -m "feat(innate): handleUse innate activation path; handleRest innate restoration; REQ-INN1–INN6 tests"
```

---

## Chunk 4: Proto + Character Sheet + Property Test

### Task 6: Proto `InnateSlotView` + `handleChar` + property test + FEATURES.md

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Modify: `internal/gameserver/grpc_service.go` (`handleChar`)
- Modify: `internal/gameserver/technology_assignment_test.go` (REQ-INN8 property test)
- Modify: `docs/requirements/FEATURES.md`

- [ ] **Step 1: Write failing property test (REQ-INN8)**

In `internal/gameserver/technology_assignment_test.go`, append:

```go
// REQ-INN8 (property): For N uses, exactly N activations consumed before exhausted.
// handleUse signature: (uid, abilityID string) (*gamev1.ServerEvent, error) — no stream, returns event.
func TestPropertyInnateUse_ExactlyNActivationsConsumed(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sessMgr := session.NewManager()
		svc := testMinimalService(t, sessMgr)

		// Generate N in [1, 5]
		n := rapid.IntRange(1, 5).Draw(rt, "maxUses")

		uid := fmt.Sprintf("prop-innate-%d", rapid.IntRange(0, 9999).Draw(rt, "uid"))
		sess, err := sessMgr.AddPlayer(session.AddPlayerOptions{
			UID: uid, Username: uid, CharName: uid, RoomID: "room_a", Role: "player",
		})
		if err != nil {
			rt.Skip()
		}

		repo := &fakeInnateRepo{slots: map[string]*session.InnateSlot{
			"acid_spit": {MaxUses: n, UsesRemaining: n},
		}}
		svc.SetInnateTechRepo(repo)
		sess.InnateTechs = map[string]*session.InnateSlot{
			"acid_spit": {MaxUses: n, UsesRemaining: n},
		}

		// Activate N times — all should return an activation message containing "acid_spit"
		for i := 0; i < n; i++ {
			evt, err := svc.handleUse(uid, "acid_spit")
			if err != nil {
				rt.Fatalf("activation %d failed: %v", i, err)
			}
			if evt == nil {
				rt.Fatalf("activation %d: nil event returned", i)
			}
			msg := evt.GetMessage().GetContent()
			if !strings.Contains(msg, "acid_spit") {
				rt.Fatalf("activation %d: expected activation message containing 'acid_spit', got: %q", i, msg)
			}
		}

		// (N+1)th activation must return "No uses remaining"
		evt, err := svc.handleUse(uid, "acid_spit")
		if err != nil {
			rt.Fatalf("(N+1)th call failed: %v", err)
		}
		if evt == nil {
			rt.Fatalf("(N+1)th call: nil event returned")
		}
		exhaustedMsg := evt.GetMessage().GetContent()
		if !strings.Contains(exhaustedMsg, "No uses of acid_spit remaining") {
			rt.Fatalf("expected 'No uses of acid_spit remaining' on (N+1)th call, got: %q", exhaustedMsg)
		}

		// UsesRemaining must never be below 0
		if sess.InnateTechs["acid_spit"].UsesRemaining < 0 {
			rt.Errorf("UsesRemaining went below zero: %d", sess.InnateTechs["acid_spit"].UsesRemaining)
		}
	})
}
```

Note: `testMinimalService` takes `(t *testing.T, sessMgr *session.Manager)` — pass the outer `t` (not `nil`). The `fakeInnateRepo` type in `technology_assignment_test.go` (package `gameserver_test`) is distinct from `innateRepoForGrpcTest` in `grpc_service_innate_test.go` (package `gameserver`); here in package `gameserver_test` we use `fakeInnateRepo`. Check the existing test file for exact `fakeInnateRepo` definition and match it.

- [ ] **Step 2: Run property test to verify it passes (activation logic already implemented)**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/... -run TestPropertyInnateUse -v 2>&1
```

Expected: PASS (since handleUse innate path was implemented in Task 5).

- [ ] **Step 3: Add `InnateSlotView` proto message and field**

In `api/proto/game/v1/game.proto`:

1. Add after `SpontaneousUsePoolView`:

```protobuf
message InnateSlotView {
    string tech_id        = 1;
    int32  uses_remaining = 2;
    int32  max_uses       = 3;
}
```

2. Add field to `CharacterSheetView` (after field 45 `spontaneous_use_pools`):

```protobuf
repeated InnateSlotView innate_slots = 46;
```

- [ ] **Step 4: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud && make proto 2>&1
```

Expected: regenerates `internal/gameserver/gamev1/game.pb.go` and `game_grpc.pb.go` without errors.

- [ ] **Step 5: Populate innate slots in `handleChar`**

In `internal/gameserver/grpc_service.go`, find `handleChar`. After the `SpontaneousUsePools` population block, add:

```go
for techID, slot := range sess.InnateTechs {
    view.InnateSlots = append(view.InnateSlots, &gamev1.InnateSlotView{
        TechId:        techID,
        UsesRemaining: int32(slot.UsesRemaining),
        MaxUses:       int32(slot.MaxUses),
    })
}
```

- [ ] **Step 6: Run full test suite**

```bash
cd /home/cjohannsen/src/mud && go test ./... 2>&1 | tail -20
```

Expected: all PASS.

- [ ] **Step 7: Update FEATURES.md**

In `docs/requirements/FEATURES.md`, replace:

```
      - [ ] Innate Technologies — region-based innate tech grants; per-tech daily uses; restore on rest; character sheet display
```

with:

```
      - [x] Innate Technologies — region-based innate tech grants; per-tech daily uses; restore on rest; character sheet display (REQ-INN1–INN9, REQ-CONTENT1–2)
```

- [ ] **Step 8: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/ internal/gameserver/grpc_service.go internal/gameserver/technology_assignment_test.go docs/requirements/FEATURES.md
git commit -m "feat(innate): InnateSlotView proto; handleChar innate slots; REQ-INN8 property test; FEATURES.md updated"
```
