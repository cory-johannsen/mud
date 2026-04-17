# Innate Technology Cantrip Parity — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make all region-granted innate technologies unlimited-use and restrict them to tech-capable archetypes only (those with a DominantTradition), matching PF2E cantrip semantics.

**Architecture:** Three independent changes land in order: (1) fix 7 region YAML data files, (2) add a one-line `DominantTradition` gate to `AssignTechnologies`, (3) ship a SQL migration that corrects existing DB rows. No new types, interfaces, or files required.

**Tech Stack:** Go 1.23, pgx/v5, rapid (property tests), YAML content files, PostgreSQL migration files.

---

## File Map

| File | Action |
|------|--------|
| `content/regions/southeast_portland.yaml` | Modify: `uses_per_day: 1` → `0` |
| `content/regions/pearl_district.yaml` | Modify: `uses_per_day: 1` → `0` |
| `content/regions/gresham_outskirts.yaml` | Modify: `uses_per_day: 1` → `0` |
| `content/regions/north_portland.yaml` | Modify: `uses_per_day: 1` → `0` |
| `content/regions/pacific_northwest.yaml` | Modify: `uses_per_day: 1` → `0` |
| `content/regions/south.yaml` | Modify: `uses_per_day: 1` → `0` |
| `content/regions/southern_california.yaml` | Modify: `uses_per_day: 1` → `0` |
| `internal/gameserver/technology_assignment.go` | Modify: add `isTechCapable` gate to innate grant blocks |
| `internal/gameserver/technology_assignment_test.go` | Modify: add 3 new tests |
| `migrations/062_innate_tech_cantrip_parity.up.sql` | Create |
| `migrations/062_innate_tech_cantrip_parity.down.sql` | Create |

---

### Task 1: Fix region YAML files — set all innate grants to unlimited

**Files:**
- Modify: `content/regions/southeast_portland.yaml`
- Modify: `content/regions/pearl_district.yaml`
- Modify: `content/regions/gresham_outskirts.yaml`
- Modify: `content/regions/north_portland.yaml`
- Modify: `content/regions/pacific_northwest.yaml`
- Modify: `content/regions/south.yaml`
- Modify: `content/regions/southern_california.yaml`

- [ ] **Step 1: Edit southeast_portland.yaml**

In `content/regions/southeast_portland.yaml`, change:
```yaml
innate_technologies:
  - id: nanite_infusion
    uses_per_day: 1
```
to:
```yaml
innate_technologies:
  - id: nanite_infusion
    uses_per_day: 0
```

- [ ] **Step 2: Edit pearl_district.yaml**

In `content/regions/pearl_district.yaml`, change:
```yaml
innate_technologies:
  - id: pressure_burst
    uses_per_day: 1
```
to:
```yaml
innate_technologies:
  - id: pressure_burst
    uses_per_day: 0
```

- [ ] **Step 3: Edit gresham_outskirts.yaml**

In `content/regions/gresham_outskirts.yaml`, change:
```yaml
innate_technologies:
  - id: acid_spit
    uses_per_day: 1
```
to:
```yaml
innate_technologies:
  - id: acid_spit
    uses_per_day: 0
```

- [ ] **Step 4: Edit north_portland.yaml**

In `content/regions/north_portland.yaml`, change:
```yaml
innate_technologies:
  - id: terror_broadcast
    uses_per_day: 1
```
to:
```yaml
innate_technologies:
  - id: terror_broadcast
    uses_per_day: 0
```

- [ ] **Step 5: Edit pacific_northwest.yaml**

In `content/regions/pacific_northwest.yaml`, change:
```yaml
innate_technologies:
  - id: atmospheric_surge
    uses_per_day: 1
```
to:
```yaml
innate_technologies:
  - id: atmospheric_surge
    uses_per_day: 0
```

- [ ] **Step 6: Edit south.yaml**

In `content/regions/south.yaml`, change:
```yaml
innate_technologies:
  - id: viscous_spray
    uses_per_day: 1
```
to:
```yaml
innate_technologies:
  - id: viscous_spray
    uses_per_day: 0
```

- [ ] **Step 7: Edit southern_california.yaml**

In `content/regions/southern_california.yaml`, change:
```yaml
innate_technologies:
  - id: chrome_reflex
    uses_per_day: 1
```
to:
```yaml
innate_technologies:
  - id: chrome_reflex
    uses_per_day: 0
```

- [ ] **Step 8: Build to verify no parse errors**

```bash
make build
```
Expected: build succeeds with no errors.

- [ ] **Step 9: Commit**

```bash
git add content/regions/southeast_portland.yaml content/regions/pearl_district.yaml \
    content/regions/gresham_outskirts.yaml content/regions/north_portland.yaml \
    content/regions/pacific_northwest.yaml content/regions/south.yaml \
    content/regions/southern_california.yaml
git commit -m "fix(content): set all region innate tech grants to unlimited (uses_per_day: 0)"
```

---

### Task 2: Add tech-capability gate to AssignTechnologies

**Files:**
- Modify: `internal/gameserver/technology_assignment.go` (lines 190–222)
- Modify: `internal/gameserver/technology_assignment_test.go`

**Context:**
- `technology.DominantTradition(archetypeID string) string` is in `internal/game/technology/flavor.go`. Returns `""` for `"aggressor"` and `"criminal"`, non-empty for all other archetypes.
- `ruleset.Archetype.ID` is the string archetype identifier (e.g., `"nerd"`, `"aggressor"`).
- The existing `fakeInnateRepo`, `fakeHardwiredRepo`, `fakePreparedRepo`, `fakeSpontaneousRepo`, and `noPrompt` helpers are defined at the top of `technology_assignment_test.go` and are reusable.
- `technology.DominantTradition` is already imported in the test file via `"github.com/cory-johannsen/mud/internal/game/technology"`.

- [ ] **Step 1: Write failing tests**

Add these three tests to `internal/gameserver/technology_assignment_test.go`, after the existing `TestAssignTechnologies_FullJob` test:

```go
// REQ-ITC-1: non-tech archetypes (aggressor, criminal) receive no innate tech from region.
func TestAssignTechnologies_NonTechArchetype_NoInnate(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}
	arch := &ruleset.Archetype{ID: "aggressor"}
	region := &ruleset.Region{
		InnateTechnologies: []ruleset.InnateGrant{{ID: "blackout_pulse", UsesPerDay: 0}},
	}
	inn := &fakeInnateRepo{}
	err := gameserver.AssignTechnologies(ctx, sess, 1, nil, arch, nil, noPrompt,
		&fakeHardwiredRepo{}, &fakePreparedRepo{}, &fakeSpontaneousRepo{}, inn, nil, region)
	require.NoError(t, err)
	assert.Empty(t, sess.InnateTechs, "aggressor archetype must receive no innate tech from region")
}

// REQ-ITC-2: tech-capable archetypes receive unlimited innate tech from region.
func TestAssignTechnologies_TechArchetype_GetsUnlimitedInnate(t *testing.T) {
	ctx := context.Background()
	sess := &session.PlayerSession{}
	arch := &ruleset.Archetype{ID: "nerd"}
	region := &ruleset.Region{
		InnateTechnologies: []ruleset.InnateGrant{{ID: "blackout_pulse", UsesPerDay: 0}},
	}
	inn := &fakeInnateRepo{}
	err := gameserver.AssignTechnologies(ctx, sess, 1, nil, arch, nil, noPrompt,
		&fakeHardwiredRepo{}, &fakePreparedRepo{}, &fakeSpontaneousRepo{}, inn, nil, region)
	require.NoError(t, err)
	require.Len(t, sess.InnateTechs, 1, "nerd archetype must receive innate tech from region")
	slot := sess.InnateTechs["blackout_pulse"]
	require.NotNil(t, slot)
	assert.Equal(t, 0, slot.MaxUses, "innate tech must be unlimited (MaxUses == 0)")
	assert.Equal(t, 0, slot.UsesRemaining, "innate tech must start with UsesRemaining == 0 (unlimited)")
}

// REQ-ITC-3: property — innate tech is granted iff DominantTradition(archetype.ID) != "".
func TestProperty_AssignTechnologies_InnateGatedByTechTradition(t *testing.T) {
	allArchetypes := []string{
		"nerd", "naturalist", "drifter", "schemer", "influencer", "zealot", // tech-capable
		"aggressor", "criminal", // non-tech
	}
	rapid.Check(t, func(rt *rapid.T) {
		archetypeID := rapid.SampledFrom(allArchetypes).Draw(rt, "archetypeID")
		sess := &session.PlayerSession{}
		arch := &ruleset.Archetype{ID: archetypeID}
		region := &ruleset.Region{
			InnateTechnologies: []ruleset.InnateGrant{{ID: "blackout_pulse", UsesPerDay: 0}},
		}
		inn := &fakeInnateRepo{}
		err := gameserver.AssignTechnologies(context.Background(), sess, 1, nil, arch, nil, noPrompt,
			&fakeHardwiredRepo{}, &fakePreparedRepo{}, &fakeSpontaneousRepo{}, inn, nil, region)
		require.NoError(rt, err)

		hasTradition := technology.DominantTradition(archetypeID) != ""
		if hasTradition {
			assert.NotEmpty(rt, sess.InnateTechs,
				"tech archetype %q must receive innate tech from region", archetypeID)
		} else {
			assert.Empty(rt, sess.InnateTechs,
				"non-tech archetype %q must receive no innate tech from region", archetypeID)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/ -run "TestAssignTechnologies_NonTechArchetype_NoInnate|TestAssignTechnologies_TechArchetype_GetsUnlimitedInnate|TestProperty_AssignTechnologies_InnateGatedByTechTradition" -v 2>&1 | tail -20
```

Expected: `FAIL` — the non-tech test passes (by coincidence, current code doesn't grant because aggressor has no grants), but the property test will expose that aggressor currently gets no innate tech from region (wait — actually aggressor currently DOES get it because the region block has no gate). So `TestAssignTechnologies_NonTechArchetype_NoInnate` should fail because the current code does grant it.

Expected output contains: `FAIL` with `"aggressor archetype must receive no innate tech from region"`.

- [ ] **Step 3: Implement the gate in technology_assignment.go**

In `internal/gameserver/technology_assignment.go`, replace the innate initialization and grant blocks (lines ~190–222):

**Before:**
```go
	// Innate: initialize map once before both archetype and region blocks
	if sess.InnateTechs == nil {
		if (archetype != nil && len(archetype.InnateTechnologies) > 0) ||
			(region != nil && len(region.InnateTechnologies) > 0) {
			sess.InnateTechs = make(map[string]*session.InnateSlot)
		}
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

**After:**
```go
	// isTechCapable is true when the archetype has a technology tradition.
	// Only tech-capable characters receive innate technology grants (cantrip parity).
	isTechCapable := archetype != nil && technology.DominantTradition(archetype.ID) != ""

	// Innate: initialize map once before both archetype and region blocks
	if sess.InnateTechs == nil && isTechCapable {
		if len(archetype.InnateTechnologies) > 0 ||
			(region != nil && len(region.InnateTechnologies) > 0) {
			sess.InnateTechs = make(map[string]*session.InnateSlot)
		}
	}

	// Innate (from archetype)
	if isTechCapable {
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
	if region != nil && isTechCapable {
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

- [ ] **Step 4: Run the new tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && go test ./internal/gameserver/ -run "TestAssignTechnologies_NonTechArchetype_NoInnate|TestAssignTechnologies_TechArchetype_GetsUnlimitedInnate|TestProperty_AssignTechnologies_InnateGatedByTechTradition" -v 2>&1 | tail -20
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Run full test suite**

```bash
make test 2>&1 | grep -E "^(ok|FAIL|---)" | tail -40
```

Expected: no FAIL lines.

- [ ] **Step 6: Commit**

```bash
git add internal/gameserver/technology_assignment.go internal/gameserver/technology_assignment_test.go
git commit -m "feat(gameserver): gate innate tech grants on tech-capable archetype (cantrip parity)"
```

---

### Task 3: Write SQL migration for existing DB rows

**Files:**
- Create: `migrations/062_innate_tech_cantrip_parity.up.sql`
- Create: `migrations/062_innate_tech_cantrip_parity.down.sql`

**Context:**
- DB table is `character_innate_technologies` with columns `character_id`, `tech_id`, `max_uses`, `uses_remaining`.
- Characters table uses `class` column for the job ID.
- Aggressor archetype jobs: `beat_down_artist`, `boot_gun`, `boot_machete`, `gangster`, `goon`, `grunt`, `mercenary`, `muscle`, `roid_rager`, `soldier`, `street_fighter`, `thug`.
- Criminal archetype jobs: `beggar`, `car_jacker`, `contract_killer`, `gambler`, `hanger_on`, `hooker`, `smuggler`, `thief`, `tomb_raider`.
- `max_uses = 0` means unlimited per `InnateSlot` convention (see `internal/game/session/technology.go`).

- [ ] **Step 1: Create up migration**

Create `migrations/062_innate_tech_cantrip_parity.up.sql`:

```sql
-- REQ-ITC-1: Remove innate tech rows for non-tech-capable characters.
-- Aggressor archetype jobs: beat_down_artist, boot_gun, boot_machete, gangster, goon, grunt,
--   mercenary, muscle, roid_rager, soldier, street_fighter, thug
-- Criminal archetype jobs: beggar, car_jacker, contract_killer, gambler, hanger_on, hooker,
--   smuggler, thief, tomb_raider
DELETE FROM character_innate_technologies
WHERE character_id IN (
    SELECT id FROM characters
    WHERE class IN (
        'beat_down_artist', 'boot_gun', 'boot_machete', 'gangster', 'goon', 'grunt',
        'mercenary', 'muscle', 'roid_rager', 'soldier', 'street_fighter', 'thug',
        'beggar', 'car_jacker', 'contract_killer', 'gambler', 'hanger_on', 'hooker',
        'smuggler', 'thief', 'tomb_raider'
    )
);

-- REQ-ITC-2: Set all remaining innate tech rows to unlimited.
-- MaxUses = 0 means unlimited per session.InnateSlot convention.
UPDATE character_innate_technologies
SET max_uses = 0, uses_remaining = 0;
```

- [ ] **Step 2: Create down migration**

Create `migrations/062_innate_tech_cantrip_parity.down.sql`:

```sql
-- Original limited-use values are not recoverable after this migration.
-- No-op.
```

- [ ] **Step 3: Build to verify migration files are picked up**

```bash
make build 2>&1 | tail -10
```

Expected: build succeeds.

- [ ] **Step 4: Run full test suite**

```bash
make test 2>&1 | grep -E "^(ok|FAIL|---)" | tail -40
```

Expected: no FAIL lines.

- [ ] **Step 5: Commit**

```bash
git add migrations/062_innate_tech_cantrip_parity.up.sql migrations/062_innate_tech_cantrip_parity.down.sql
git commit -m "feat(migrations): 062 innate tech cantrip parity — fix unlimited use and remove non-tech rows"
```

---

### Task 4: Push and deploy

**Files:** none

- [ ] **Step 1: Push all commits**

```bash
git push
```

Expected: `main -> main` with 3 commits pushed.

- [ ] **Step 2: Deploy**

```bash
make k8s-redeploy 2>&1 | tail -10
```

Expected: `Release "mud" has been upgraded. Happy Helming!`
